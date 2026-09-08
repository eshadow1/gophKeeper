package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/eshadow1/gophkeeper/gen/pb"
	"github.com/eshadow1/gophkeeper/internal/config"
	"github.com/eshadow1/gophkeeper/internal/crypto"
	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/eshadow1/gophkeeper/internal/model"

	"github.com/shirou/gopsutil/v3/mem"
)

const (
	CryptoKey             = "KEY_USER"
	filePern              = 0600
	defaultMaxSizeBinFile = 500 * 1024 * 1024
	percentMemUsed        = 0.3
)

// Cryptographer определяет интерфейс для операций шифрования.
type Cryptographer interface {
	// Encrypt шифрует открытые данные.
	Encrypt(plaintext []byte, key []byte) ([]byte, error)
	// Decrypt дешифрует данные.
	Decrypt(ciphertext []byte, key []byte) ([]byte, error)
}

// GRPCClient определяет интерфейс для взаимодействия с gRPC-сервером.
type GRPCClient interface {
	Register(ctx context.Context, username, password string) (string, error)
	Login(ctx context.Context, username, password string) (string, error)
	CreateItem(ctx context.Context, dataType string, encryptedData []byte, metaInfo string) (*pb.Item, error)
	UpdateItem(ctx context.Context, id string, dataType string, encryptedData []byte, metaInfo string) error
	DeleteItem(ctx context.Context, id string) error
	GetItems(ctx context.Context, timeUpdate int64) ([]*pb.Item, error)
	SetToken(token string)
	Close() error
}

// Repository определяет интерфейс локального хранилища данных.
type Repository interface {
	Save(item *model.Item)
	GetAll() []*model.Item
	Count() int
	Clear()
}

// Validator определяет интерфейс валидации
type Validator interface {
	Validate(ctx context.Context, dataType model.ItemType, payload any) error
}

type keeperClient struct {
	crypto        Cryptographer
	client        GRPCClient
	repo          Repository
	v             Validator
	cfg           *config.ClientConfig
	key           []byte
	tokenFilePath string
}

// NewClient создает новый клиент для сервиса
func NewClient(cfg *config.ClientConfig, cryptographer Cryptographer, client GRPCClient, repository Repository, v Validator) *keeperClient {
	return &keeperClient{
		crypto: cryptographer,
		client: client,
		repo:   repository,
		v:      v,
		cfg:    cfg,
	}
}

// initTokenFile инициализирует путь к временному файлу токена в системной директории Temp.
func (kc *keeperClient) initTokenFile() {
	if kc.tokenFilePath == "" {
		kc.tokenFilePath = filepath.Join(os.TempDir(), "gophkeeper.token")
	}
}

// saveToken сохраняет токен во временный файл с правами доступа 0600.
func (kc *keeperClient) saveToken(token string) error {
	kc.initTokenFile()
	if err := os.WriteFile(kc.tokenFilePath, []byte(token), filePern); err != nil {
		return fmt.Errorf("не удалось сохранить токен: %w", err)
	}
	kc.client.SetToken(token)
	return nil
}

// LoadToken загружает токен из временного файла и устанавливает его в gRPC-клиент.
func (kc *keeperClient) LoadToken() error {
	kc.initTokenFile()
	data, err := os.ReadFile(kc.tokenFilePath)
	if err != nil {
		return fmt.Errorf("не удалось загрузить токен (возможно, сессия не начата или завершена): %w", err)
	}
	kc.client.SetToken(string(data))
	return nil
}

// Logout безвозвратно удаляет временный файл токена, корректно завершая сессию.
func (kc *keeperClient) Logout() error {
	kc.initTokenFile()
	kc.client.SetToken("")
	kc.repo.Clear()
	kc.key = nil

	err := os.Remove(kc.tokenFilePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("не удалось очистить временный файл сессии: %w", err)
	}
	return nil
}

// Register регистрирует пользователя и сохраняет полученный токен.
func (kc *keeperClient) Register(ctx context.Context, username, password string) error {
	token, err := kc.client.Register(ctx, username, password)
	if err != nil {
		return err
	}

	loggers.Log.Info("successfully registered new user")
	kc.key = crypto.DeriveKey(username + password)
	return kc.saveToken(token)
}

// Login аутентифицирует пользователя и сохраняет полученный токен.
func (kc *keeperClient) Login(ctx context.Context, username, password string) error {
	token, err := kc.client.Login(ctx, username, password)
	if err != nil {
		return err
	}
	loggers.Log.Info("successfully logged in")

	kc.key = crypto.DeriveKey(username + password)
	return kc.saveToken(token)
}

// SyncFromServer загружает все данные с сервера и сохраняет их в памяти в зашифрованном виде.
func (kc *keeperClient) SyncFromServer(ctx context.Context) error {
	pbItems, err := kc.client.GetItems(ctx, 0)
	if err != nil {
		return err
	}
	kc.repo.Clear()
	for _, pbItem := range pbItems {
		item := &model.Item{
			ID:            pbItem.Id,
			DataType:      model.ItemType(pbItem.DataType),
			EncryptedData: pbItem.EncryptedData,
			MetaInfo:      pbItem.MetaInfo,
			CreatedAt:     time.Unix(0, pbItem.CreatedAt),
			UpdatedAt:     time.Unix(0, pbItem.UpdatedAt),
		}
		kc.repo.Save(item)
	}
	loggers.Log.Info("save synced", "get items", len(pbItems), "save items", kc.repo.Count())
	return nil
}

// AddItem шифрует полезные данные, отправляет их на сервер и сохраняет в локальном кэше.
func (kc *keeperClient) AddItem(ctx context.Context, dataType model.ItemType, payload any, metaInfo string) error {
	if errValidate := kc.v.Validate(ctx, dataType, payload); errValidate != nil {
		return errValidate
	}

	if dataType == model.BinaryItem {
		binary := payload.(model.BinaryPayload)
		info, errStat := os.Stat(binary.FilePath)
		if errStat != nil {
			return fmt.Errorf("stat: %w", errStat)
		}
		binary.Size = info.Size()
		binary.MimeType = mime.TypeByExtension(filepath.Ext(binary.FilePath))

		maxSizeFile := kc.getMaxAllowedFileSize()
		if info.Size() > maxSizeFile {
			return fmt.Errorf("file size (%d bytes) exceeds safe memory limit (%d bytes)", info.Size(), maxSizeFile)
		}

		var errRead error
		binary.Data, errRead = os.ReadFile(binary.FilePath)
		if errRead != nil {
			return fmt.Errorf("read file: %w", errRead)
		}
		binary.FilePath = info.Name()
		payload = binary
	}

	dataBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	encryptedData, err := kc.crypto.Encrypt(dataBytes, kc.key)
	if err != nil {
		return err
	}

	pbItem, err := kc.client.CreateItem(ctx, string(dataType), encryptedData, metaInfo)
	if err != nil {
		return err
	}

	loggers.Log.Info("Save Item")

	item := &model.Item{
		ID:            pbItem.Id,
		DataType:      model.ItemType(pbItem.DataType),
		EncryptedData: pbItem.EncryptedData,
		MetaInfo:      pbItem.MetaInfo,
		CreatedAt:     time.Unix(0, pbItem.CreatedAt),
		UpdatedAt:     time.Unix(0, pbItem.UpdatedAt),
	}
	kc.repo.Save(item)
	return nil
}

// UpdateItem обновляет данные и отправляет их на сервер.
func (kc *keeperClient) UpdateItem(ctx context.Context, id string, dataType model.ItemType, payload any, metaInfo string) error {
	if errValidate := kc.v.Validate(ctx, dataType, payload); errValidate != nil {
		return errValidate
	}

	if dataType == model.BinaryItem {
		binary := payload.(model.BinaryPayload)
		info, errStat := os.Stat(binary.FilePath)
		if errStat != nil {
			return fmt.Errorf("stat: %w", errStat)
		}
		binary.Size = info.Size()
		binary.MimeType = mime.TypeByExtension(filepath.Ext(binary.FilePath))
		var errRead error
		binary.Data, errRead = os.ReadFile(binary.FilePath)
		if errRead != nil {
			return fmt.Errorf("read file: %w", errRead)
		}
		binary.FilePath = info.Name()
		payload = binary
	}

	dataBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	encryptedData, err := kc.crypto.Encrypt(dataBytes, kc.key)
	if err != nil {
		return err
	}

	err = kc.client.UpdateItem(ctx, id, string(dataType), encryptedData, metaInfo)
	if err != nil {
		return err
	}

	loggers.Log.Info("Update Item")

	return nil
}

// DeleteItem удаляет запись и отправляет эту информацию на сервер.
func (kc *keeperClient) DeleteItem(ctx context.Context, id string) error {
	err := kc.client.DeleteItem(ctx, id)
	if err != nil {
		return err
	}
	loggers.Log.Info("Delete Item")
	return nil
}

// DecryptAndParse расшифровывает данные элемента и десериализует их в переданную структуру.
// Вызывать только при непосредственном отображении данных пользователю.
func (kc *keeperClient) DecryptAndParse(item *model.Item, payload any) error {
	if item == nil {
		return errors.New("item is null")
	}
	decryptedData, err := kc.crypto.Decrypt(item.EncryptedData, kc.key)
	if err != nil {
		return err
	}
	return json.Unmarshal(decryptedData, payload)
}

// ListItems возвращает все элементы из локального хранилища в памяти.
func (kc *keeperClient) ListItems() []*model.Item {
	return kc.repo.GetAll()
}

func (kc *keeperClient) getMaxAllowedFileSize() int64 {
	v, err := mem.VirtualMemory()
	if err != nil {
		return defaultMaxSizeBinFile
	}

	return int64(float64(v.Available) * percentMemUsed)
}
