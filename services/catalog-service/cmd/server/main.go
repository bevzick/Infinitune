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

	catalogv1 "github.com/bevzick/Infinitune/services/catalog-service/api/catalog/v1"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/config"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/correlation"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/database"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/events"
	event_kafka "github.com/bevzick/Infinitune/services/catalog-service/internal/events/kafka"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/grpcapi"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/httpapi"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	postgres_repository "github.com/bevzick/Infinitune/services/catalog-service/internal/repository/postgres"
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
	databasePool, err := database.Open(startupCtx, cfg.Database)
	startupCancel()
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer databasePool.Close()
	eventProducer, err := event_kafka.NewProducer(cfg.Events.Brokers)
	if err != nil {
		logger.Error("failed to initialize Kafka producer", "error", err)
		os.Exit(1)
	}
	defer eventProducer.Close()
	metadataRepository := postgres_repository.NewMetadataRepository(databasePool, events.Topics{
		ReleasePublished: cfg.Events.PublishedTopic,
		ReleaseArchived:  cfg.Events.ArchivedTopic,
	})
	metadataService := metadata.NewService(metadataRepository)
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		logger.Error("failed to listen for gRPC", "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(correlation.UnaryServerInterceptor(logger)))
	catalogv1.RegisterCatalogServiceServer(grpcServer, grpcapi.NewCatalogServer(metadataService))

	eventMetrics := events.NewMetrics("catalog")
	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: eventMetrics.Wrap(httpapi.NewRouter(httpapi.Dependencies{
			Database: databasePool,
			Catalog:  metadataService,
			Origins:  cfg.Origins,
			Logger:   logger,
		})),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	approvalConsumer, err := event_kafka.NewConsumer(
		cfg.Events.Brokers, cfg.Events.ContentApprovedTopic, cfg.Events.ContentConsumerGroup,
		cfg.Events.DeadLetterTopic,
		events.NewContentApprovalHandler(
			postgres_repository.NewContentApprovalRepository(databasePool),
		), logger, eventMetrics,
	)
	if err != nil {
		logger.Error("failed to initialize Content Kafka consumer", "error", err)
		os.Exit(1)
	}
	defer approvalConsumer.Close()
	go approvalConsumer.Run(ctx)
	outboxDispatcher := events.NewDispatcher(
		postgres_repository.NewOutboxRepository(databasePool), eventProducer, logger,
		cfg.Events.OutboxInterval, cfg.Events.OutboxBatch, eventMetrics,
	)
	go outboxDispatcher.Run(ctx)

	serverErrors := make(chan error, 2)
	go func() {
		logger.Info("catalog service started", "address", cfg.HTTPAddress)
		serverErrors <- server.ListenAndServe()
	}()
	go func() {
		logger.Info("catalog gRPC service started", "address", cfg.GRPCAddress)
		serverErrors <- grpcServer.Serve(grpcListener)
	}()

	select {
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("catalog service stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutting down catalog service")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shut down catalog service", "error", err)
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

	logger.Info("catalog service stopped")
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
