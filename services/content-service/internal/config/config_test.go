package config

import (
	"testing"
	"time"
)

func TestLoadUsesContentDatabase(t *testing.T) {
	t.Setenv("CONTENT_HTTP_HOST", "127.0.0.1")
	t.Setenv("CONTENT_HTTP_PORT", "8081")
	t.Setenv("CONTENT_GRPC_HOST", "127.0.0.2")
	t.Setenv("CONTENT_GRPC_PORT", "9091")
	t.Setenv("USER_GRPC_HOST", "users")
	t.Setenv("USER_GRPC_PORT", "9092")
	t.Setenv("MEDIA_GRPC_HOST", "media")
	t.Setenv("MEDIA_GRPC_PORT", "9093")
	t.Setenv("CATALOG_GRPC_HOST", "catalog")
	t.Setenv("CATALOG_GRPC_PORT", "9094")
	t.Setenv("CONTENT_ALLOWED_ORIGINS", "https://studio.example")
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "content_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_CONTENT_DB", "content_db")
	t.Setenv("KAFKA_BROKERS", "kafka:19092,kafka-backup:29092")
	t.Setenv("KAFKA_MEDIA_ASSET_READY_TOPIC", "media-ready-test")
	t.Setenv("KAFKA_CONTENT_MEDIA_GROUP", "content-media-test")
	t.Setenv("KAFKA_CONTENT_RELEASE_APPROVED_TOPIC", "content-approved-test")
	t.Setenv("CONTENT_OUTBOX_POLL_INTERVAL", "2s")
	t.Setenv("CONTENT_OUTBOX_BATCH_SIZE", "25")
	t.Setenv("KAFKA_DEAD_LETTER_TOPIC", "events-dlq-test")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.HTTPAddress != "127.0.0.1:8081" {
		t.Errorf("HTTPAddress = %q", config.HTTPAddress)
	}
	if config.GRPCAddress != "127.0.0.2:9091" || config.UserAddress != "users:9092" {
		t.Errorf("GRPCAddress = %q, UserAddress = %q", config.GRPCAddress, config.UserAddress)
	}
	if config.MediaAddress != "media:9093" {
		t.Errorf("MediaAddress = %q", config.MediaAddress)
	}
	if config.CatalogAddress != "catalog:9094" {
		t.Errorf("CatalogAddress = %q", config.CatalogAddress)
	}
	if len(config.Origins) != 1 || config.Origins[0] != "https://studio.example" {
		t.Errorf("Origins = %v", config.Origins)
	}
	if config.Database.Host != "postgres" || config.Database.Port != 5433 ||
		config.Database.User != "content_user" || config.Database.Password != "secret" ||
		config.Database.Name != "content_db" {
		t.Errorf("Database = %+v", config.Database)
	}
	if len(config.Events.Brokers) != 2 || config.Events.MediaReadyTopic != "media-ready-test" ||
		config.Events.MediaConsumerGroup != "content-media-test" ||
		config.Events.ReleaseApprovedTopic != "content-approved-test" ||
		config.Events.OutboxInterval != 2*time.Second || config.Events.OutboxBatch != 25 ||
		config.Events.DeadLetterTopic != "events-dlq-test" {
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
		{name: "invalid HTTP port", key: "CONTENT_HTTP_PORT", value: "invalid"},
		{name: "invalid gRPC port", key: "CONTENT_GRPC_PORT", value: "0"},
		{name: "invalid user gRPC port", key: "USER_GRPC_PORT", value: "70000"},
		{name: "invalid media gRPC port", key: "MEDIA_GRPC_PORT", value: "0"},
		{name: "invalid catalog gRPC port", key: "CATALOG_GRPC_PORT", value: "0"},
		{name: "invalid allowed origin", key: "CONTENT_ALLOWED_ORIGINS", value: "localhost:5173"},
		{name: "invalid database port", key: "POSTGRES_PORT", value: "0"},
		{name: "missing database user", key: "POSTGRES_USER", value: ""},
		{name: "missing database password", key: "POSTGRES_PASSWORD", value: ""},
		{name: "missing database name", key: "POSTGRES_CONTENT_DB", value: ""},
		{name: "invalid Kafka broker", key: "KAFKA_BROKERS", value: "kafka"},
		{name: "blank Kafka group", key: "KAFKA_CONTENT_MEDIA_GROUP", value: " "},
		{name: "invalid outbox batch", key: "CONTENT_OUTBOX_BATCH_SIZE", value: "0"},
		{name: "blank dead-letter topic", key: "KAFKA_DEAD_LETTER_TOPIC", value: " "},
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
