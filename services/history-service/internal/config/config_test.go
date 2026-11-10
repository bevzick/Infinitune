package config

import "testing"

func TestLoadUsesHistoryDatabaseEnvironment(t *testing.T) {
	t.Setenv("POSTGRES_USER", "history_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_HISTORY_DB", "history_db")
	t.Setenv("HISTORY_HTTP_HOST", "127.0.0.1")
	t.Setenv("HISTORY_HTTP_PORT", "9080")
	t.Setenv("CATALOG_GRPC_HOST", "catalog")
	t.Setenv("CATALOG_GRPC_PORT", "9091")
	t.Setenv("HISTORY_ALLOWED_ORIGINS", "https://app.example.test")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddress != "127.0.0.1:9080" {
		t.Fatalf("HTTPAddress = %q", cfg.HTTPAddress)
	}
	if cfg.Database.User != "history_user" || cfg.Database.Name != "history_db" {
		t.Fatalf("Database = %+v", cfg.Database)
	}
	if cfg.CatalogAddress != "catalog:9091" || len(cfg.Origins) != 1 {
		t.Fatalf("Catalog/origins = %q/%v", cfg.CatalogAddress, cfg.Origins)
	}
}

func TestLoadRequiresHistoryDatabase(t *testing.T) {
	t.Setenv("POSTGRES_USER", "history_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_HISTORY_DB", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing database error")
	}
}
