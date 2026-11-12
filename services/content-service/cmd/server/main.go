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

	catalogv1 "github.com/bevzick/Infinitune/services/content-service/api/catalog/v1"
	contentv1 "github.com/bevzick/Infinitune/services/content-service/api/content/v1"
	mediav1 "github.com/bevzick/Infinitune/services/content-service/api/media/v1"
	userv1 "github.com/bevzick/Infinitune/services/content-service/api/user/v1"
	content_catalog "github.com/bevzick/Infinitune/services/content-service/internal/catalog"
	"github.com/bevzick/Infinitune/services/content-service/internal/config"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/database"
	"github.com/bevzick/Infinitune/services/content-service/internal/events"
	event_kafka "github.com/bevzick/Infinitune/services/content-service/internal/events/kafka"
	"github.com/bevzick/Infinitune/services/content-service/internal/grpcapi"
	"github.com/bevzick/Infinitune/services/content-service/internal/httpapi"
	"github.com/bevzick/Infinitune/services/content-service/internal/identity"
	content_media "github.com/bevzick/Infinitune/services/content-service/internal/media"
	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/bevzick/Infinitune/services/content-service/internal/playback"
	"github.com/bevzick/Infinitune/services/content-service/internal/publication"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	postgres_repository "github.com/bevzick/Infinitune/services/content-service/internal/repository/postgres"
	"github.com/bevzick/Infinitune/services/content-service/internal/rights"
	"github.com/bevzick/Infinitune/services/content-service/internal/submission"
	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/bevzick/Infinitune/services/content-service/internal/user"
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
	producer, err := event_kafka.NewProducer(cfg.Events.Brokers)
	if err != nil {
		logger.Error("failed to initialize Kafka producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()
	repository := postgres_repository.NewCreatorRepository(databasePool)
	creatorService := creator.NewService(repository)
	tokenVerifier, err := identity.NewVerifier(cfg.Identity, nil)
	if err != nil {
		logger.Error("failed to initialize identity verifier", "error", err)
		os.Exit(1)
	}
	userConnection, err := grpc.NewClient(cfg.UserAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to initialize user service client", "error", err)
		os.Exit(1)
	}
	defer userConnection.Close()
	mediaConnection, err := grpc.NewClient(
		cfg.MediaAddress, grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to initialize media service client", "error", err)
		os.Exit(1)
	}
	defer mediaConnection.Close()
	catalogConnection, err := grpc.NewClient(
		cfg.CatalogAddress, grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to initialize catalog service client", "error", err)
		os.Exit(1)
	}
	defer catalogConnection.Close()
	memberService := creator.NewMemberService(repository,
		user.NewDirectory(userv1.NewUserServiceClient(userConnection)))
	accessService := creator.NewAccessService(repository)
	mediaReadiness := postgres_repository.NewMediaReadinessRepository(databasePool)
	mediaAssets := content_media.NewProjectedStore(
		mediaReadiness, content_media.NewClient(mediav1.NewMediaServiceClient(mediaConnection)),
	)
	rightsService := rights.NewService(
		postgres_repository.NewRightsRepository(databasePool), accessService,
	)
	submissionService := submission.NewService(
		postgres_repository.NewSubmissionRepository(databasePool), accessService,
		mediaAssets,
	)
	moderationService := moderation.NewService(
		postgres_repository.NewModerationRepository(databasePool, cfg.Events.ReleaseApprovedTopic),
	)
	catalogClient := content_catalog.NewClient(catalogv1.NewCatalogServiceClient(catalogConnection))
	publicationService := publication.NewService(
		postgres_repository.NewPublicationRepository(databasePool), accessService,
		mediaAssets,
		catalogClient,
	)
	releaseService := release.NewService(
		postgres_repository.NewReleaseRepository(databasePool), memberService, catalogClient,
	)
	uploadService := upload.NewService(
		postgres_repository.NewUploadRepository(databasePool), memberService,
	)
	playbackService := playback.NewService(
		postgres_repository.NewPlaybackRepository(databasePool),
	)

	httpServer := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Database: databasePool, Tokens: tokenVerifier, Creators: creatorService,
			Members: memberService, Rights: rightsService,
			Submissions:  submissionService,
			Moderation:   moderationService,
			Publications: publicationService,
			Releases:     releaseService,
			Origins:      cfg.Origins, Logger: logger,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		logger.Error("failed to listen for gRPC", "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	contentv1.RegisterContentServiceServer(
		grpcServer, grpcapi.NewContentServer(accessService, uploadService, playbackService),
	)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dispatcher := events.NewDispatcher(
		postgres_repository.NewOutboxRepository(databasePool), producer, logger,
		cfg.Events.OutboxInterval, cfg.Events.OutboxBatch,
	)
	go dispatcher.Run(ctx)
	mediaConsumer, err := event_kafka.NewConsumer(
		cfg.Events.Brokers, cfg.Events.MediaReadyTopic, cfg.Events.MediaConsumerGroup,
		cfg.Events.DeadLetterTopic,
		events.NewMediaHandler(mediaReadiness), logger,
	)
	if err != nil {
		logger.Error("failed to initialize Media Kafka consumer", "error", err)
		os.Exit(1)
	}
	defer mediaConsumer.Close()
	go mediaConsumer.Run(ctx)
	serverErrors := make(chan error, 2)
	go func() {
		logger.Info("content service started", "address", cfg.HTTPAddress)
		serverErrors <- httpServer.ListenAndServe()
	}()
	go func() {
		logger.Info("content gRPC service started", "address", cfg.GRPCAddress)
		serverErrors <- grpcServer.Serve(grpcListener)
	}()

	select {
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("content service stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutting down content service")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	if err = httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shut down content service", "error", err)
		os.Exit(1)
	}
	grpcStopped := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(grpcStopped) }()
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	logger.Info("content service stopped")
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
