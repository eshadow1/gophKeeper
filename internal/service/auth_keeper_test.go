package service

import (
	"context"
	"errors"
	"testing"

	"github.com/eshadow1/gophkeeper/internal/config"
	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/eshadow1/gophkeeper/internal/model"
	"github.com/eshadow1/gophkeeper/internal/repository"
	mockservice "github.com/eshadow1/gophkeeper/mocks/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	loggers.CreateLogger("info")
}

func TestAuthKeeper_Register(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTSecret: []byte("test_secret_key"),
	}

	tests := []struct {
		name            string
		userUnsave      *model.UserUnsave
		mockRepoSetup   func(*mockservice.MockAuthRepository)
		mockWorkerSetup func(*mockservice.MockAuthWorker)
		wantToken       string
		wantErr         error
	}{
		{
			name: "успешная регистрация",
			userUnsave: &model.UserUnsave{
				Username: "new_user",
				Password: "secure_password_123",
			},
			mockRepoSetup: func(m *mockservice.MockAuthRepository) {
				m.EXPECT().CreateUser(mock.Anything, mock.MatchedBy(func(u *model.User) bool {
					return u.Username == "new_user" &&
						bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("secure_password_123")) == nil
				})).Return(&model.User{
					ID:       "user-id-1",
					Username: "new_user",
				}, nil)
			},
			mockWorkerSetup: func(m *mockservice.MockAuthWorker) {
				m.EXPECT().CreateJWT("user-id-1", cfg.JWTSecret).Return("valid_jwt_token_abc", nil)
			},
			wantToken: "valid_jwt_token_abc",
			wantErr:   nil,
		},
		{
			name: "ошибка: пользователь уже существует",
			userUnsave: &model.UserUnsave{
				Username: "existing_user",
				Password: "password",
			},
			mockRepoSetup: func(m *mockservice.MockAuthRepository) {
				m.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(nil, repository.ErrUserAlreadyExists)
			},
			mockWorkerSetup: func(m *mockservice.MockAuthWorker) {},
			wantToken:       "",
			wantErr:         ErrUserAlreadyExists,
		},
		{
			name: "ошибка: внутренняя ошибка базы данных",
			userUnsave: &model.UserUnsave{
				Username: "db_error_user",
				Password: "password",
			},
			mockRepoSetup: func(m *mockservice.MockAuthRepository) {
				m.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(nil, errors.New("connection refused"))
			},
			mockWorkerSetup: func(m *mockservice.MockAuthWorker) {},
			wantToken:       "",
			wantErr:         errors.New("connection refused"),
		},
		{
			name: "ошибка: сбой генерации JWT",
			userUnsave: &model.UserUnsave{
				Username: "jwt_fail_user",
				Password: "password",
			},
			mockRepoSetup: func(m *mockservice.MockAuthRepository) {
				m.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(&model.User{
					ID:       "user-id-2",
					Username: "jwt_fail_user",
				}, nil)
			},
			mockWorkerSetup: func(m *mockservice.MockAuthWorker) {
				m.EXPECT().CreateJWT("user-id-2", cfg.JWTSecret).Return("", errors.New("jwt generation failed"))
			},
			wantToken: "",
			wantErr:   errors.New("jwt generation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mockservice.NewMockAuthRepository(t)
			mockWorker := mockservice.NewMockAuthWorker(t)

			tt.mockRepoSetup(mockRepo)
			tt.mockWorkerSetup(mockWorker)

			svc := NewAuthKeeper(mockWorker, mockRepo, cfg)

			gotToken, err := svc.Register(context.Background(), tt.userUnsave)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
				assert.Empty(t, gotToken)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantToken, gotToken)
			}
		})
	}
}

func TestAuthKeeper_Login(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTSecret: []byte("test_secret_key"),
	}

	correctPassword := "correct_password_123"
	validHash, errCrypto := bcrypt.GenerateFromPassword([]byte(correctPassword), bcrypt.DefaultCost)
	require.NoError(t, errCrypto)

	tests := []struct {
		name            string
		userUnsave      *model.UserUnsave
		mockRepoSetup   func(*mockservice.MockAuthRepository)
		mockWorkerSetup func(*mockservice.MockAuthWorker)
		wantToken       string
		wantErr         error
	}{
		{
			name: "успешный вход",
			userUnsave: &model.UserUnsave{
				Username: "valid_user",
				Password: correctPassword,
			},
			mockRepoSetup: func(m *mockservice.MockAuthRepository) {
				m.EXPECT().GetUser(mock.Anything, "valid_user").Return(&model.User{
					ID:           "user-id-1",
					Username:     "valid_user",
					PasswordHash: string(validHash),
				}, nil)
			},
			mockWorkerSetup: func(m *mockservice.MockAuthWorker) {
				m.EXPECT().CreateJWT("user-id-1", cfg.JWTSecret).Return("login_jwt_token_xyz", nil)
			},
			wantToken: "login_jwt_token_xyz",
			wantErr:   nil,
		},
		{
			name: "ошибка: неверный пароль",
			userUnsave: &model.UserUnsave{
				Username: "valid_user",
				Password: "wrong_password",
			},
			mockRepoSetup: func(m *mockservice.MockAuthRepository) {
				m.EXPECT().GetUser(mock.Anything, "valid_user").Return(&model.User{
					ID:           "user-id-1",
					Username:     "valid_user",
					PasswordHash: string(validHash),
				}, nil)
			},
			mockWorkerSetup: func(m *mockservice.MockAuthWorker) {},
			wantToken:       "",
			wantErr:         ErrCompareHash,
		},
		{
			name: "ошибка: пользователь не найден",
			userUnsave: &model.UserUnsave{
				Username: "ghost_user",
				Password: "any_password",
			},
			mockRepoSetup: func(m *mockservice.MockAuthRepository) {
				m.EXPECT().GetUser(mock.Anything, "ghost_user").Return(nil, repository.ErrUserNotFound)
			},
			mockWorkerSetup: func(m *mockservice.MockAuthWorker) {},
			wantToken:       "",
			wantErr:         ErrUserNotFound,
		},
		{
			name: "ошибка: внутренняя ошибка базы данных при входе",
			userUnsave: &model.UserUnsave{
				Username: "db_error_user",
				Password: "password",
			},
			mockRepoSetup: func(m *mockservice.MockAuthRepository) {
				m.EXPECT().GetUser(mock.Anything, "db_error_user").Return(nil, errors.New("db timeout"))
			},
			mockWorkerSetup: func(m *mockservice.MockAuthWorker) {},
			wantToken:       "",
			wantErr:         errors.New("db timeout"),
		},
		{
			name: "ошибка: сбой генерации JWT при входе",
			userUnsave: &model.UserUnsave{
				Username: "valid_user",
				Password: correctPassword,
			},
			mockRepoSetup: func(m *mockservice.MockAuthRepository) {
				m.EXPECT().GetUser(mock.Anything, "valid_user").Return(&model.User{
					ID:           "user-id-1",
					Username:     "valid_user",
					PasswordHash: string(validHash),
				}, nil)
			},
			mockWorkerSetup: func(m *mockservice.MockAuthWorker) {
				m.EXPECT().CreateJWT("user-id-1", cfg.JWTSecret).Return("", errors.New("jwt worker panic"))
			},
			wantToken: "",
			wantErr:   errors.New("jwt worker panic"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mockservice.NewMockAuthRepository(t)
			mockWorker := mockservice.NewMockAuthWorker(t)

			tt.mockRepoSetup(mockRepo)
			tt.mockWorkerSetup(mockWorker)

			svc := NewAuthKeeper(mockWorker, mockRepo, cfg)

			gotToken, errLogin := svc.Login(context.Background(), tt.userUnsave)

			if tt.wantErr != nil {
				require.Error(t, errLogin)
				assert.EqualError(t, errLogin, tt.wantErr.Error())
				assert.Empty(t, gotToken)
			} else {
				require.NoError(t, errLogin)
				assert.Equal(t, tt.wantToken, gotToken)
			}
		})
	}
}
