package config

import (
	"testing"
	"time"
)

func TestLoadUsesRecommendationDatabaseEnvironment(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "recommendation_db")
	t.Setenv("RECOMMENDATION_HTTP_HOST", "127.0.0.1")
	t.Setenv("RECOMMENDATION_HTTP_PORT", "9080")
	t.Setenv("USER_GRPC_HOST", "user-service")
	t.Setenv("INTERACTION_GRPC_HOST", "interaction-service")
	t.Setenv("HISTORY_GRPC_HOST", "history-service")
	t.Setenv("CATALOG_GRPC_HOST", "catalog-service")
	t.Setenv("CONTENT_GRPC_HOST", "content-service")
	t.Setenv("RECOMMENDATION_RUN_TTL", "45m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddress != "127.0.0.1:9080" {
		t.Fatalf("HTTPAddress = %q", cfg.HTTPAddress)
	}
	if cfg.Database.User != "recommendation_user" ||
		cfg.Database.Name != "recommendation_db" {
		t.Fatalf("Database = %+v", cfg.Database)
	}
	if cfg.UserAddress != "user-service:9090" || cfg.InteractionAddress != "interaction-service:9090" || cfg.HistoryAddress != "history-service:9090" || cfg.CatalogAddress != "catalog-service:9090" || cfg.ContentAddress != "content-service:9090" {
		t.Fatalf("gRPC addresses = %q, %q, %q, %q, %q", cfg.UserAddress, cfg.InteractionAddress, cfg.HistoryAddress, cfg.CatalogAddress, cfg.ContentAddress)
	}
	if cfg.RunTTL != 45*time.Minute {
		t.Fatalf("RunTTL = %s", cfg.RunTTL)
	}
}

func TestLoadRejectsInvalidRunTTL(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "recommendation_db")
	t.Setenv("RECOMMENDATION_RUN_TTL", "0s")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestLoadRequiresRecommendationDatabase(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing database error")
	}
}
