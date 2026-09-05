// Package middleware реализует промежуточный этап в работе grpc
package middleware

import (
	"context"
	"strings"

	"github.com/eshadow1/gophkeeper/internal/config"
	"github.com/eshadow1/gophkeeper/internal/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context возвращает модифицированный контекст с добавленным UserID.
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// JWTWorker интерфейс работы с JWT
type JWTWorker interface {
	GetUID(context.Context, string, []byte) (model.TokenAuth, error)
}

// UnaryAuthInterceptor создает unary middleware для проверки JWT-токена в gRPC.
func UnaryAuthInterceptor(cfg *config.AuthConfig, worker JWTWorker) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == "/keeper.GophKeeperService/Register" ||
			info.FullMethod == "/keeper.GophKeeperService/Login" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}

		tokenID, errGetUID := worker.GetUID(ctx, strings.TrimPrefix(authHeaders[0], "Bearer "), cfg.JWTSecret)
		if errGetUID != nil {
			return nil, status.Error(codes.Internal, "failed to create user token")
		}

		if tokenID.UID == "" {
			return nil, status.Error(codes.Unauthenticated, "invalid user ID in token")
		}

		ctx = context.WithValue(ctx, model.UserIDContextKey, tokenID.UID)
		return handler(ctx, req)
	}
}

// StreamAuthInterceptor извлекает и проверяет JWT-токен из метаданных потокового запроса.
func StreamAuthInterceptor(cfg *config.AuthConfig, worker JWTWorker) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(stream.Context())
		if !ok {
			return status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return status.Errorf(codes.Unauthenticated, "missing authorization token")
		}

		tokenID, errGetUID := worker.GetUID(stream.Context(), strings.TrimPrefix(authHeaders[0], "Bearer "), cfg.JWTSecret)
		if errGetUID != nil {
			return status.Error(codes.Internal, "failed to create user token")
		}

		if tokenID.UID == "" {
			return status.Error(codes.Unauthenticated, "invalid user ID in token")
		}

		ctx := context.WithValue(stream.Context(), model.UserIDContextKey, tokenID.UID)
		wrapped := &wrappedServerStream{
			ServerStream: stream,
			ctx:          ctx,
		}

		return handler(srv, wrapped)
	}
}
