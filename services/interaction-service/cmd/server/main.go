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

	catalogv1 "github.com/bevzick/Infinitune/services/interaction-service/api/catalog/v1"
	commentv1 "github.com/bevzick/Infinitune/services/interaction-service/api/comment/v1"
	signalv1 "github.com/bevzick/Infinitune/services/interaction-service/api/signal/v1"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/catalog"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/commentlookup"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/config"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/database"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/grpcapi"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/httpapi"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/identity"
	"github.com/bevzick/Infinitune/services/interaction-service/internal/interaction"
	postgres_repository "github.com/bevzick/Infinitune/services/interaction-service/internal/repository/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	databasePool, err := database.Open(startupCtx, cfg.Database)
	startupCancel()
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer databasePool.Close()
	tokenVerifier, err := identity.NewVerifier(cfg.Identity, nil)
	if err != nil {
		logger.Error("failed to initialize identity verifier", "error", err)
		os.Exit(1)
	}
	catalogConnection, err := grpc.NewClient(
		cfg.CatalogAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to configure Catalog gRPC client", "error", err)
		os.Exit(1)
	}
	defer catalogConnection.Close()
	commentConnection, err := grpc.NewClient(
		cfg.CommentAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to configure Comment gRPC client", "error", err)
		os.Exit(1)
	}
	defer commentConnection.Close()

	repository := postgres_repository.NewLikeRepository(databasePool)
	likes := interaction.NewService(
		repository,
		catalog.NewValidator(catalogv1.NewCatalogServiceClient(catalogConnection)),
		commentlookup.NewValidator(commentv1.NewCommentServiceClient(commentConnection)),
	)

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Database: databasePool, Tokens: tokenVerifier, Likes: likes, Follows: likes,
			Origins: cfg.Origins, Logger: logger,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		logger.Error("failed to listen for gRPC", "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	signalv1.RegisterSignalServiceServer(grpcServer, grpcapi.NewSignalServer(likes))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 2)
	go func() {
		logger.Info("interaction service started", "address", cfg.HTTPAddress)
		serverErrors <- server.ListenAndServe()
	}()
	go func() {
		logger.Info("interaction gRPC service started", "address", cfg.GRPCAddress)
		serverErrors <- grpcServer.Serve(grpcListener)
	}()

	select {
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("interaction service stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutting down interaction service")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shut down interaction service", "error", err)
		os.Exit(1)
	}
	grpcStopped := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(grpcStopped) }()
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	logger.Info("interaction service stopped")
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
