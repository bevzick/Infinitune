package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/config"
	"github.com/bevzick/Infinitune/services/auth-service/internal/httpapi"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httpapi.NewRouter(),
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
