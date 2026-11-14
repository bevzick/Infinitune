package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/bevzick/Infinitune/services/media-service/internal/config"
	"github.com/bevzick/Infinitune/services/media-service/internal/database"
	"github.com/bevzick/Infinitune/services/media-service/internal/devseed"
	"github.com/bevzick/Infinitune/services/media-service/internal/storage"
)

const seedTimeout = 30 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), seedTimeout)
	defer cancel()
	databasePool, err := database.Open(ctx, cfg.Database)
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
	if err = objectStorage.Check(ctx); err != nil {
		logger.Error("failed to connect to object storage", "error", err)
		os.Exit(1)
	}
	if err = devseed.Seed(ctx, databasePool, objectStorage); err != nil {
		logger.Error("failed to seed demo media", "error", err)
		os.Exit(1)
	}
	logger.Info("demo media seed completed")
}
