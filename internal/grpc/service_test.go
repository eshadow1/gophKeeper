package grpc

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/eshadow1/gophkeeper/gen/pb"
	"github.com/eshadow1/gophkeeper/internal/config"
	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/eshadow1/gophkeeper/internal/model"
	"github.com/eshadow1/gophkeeper/internal/service"

	mockgrpc "github.com/eshadow1/gophkeeper/mocks/grpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func init() {
	loggers.CreateLogger("info")
}

type baseMockStream struct {
	ctx context.Context
}

func (b *baseMockStream) SetHeader(md metadata.MD) error  { return nil }
func (b *baseMockStream) SendHeader(md metadata.MD) error { return nil }
func (b *baseMockStream) SetTrailer(md metadata.MD)       {}
func (b *baseMockStream) Context() context.Context        { return b.ctx }
func (b *baseMockStream) SendMsg(m any) error             { return nil }
func (b *baseMockStream) RecvMsg(m any) error             { return nil }

type mockCreateItemStream struct {
	*baseMockStream
	chunks  []*pb.ItemChunk
	idx     int
	recvErr error
	sent    *pb.Item
}

func (m *mockCreateItemStream) Recv() (*pb.ItemChunk, error) {
	if m.recvErr != nil {
		return nil, m.recvErr
	}
	if m.idx >= len(m.chunks) {
		return nil, io.EOF
	}
	chunk := m.chunks[m.idx]
	m.idx++
	return chunk, nil
}

func (m *mockCreateItemStream) SendAndClose(res *pb.Item) error {
	m.sent = res
	return nil
}

type mockUpdateItemStream struct {
	*baseMockStream
	chunks  []*pb.ItemChunk
	idx     int
	recvErr error
}

func (m *mockUpdateItemStream) Recv() (*pb.ItemChunk, error) {
	if m.recvErr != nil {
		return nil, m.recvErr
	}
	if m.idx >= len(m.chunks) {
		return nil, io.EOF
	}
	chunk := m.chunks[m.idx]
	m.idx++
	return chunk, nil
}

func (m *mockUpdateItemStream) SendAndClose(res *pb.UpdateItemResponse) error {
	return nil
}

type mockGetItemsStream struct {
	*baseMockStream
	sentItems []*pb.Item
	sendErr   error
}

func (m *mockGetItemsStream) Send(res *pb.Item) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentItems = append(m.sentItems, res)
	return nil
}

func newTestServer(auth *mockgrpc.MockAuthenticator, keeper *mockgrpc.MockKeepService) *Server {
	return NewServer(keeper, auth, &config.ServerConfig{})
}

func ctxWithUserID(userID string) context.Context {
	return context.WithValue(context.Background(), model.UserIDContextKey, userID)
}

func ctxWithoutUserID() context.Context {
	return context.Background()
}

func TestServer_Register(t *testing.T) {
	tests := []struct {
		name      string
		req       *pb.RegisterRequest
		mockSetup func(m *mockgrpc.MockAuthenticator)
		wantCode  codes.Code
		wantToken string
	}{
		{
			name: "успешная регистрация",
			req:  &pb.RegisterRequest{Username: "user1", Password: "password123"},
			mockSetup: func(m *mockgrpc.MockAuthenticator) {
				m.On("Register", mock.Anything, &model.UserUnsave{Username: "user1", Password: "password123"}).
					Return("jwt_token", nil)
			},
			wantCode:  codes.OK,
			wantToken: "jwt_token",
		},
		{
			name:      "ошибка: пустой username",
			req:       &pb.RegisterRequest{Username: "", Password: "password123"},
			mockSetup: func(m *mockgrpc.MockAuthenticator) {},
			wantCode:  codes.InvalidArgument,
		},
		{
			name:      "ошибка: короткий пароль",
			req:       &pb.RegisterRequest{Username: "user1", Password: "123"},
			mockSetup: func(m *mockgrpc.MockAuthenticator) {},
			wantCode:  codes.InvalidArgument,
		},
		{
			name: "ошибка: пользователь уже существует",
			req:  &pb.RegisterRequest{Username: "user1", Password: "password123"},
			mockSetup: func(m *mockgrpc.MockAuthenticator) {
				m.On("Register", mock.Anything, mock.Anything).Return("", service.ErrUserAlreadyExists)
			},
			wantCode: codes.AlreadyExists,
		},
		{
			name: "ошибка: внутренняя ошибка сервиса",
			req:  &pb.RegisterRequest{Username: "user1", Password: "password123"},
			mockSetup: func(m *mockgrpc.MockAuthenticator) {
				m.On("Register", mock.Anything, mock.Anything).Return("", errors.New("db error"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuth := mockgrpc.NewMockAuthenticator(t)
			tt.mockSetup(mockAuth)
			srv := newTestServer(mockAuth, mockgrpc.NewMockKeepService(t))

			resp, err := srv.Register(context.Background(), tt.req)

			st, ok := status.FromError(err)
			require.True(t, ok, "ошибка должна быть gRPC статусом")
			assert.Equal(t, tt.wantCode, st.Code())

			if tt.wantCode == codes.OK {
				require.NotNil(t, resp)
				assert.Equal(t, tt.wantToken, resp.Token)
			}
			mockAuth.AssertExpectations(t)
		})
	}
}

func TestServer_Login(t *testing.T) {
	tests := []struct {
		name      string
		req       *pb.LoginRequest
		mockSetup func(m *mockgrpc.MockAuthenticator)
		wantCode  codes.Code
	}{
		{
			name: "успешный вход",
			req:  &pb.LoginRequest{Username: "user1", Password: "password123"},
			mockSetup: func(m *mockgrpc.MockAuthenticator) {
				m.On("Login", mock.Anything, &model.UserUnsave{Username: "user1", Password: "password123"}).
					Return("jwt_token", nil)
			},
			wantCode: codes.OK,
		},
		{
			name:      "ошибка: пустые поля",
			req:       &pb.LoginRequest{Username: "", Password: ""},
			mockSetup: func(m *mockgrpc.MockAuthenticator) {},
			wantCode:  codes.InvalidArgument,
		},
		{
			name: "ошибка: пользователь не найден",
			req:  &pb.LoginRequest{Username: "user1", Password: "wrong"},
			mockSetup: func(m *mockgrpc.MockAuthenticator) {
				m.On("Login", mock.Anything, mock.Anything).Return("", service.ErrUserNotFound)
			},
			wantCode: codes.Unauthenticated,
		},
		{
			name: "ошибка: неверный хеш пароля",
			req:  &pb.LoginRequest{Username: "user1", Password: "wrong"},
			mockSetup: func(m *mockgrpc.MockAuthenticator) {
				m.On("Login", mock.Anything, mock.Anything).Return("", service.ErrCompareHash)
			},
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuth := mockgrpc.NewMockAuthenticator(t)
			tt.mockSetup(mockAuth)
			srv := newTestServer(mockAuth, mockgrpc.NewMockKeepService(t))

			resp, err := srv.Login(context.Background(), tt.req)

			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.wantCode, st.Code())

			if tt.wantCode == codes.OK {
				require.NotNil(t, resp)
				assert.Equal(t, "jwt_token", resp.Token)
			}
			mockAuth.AssertExpectations(t)
		})
	}
}

func TestServer_CreateItem(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		chunks    []*pb.ItemChunk
		recvErr   error
		mockSetup func(m *mockgrpc.MockKeepService)
		wantCode  codes.Code
	}{
		{
			name: "успешное создание (несколько чанков)",
			ctx:  ctxWithUserID("user-1"),
			chunks: []*pb.ItemChunk{
				{DataType: "text", MetaInfo: "meta", EncryptedData: []byte("part1"), IsLast: false},
				{DataType: "text", MetaInfo: "meta", EncryptedData: []byte("part2"), IsLast: true},
			},
			mockSetup: func(m *mockgrpc.MockKeepService) {
				m.On("CreateItem", mock.Anything, mock.MatchedBy(func(item *model.ItemDB) bool {
					return item.UserID == "user-1" && item.DataType == "text" && string(item.EncryptedData) == "part1part2"
				})).Return(&model.ItemDB{ID: "new-id", DataType: "text"}, nil)
			},
			wantCode: codes.OK,
		},
		{
			name:      "ошибка: нет пользователя в контексте",
			ctx:       ctxWithoutUserID(),
			chunks:    []*pb.ItemChunk{{DataType: "text", EncryptedData: []byte("data"), IsLast: true}},
			mockSetup: func(m *mockgrpc.MockKeepService) {},
			wantCode:  codes.Unauthenticated,
		},
		{
			name:      "ошибка: поток закрыт без флага is_last",
			ctx:       ctxWithUserID("user-1"),
			chunks:    []*pb.ItemChunk{{DataType: "text", EncryptedData: []byte("data"), IsLast: false}},
			recvErr:   io.EOF, // Эмулируем обрыв потока
			mockSetup: func(m *mockgrpc.MockKeepService) {},
			wantCode:  codes.InvalidArgument,
		},
		{
			name: "ошибка: пустой DataType в первом чанке",
			ctx:  ctxWithUserID("user-1"),
			chunks: []*pb.ItemChunk{
				{DataType: "", EncryptedData: []byte("data"), IsLast: true},
			},
			mockSetup: func(m *mockgrpc.MockKeepService) {},
			wantCode:  codes.InvalidArgument,
		},
		{
			name: "ошибка: сбой сохранения в keeper",
			ctx:  ctxWithUserID("user-1"),
			chunks: []*pb.ItemChunk{
				{DataType: "text", EncryptedData: []byte("data"), IsLast: true},
			},
			mockSetup: func(m *mockgrpc.MockKeepService) {
				m.On("CreateItem", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockKeeper := mockgrpc.NewMockKeepService(t)
			tt.mockSetup(mockKeeper)
			srv := newTestServer(mockgrpc.NewMockAuthenticator(t), mockKeeper)

			stream := &mockCreateItemStream{
				baseMockStream: &baseMockStream{ctx: tt.ctx},
				chunks:         tt.chunks,
				recvErr:        tt.recvErr,
			}

			err := srv.CreateItem(stream)

			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.wantCode, st.Code())
			mockKeeper.AssertExpectations(t)
		})
	}
}

func TestServer_UpdateItem(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		chunks    []*pb.ItemChunk
		mockSetup func(m *mockgrpc.MockKeepService)
		wantCode  codes.Code
	}{
		{
			name: "успешное обновление",
			ctx:  ctxWithUserID("user-1"),
			chunks: []*pb.ItemChunk{
				{Id: "item-1", DataType: "text", MetaInfo: "meta", EncryptedData: []byte("upd"), IsLast: true},
			},
			mockSetup: func(m *mockgrpc.MockKeepService) {
				m.On("UpdateItem", mock.Anything, mock.MatchedBy(func(item *model.ItemDB) bool {
					return item.ID == "item-1" && item.UserID == "user-1"
				})).Return(nil)
			},
			wantCode: codes.OK,
		},
		{
			name:      "ошибка: нет пользователя в контексте",
			ctx:       ctxWithoutUserID(),
			chunks:    []*pb.ItemChunk{{Id: "item-1", DataType: "text", EncryptedData: []byte("upd"), IsLast: true}},
			mockSetup: func(m *mockgrpc.MockKeepService) {},
			wantCode:  codes.Unauthenticated,
		},
		{
			name: "ошибка: сбой обновления в keeper",
			ctx:  ctxWithUserID("user-1"),
			chunks: []*pb.ItemChunk{
				{Id: "item-1", DataType: "text", EncryptedData: []byte("upd"), IsLast: true},
			},
			mockSetup: func(m *mockgrpc.MockKeepService) {
				m.On("UpdateItem", mock.Anything, mock.Anything).Return(errors.New("db error"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockKeeper := mockgrpc.NewMockKeepService(t)
			tt.mockSetup(mockKeeper)
			srv := newTestServer(mockgrpc.NewMockAuthenticator(t), mockKeeper)

			stream := &mockUpdateItemStream{
				baseMockStream: &baseMockStream{ctx: tt.ctx},
				chunks:         tt.chunks,
			}

			err := srv.UpdateItem(stream)

			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.wantCode, st.Code())
			mockKeeper.AssertExpectations(t)
		})
	}
}

func TestServer_DeleteItem(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		req       *pb.Item
		mockSetup func(m *mockgrpc.MockKeepService)
		wantCode  codes.Code
	}{
		{
			name: "успешное удаление",
			ctx:  ctxWithUserID("user-1"),
			req:  &pb.Item{Id: "item-1"},
			mockSetup: func(m *mockgrpc.MockKeepService) {
				m.On("DeleteItem", mock.Anything, mock.MatchedBy(func(item *model.ItemDB) bool {
					return item.ID == "item-1" && item.UserID == "user-1"
				})).Return(nil)
			},
			wantCode: codes.OK,
		},
		{
			name:      "ошибка: нет пользователя в контексте",
			ctx:       ctxWithoutUserID(),
			req:       &pb.Item{Id: "item-1"},
			mockSetup: func(m *mockgrpc.MockKeepService) {},
			wantCode:  codes.Unauthenticated,
		},
		{
			name: "ошибка: сбой удаления в keeper",
			ctx:  ctxWithUserID("user-1"),
			req:  &pb.Item{Id: "item-1"},
			mockSetup: func(m *mockgrpc.MockKeepService) {
				m.On("DeleteItem", mock.Anything, mock.Anything).Return(errors.New("db error"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockKeeper := mockgrpc.NewMockKeepService(t)
			tt.mockSetup(mockKeeper)
			srv := newTestServer(mockgrpc.NewMockAuthenticator(t), mockKeeper)

			resp, err := srv.DeleteItem(tt.ctx, tt.req)

			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.wantCode, st.Code())
			assert.Nil(t, resp)

			mockKeeper.AssertExpectations(t)
		})
	}
}

func TestServer_GetItems(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		req       *pb.GetItemsRequest
		mockSetup func(m *mockgrpc.MockKeepService)
		streamErr error
		wantCode  codes.Code
		wantCount int
	}{
		{
			name: "успешное получение элементов",
			ctx:  ctxWithUserID("user-1"),
			req:  &pb.GetItemsRequest{TimeUpdate: 1000},
			mockSetup: func(m *mockgrpc.MockKeepService) {
				m.On("GetItems", mock.Anything, "user-1", mock.Anything).Return([]*model.ItemDB{
					{ID: "1", DataType: "text", CreatedAt: time.Now()},
					{ID: "2", DataType: "card", CreatedAt: time.Now()},
				}, nil)
			},
			wantCode:  codes.OK,
			wantCount: 2,
		},
		{
			name:      "ошибка: нет пользователя в контексте",
			ctx:       ctxWithoutUserID(),
			req:       &pb.GetItemsRequest{TimeUpdate: 1000},
			mockSetup: func(m *mockgrpc.MockKeepService) {},
			wantCode:  codes.Unauthenticated,
		},
		{
			name: "ошибка: сбой получения из keeper",
			ctx:  ctxWithUserID("user-1"),
			req:  &pb.GetItemsRequest{TimeUpdate: 1000},
			mockSetup: func(m *mockgrpc.MockKeepService) {
				m.On("GetItems", mock.Anything, "user-1", mock.Anything).Return(nil, errors.New("db error"))
			},
			wantCode: codes.Internal,
		},
		{
			name: "ошибка: сбой отправки по стриму",
			ctx:  ctxWithUserID("user-1"),
			req:  &pb.GetItemsRequest{TimeUpdate: 1000},
			mockSetup: func(m *mockgrpc.MockKeepService) {
				m.On("GetItems", mock.Anything, "user-1", mock.Anything).Return([]*model.ItemDB{
					{ID: "1", DataType: "text", CreatedAt: time.Now()},
				}, nil)
			},
			streamErr: errors.New("network broken"),
			wantCode:  codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockKeeper := mockgrpc.NewMockKeepService(t)
			tt.mockSetup(mockKeeper)
			srv := newTestServer(mockgrpc.NewMockAuthenticator(t), mockKeeper)

			stream := &mockGetItemsStream{
				baseMockStream: &baseMockStream{ctx: tt.ctx},
				sendErr:        tt.streamErr,
			}

			err := srv.GetItems(tt.req, stream)

			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.wantCode, st.Code())

			if tt.wantCode == codes.OK {
				assert.Len(t, stream.sentItems, tt.wantCount)
			}
			mockKeeper.AssertExpectations(t)
		})
	}
}
