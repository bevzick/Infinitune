package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	configuration, err := parseConfig(os.Args[1:], os.LookupEnv, os.Stderr)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	summary, err := replay(ctx, configuration, logger)
	if err != nil {
		logger.Error("dead-letter replay failed", "inspected", summary.inspected, "replayed", summary.replayed, "error", err)
		os.Exit(1)
	}
	logger.Info("dead-letter replay completed", "execute", configuration.execute, "inspected", summary.inspected, "replayed", summary.replayed)
}
