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

	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/bevzick/Infinitune/services/auth-service/internal/config"
	"github.com/bevzick/Infinitune/services/auth-service/internal/database"
	"github.com/bevzick/Infinitune/services/auth-service/internal/httpapi"
	"github.com/bevzick/Infinitune/services/auth-service/internal/password"
	postgres_repository "github.com/bevzick/Infinitune/services/auth-service/internal/repository/postgres"
	redis_repository "github.com/bevzick/Infinitune/services/auth-service/internal/repository/redis"
	"github.com/bevzick/Infinitune/services/auth-service/internal/session"
	"github.com/bevzick/Infinitune/services/auth-service/internal/token"
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
	if err != nil {
		startupCancel()
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer databasePool.Close()
	redisClient, err := database.OpenRedis(startupCtx, cfg.Redis)
	startupCancel()
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			logger.Error("failed to close redis client", "error", closeErr)
		}
	}()
	refreshSessions, err := session.NewManager(
		redis_repository.NewRefreshSessionRepository(redisClient),
		cfg.RefreshTTL,
	)
	if err != nil {
		logger.Error("failed to initialize refresh sessions", "error", err)
		os.Exit(1)
	}
	passwordHasher, err := password.NewArgon2ID(cfg.Password)
	if err != nil {
		logger.Error("failed to initialize password hasher", "error", err)
		os.Exit(1)
	}
	registrationService := auth.NewRegistrationService(
		postgres_repository.NewRegistrationRepository(databasePool),
		passwordHasher,
	)
	tokenIssuer, err := token.NewEd25519Issuer(cfg.Token)
	if err != nil {
		logger.Error("failed to initialize JWT issuer", "error", err)
		os.Exit(1)
	}
	loginService, err := auth.NewLoginService(
		postgres_repository.NewLoginRepository(databasePool),
		passwordHasher,
		tokenIssuer,
		refreshSessions,
	)
	if err != nil {
		logger.Error("failed to initialize login service", "error", err)
		os.Exit(1)
	}
	refreshService := auth.NewRefreshService(
		postgres_repository.NewLoginRepository(databasePool),
		tokenIssuer,
		refreshSessions,
	)

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Database:     databasePool,
			Redis:        redisClient,
			Registration: registrationService,
			Login:        loginService,
			Sessions:     refreshService,
			Cookies: httpapi.SessionCookieConfig{
				Secure:   cfg.Cookie.Secure,
				SameSite: cfg.Cookie.SameSite,
			},
			Origins: cfg.Origins,
			Logger:  logger,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("auth service started", "address", cfg.HTTPAddress)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("auth service stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutting down auth service")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err = server.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shut down auth service", "error", err)
		os.Exit(1)
	}

	logger.Info("auth service stopped")
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
