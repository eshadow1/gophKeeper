// Package configs предоставляет функциональность для инициализации и управления
// конфигурацией приложения. Он поддерживает загрузку параметров через флаги
// командной строки и переменные окружения.
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultEmptyString -  пустая строка
	DefaultEmptyString = ""
	// DefaultGRPCAddr — адрес HTTP-сервера приложения по умолчанию.
	DefaultGRPCAddr = ":50051"
	// DefaultLevelLog — уровень логирования по умолчанию.
	DefaultLevelLog = "info"
	// DefaultMigrationPath — путь к директории с миграциями базы данных по умолчанию.
	DefaultMigrationPath = "./migrations"
	// DefaultPathTLSCert - путь к сертификату
	DefaultPathTLSCert = "./cert/cert.pem"
	// DefaultPathTLSKey - путь к ключу
	DefaultPathTLSKey = "./cert/key.pem"
	// DefaultServerName - имя сервера
	DefaultServerName = "localhost"
)

// LogConfig описывает конфигурацию подсистемы логирования.
type LogConfig struct {
	// Level — уровень детализации логов (например, info, debug, error).
	Level string
}

// StorageConfig описывает конфигурацию для работы с хранилищем данных
type StorageConfig struct {
	// PathDB — URI для подключения к базе данных
	PathDB string
	// PathMigrations — путь к файлам миграций базы данных.
	PathMigrations string
}

// AuthConfig описывает конфигурацию для модуля аутентификации и работы с токенами.
type AuthConfig struct {
	// JWTSecret — секретный ключ для подписи и проверки JWT-токенов.
	JWTSecret []byte
	// TokenIssuer — название издателя (issuer), указываемого в JWT-токенах.
	TokenIssuer string
}

// ConfigJSON является главной структурой конфигурации приложения
type ConfigJSON struct {
	// GRPCAddr — сетевой адрес для gRPC, который дописывается.
	GRPCAddr string `json:"grpc_address"`
	// LogLevel — уровень детализации логов (например, info, debug, error).
	LogLevel string `json:"log_level"`
	// StoragePathDB — URI для подключения к базе данных
	StoragePathDB string `json:"database_dsn"`
	// StoragePathMigrations — путь к файлам миграций базы данных.
	StoragePathMigrations string `json:"migrations_path"`
}

// TLSServerConfig содержит параметры для настройки защищенного соединения.
type TLSServerConfig struct {
	// CertFile - путь к файлу сертификата
	CertFile string
	// KeyFile - путь к файлу приватного ключа
	KeyFile string
}

// ServerConfig является главной структурой конфигурации приложения
type ServerConfig struct {
	// GRPCAddr — сетевой адрес для gRPC, который дописывается.
	GRPCAddr string
	// Log содержит настройки логирования.
	Log LogConfig
	// Storage содержит настройки подключения к хранилищу данных.
	Storage StorageConfig
	// Auth содержит настройки аутентификации.
	Auth AuthConfig
	// TLS  содержит настройки защищенного соединения
	TLS TLSServerConfig
}

// NewConfig создает и возвращает указатель на новый экземпляр структуры ServerConfig.
func NewServer() *ServerConfig {
	return &ServerConfig{}
}

// Init инициализирует конфигурацию.
func (c *ServerConfig) Init() {
	configFile := c.getConfigPath()
	cfg, errParse := c.parseWithJSON(configFile)
	if errParse != nil {
		fmt.Fprintf(os.Stderr, "failed to parse file: %s\n", errParse)
	}

	c.GRPCAddr = c.updateEnv("GRPC_ADDRESS", cfg.GRPCAddr)

	c.Log.Level = c.updateEnv("LOG_LEVEL", cfg.LogLevel)

	c.Storage.PathDB = c.updateEnv("DATABASE_DSN", cfg.StoragePathDB)
	c.Storage.PathMigrations = c.updateEnv("MIGRATION_PATH", cfg.StoragePathMigrations)

	c.Auth.JWTSecret = []byte(c.updateEnv("JWT_SECRET", DefaultEmptyString))
	c.Auth.TokenIssuer = c.updateEnv("TOKEN_ISSUER", DefaultEmptyString)

	c.TLS.CertFile = c.updateEnv("TLS_CERT_FILE", DefaultPathTLSCert)
	c.TLS.KeyFile = c.updateEnv("TLS_KEY_FILE", DefaultPathTLSKey)

	c.parseWithFlag()
}

func (*ServerConfig) parseWithJSON(path string) (*ConfigJSON, error) {
	cfg := &ConfigJSON{
		GRPCAddr:              DefaultGRPCAddr,
		LogLevel:              DefaultLevelLog,
		StoragePathDB:         DefaultEmptyString,
		StoragePathMigrations: DefaultMigrationPath,
	}
	if path == "" {
		return cfg, nil
	}

	file, errOpen := os.Open(path)
	if errOpen != nil {
		return cfg, fmt.Errorf("failed to open json file: %w", errOpen)
	}
	defer file.Close()

	if errUnmarshal := json.NewDecoder(file).Decode(&cfg); errUnmarshal != nil {
		return cfg, fmt.Errorf("failed to parse json file: %w", errUnmarshal)
	}

	return cfg, nil
}

func (*ServerConfig) getConfigPath() string {
	if path, ok := os.LookupEnv("CONFIG"); ok {
		return path
	}

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]

		if strings.HasPrefix(arg, "-c=") {
			return strings.TrimPrefix(arg, "-c=")
		} else if strings.HasPrefix(arg, "-config=") {
			return strings.TrimPrefix(arg, "-config=")
		} else if arg == "-c" || arg == "-config" {
			if i+1 < len(os.Args) && !strings.HasPrefix(os.Args[i+1], "-") {
				return os.Args[i+1]
			}
		}
	}

	return DefaultEmptyString
}

func (*ServerConfig) updateEnv(name, defaultValue string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return defaultValue
}

func (c *ServerConfig) parseWithFlag() {
	flag.StringVar(&c.GRPCAddr, "g", c.GRPCAddr, "host:port")
	flag.StringVar(&c.Log.Level, "l", c.Log.Level, "level log")
	flag.StringVar(&c.Storage.PathDB, "d", c.Storage.PathDB, "file storage path")
	flag.StringVar(&c.Storage.PathMigrations, "m", c.Storage.PathMigrations, "migrations path")

	flag.Parse()
}
