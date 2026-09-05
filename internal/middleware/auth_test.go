package middleware

import (
	"context"
	"testing"

	"github.com/eshadow1/gophkeeper/internal/config"
	"github.com/eshadow1/gophkeeper/internal/model"
	mockmiddleware "github.com/eshadow1/gophkeeper/mocks/middleware"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestUnaryAuthInterceptor(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTSecret: []byte("super_secret_key"),
	}

	tests := []struct {
		name           string
		method         string
		incomingMD     metadata.MD
		mockSetup      func(*mockmiddleware.MockJWTWorker)
		expectedCode   codes.Code
		expectedErrMsg string
		expectedUID    string
	}{
		{
			name:   "успешная аутентификация",
			method: "/keeper.GophKeeperService/GetData",
			incomingMD: metadata.MD{
				"authorization": []string{"Bearer valid_token_123"},
			},
			mockSetup: func(mw *mockmiddleware.MockJWTWorker) {
				mw.EXPECT().
					GetUID(mock.Anything, "valid_token_123", cfg.JWTSecret).
					Return(model.TokenAuth{UID: "user_999"}, nil)
			},
			expectedCode: codes.OK,
			expectedUID:  "user_999",
		},
		{
			name:         "пропуск аутентификации для Register",
			method:       "/keeper.GophKeeperService/Register",
			mockSetup:    func(mw *mockmiddleware.MockJWTWorker) {},
			expectedCode: codes.OK,
		},
		{
			name:         "пропуск аутентификации для Login",
			method:       "/keeper.GophKeeperService/Login",
			mockSetup:    func(mw *mockmiddleware.MockJWTWorker) {},
			expectedCode: codes.OK,
		},
		{
			name:           "отсутствуют метаданные",
			method:         "/keeper.GophKeeperService/GetData",
			incomingMD:     nil,
			mockSetup:      func(mw *mockmiddleware.MockJWTWorker) {},
			expectedCode:   codes.Unauthenticated,
			expectedErrMsg: "missing metadata",
		},
		{
			name:   "отсутствует заголовок authorization",
			method: "/keeper.GophKeeperService/GetData",
			incomingMD: metadata.MD{
				"some-other-header": []string{"value"},
			},
			mockSetup:      func(mw *mockmiddleware.MockJWTWorker) {},
			expectedCode:   codes.Unauthenticated,
			expectedErrMsg: "missing authorization token",
		},
		{
			name:   "ошибка при получении UID из токена",
			method: "/keeper.GophKeeperService/GetData",
			incomingMD: metadata.MD{
				"authorization": []string{"Bearer invalid_token"},
			},
			mockSetup: func(mw *mockmiddleware.MockJWTWorker) {
				mw.EXPECT().
					GetUID(mock.Anything, "invalid_token", cfg.JWTSecret).
					Return(model.TokenAuth{}, assert.AnError)
			},
			expectedCode:   codes.Internal,
			expectedErrMsg: "failed to create user token",
		},
		{
			name:   "пустой UID в ответе воркера",
			method: "/keeper.GophKeeperService/GetData",
			incomingMD: metadata.MD{
				"authorization": []string{"Bearer empty_uid_token"},
			},
			mockSetup: func(mw *mockmiddleware.MockJWTWorker) {
				mw.EXPECT().
					GetUID(mock.Anything, "empty_uid_token", cfg.JWTSecret).
					Return(model.TokenAuth{UID: ""}, nil)
			},
			expectedCode:   codes.Unauthenticated,
			expectedErrMsg: "invalid user ID in token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := mockmiddleware.NewMockJWTWorker(t)
			tt.mockSetup(mockWorker)

			interceptor := UnaryAuthInterceptor(cfg, mockWorker)

			ctx := context.Background()
			if tt.incomingMD != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.incomingMD)
			}

			info := &grpc.UnaryServerInfo{
				FullMethod: tt.method,
			}

			var capturedCtx context.Context
			handler := func(ctx context.Context, req any) (any, error) {
				capturedCtx = ctx
				return "success_response", nil
			}

			resp, err := interceptor(ctx, nil, info, handler)

			if tt.expectedCode != codes.OK {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok, "ошибка должна быть статусом gRPC")
				assert.Equal(t, tt.expectedCode, st.Code())
				assert.Contains(t, st.Message(), tt.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "success_response", resp)

			if tt.expectedUID != "" {
				require.NotNil(t, capturedCtx)
				uid := capturedCtx.Value(model.UserIDContextKey)
				assert.Equal(t, tt.expectedUID, uid, "UID в контексте должен совпадать с ожидаемым")
			}
		})
	}
}

func TestStreamAuthInterceptor(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTSecret: []byte("super_secret_key"),
	}

	tests := []struct {
		name           string
		incomingMD     metadata.MD
		mockSetup      func(*mockmiddleware.MockJWTWorker)
		expectedCode   codes.Code
		expectedErrMsg string
		expectedUID    string
	}{
		{
			name: "успешная аутентификация стрима",
			incomingMD: metadata.MD{
				"authorization": []string{"Bearer stream_token_456"},
			},
			mockSetup: func(mw *mockmiddleware.MockJWTWorker) {
				mw.EXPECT().
					GetUID(mock.Anything, "stream_token_456", cfg.JWTSecret).
					Return(model.TokenAuth{UID: "stream_user_111"}, nil)
			},
			expectedCode: codes.OK,
			expectedUID:  "stream_user_111",
		},
		{
			name:           "отсутствуют метаданные в стриме",
			incomingMD:     nil,
			mockSetup:      func(mw *mockmiddleware.MockJWTWorker) {},
			expectedCode:   codes.Unauthenticated,
			expectedErrMsg: "missing metadata",
		},
		{
			name: "отсутствует заголовок authorization в стриме",
			incomingMD: metadata.MD{
				"other-header": []string{"value"},
			},
			mockSetup:      func(mw *mockmiddleware.MockJWTWorker) {},
			expectedCode:   codes.Unauthenticated,
			expectedErrMsg: "missing authorization token",
		},
		{
			name: "ошибка воркера при валидации стрим-токена",
			incomingMD: metadata.MD{
				"authorization": []string{"Bearer bad_stream_token"},
			},
			mockSetup: func(mw *mockmiddleware.MockJWTWorker) {
				mw.EXPECT().
					GetUID(mock.Anything, "bad_stream_token", cfg.JWTSecret).
					Return(model.TokenAuth{}, assert.AnError)
			},
			expectedCode:   codes.Internal,
			expectedErrMsg: "failed to create user token",
		},
		{
			name: "пустой UID в стрим-токене",
			incomingMD: metadata.MD{
				"authorization": []string{"Bearer empty_stream_token"},
			},
			mockSetup: func(mw *mockmiddleware.MockJWTWorker) {
				mw.EXPECT().
					GetUID(mock.Anything, "empty_stream_token", cfg.JWTSecret).
					Return(model.TokenAuth{UID: ""}, nil)
			},
			expectedCode:   codes.Unauthenticated,
			expectedErrMsg: "invalid user ID in token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := mockmiddleware.NewMockJWTWorker(t)
			tt.mockSetup(mockWorker)

			interceptor := StreamAuthInterceptor(cfg, mockWorker)

			ctx := context.Background()
			if tt.incomingMD != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.incomingMD)
			}

			stream := &mockServerStream{
				ctx: ctx,
			}

			var capturedStream grpc.ServerStream
			handler := func(srv any, stream grpc.ServerStream) error {
				capturedStream = stream
				return nil
			}

			err := interceptor(nil, stream, nil, handler)

			if tt.expectedCode != codes.OK {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok, "ошибка должна быть статусом gRPC")
				assert.Equal(t, tt.expectedCode, st.Code())
				assert.Contains(t, st.Message(), tt.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, capturedStream)

			if tt.expectedUID != "" {
				wrappedCtx := capturedStream.Context()
				require.NotNil(t, wrappedCtx)
				uid := wrappedCtx.Value(model.UserIDContextKey)
				assert.Equal(t, tt.expectedUID, uid, "UID в контексте стрима должен совпадать с ожидаемым")
			}
		})
	}
}
