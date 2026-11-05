package config

import "testing"

func TestLoadUsesCatalogDatabase(t *testing.T) {
	t.Setenv("POSTGRES_USER", "catalog_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_CATALOG_DB", "catalog_db")
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("CATALOG_HTTP_HOST", "127.0.0.1")
	t.Setenv("CATALOG_HTTP_PORT", "8081")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.HTTPAddress != "127.0.0.1:8081" {
		t.Errorf("HTTPAddress = %q, want %q", config.HTTPAddress, "127.0.0.1:8081")
	}
	if config.Database.Name != "catalog_db" || config.Database.User != "catalog_user" ||
		config.Database.Host != "postgres" || config.Database.Port != 5433 {
		t.Errorf("Database = %+v", config.Database)
	}
}

func TestLoadRequiresDatabaseCredentials(t *testing.T) {
	t.Setenv("POSTGRES_USER", "")
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("POSTGRES_CATALOG_DB", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestParsePort(t *testing.T) {
	t.Parallel()

	if port, err := parsePort("PORT", "8080"); err != nil || port != 8080 {
		t.Fatalf("parsePort(8080) = %d, %v", port, err)
	}
	for _, value := range []string{"0", "65536", "invalid"} {
		if _, err := parsePort("PORT", value); err == nil {
			t.Fatalf("parsePort(%q) error = nil", value)
		}
	}
}
