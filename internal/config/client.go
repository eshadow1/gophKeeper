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
}

// NewClient создает и возвращает указатель на новый экземпляр структуры ServerConfig.
func NewClient() *ClientConfig {
	return &ClientConfig{}
}

// Init инициализирует конфигурацию.
func (c *ClientConfig) Init() {
	c.parseWithFlag()

	c.GRPCAddr = c.updateEnv("GRPC_ADDRESS", c.GRPCAddr)
	c.Log.Level = c.updateEnv("LOG_LEVEL", c.Log.Level)
}

func (*ClientConfig) updateEnv(name, defaultValue string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return defaultValue
}

func (c *ClientConfig) parseWithFlag() {
	flag.StringVar(&c.GRPCAddr, "g", DefaultGRPCAddr, "host:port")
	flag.StringVar(&c.Log.Level, "l", DefaultLevelLog, "level log")

	flag.Parse()
}
