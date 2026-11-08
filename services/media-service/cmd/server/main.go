package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mediav1 "github.com/bevzick/Infinitune/services/media-service/api/media/v1"
	"github.com/bevzick/Infinitune/services/media-service/internal/config"
	"github.com/bevzick/Infinitune/services/media-service/internal/database"
	"github.com/bevzick/Infinitune/services/media-service/internal/grpcapi"
	"github.com/bevzick/Infinitune/services/media-service/internal/httpapi"
	"github.com/bevzick/Infinitune/services/media-service/internal/media"
	postgres_repository "github.com/bevzick/Infinitune/services/media-service/internal/repository/postgres"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
	"google.golang.org/grpc"
)

const (
	readHeaderTimeout = 5 * time.Second
	startupTimeout    = 10 * time.Second
	shutdownTimeout   = 10 * time.Second
	probeTimeout      = 3 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err = checkHealth(cfg.HTTPAddress); err != nil {
			logger.Error("healthcheck failed", "error", err)
			os.Exit(1)
		}
		return
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), startupTimeout)
	defer startupCancel()
	databasePool, err := database.Open(startupCtx, cfg.Database)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer databasePool.Close()
	objectStorage, err := storage.Open(cfg.Storage)
	if err != nil {
		logger.Error("failed to configure object storage", "error", err)
		os.Exit(1)
	}
	if err = objectStorage.Check(startupCtx); err != nil {
		logger.Error("failed to connect to object storage", "error", err)
		os.Exit(1)
	}
	metadataService := media.NewService(postgres_repository.NewMediaRepository(databasePool))
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		logger.Error("failed to listen for gRPC", "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	mediav1.RegisterMediaServiceServer(grpcServer, grpcapi.NewMediaServer(metadataService))

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Database: databasePool,
			Storage:  objectStorage,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 2)
	go func() {
		logger.Info("media service started", "address", cfg.HTTPAddress)
		serverErrors <- server.ListenAndServe()
	}()
	go func() {
		logger.Info("media gRPC service started", "address", cfg.GRPCAddress)
		serverErrors <- grpcServer.Serve(grpcListener)
	}()

	select {
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("media service stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutting down media service")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shut down media service", "error", err)
		os.Exit(1)
	}
	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	logger.Info("media service stopped")
}

func checkHealth(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse HTTP address: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://"+net.JoinHostPort("127.0.0.1", port)+"/health", nil)
	if err != nil {
		return fmt.Errorf("create healthcheck request: %w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("request health endpoint: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	return nil
}
