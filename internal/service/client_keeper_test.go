package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eshadow1/gophkeeper/gen/pb"
	"github.com/eshadow1/gophkeeper/internal/config"
	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/eshadow1/gophkeeper/internal/model"

	mockservice "github.com/eshadow1/gophkeeper/mocks/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	loggers.CreateLogger("info")
}

func createTempFile(t *testing.T, name, content string) string {
	path := filepath.Join(t.TempDir(), name)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

func TestKeeperClient_AuthAndToken(t *testing.T) {
	cfg := &config.ClientConfig{}

	tests := []struct {
		name        string
		setupClient func() *keeperClient
		action      func(*testing.T, *keeperClient) error
		mockSetup   func(*mockservice.MockGRPCClient, *mockservice.MockRepository)
		wantErr     bool
		errContains string
	}{
		{
			name: "Register: успешная регистрация",
			setupClient: func() *keeperClient {
				return NewClient(cfg, nil, nil, nil, nil)
			},
			action: func(t *testing.T, kc *keeperClient) error {
				return kc.Register(context.Background(), "user", "pass")
			},
			mockSetup: func(mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mClient.EXPECT().Register(mock.Anything, "user", "pass").Return("test_token_123", nil)
				mClient.EXPECT().SetToken("test_token_123").Return()
			},
			wantErr: false,
		},
		{
			name: "Register: ошибка gRPC",
			setupClient: func() *keeperClient {
				return NewClient(cfg, nil, nil, nil, nil)
			},
			action: func(t *testing.T, kc *keeperClient) error {
				return kc.Register(context.Background(), "user", "pass")
			},
			mockSetup: func(mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mClient.EXPECT().Register(mock.Anything, "user", "pass").Return("", errors.New("grpc error"))
			},
			wantErr:     true,
			errContains: "grpc error",
		},
		{
			name: "Login: успешный вход",
			setupClient: func() *keeperClient {
				return NewClient(cfg, nil, nil, nil, nil)
			},
			action: func(t *testing.T, kc *keeperClient) error {
				return kc.Login(context.Background(), "user", "pass")
			},
			mockSetup: func(mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mClient.EXPECT().Login(mock.Anything, "user", "pass").Return("test_token_456", nil)
				mClient.EXPECT().SetToken("test_token_456").Return()
			},
			wantErr: false,
		},
		{
			name: "LoadToken: успешная загрузка",
			setupClient: func() *keeperClient {
				kc := NewClient(cfg, nil, nil, nil, nil)
				tokenPath := filepath.Join(t.TempDir(), "gophkeeper.token")
				os.WriteFile(tokenPath, []byte("saved_token"), 0600)
				kc.tokenFilePath = tokenPath
				return kc
			},
			action: func(t *testing.T, kc *keeperClient) error {
				return kc.LoadToken()
			},
			mockSetup: func(mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mClient.EXPECT().SetToken("saved_token").Return()
			},
			wantErr: false,
		},
		{
			name: "LoadToken: файл не существует",
			setupClient: func() *keeperClient {
				kc := NewClient(cfg, nil, nil, nil, nil)
				kc.tokenFilePath = filepath.Join(t.TempDir(), "non_existent.token")
				return kc
			},
			action: func(t *testing.T, kc *keeperClient) error {
				return kc.LoadToken()
			},
			mockSetup:   func(mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {},
			wantErr:     true,
			errContains: "не удалось загрузить токен",
		},
		{
			name: "Logout: успешный выход",
			setupClient: func() *keeperClient {
				kc := NewClient(cfg, nil, nil, nil, nil)
				tokenPath := filepath.Join(t.TempDir(), "gophkeeper.token")
				os.WriteFile(tokenPath, []byte("token"), 0600)
				kc.tokenFilePath = tokenPath
				return kc
			},
			action: func(t *testing.T, kc *keeperClient) error {
				return kc.Logout()
			},
			mockSetup: func(mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mClient.EXPECT().SetToken("").Return()
				mRepo.EXPECT().Clear().Return()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockservice.NewMockGRPCClient(t)
			mockRepo := mockservice.NewMockRepository(t)

			kc := tt.setupClient()
			kc.client = mockClient
			kc.repo = mockRepo

			tt.mockSetup(mockClient, mockRepo)

			err := tt.action(t, kc)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestKeeperClient_SyncAndList(t *testing.T) {
	cfg := &config.ClientConfig{}

	tests := []struct {
		name      string
		action    func(*testing.T, *keeperClient) error
		mockSetup func(*mockservice.MockGRPCClient, *mockservice.MockRepository)
		wantErr   bool
	}{
		{
			name: "SyncFromServer: успешная синхронизация",
			action: func(t *testing.T, kc *keeperClient) error {
				return kc.SyncFromServer(context.Background())
			},
			mockSetup: func(mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				pbItems := []*pb.Item{
					{Id: "1", DataType: "text", EncryptedData: []byte("enc1"), MetaInfo: "meta", CreatedAt: time.Now().UnixNano()},
				}
				mClient.EXPECT().GetItems(mock.Anything, int64(0)).Return(pbItems, nil)
				mRepo.EXPECT().Clear().Return()
				mRepo.EXPECT().Save(mock.MatchedBy(func(item *model.Item) bool {
					return item.ID == "1" && item.DataType == "text"
				})).Return()
				mRepo.EXPECT().Count().Return(1)
			},
			wantErr: false,
		},
		{
			name: "SyncFromServer: ошибка gRPC",
			action: func(t *testing.T, kc *keeperClient) error {
				return kc.SyncFromServer(context.Background())
			},
			mockSetup: func(mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mClient.EXPECT().GetItems(mock.Anything, int64(0)).Return(nil, errors.New("sync failed"))
			},
			wantErr: true,
		},
		{
			name: "ListItems: возврат данных из репозитория",
			action: func(t *testing.T, kc *keeperClient) error {
				items := kc.ListItems()
				assert.Len(t, items, 2)
				return nil
			},
			mockSetup: func(mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mRepo.EXPECT().GetAll().Return([]*model.Item{{ID: "1"}, {ID: "2"}})
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockservice.NewMockGRPCClient(t)
			mockRepo := mockservice.NewMockRepository(t)
			kc := NewClient(cfg, nil, mockClient, mockRepo, nil)

			tt.mockSetup(mockClient, mockRepo)

			err := tt.action(t, kc)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestKeeperClient_AddItem(t *testing.T) {
	cfg := &config.ClientConfig{}
	testKey := []byte("test_key_32_bytes_long_enough_12")

	tests := []struct {
		name        string
		dataType    model.ItemType
		payload     any
		metaInfo    string
		setupFiles  func(t *testing.T) string
		mockSetup   func(*mockservice.MockValidator, *mockservice.MockCryptographer, *mockservice.MockGRPCClient, *mockservice.MockRepository)
		wantErr     bool
		errContains string
	}{
		{
			name:     "AddItem: ошибка валидации",
			dataType: model.TextItem,
			payload:  model.TextPayload{Content: ""},
			mockSetup: func(mVal *mockservice.MockValidator, mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mVal.EXPECT().Validate(mock.Anything, model.TextItem, mock.Anything).Return(errors.New("validation failed"))
			},
			wantErr:     true,
			errContains: "validation failed",
		},
		{
			name:     "AddItem: успешное добавление текстового элемента",
			dataType: model.TextItem,
			payload:  model.TextPayload{Content: "secret"},
			metaInfo: "my_meta",
			mockSetup: func(mVal *mockservice.MockValidator, mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mVal.EXPECT().Validate(mock.Anything, model.TextItem, mock.Anything).Return(nil)

				mCrypt.EXPECT().Encrypt(mock.Anything, testKey).Return([]byte("encrypted_data"), nil)

				mClient.EXPECT().CreateItem(mock.Anything, "text", []byte("encrypted_data"), "my_meta").Return(&pb.Item{
					Id: "new_id", DataType: "text", EncryptedData: []byte("encrypted_data"), MetaInfo: "my_meta", CreatedAt: time.Now().UnixNano(),
				}, nil)

				mRepo.EXPECT().Save(mock.MatchedBy(func(item *model.Item) bool {
					return item.ID == "new_id" && item.DataType == "text"
				})).Return()
			},
			wantErr: false,
		},
		{
			name:     "AddItem: успешное добавление бинарного элемента",
			dataType: model.BinaryItem,
			payload:  model.BinaryPayload{},
			setupFiles: func(t *testing.T) string {
				return createTempFile(t, "test.bin", "binary content data")
			},
			mockSetup: func(mVal *mockservice.MockValidator, mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mVal.EXPECT().Validate(mock.Anything, model.BinaryItem, mock.Anything).Return(nil)

				mCrypt.EXPECT().Encrypt(mock.Anything, testKey).Return([]byte("enc_bin"), nil)

				mClient.EXPECT().CreateItem(mock.Anything, "binary", []byte("enc_bin"), "").Return(&pb.Item{
					Id: "bin_id", DataType: "binary", EncryptedData: []byte("enc_bin"), CreatedAt: time.Now().UnixNano(),
				}, nil)

				mRepo.EXPECT().Save(mock.Anything).Return()
			},
			wantErr: false,
		},
		{
			name:     "AddItem: бинарный элемент, файл не существует",
			dataType: model.BinaryItem,
			payload:  model.BinaryPayload{FilePath: "/non/existent/file.txt"},
			mockSetup: func(mVal *mockservice.MockValidator, mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mVal.EXPECT().Validate(mock.Anything, model.BinaryItem, mock.Anything).Return(nil)
			},
			wantErr:     true,
			errContains: "stat:",
		},
		{
			name:     "AddItem: ошибка шифрования",
			dataType: model.TextItem,
			payload:  model.TextPayload{Content: "secret"},
			mockSetup: func(mVal *mockservice.MockValidator, mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient, mRepo *mockservice.MockRepository) {
				mVal.EXPECT().Validate(mock.Anything, model.TextItem, mock.Anything).Return(nil)
				mCrypt.EXPECT().Encrypt(mock.Anything, testKey).Return(nil, errors.New("crypto failed"))
			},
			wantErr:     true,
			errContains: "crypto failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVal := mockservice.NewMockValidator(t)
			mockCrypt := mockservice.NewMockCryptographer(t)
			mockClient := mockservice.NewMockGRPCClient(t)
			mockRepo := mockservice.NewMockRepository(t)

			kc := NewClient(cfg, mockCrypt, mockClient, mockRepo, mockVal)
			kc.key = testKey

			if tt.setupFiles != nil {
				filePath := tt.setupFiles(t)
				if binPayload, ok := tt.payload.(model.BinaryPayload); ok {
					binPayload.FilePath = filePath
					tt.payload = binPayload
				}
			}

			tt.mockSetup(mockVal, mockCrypt, mockClient, mockRepo)

			err := kc.AddItem(context.Background(), tt.dataType, tt.payload, tt.metaInfo)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestKeeperClient_UpdateItem(t *testing.T) {
	cfg := &config.ClientConfig{}
	testKey := []byte("test_key_32_bytes_long_enough_12")

	tests := []struct {
		name        string
		dataType    model.ItemType
		payload     any
		mockSetup   func(*mockservice.MockValidator, *mockservice.MockCryptographer, *mockservice.MockGRPCClient)
		wantErr     bool
		errContains string
	}{
		{
			name:     "UpdateItem: успешное обновление",
			dataType: model.TextItem,
			payload:  model.TextPayload{Content: "updated"},
			mockSetup: func(mVal *mockservice.MockValidator, mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient) {
				mVal.EXPECT().Validate(mock.Anything, model.TextItem, mock.Anything).Return(nil)
				mCrypt.EXPECT().Encrypt(mock.Anything, testKey).Return([]byte("enc_upd"), nil)
				mClient.EXPECT().UpdateItem(mock.Anything, "item-1", "text", []byte("enc_upd"), "new_meta").Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "UpdateItem: ошибка валидации",
			dataType: model.TextItem,
			payload:  model.TextPayload{Content: ""},
			mockSetup: func(mVal *mockservice.MockValidator, mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient) {
				mVal.EXPECT().Validate(mock.Anything, model.TextItem, mock.Anything).Return(errors.New("invalid"))
			},
			wantErr:     true,
			errContains: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVal := mockservice.NewMockValidator(t)
			mockCrypt := mockservice.NewMockCryptographer(t)
			mockClient := mockservice.NewMockGRPCClient(t)
			mockRepo := mockservice.NewMockRepository(t)

			kc := NewClient(cfg, mockCrypt, mockClient, mockRepo, mockVal)
			kc.key = testKey

			tt.mockSetup(mockVal, mockCrypt, mockClient)

			err := kc.UpdateItem(context.Background(), "item-1", tt.dataType, tt.payload, "new_meta")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestKeeperClient_DeleteAndDecrypt(t *testing.T) {
	cfg := &config.ClientConfig{}
	testKey := []byte("test_key_32_bytes_long_enough_12")

	tests := []struct {
		name        string
		action      func(*testing.T, *keeperClient) error
		mockSetup   func(*mockservice.MockCryptographer, *mockservice.MockGRPCClient)
		wantErr     bool
		errContains string
	}{
		{
			name: "DeleteItem: успешное удаление",
			action: func(t *testing.T, kc *keeperClient) error {
				return kc.DeleteItem(context.Background(), "item-1")
			},
			mockSetup: func(mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient) {
				mClient.EXPECT().DeleteItem(mock.Anything, "item-1").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "DecryptAndParse: ошибка, nil элемент",
			action: func(t *testing.T, kc *keeperClient) error {
				var payload model.TextPayload
				return kc.DecryptAndParse(nil, &payload)
			},
			mockSetup:   func(mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient) {},
			wantErr:     true,
			errContains: "item is null",
		},
		{
			name: "DecryptAndParse: ошибка дешифрования",
			action: func(t *testing.T, kc *keeperClient) error {
				item := &model.Item{EncryptedData: []byte("bad_data")}
				var payload model.TextPayload
				return kc.DecryptAndParse(item, &payload)
			},
			mockSetup: func(mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient) {
				mCrypt.EXPECT().Decrypt([]byte("bad_data"), testKey).Return(nil, errors.New("decrypt failed"))
			},
			wantErr:     true,
			errContains: "decrypt failed",
		},
		{
			name: "DecryptAndParse: успешная расшифровка и парсинг",
			action: func(t *testing.T, kc *keeperClient) error {
				original := model.TextPayload{Content: "hello"}
				data, _ := json.Marshal(original)

				item := &model.Item{EncryptedData: data}
				var payload model.TextPayload

				return kc.DecryptAndParse(item, &payload)
			},
			mockSetup: func(mCrypt *mockservice.MockCryptographer, mClient *mockservice.MockGRPCClient) {
				mCrypt.EXPECT().Decrypt(mock.Anything, testKey).Return([]byte(`{"Content":"hello"}`), nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCrypt := mockservice.NewMockCryptographer(t)
			mockClient := mockservice.NewMockGRPCClient(t)

			kc := NewClient(cfg, mockCrypt, mockClient, nil, nil)
			kc.key = testKey

			tt.mockSetup(mockCrypt, mockClient)

			err := tt.action(t, kc)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
