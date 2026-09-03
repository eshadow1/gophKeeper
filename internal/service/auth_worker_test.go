package service

import (
	"testing"

	"github.com/eshadow1/gophkeeper/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJwtWorker_NewJWTWorker(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTSecret:   []byte("test-secret-key"),
		TokenIssuer: "test-issuer",
	}
	worker := NewJWTWorker(cfg)
	assert.NotNil(t, worker)
	assert.Equal(t, cfg, worker.cfg)
}

func TestJwtWorker_GenerateUserID(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTSecret:   []byte("test-secret-key"),
		TokenIssuer: "test-issuer",
	}
	worker := NewJWTWorker(cfg)

	userID := worker.GenerateUserID()
	_, err := uuid.Parse(userID)
	require.NoError(t, err)

	userID2 := worker.GenerateUserID()
	assert.NotEqual(t, userID, userID2)
}

func TestJwtWorker_CreateJWT(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTSecret:   []byte("test-secret-key"),
		TokenIssuer: "test-issuer",
	}
	worker := NewJWTWorker(cfg)

	tests := []struct {
		name    string
		userID  string
		secret  []byte
		wantErr bool
	}{
		{
			name:    "success",
			userID:  "user-123",
			secret:  cfg.JWTSecret,
			wantErr: false,
		},
		{
			name:    "empty userID",
			userID:  "",
			secret:  cfg.JWTSecret,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := worker.CreateJWT(tt.userID, tt.secret)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, token)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, token)

			claims, err := worker.ValidateJWT(token, cfg.JWTSecret)
			require.NoError(t, err)
			assert.NotNil(t, claims)
			assert.Equal(t, tt.userID, claims.UserID)
			assert.Equal(t, cfg.TokenIssuer, claims.Issuer)
			assert.NotZero(t, claims.IssuedAt)
			assert.NotZero(t, claims.ExpiresAt)
		})
	}
}

func TestJwtWorker_GetUID(t *testing.T) {
	cfg := &config.AuthConfig{
		JWTSecret:   []byte("test-secret-key"),
		TokenIssuer: "test-issuer",
	}
	worker := NewJWTWorker(cfg)

	tests := []struct {
		name    string
		token   string
		secret  []byte
		wantErr bool
	}{
		{
			name:    "incorrect token",
			token:   "user-123",
			secret:  cfg.JWTSecret,
			wantErr: true,
		},
		{
			name:    "empty token",
			token:   "",
			secret:  cfg.JWTSecret,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := worker.GetUID(t.Context(), tt.token, tt.secret)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
