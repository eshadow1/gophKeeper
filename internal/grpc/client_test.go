package grpc

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/eshadow1/gophkeeper/gen/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MockServiceClient struct {
	mock.Mock
}

func (m *MockServiceClient) Register(ctx context.Context, in *pb.RegisterRequest, opts ...grpc.CallOption) (*pb.RegisterResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) != nil {
		return args.Get(0).(*pb.RegisterResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceClient) Login(ctx context.Context, in *pb.LoginRequest, opts ...grpc.CallOption) (*pb.LoginResponse, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) != nil {
		return args.Get(0).(*pb.LoginResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceClient) CreateItem(ctx context.Context, opts ...grpc.CallOption) (pb.GophKeeperService_CreateItemClient, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) != nil {
		return args.Get(0).(pb.GophKeeperService_CreateItemClient), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceClient) UpdateItem(ctx context.Context, opts ...grpc.CallOption) (pb.GophKeeperService_UpdateItemClient, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) != nil {
		return args.Get(0).(pb.GophKeeperService_UpdateItemClient), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceClient) DeleteItem(ctx context.Context, in *pb.Item, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) != nil {
		return args.Get(0).(*emptypb.Empty), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceClient) GetItems(ctx context.Context, in *pb.GetItemsRequest, opts ...grpc.CallOption) (pb.GophKeeperService_GetItemsClient, error) {
	args := m.Called(ctx, in, opts)
	if args.Get(0) != nil {
		return args.Get(0).(pb.GophKeeperService_GetItemsClient), args.Error(1)
	}
	return nil, args.Error(1)
}

type MockCreateItemStream struct {
	mock.Mock
}

func (m *MockCreateItemStream) Send(req *pb.ItemChunk) error { return m.Called(req).Error(0) }
func (m *MockCreateItemStream) CloseAndRecv() (*pb.Item, error) {
	args := m.Called()
	if args.Get(0) != nil {
		return args.Get(0).(*pb.Item), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockCreateItemStream) Header() (metadata.MD, error) { return nil, nil }
func (m *MockCreateItemStream) Trailer() metadata.MD         { return nil }
func (m *MockCreateItemStream) CloseSend() error             { return nil }
func (m *MockCreateItemStream) Context() context.Context     { return context.Background() }
func (m *MockCreateItemStream) SendMsg(any) error            { return nil }
func (m *MockCreateItemStream) RecvMsg(any) error            { return nil }

type MockUpdateItemStream struct {
	mock.Mock
}

func (m *MockUpdateItemStream) Send(req *pb.ItemChunk) error { return m.Called(req).Error(0) }
func (m *MockUpdateItemStream) CloseAndRecv() (*emptypb.Empty, error) {
	args := m.Called()
	if args.Get(0) != nil {
		return args.Get(0).(*emptypb.Empty), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockUpdateItemStream) Header() (metadata.MD, error) { return nil, nil }
func (m *MockUpdateItemStream) Trailer() metadata.MD         { return nil }
func (m *MockUpdateItemStream) CloseSend() error             { return nil }
func (m *MockUpdateItemStream) Context() context.Context     { return context.Background() }
func (m *MockUpdateItemStream) SendMsg(any) error            { return nil }
func (m *MockUpdateItemStream) RecvMsg(any) error            { return nil }

func newTestClient(mockSvc *MockServiceClient) *grpcClient {
	return &grpcClient{
		conn:   nil,
		client: mockSvc,
	}
}

func TestGRPCClient_Register(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		password  string
		mockSetup func(m *MockServiceClient)
		wantToken string
		wantErr   bool
	}{
		{
			name:     "успешная регистрация",
			username: "user1",
			password: "pass1",
			mockSetup: func(m *MockServiceClient) {
				m.On("Register", mock.Anything, &pb.RegisterRequest{Username: "user1", Password: "pass1"}, mock.Anything).
					Return(&pb.RegisterResponse{Token: "jwt_token_123"}, nil)
			},
			wantToken: "jwt_token_123",
			wantErr:   false,
		},
		{
			name:     "ошибка регистрации",
			username: "user1",
			password: "pass1",
			mockSetup: func(m *MockServiceClient) {
				m.On("Register", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.New("already exists"))
			},
			wantToken: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockServiceClient)
			tt.mockSetup(mockSvc)
			client := newTestClient(mockSvc)

			token, err := client.Register(context.Background(), tt.username, tt.password)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantToken, token)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestGRPCClient_Login(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		password  string
		mockSetup func(m *MockServiceClient)
		wantToken string
		wantErr   bool
	}{
		{
			name:     "успешный вход",
			username: "user1",
			password: "pass1",
			mockSetup: func(m *MockServiceClient) {
				m.On("Login", mock.Anything, &pb.LoginRequest{Username: "user1", Password: "pass1"}, mock.Anything).
					Return(&pb.LoginResponse{Token: "jwt_token_456"}, nil)
			},
			wantToken: "jwt_token_456",
			wantErr:   false,
		},
		{
			name:     "ошибка входа",
			username: "user1",
			password: "wrong_pass",
			mockSetup: func(m *MockServiceClient) {
				m.On("Login", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.New("invalid credentials"))
			},
			wantToken: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockServiceClient)
			tt.mockSetup(mockSvc)
			client := newTestClient(mockSvc)

			token, err := client.Login(context.Background(), tt.username, tt.password)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantToken, token)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestGRPCClient_CreateItem(t *testing.T) {
	largeData := make([]byte, 2*1024*1024+512*1024)

	tests := []struct {
		name        string
		dataType    string
		data        []byte
		metaInfo    string
		mockSetup   func(m *MockServiceClient, stream *MockCreateItemStream)
		wantErr     bool
		errContains string
	}{
		{
			name:     "успешная отправка одним чанком",
			dataType: "text",
			data:     []byte("small data"),
			metaInfo: "meta",
			mockSetup: func(m *MockServiceClient, stream *MockCreateItemStream) {
				m.On("CreateItem", mock.Anything, mock.Anything).Return(stream, nil)
				stream.On("Send", mock.MatchedBy(func(req *pb.ItemChunk) bool {
					return req.DataType == "text" && req.MetaInfo == "meta" &&
						string(req.EncryptedData) == "small data" && req.IsLast == true
				})).Return(nil)
				stream.On("CloseAndRecv").Return(&pb.Item{Id: "new_id"}, nil)
			},
			wantErr: false,
		},
		{
			name:     "успешная отправка несколькими чанками",
			dataType: "binary",
			data:     largeData,
			metaInfo: "file",
			mockSetup: func(m *MockServiceClient, stream *MockCreateItemStream) {
				m.On("CreateItem", mock.Anything, mock.Anything).Return(stream, nil)

				stream.On("Send", mock.MatchedBy(func(req *pb.ItemChunk) bool {
					return len(req.EncryptedData) == 1024*1024 && req.IsLast == false
				})).Return(nil)

				stream.On("Send", mock.MatchedBy(func(req *pb.ItemChunk) bool {
					return len(req.EncryptedData) == 1024*1024 && req.IsLast == false
				})).Return(nil)

				stream.On("Send", mock.MatchedBy(func(req *pb.ItemChunk) bool {
					return len(req.EncryptedData) == 512*1024 && req.IsLast == true
				})).Return(nil)

				stream.On("CloseAndRecv").Return(&pb.Item{Id: "bin_id"}, nil)
			},
			wantErr: false,
		},
		{
			name:     "ошибка при Send",
			dataType: "text",
			data:     []byte("data"),
			mockSetup: func(m *MockServiceClient, stream *MockCreateItemStream) {
				m.On("CreateItem", mock.Anything, mock.Anything).Return(stream, nil)
				stream.On("Send", mock.Anything).Return(errors.New("network error"))
			},
			wantErr:     true,
			errContains: "network error",
		},
		{
			name:     "ошибка при CloseAndRecv",
			dataType: "text",
			data:     []byte("data"),
			mockSetup: func(m *MockServiceClient, stream *MockCreateItemStream) {
				m.On("CreateItem", mock.Anything, mock.Anything).Return(stream, nil)
				stream.On("Send", mock.Anything).Return(nil)
				stream.On("CloseAndRecv").Return(nil, errors.New("server validation failed"))
			},
			wantErr:     true,
			errContains: "server validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockServiceClient)
			mockStream := new(MockCreateItemStream)
			tt.mockSetup(mockSvc, mockStream)

			client := newTestClient(mockSvc)
			item, err := client.CreateItem(context.Background(), tt.dataType, tt.data, tt.metaInfo)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, item)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, item)
			}
			mockSvc.AssertExpectations(t)
			mockStream.AssertExpectations(t)
		})
	}
}

func TestGRPCClient_UpdateItem(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		data        []byte
		mockSetup   func(m *MockServiceClient, stream *MockUpdateItemStream)
		wantErr     bool
		errContains string
	}{
		{
			name: "успешное обновление",
			id:   "item-1",
			data: []byte("updated data"),
			mockSetup: func(m *MockServiceClient, stream *MockUpdateItemStream) {
				m.On("UpdateItem", mock.Anything, mock.Anything).Return(stream, nil)
				stream.On("Send", mock.MatchedBy(func(req *pb.ItemChunk) bool {
					return req.Id == "item-1" && req.IsLast == true
				})).Return(nil)
				stream.On("CloseAndRecv").Return(nil, io.EOF)
			},
			wantErr: false,
		},
		{
			name: "ошибка при CloseAndRecv (не EOF)",
			id:   "item-1",
			data: []byte("data"),
			mockSetup: func(m *MockServiceClient, stream *MockUpdateItemStream) {
				m.On("UpdateItem", mock.Anything, mock.Anything).Return(stream, nil)
				stream.On("Send", mock.Anything).Return(nil)
				stream.On("CloseAndRecv").Return(nil, errors.New("update failed"))
			},
			wantErr:     true,
			errContains: "update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockServiceClient)
			mockStream := new(MockUpdateItemStream)
			tt.mockSetup(mockSvc, mockStream)

			client := newTestClient(mockSvc)
			err := client.UpdateItem(context.Background(), tt.id, "text", tt.data, "meta")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
			mockSvc.AssertExpectations(t)
			mockStream.AssertExpectations(t)
		})
	}
}

func TestGRPCClient_DeleteItem(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		mockSetup func(m *MockServiceClient)
		wantErr   bool
	}{
		{
			name: "успешное удаление",
			id:   "item-1",
			mockSetup: func(m *MockServiceClient) {
				m.On("DeleteItem", mock.Anything, &pb.Item{Id: "item-1"}, mock.Anything).
					Return(&emptypb.Empty{}, nil)
			},
			wantErr: false,
		},
		{
			name: "ошибка удаления",
			id:   "item-999",
			mockSetup: func(m *MockServiceClient) {
				m.On("DeleteItem", mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.New("not found"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockServiceClient)
			tt.mockSetup(mockSvc)
			client := newTestClient(mockSvc)

			err := client.DeleteItem(context.Background(), tt.id)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}
