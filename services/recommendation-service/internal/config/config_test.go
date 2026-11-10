package config

import "testing"

func TestLoadUsesRecommendationDatabaseEnvironment(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "recommendation_db")
	t.Setenv("RECOMMENDATION_HTTP_HOST", "127.0.0.1")
	t.Setenv("RECOMMENDATION_HTTP_PORT", "9080")
	t.Setenv("USER_GRPC_HOST", "user-service")
	t.Setenv("INTERACTION_GRPC_HOST", "interaction-service")
	t.Setenv("HISTORY_GRPC_HOST", "history-service")

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
	if cfg.UserAddress != "user-service:9090" || cfg.InteractionAddress != "interaction-service:9090" || cfg.HistoryAddress != "history-service:9090" {
		t.Fatalf("gRPC addresses = %q, %q, %q", cfg.UserAddress, cfg.InteractionAddress, cfg.HistoryAddress)
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
