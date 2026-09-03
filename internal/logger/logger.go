// Package loggers предоставляет централизованный логгер для приложения,
// построенный на базе библиотеки go.uber.org/zap.
// Пакет инициализирует глобальный экземпляр SugaredLogger с указанным уровнем детализации.
package loggers

import (
	"go.uber.org/zap"
)

// Log — глобальный экземпляр zap.SugaredLogger, доступный во всем приложении.
var Log *zap.SugaredLogger

// CreateLogger инициализирует глобальный логгер с указанным уровнем детализации.
// Принимает строковое представление уровня (например, "info", "debug", "warn", "error").
func CreateLogger(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	zl, err := cfg.Build()
	if err != nil {
		return err
	}
	Log = zl.Sugar()
	return nil
}
