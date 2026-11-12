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

	catalogv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/catalog/v1"
	contentv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/content/v1"
	historyv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/history/v1"
	interactionv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/interaction/v1"
	rankingv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/ranking/v1"
	userv1 "github.com/bevzick/Infinitune/services/recommendation-service/api/user/v1"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/config"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/database"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/events"
	event_kafka "github.com/bevzick/Infinitune/services/recommendation-service/internal/events/kafka"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/grpcapi"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/httpapi"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/identity"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/ranking"
	"github.com/bevzick/Infinitune/services/recommendation-service/internal/recommendation"
	postgres_repository "github.com/bevzick/Infinitune/services/recommendation-service/internal/repository/postgres"
	preferencesignal "github.com/bevzick/Infinitune/services/recommendation-service/internal/signal"
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
	startupCtx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	databasePool, err := database.Open(startupCtx, cfg.Database)
	cancel()
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
	userConnection, err := grpc.NewClient(cfg.UserAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to configure User gRPC client", "error", err)
		os.Exit(1)
	}
	defer userConnection.Close()
	interactionConnection, err := grpc.NewClient(cfg.InteractionAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to configure Interaction gRPC client", "error", err)
		os.Exit(1)
	}
	defer interactionConnection.Close()
	historyConnection, err := grpc.NewClient(cfg.HistoryAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to configure History gRPC client", "error", err)
		os.Exit(1)
	}
	defer historyConnection.Close()
	catalogConnection, err := grpc.NewClient(cfg.CatalogAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to configure Catalog gRPC client", "error", err)
		os.Exit(1)
	}
	defer catalogConnection.Close()
	contentConnection, err := grpc.NewClient(cfg.ContentAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to configure Content gRPC client", "error", err)
		os.Exit(1)
	}
	defer contentConnection.Close()
	collector := preferencesignal.NewCollector(
		preferencesignal.NewUserClient(userv1.NewUserServiceClient(userConnection)),
		preferencesignal.NewInteractionClient(interactionv1.NewSignalServiceClient(interactionConnection)),
		preferencesignal.NewHistoryClient(historyv1.NewSignalServiceClient(historyConnection)),
		100,
	)
	rankingService := ranking.NewService(
		collector,
		ranking.NewCatalogSource(catalogv1.NewCatalogServiceClient(catalogConnection)),
		ranking.NewContentClient(contentv1.NewContentServiceClient(contentConnection)),
	)
	runRepository := postgres_repository.NewRunRepository(databasePool)
	runService, err := recommendation.NewService(
		rankingService,
		runRepository,
		ranking.AlgorithmVersion,
		cfg.RunTTL,
	)
	if err != nil {
		logger.Error("failed to configure recommendation run service", "error", err)
		os.Exit(1)
	}
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		logger.Error("failed to listen for gRPC", "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	rankingv1.RegisterRankingServiceServer(grpcServer, grpcapi.NewRankingServer(rankingService))
	server := &http.Server{
		Addr: cfg.HTTPAddress, Handler: httpapi.NewRouter(httpapi.Dependencies{
			Database: databasePool, Tokens: tokenVerifier, Recommendations: runService,
			Origins: cfg.Origins, Logger: logger,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	historyConsumer, err := event_kafka.NewConsumer(
		cfg.Events.Brokers,
		[]string{cfg.Events.HistoryTopic, cfg.Events.HistoryClearedTopic},
		cfg.Events.ConsumerGroup,
		events.NewHistoryHandler(runRepository), logger,
	)
	if err != nil {
		logger.Error("failed to initialize Kafka consumer", "error", err)
		os.Exit(1)
	}
	defer historyConsumer.Close()
	go historyConsumer.Run(ctx)
	errorsChannel := make(chan error, 2)
	go func() {
		logger.Info("recommendation service started", "address", cfg.HTTPAddress)
		errorsChannel <- server.ListenAndServe()
	}()
	go func() {
		logger.Info("recommendation gRPC service started", "address", cfg.GRPCAddress)
		errorsChannel <- grpcServer.Serve(grpcListener)
	}()
	select {
	case err = <-errorsChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("recommendation service stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutting down recommendation service")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shut down recommendation service", "error", err)
		os.Exit(1)
	}
	grpcStopped := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(grpcStopped) }()
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
}

func checkHealth(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse HTTP address: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"http://"+net.JoinHostPort("127.0.0.1", port)+"/health",
		nil,
	)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	return nil
}
