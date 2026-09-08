package config

import (
	"flag"
	"os"
)

// ClientConfig является главной структурой конфигурации приложения
type ClientConfig struct {
	// GRPCAddr — сетевой адрес для gRPC, который дописывается.
	GRPCAddr string
	// Log содержит настройки логирования.
	Log LogConfig
	// TLS содержит настройки защищенного соединения
	TLS TLSConfig
}

// TLSConfig содержит параметры для настройки защищенного соединения.
type TLSConfig struct {
	// CACertPath — путь к файлу сертификата.
	CACertPath string
	// ServerName — имя сервера, которое ожидается в сертификате (Common Name или SAN).
	ServerName string
}

// NewClient создает и возвращает указатель на новый экземпляр структуры ServerConfig.
func NewClient() *ClientConfig {
	return &ClientConfig{}
}

// Init инициализирует конфигурацию.
func (c *ClientConfig) Init() {
	c.GRPCAddr = c.updateEnv("GRPC_ADDRESS", DefaultGRPCAddr)
	c.Log.Level = c.updateEnv("LOG_LEVEL", DefaultLevelLog)

	c.TLS.ServerName = c.updateEnv("TLS_SERVER_NAME", DefaultServerName)
	c.TLS.CACertPath = c.updateEnv("TLS_CA_CERT_PATH", DefaultPathTLSCert)
	c.parseWithFlag()
}

func (*ClientConfig) updateEnv(name, defaultValue string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return defaultValue
}

func (c *ClientConfig) parseWithFlag() {
	flag.StringVar(&c.GRPCAddr, "g", c.GRPCAddr, "host:port")
	flag.StringVar(&c.Log.Level, "l", c.Log.Level, "level log")
	flag.StringVar(&c.TLS.ServerName, "tls", c.TLS.ServerName, "server name")
	flag.StringVar(&c.TLS.CACertPath, "tlsc", c.TLS.CACertPath, "tls cert path")

	flag.Parse()
}
