package config

import "testing"

func TestLoadUsesRecommendationDatabaseEnvironment(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "recommendation_db")
	t.Setenv("RECOMMENDATION_HTTP_HOST", "127.0.0.1")
	t.Setenv("RECOMMENDATION_HTTP_PORT", "9080")

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
}

func TestLoadRequiresRecommendationDatabase(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing database error")
	}
}
