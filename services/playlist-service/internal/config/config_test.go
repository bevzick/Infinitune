package config

import (
	"slices"
	"testing"
)

func TestLoadUsesPlaylistDatabase(t *testing.T) {
	t.Setenv("PLAYLIST_HTTP_HOST", "127.0.0.1")
	t.Setenv("PLAYLIST_HTTP_PORT", "8081")
	t.Setenv("CATALOG_GRPC_HOST", "catalog")
	t.Setenv("CATALOG_GRPC_PORT", "9091")
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "playlist_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_PLAYLIST_DB", "playlist_db")
	t.Setenv("PLAYLIST_AUTH_JWKS_URL", "http://auth:8080/v1/auth/jwks")
	t.Setenv("AUTH_JWT_ISSUER", "test-issuer")
	t.Setenv("AUTH_JWT_AUDIENCE", "test-audience")
	t.Setenv("AUTH_JWT_KEY_ID", "test-key")
	t.Setenv("PLAYLIST_ALLOWED_ORIGINS", "http://localhost:5173, https://app.example.test")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.HTTPAddress != "127.0.0.1:8081" {
		t.Errorf("HTTPAddress = %q", config.HTTPAddress)
	}
	if config.CatalogAddress != "catalog:9091" {
		t.Errorf("CatalogAddress = %q", config.CatalogAddress)
	}
	if config.Database.Host != "postgres" || config.Database.Port != 5433 ||
		config.Database.User != "playlist_user" || config.Database.Password != "secret" ||
		config.Database.Name != "playlist_db" {
		t.Errorf("Database = %+v", config.Database)
	}
	if config.Identity.JWKSURL != "http://auth:8080/v1/auth/jwks" ||
		config.Identity.Issuer != "test-issuer" || config.Identity.Audience != "test-audience" ||
		config.Identity.KeyID != "test-key" {
		t.Errorf("Identity = %+v", config.Identity)
	}
	if !slices.Equal(config.Origins, []string{"http://localhost:5173", "https://app.example.test"}) {
		t.Errorf("Origins = %v", config.Origins)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	setRequiredEnvironment(t)
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "invalid HTTP port", key: "PLAYLIST_HTTP_PORT", value: "invalid"},
		{name: "invalid Catalog port", key: "CATALOG_GRPC_PORT", value: "65536"},
		{name: "invalid database port", key: "POSTGRES_PORT", value: "0"},
		{name: "missing database user", key: "POSTGRES_USER", value: ""},
		{name: "missing database password", key: "POSTGRES_PASSWORD", value: ""},
		{name: "missing database name", key: "POSTGRES_PLAYLIST_DB", value: ""},
		{name: "invalid allowed origin", key: "PLAYLIST_ALLOWED_ORIGINS", value: "javascript:alert(1)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil")
			}
		})
	}
}

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("PLAYLIST_HTTP_PORT", "8080")
	t.Setenv("CATALOG_GRPC_PORT", "9090")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "playlist_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_PLAYLIST_DB", "playlist_db")
}
