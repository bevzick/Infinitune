package config

import (
	"slices"
	"testing"
	"time"
)

func TestLoadUsesCatalogDatabase(t *testing.T) {
	t.Setenv("POSTGRES_USER", "catalog_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_CATALOG_DB", "catalog_db")
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("CATALOG_HTTP_HOST", "127.0.0.1")
	t.Setenv("CATALOG_HTTP_PORT", "8081")
	t.Setenv("CATALOG_GRPC_HOST", "127.0.0.2")
	t.Setenv("CATALOG_GRPC_PORT", "9091")
	t.Setenv("CATALOG_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("KAFKA_BROKERS", "kafka-a:9092,kafka-b:9092")
	t.Setenv("KAFKA_CATALOG_PUBLISHED_TOPIC", "catalog-published")
	t.Setenv("KAFKA_CATALOG_ARCHIVED_TOPIC", "catalog-archived")
	t.Setenv("KAFKA_CONTENT_RELEASE_APPROVED_TOPIC", "content-approved")
	t.Setenv("KAFKA_CATALOG_CONTENT_GROUP", "catalog-content-test")
	t.Setenv("CATALOG_OUTBOX_POLL_INTERVAL", "250ms")
	t.Setenv("CATALOG_OUTBOX_BATCH_SIZE", "25")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.HTTPAddress != "127.0.0.1:8081" {
		t.Errorf("HTTPAddress = %q, want %q", config.HTTPAddress, "127.0.0.1:8081")
	}
	if config.GRPCAddress != "127.0.0.2:9091" {
		t.Errorf("GRPCAddress = %q, want %q", config.GRPCAddress, "127.0.0.2:9091")
	}
	if config.Database.Name != "catalog_db" || config.Database.User != "catalog_user" ||
		config.Database.Host != "postgres" || config.Database.Port != 5433 {
		t.Errorf("Database = %+v", config.Database)
	}
	if !slices.Equal(config.Origins, []string{"https://app.example.com"}) {
		t.Errorf("Origins = %v", config.Origins)
	}
	if !slices.Equal(config.Events.Brokers, []string{"kafka-a:9092", "kafka-b:9092"}) ||
		config.Events.PublishedTopic != "catalog-published" ||
		config.Events.ArchivedTopic != "catalog-archived" ||
		config.Events.ContentApprovedTopic != "content-approved" ||
		config.Events.ContentConsumerGroup != "catalog-content-test" ||
		config.Events.OutboxInterval != 250*time.Millisecond || config.Events.OutboxBatch != 25 {
		t.Errorf("Events = %+v", config.Events)
	}
}

func TestLoadRejectsInvalidKafkaConfiguration(t *testing.T) {
	t.Setenv("POSTGRES_USER", "catalog_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_CATALOG_DB", "catalog_db")
	t.Setenv("KAFKA_BROKERS", "missing-port")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
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

func TestParseAllowedOrigins(t *testing.T) {
	t.Parallel()

	origins, err := parseAllowedOrigins(
		"https://app.example.com, http://localhost:5173, https://app.example.com",
	)
	if err != nil {
		t.Fatalf("parseAllowedOrigins() error = %v", err)
	}
	want := []string{"https://app.example.com", "http://localhost:5173"}
	if !slices.Equal(origins, want) {
		t.Fatalf("parseAllowedOrigins() = %v, want %v", origins, want)
	}
}

func TestParseAllowedOriginsRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"javascript:alert(1)",
		"https://user@example.com",
		"https://example.com/path",
		"https://example.com?query=value",
		"https://example.com#fragment",
		"example.com",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			if _, err := parseAllowedOrigins(value); err == nil {
				t.Fatal("parseAllowedOrigins() error = nil")
			}
		})
	}
}
