// Package grpc реализует клиентскую и серверную логику протокола GRPC приложения GophKeeper.
// Клиент управляет аутентификацией и синхронизацией данных.
package grpc

import (
	"context"
	"errors"
	"io"

	"github.com/eshadow1/gophkeeper/gen/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
func NewGRPCClient(addr string) (*grpcClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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
	stream, err := g.client.CreateItem(g.getCtx(ctx))
	if err != nil {
		return nil, err
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

		if err := stream.Send(req); err != nil {
			return nil, err
		}

		offset = end
	}

	return stream.CloseAndRecv()
}

// UpdateItem обновляет элемент пользователя
func (g *grpcClient) UpdateItem(ctx context.Context, id, dataType string, encryptedData []byte, metaInfo string) error {
	stream, err := g.client.UpdateItem(g.getCtx(ctx))
	if err != nil {
		return err
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

		if err := stream.Send(req); err != nil {
			return err
		}

		offset = end
	}

	_, errClose := stream.CloseAndRecv()
	if !errors.Is(errClose, io.EOF) {
		return errClose
	}
	return nil
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
