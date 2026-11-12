package config

import (
	"slices"
	"testing"
	"time"
)

func TestLoadUsesMediaDependencies(t *testing.T) {
	t.Setenv("MEDIA_HTTP_HOST", "127.0.0.1")
	t.Setenv("MEDIA_HTTP_PORT", "8081")
	t.Setenv("MEDIA_GRPC_HOST", "127.0.0.2")
	t.Setenv("MEDIA_GRPC_PORT", "9091")
	t.Setenv("CONTENT_GRPC_HOST", "content")
	t.Setenv("CONTENT_GRPC_PORT", "9092")
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "media_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_MEDIA_DB", "media_db")
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_ROOT_USER", "minio_user")
	t.Setenv("MINIO_ROOT_PASSWORD", "minio_secret")
	t.Setenv("MINIO_MEDIA_BUCKET", "media")
	t.Setenv("MINIO_USE_SSL", "true")
	t.Setenv("MEDIA_AUTH_JWKS_URL", "http://auth:8080/v1/auth/jwks")
	t.Setenv("AUTH_JWT_ISSUER", "test-issuer")
	t.Setenv("AUTH_JWT_AUDIENCE", "test-audience")
	t.Setenv("AUTH_JWT_KEY_ID", "test-key")
	t.Setenv("MEDIA_ALLOWED_ORIGINS", "http://localhost:5173, https://app.example.test")
	t.Setenv("KAFKA_BROKERS", "kafka:19092,kafka-backup:29092")
	t.Setenv("KAFKA_MEDIA_ASSET_READY_TOPIC", "media-ready-test")
	t.Setenv("MEDIA_OUTBOX_POLL_INTERVAL", "2s")
	t.Setenv("MEDIA_OUTBOX_BATCH_SIZE", "25")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.HTTPAddress != "127.0.0.1:8081" {
		t.Errorf("HTTPAddress = %q", config.HTTPAddress)
	}
	if config.GRPCAddress != "127.0.0.2:9091" {
		t.Errorf("GRPCAddress = %q", config.GRPCAddress)
	}
	if config.ContentAddress != "content:9092" {
		t.Errorf("ContentAddress = %q", config.ContentAddress)
	}
	if config.Database.Name != "media_db" || config.Database.User != "media_user" ||
		config.Database.Host != "postgres" || config.Database.Port != 5433 {
		t.Errorf("Database = %+v", config.Database)
	}
	if config.Storage.Endpoint != "minio:9000" || config.Storage.Bucket != "media" ||
		config.Storage.AccessKey != "minio_user" || !config.Storage.Secure {
		t.Errorf("Storage = %+v", config.Storage)
	}
	if config.Identity.JWKSURL != "http://auth:8080/v1/auth/jwks" ||
		config.Identity.Issuer != "test-issuer" || config.Identity.Audience != "test-audience" ||
		config.Identity.KeyID != "test-key" {
		t.Errorf("Identity = %+v", config.Identity)
	}
	if !slices.Equal(config.Origins, []string{"http://localhost:5173", "https://app.example.test"}) {
		t.Errorf("Origins = %v", config.Origins)
	}
	if len(config.Events.Brokers) != 2 || config.Events.AssetReadyTopic != "media-ready-test" ||
		config.Events.OutboxInterval != 2*time.Second || config.Events.OutboxBatch != 25 {
		t.Errorf("Events = %+v", config.Events)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	setRequiredEnvironment(t)
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"invalid HTTP port", "MEDIA_HTTP_PORT", "invalid"},
		{"invalid gRPC port", "MEDIA_GRPC_PORT", "65536"},
		{"invalid content gRPC port", "CONTENT_GRPC_PORT", "0"},
		{"invalid database port", "POSTGRES_PORT", "0"},
		{"missing database user", "POSTGRES_USER", ""},
		{"missing storage password", "MINIO_ROOT_PASSWORD", ""},
		{"endpoint contains scheme", "MINIO_ENDPOINT", "http://minio:9000"},
		{"invalid TLS flag", "MINIO_USE_SSL", "sometimes"},
		{"invalid allowed origin", "MEDIA_ALLOWED_ORIGINS", "javascript:alert(1)"},
		{"invalid Kafka broker", "KAFKA_BROKERS", "kafka"},
		{"invalid outbox batch", "MEDIA_OUTBOX_BATCH_SIZE", "0"},
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
	t.Setenv("MEDIA_HTTP_PORT", "8080")
	t.Setenv("MEDIA_GRPC_PORT", "9090")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "media_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_MEDIA_DB", "media_db")
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_ROOT_USER", "minio_user")
	t.Setenv("MINIO_ROOT_PASSWORD", "minio_secret")
	t.Setenv("MINIO_USE_SSL", "false")
}
