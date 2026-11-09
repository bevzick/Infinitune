package config

import "testing"

func TestLoadUsesContentDatabase(t *testing.T) {
	t.Setenv("CONTENT_HTTP_HOST", "127.0.0.1")
	t.Setenv("CONTENT_HTTP_PORT", "8081")
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "content_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_CONTENT_DB", "content_db")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.HTTPAddress != "127.0.0.1:8081" {
		t.Errorf("HTTPAddress = %q", config.HTTPAddress)
	}
	if config.Database.Host != "postgres" || config.Database.Port != 5433 ||
		config.Database.User != "content_user" || config.Database.Password != "secret" ||
		config.Database.Name != "content_db" {
		t.Errorf("Database = %+v", config.Database)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	setRequiredEnvironment(t)
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "invalid HTTP port", key: "CONTENT_HTTP_PORT", value: "invalid"},
		{name: "invalid database port", key: "POSTGRES_PORT", value: "0"},
		{name: "missing database user", key: "POSTGRES_USER", value: ""},
		{name: "missing database password", key: "POSTGRES_PASSWORD", value: ""},
		{name: "missing database name", key: "POSTGRES_CONTENT_DB", value: ""},
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
	t.Setenv("CONTENT_HTTP_PORT", "8080")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "content_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_CONTENT_DB", "content_db")
}
