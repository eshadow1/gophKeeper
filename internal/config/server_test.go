package config

import (
	"testing"

	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigServer_Init(t *testing.T) {
	errLog := loggers.CreateLogger("Debug")
	require.NoError(t, errLog)

	tests := []struct {
		name                  string
		addr                  string
		logLevel              string
		storagePathDB         string
		storagePathMigrations string
		authToken             string
		authJWTSecret         []byte
	}{
		{
			name:                  "success",
			addr:                  DefaultGRPCAddr,
			logLevel:              DefaultLevelLog,
			storagePathDB:         DefaultEmptyString,
			storagePathMigrations: DefaultMigrationPath,
			authToken:             DefaultEmptyString,
			authJWTSecret:         make([]byte, 0),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := NewServer()
			cfg.Init()

			assert.Equal(t, test.addr, cfg.GRPCAddr)
			assert.Equal(t, test.logLevel, cfg.Log.Level)
			assert.Equal(t, test.storagePathDB, cfg.Storage.PathDB)
			assert.Equal(t, test.storagePathMigrations, cfg.Storage.PathMigrations)
			assert.Equal(t, test.authToken, cfg.Auth.TokenIssuer)
			assert.Equal(t, test.authJWTSecret, cfg.Auth.JWTSecret)
		})
	}
}
