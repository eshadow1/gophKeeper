// Package main является точкой входа в TUI-приложение GophKeeper.
// Приложение реализует терминальный пользовательский интерфейс на основе библиотеки charmbracelet/bubbletea.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eshadow1/gophkeeper/internal/config"
	"github.com/eshadow1/gophkeeper/internal/crypto"
	"github.com/eshadow1/gophkeeper/internal/grpc"
	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/eshadow1/gophkeeper/internal/repository"
	"github.com/eshadow1/gophkeeper/internal/service"
	"github.com/eshadow1/gophkeeper/internal/tui"
)

const (
	// defaultVersionValue - дефолтное значение для формирования информации о версии
	defaultVersionValue = "N/A"
)

var (
	buildVersionClient = defaultVersionValue
	buildDateClient    = defaultVersionValue
	buildCommitClient  = defaultVersionValue
)

// main является точкой входа в TUI-приложение.
func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("Build version: %s\nBuild date: %s\nBuild commit: %s\n",
			buildVersionClient, buildDateClient, buildCommitClient)
		return
	}

	cfg := config.NewClient()
	cfg.Init()

	if err := loggers.CreateLogger(cfg.Log.Level); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка инициализации логгера: %v\n", err)
		os.Exit(1)
	}

	grpcClient, err := grpc.NewGRPCClient(cfg.GRPCAddr, &cfg.TLS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось подключиться к gRPC-серверу: %v\n", err)
		os.Exit(1)
	}
	defer grpcClient.Close()

	cryptoProvider := crypto.New()
	repo := repository.NewMemoryRepository()
	v := service.NewValidator()
	keeper := service.NewClient(cfg, cryptoProvider, grpcClient, repo, v)

	m := tui.NewModel(keeper, tui.NewDoModel(keeper))
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, errTea := p.Run(); errTea != nil {
		fmt.Fprintf(os.Stderr, "Ошибка запуска TUI: %v\n", errTea)
		return
	}
}
