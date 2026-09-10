// Package grpc реализует клиентскую и серверную логику протокола GRPC приложения GophKeeper.
// Клиент управляет аутентификацией и синхронизацией данных.
package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/eshadow1/gophkeeper/gen/pb"
	"github.com/eshadow1/gophkeeper/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

const (
	// maxRecvMsgSize определяет максимальный размер входящего gRPC-сообщения.
	maxRecvMsgSize = 4 * 1024 * 1024 * 1024
	// maxSendMsgSize определяет максимальный размер исходящего gRPC-сообщения.
	maxSendMsgSize = 4 * 1024 * 1024 * 1024
	// chunkSize определяет размер одного чанка при потоковой загрузке.
	chunkSize = 1024 * 1024
)

// grpcClient реализует интерфейс GRPCClient для взаимодействия с сервером.
type grpcClient struct {
	conn   *grpc.ClientConn
	client pb.GophKeeperServiceClient
	token  string
}

// NewGRPCClient создает и инициализирует новое gRPC-соединение с сервером.
func NewGRPCClient(addr string, cfg *config.TLSConfig) (*grpcClient, error) {
	caCert, err := os.ReadFile(cfg.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать сертификат CA: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("не удалось добавить сертификат в пул доверенных")
	}

	tlsConfig := &tls.Config{
		RootCAs:    caCertPool,
		ServerName: cfg.ServerName,
	}

	creds := credentials.NewTLS(tlsConfig)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxRecvMsgSize),
			grpc.MaxCallSendMsgSize(maxSendMsgSize),
		),
	)
	if err != nil {
		return nil, err
	}
	return &grpcClient{
		conn:   conn,
		client: pb.NewGophKeeperServiceClient(conn),
	}, nil
}

// SetToken устанавливает JWT-токен для последующих аутентифицированных запросов.
func (g *grpcClient) SetToken(token string) {
	g.token = token
}

// getCtx возвращает контекст с добавленными метаданными авторизации, если токен установлен.
func (g *grpcClient) getCtx(ctx context.Context) context.Context {
	if g.token != "" {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+g.token)
	}
	return ctx
}

// Register отправляет запрос на регистрацию нового пользователя.
func (g *grpcClient) Register(ctx context.Context, username, password string) (string, error) {
	resp, err := g.client.Register(ctx, &pb.RegisterRequest{Username: username, Password: password})
	if err != nil {
		return "", err
	}
	return resp.Token, nil
}

// Login отправляет запрос на аутентификацию пользователя.
func (g *grpcClient) Login(ctx context.Context, username, password string) (string, error) {
	resp, err := g.client.Login(ctx, &pb.LoginRequest{Username: username, Password: password})
	if err != nil {
		return "", err
	}
	return resp.Token, nil
}

// CreateItem отправляет запрос на создание нового зашифрованного элемента.
func (g *grpcClient) CreateItem(ctx context.Context, dataType string, encryptedData []byte, metaInfo string) (*pb.Item, error) {
	stream, errCreate := g.client.CreateItem(g.getCtx(ctx))
	if errCreate != nil {
		return nil, errCreate
	}

	totalSize := len(encryptedData)
	offset := 0

	for offset < totalSize {
		end := offset + chunkSize
		if end > totalSize {
			end = totalSize
		}

		chunkData := encryptedData[offset:end]
		isLast := (end == totalSize)

		req := &pb.ItemChunk{
			DataType:      dataType,
			MetaInfo:      metaInfo,
			EncryptedData: chunkData,
			IsLast:        isLast,
		}

		if errSend := stream.Send(req); errSend != nil {
			return nil, errSend
		}

		offset = end
	}

	return stream.CloseAndRecv()
}

// UpdateItem обновляет элемент пользователя
func (g *grpcClient) UpdateItem(ctx context.Context, id, dataType string, encryptedData []byte, metaInfo string) error {
	stream, errUpdate := g.client.UpdateItem(g.getCtx(ctx))
	if errUpdate != nil {
		return errUpdate
	}

	totalSize := len(encryptedData)
	offset := 0

	for offset < totalSize {
		end := offset + chunkSize
		if end > totalSize {
			end = totalSize
		}

		chunkData := encryptedData[offset:end]
		isLast := (end == totalSize)

		req := &pb.ItemChunk{
			DataType:      dataType,
			MetaInfo:      metaInfo,
			EncryptedData: chunkData,
			IsLast:        isLast,
			Id:            id,
		}

		if errSend := stream.Send(req); errSend != nil {
			return errSend
		}

		offset = end
	}

	_, errClose := stream.CloseAndRecv()

	return errClose
}

// DeleteItem удаляет элемент пользователя.
func (g *grpcClient) DeleteItem(ctx context.Context, id string) error {
	_, err := g.client.DeleteItem(g.getCtx(ctx), &pb.Item{
		Id: id,
	})

	return err
}

// GetItems запрашивает список всех элементов данных пользователя.
func (g *grpcClient) GetItems(ctx context.Context, timeUpdate int64) ([]*pb.Item, error) {
	stream, err := g.client.GetItems(g.getCtx(ctx), &pb.GetItemsRequest{TimeUpdate: timeUpdate})
	if err != nil {
		return nil, err
	}

	var items []*pb.Item
	for {
		item, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break // Поток завершен
		}
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// Close закрывает gRPC-соединение и освобождает ресурсы.
func (g *grpcClient) Close() error {
	return g.conn.Close()
}
