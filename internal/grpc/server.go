package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/eshadow1/gophkeeper/gen/pb"
	"github.com/eshadow1/gophkeeper/internal/config"
	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/eshadow1/gophkeeper/internal/middleware"
	"github.com/eshadow1/gophkeeper/internal/model"
	"github.com/eshadow1/gophkeeper/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

const (
	minSizePassword = 8
)

type Authenticator interface {
	Register(ctx context.Context, user *model.UserUnsave) (string, error)
	Login(ctx context.Context, user *model.UserUnsave) (string, error)
}

type KeepService interface {
	CreateItem(ctx context.Context, item *model.ItemDB) (*model.ItemDB, error)
	UpdateItem(ctx context.Context, item *model.ItemDB) error
	DeleteItem(ctx context.Context, item *model.ItemDB) error
	GetItems(ctx context.Context, userID string, timeUpdate time.Time) ([]*model.ItemDB, error)
}

// Server реализует gRPC сервис GophKeeper с бизнес-логикой
// регистрации, аутентификации и управления приватными данными.
type Server struct {
	pb.UnimplementedGophKeeperServiceServer
	auth   Authenticator
	keeper KeepService
	cfg    *config.ServerConfig
}

// NewServer создает новый экземпляр gRPC сервера GophKeeper.
func NewServer(ks KeepService, a Authenticator, cfg *config.ServerConfig) *Server {
	return &Server{
		auth:   a,
		keeper: ks,
		cfg:    cfg,
	}
}

// Register регистрирует нового пользователя в системе.
// Хэширует пароль через bcrypt и возвращает JWT токен.
func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username and password are required")
	}

	if len(req.Password) < minSizePassword {
		return nil, status.Errorf(codes.InvalidArgument, "password must be at least 8 characters")
	}

	token, errRegister := s.auth.Register(ctx, &model.UserUnsave{Username: req.Username, Password: req.Password})
	if errRegister != nil {
		if errors.Is(errRegister, service.ErrUserAlreadyExists) {
			loggers.Log.Info("User already exists", "username", req.Username)
			return nil, status.Errorf(codes.AlreadyExists, "user already exists")
		}

		loggers.Log.Error("failed to find user: ", errRegister)
		return nil, status.Errorf(codes.Internal, "failed to create user")
	}

	return &pb.RegisterResponse{Token: token}, nil
}

// Login выполняет аутентификацию пользователя и возвращает JWT токен.
func (s *Server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, status.Errorf(codes.InvalidArgument, "username and password are required")
	}

	token, errGet := s.auth.Login(ctx, &model.UserUnsave{Username: req.Username, Password: req.Password})
	if errGet != nil {
		if errors.Is(errGet, service.ErrUserNotFound) || errors.Is(errGet, service.ErrCompareHash) {
			loggers.Log.Info("User not found or password invalid", "username", req.Username)
			return nil, status.Errorf(codes.Unauthenticated, "invalid username or password")
		}

		loggers.Log.Error("failed to find user: ", errGet)
		return nil, status.Errorf(codes.Internal, "failed to authenticate")
	}

	return &pb.LoginResponse{Token: token}, nil
}

// CreateItem создает новый приватный элемент для авторизованного пользователя.
// Данные шифруются перед сохранением в БД.
func (s *Server) CreateItem(stream pb.GophKeeperService_CreateItemServer) error {
	userID, errGet := s.getUserIDFromContext(stream.Context())
	if errGet != nil {
		return status.Errorf(codes.Unauthenticated, "user not found in context")
	}

	var dataType, metaInfo string
	var encryptedData []byte
	chunkCount := 0

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			// Клиент закрыл поток, не отправив флаг is_last
			return status.Errorf(codes.InvalidArgument, "stream closed without is_last flag")
		}
		if err != nil {
			loggers.Log.Error("failed to receive chunk: ", err)
			return status.Errorf(codes.Internal, "failed to receive chunk")
		}

		if chunkCount == 0 {
			if chunk.DataType == "" {
				return status.Errorf(codes.InvalidArgument, "data_type is required in the first chunk")
			}
			dataType = chunk.DataType
			metaInfo = chunk.MetaInfo
		}

		encryptedData = append(encryptedData, chunk.EncryptedData...)

		if chunk.IsLast {
			break
		}
		chunkCount++
	}

	if dataType == "" || len(encryptedData) == 0 {
		return status.Errorf(codes.InvalidArgument, "Data Type and EncryptedData are empty")
	}

	item := &model.ItemDB{
		UserID:        userID,
		DataType:      dataType,
		EncryptedData: encryptedData,
		MetaInfo:      metaInfo,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	newItem, errCreate := s.keeper.CreateItem(stream.Context(), item)
	if errCreate != nil {
		loggers.Log.Error("failed to create item: ", errCreate)
		return status.Errorf(codes.Internal, "failed to create item")
	}

	if newItem == nil {
		loggers.Log.Error("failed to create not unique item")
		return status.Errorf(codes.Internal, "failed to create item")
	}

	loggers.Log.Info("create item successfully via stream")

	return stream.SendAndClose(s.itemToProto(newItem))
}

// UpdateItem обновляет существующий приватный элемент.
func (s *Server) UpdateItem(stream pb.GophKeeperService_UpdateItemServer) error {
	userID, errGet := s.getUserIDFromContext(stream.Context())
	if errGet != nil {
		return status.Errorf(codes.Unauthenticated, "user not found in context")
	}

	var dataType, metaInfo, id string
	var encryptedData []byte
	chunkCount := 0

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return status.Errorf(codes.InvalidArgument, "stream closed without is_last flag")
		}
		if err != nil {
			loggers.Log.Error("failed to receive chunk: ", err)
			return status.Errorf(codes.Internal, "failed to receive chunk")
		}

		if chunkCount == 0 {
			if chunk.DataType == "" {
				return status.Errorf(codes.InvalidArgument, "data_type is required in the first chunk")
			}
			dataType = chunk.DataType
			metaInfo = chunk.MetaInfo
			id = chunk.Id
		}

		encryptedData = append(encryptedData, chunk.EncryptedData...)

		if chunk.IsLast {
			break
		}
		chunkCount++
	}

	if dataType == "" || len(encryptedData) == 0 {
		return status.Errorf(codes.InvalidArgument, "Data Type and EncryptedData are empty")
	}

	item := &model.ItemDB{
		ID:            id,
		UserID:        userID,
		DataType:      dataType,
		EncryptedData: encryptedData,
		MetaInfo:      metaInfo,
		CreatedAt:     time.Now(),
	}

	errCreate := s.keeper.UpdateItem(stream.Context(), item)
	if errCreate != nil {
		loggers.Log.Error("failed to create item: ", errCreate)
		return status.Errorf(codes.Internal, "failed to create item")
	}

	loggers.Log.Info("update item successfully via stream")
	return nil
}

// DeleteItem помечает элемент как удаленный (soft delete).
func (s *Server) DeleteItem(ctx context.Context, req *pb.Item) (*pb.DeleteItemResponse, error) {
	userID, errGet := s.getUserIDFromContext(ctx)
	if errGet != nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not found in context")
	}

	item := s.protoToItem(req)
	item.UserID = userID

	errDelete := s.keeper.DeleteItem(ctx, item)
	if errDelete != nil {
		loggers.Log.Error("failed to delete item: ", errDelete)
		return nil, status.Errorf(codes.Internal, "failed to delete item")
	}

	loggers.Log.Info("delete item")
	return nil, nil
}

// GetItems возвращает элементы пользователя потоково, по одному за раз.
func (s *Server) GetItems(req *pb.GetItemsRequest, stream pb.GophKeeperService_GetItemsServer) error {
	userID, errGet := s.getUserIDFromContext(stream.Context())
	if errGet != nil {
		return status.Errorf(codes.Unauthenticated, "user not found in context")
	}

	items, errGetItems := s.keeper.GetItems(stream.Context(), userID, time.Unix(0, req.TimeUpdate))
	if errGetItems != nil {
		loggers.Log.Error("failed to get items: ", errGetItems)
		return status.Errorf(codes.Internal, "failed to get items")
	}

	for _, item := range items {
		if err := stream.Send(s.itemToProto(item)); err != nil {
			loggers.Log.Error("failed to send item: ", err)
			return status.Errorf(codes.Internal, "failed to send item")
		}
	}

	loggers.Log.Infof("sent %d items to user %s", len(items), userID)
	return nil
}

// getUserIDFromContext извлекает идентификатор пользователя из контекста gRPC.
func (*Server) getUserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(model.UserIDContextKey).(string)
	if !ok || userID == "" {
		return "", errors.New("user ID not found in context")
	}
	return userID, nil
}

// itemToProto преобразует внутреннюю модель Item в proto ItemResponse.
func (*Server) itemToProto(item *model.ItemDB) *pb.Item {
	return &pb.Item{
		Id:            item.ID,
		DataType:      item.DataType,
		EncryptedData: item.EncryptedData,
		MetaInfo:      item.MetaInfo,
		CreatedAt:     item.CreatedAt.UnixNano(),
		UpdatedAt:     item.UpdatedAt.UnixNano(),
	}
}

func (*Server) protoToItem(req *pb.Item) *model.ItemDB {
	return &model.ItemDB{
		ID:            req.Id,
		DataType:      req.DataType,
		EncryptedData: req.EncryptedData,
		MetaInfo:      req.MetaInfo,
		CreatedAt:     time.Unix(0, req.CreatedAt),
		UpdatedAt:     time.Unix(0, req.UpdatedAt),
	}
}

// InitGRPCServer создает и настраивает gRPC сервер.
func InitGRPCServer(ctx context.Context, cfg *config.ServerConfig, ks KeepService, a Authenticator) (*grpc.Server, net.Listener, error) {
	creds, err := credentials.NewServerTLSFromFile(cfg.TLS.CertFile, cfg.TLS.KeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось загрузить TLS сертификаты: %w", err)
	}

	lc := &net.ListenConfig{}

	lis, err := lc.Listen(ctx, "tcp", cfg.GRPCAddr)
	if err != nil {
		return nil, nil, err
	}
	worker := service.NewJWTWorker(&cfg.Auth)

	grpcSrv := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(middleware.UnaryAuthInterceptor(&cfg.Auth, worker)),
		grpc.StreamInterceptor(middleware.StreamAuthInterceptor(&cfg.Auth, worker)),
	)

	pb.RegisterGophKeeperServiceServer(grpcSrv, NewServer(ks, a, cfg))

	return grpcSrv, lis, nil
}
