package config

import (
	"testing"
	"time"
)

func TestLoadUsesInteractionDatabase(t *testing.T) {
	t.Setenv("INTERACTION_HTTP_HOST", "127.0.0.1")
	t.Setenv("INTERACTION_HTTP_PORT", "8087")
	t.Setenv("INTERACTION_GRPC_HOST", "127.0.0.1")
	t.Setenv("INTERACTION_GRPC_PORT", "9087")
	t.Setenv("CATALOG_GRPC_HOST", "catalog-service")
	t.Setenv("CATALOG_GRPC_PORT", "9091")
	t.Setenv("COMMENT_GRPC_HOST", "comment-service")
	t.Setenv("COMMENT_GRPC_PORT", "9092")
	t.Setenv("INTERACTION_AUTH_JWKS_URL", "http://auth-service:8080/v1/auth/jwks")
	t.Setenv("INTERACTION_ALLOWED_ORIGINS", "http://localhost:5173,https://app.example.test")
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "interaction_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_INTERACTION_DB", "interaction_db")
	t.Setenv("KAFKA_BROKERS", "kafka:19092,kafka-backup:29092")
	t.Setenv("KAFKA_INTERACTION_PREFERENCE_TOPIC", "preferences-test")
	t.Setenv("INTERACTION_OUTBOX_POLL_INTERVAL", "2s")
	t.Setenv("INTERACTION_OUTBOX_BATCH_SIZE", "25")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.HTTPAddress != "127.0.0.1:8087" || config.GRPCAddress != "127.0.0.1:9087" {
		t.Fatalf("HTTPAddress/GRPCAddress = %q/%q", config.HTTPAddress, config.GRPCAddress)
	}
	if config.CatalogAddress != "catalog-service:9091" || config.CommentAddress != "comment-service:9092" ||
		config.Identity.JWKSURL != "http://auth-service:8080/v1/auth/jwks" ||
		len(config.Origins) != 2 {
		t.Fatalf("CatalogAddress = %q, CommentAddress = %q, Identity = %+v, Origins = %v",
			config.CatalogAddress, config.CommentAddress, config.Identity, config.Origins)
	}
	if config.Database.Host != "postgres" || config.Database.Port != 5433 ||
		config.Database.User != "interaction_user" ||
		config.Database.Password != "secret" || config.Database.Name != "interaction_db" {
		t.Fatalf("Database = %+v", config.Database)
	}
	if len(config.Events.Brokers) != 2 || config.Events.PreferenceTopic != "preferences-test" ||
		config.Events.OutboxInterval != 2*time.Second || config.Events.OutboxBatch != 25 {
		t.Fatalf("Events = %+v", config.Events)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	setRequiredEnvironment(t)
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "invalid HTTP port", key: "INTERACTION_HTTP_PORT", value: "0"},
		{name: "invalid gRPC port", key: "INTERACTION_GRPC_PORT", value: "0"},
		{name: "invalid database port", key: "POSTGRES_PORT", value: "invalid"},
		{name: "invalid Catalog port", key: "CATALOG_GRPC_PORT", value: "70000"},
		{name: "invalid Comment port", key: "COMMENT_GRPC_PORT", value: "0"},
		{name: "invalid origin", key: "INTERACTION_ALLOWED_ORIGINS", value: "localhost:5173"},
		{name: "missing database user", key: "POSTGRES_USER", value: ""},
		{name: "missing database password", key: "POSTGRES_PASSWORD", value: ""},
		{name: "missing database name", key: "POSTGRES_INTERACTION_DB", value: ""},
		{name: "invalid Kafka broker", key: "KAFKA_BROKERS", value: "kafka"},
		{name: "invalid outbox batch", key: "INTERACTION_OUTBOX_BATCH_SIZE", value: "0"},
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
	t.Setenv("INTERACTION_HTTP_PORT", "8080")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "interaction_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_INTERACTION_DB", "interaction_db")
}
