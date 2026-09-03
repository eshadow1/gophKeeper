// Package main является точкой входа серверной части gophkeeper
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/eshadow1/gophkeeper/internal/config"
	grpcserver "github.com/eshadow1/gophkeeper/internal/grpc"
	loggers "github.com/eshadow1/gophkeeper/internal/logger"
	"github.com/eshadow1/gophkeeper/internal/repository"
	"github.com/eshadow1/gophkeeper/internal/service"
)

const (
	// defaultShutdownTimeout — максимальное время, отводимое на graceful shutdown
	// сервера и фоновых процессов.
	defaultShutdownTimeout = 30 * time.Second
	// defaultVersionValue - дефолтное значение для формирования информации о версии
	defaultVersionValue = "N/A"
)

var (
	buildVersionServer = defaultVersionValue
	buildDateServer    = defaultVersionValue
	buildCommitServer  = defaultVersionValue
)

// main — точка входа приложения. Выполняет инициализацию всех компонентов,
// запуск GRPC-сервера, ожидание сигнала завершения и graceful shutdown.
func main() {
	fmt.Printf("Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		buildVersionServer, buildDateServer, buildCommitServer)

	ctxStart, cancelStart := context.WithCancel(context.Background())
	defer cancelStart()

	cfg := config.NewServer()
	cfg.Init()

	errCreateLog := loggers.CreateLogger(cfg.Log.Level)
	if errCreateLog != nil {
		fmt.Println("Error creating logger:", errCreateLog)
		return
	}

	storage, err := repository.NewPostgreSQL(cfg.Storage)
	if err != nil {
		loggers.Log.Error("failed to initialize database", err)
		return
	}
	defer storage.Close()

	authWorker := service.NewJWTWorker(&cfg.Auth)
	auth := service.NewAuthKeeper(authWorker, storage, &cfg.Auth)
	keeperService := service.NewKeeperService(storage, cfg, repository.NewUpdateUsers())

	grpcSrv, grpcLis, errInitGRPC := grpcserver.InitGRPCServer(ctxStart, cfg, keeperService, auth)
	if errInitGRPC != nil {
		loggers.Log.Error("failed to initialize grpc server", errInitGRPC)
		return
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		loggers.Log.Infof("Starting gRPC server on %s", cfg.GRPCAddr)
		if errGRPC := grpcSrv.Serve(grpcLis); errGRPC != nil {
			loggers.Log.Errorf("gRPC server failed: %v", errGRPC)
		}
	}()

	<-quit
	loggers.Log.Infoln("Shutting down server...")

	ctxStop, cancelStop := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancelStop()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		stopped := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
			loggers.Log.Infoln("gRPC server gracefully stopped")
		case <-ctxStop.Done():
			loggers.Log.Warnln("gRPC server graceful stop timed out, forcing stop")
			grpcSrv.Stop() // Принудительная остановка
		}
	}()

	wg.Wait()
	loggers.Log.Infoln("All servers shut down successfully")
}
