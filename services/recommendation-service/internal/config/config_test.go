package config

import (
	"testing"
	"time"
)

func TestLoadUsesRecommendationDatabaseEnvironment(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "recommendation_db")
	t.Setenv("RECOMMENDATION_HTTP_HOST", "127.0.0.1")
	t.Setenv("RECOMMENDATION_HTTP_PORT", "9080")
	t.Setenv("USER_GRPC_HOST", "user-service")
	t.Setenv("INTERACTION_GRPC_HOST", "interaction-service")
	t.Setenv("HISTORY_GRPC_HOST", "history-service")
	t.Setenv("CATALOG_GRPC_HOST", "catalog-service")
	t.Setenv("CONTENT_GRPC_HOST", "content-service")
	t.Setenv("RECOMMENDATION_RUN_TTL", "45m")
	t.Setenv("RECOMMENDATION_AUTH_JWKS_URL", "https://auth.example.test/v1/auth/jwks")
	t.Setenv("RECOMMENDATION_ALLOWED_ORIGINS", "https://app.example.test")
	t.Setenv("KAFKA_BROKERS", "kafka-a:9092,kafka-b:9092")
	t.Setenv("KAFKA_HISTORY_TOPIC", "history-events")
	t.Setenv("KAFKA_HISTORY_CLEARED_TOPIC", "history-cleared")
	t.Setenv("KAFKA_RECOMMENDATION_GROUP", "recommendation-test")
	t.Setenv("KAFKA_CATALOG_PUBLISHED_TOPIC", "catalog-published")
	t.Setenv("KAFKA_CATALOG_ARCHIVED_TOPIC", "catalog-archived")
	t.Setenv("KAFKA_RECOMMENDATION_CATALOG_GROUP", "recommendation-catalog-test")
	t.Setenv("KAFKA_INTERACTION_PREFERENCE_TOPIC", "interaction-preferences")
	t.Setenv("KAFKA_DEAD_LETTER_TOPIC", "events-dlq-test")
	t.Setenv("KAFKA_RECOMMENDATION_INTERACTION_GROUP", "recommendation-interaction-test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddress != "127.0.0.1:9080" {
		t.Fatalf("HTTPAddress = %q", cfg.HTTPAddress)
	}
	if cfg.GRPCAddress != "0.0.0.0:9090" {
		t.Fatalf("GRPCAddress = %q", cfg.GRPCAddress)
	}
	if cfg.Database.User != "recommendation_user" ||
		cfg.Database.Name != "recommendation_db" {
		t.Fatalf("Database = %+v", cfg.Database)
	}
	if cfg.UserAddress != "user-service:9090" || cfg.InteractionAddress != "interaction-service:9090" || cfg.HistoryAddress != "history-service:9090" || cfg.CatalogAddress != "catalog-service:9090" || cfg.ContentAddress != "content-service:9090" {
		t.Fatalf("gRPC addresses = %q, %q, %q, %q, %q", cfg.UserAddress, cfg.InteractionAddress, cfg.HistoryAddress, cfg.CatalogAddress, cfg.ContentAddress)
	}
	if cfg.RunTTL != 45*time.Minute {
		t.Fatalf("RunTTL = %s", cfg.RunTTL)
	}
	if cfg.Identity.JWKSURL != "https://auth.example.test/v1/auth/jwks" || len(cfg.Origins) != 1 || cfg.Origins[0] != "https://app.example.test" {
		t.Fatalf("identity/origins = %+v, %v", cfg.Identity, cfg.Origins)
	}
	if len(cfg.Events.Brokers) != 2 || cfg.Events.Brokers[1] != "kafka-b:9092" ||
		cfg.Events.HistoryTopic != "history-events" ||
		cfg.Events.HistoryClearedTopic != "history-cleared" ||
		cfg.Events.HistoryConsumerGroup != "recommendation-test" ||
		cfg.Events.CatalogPublishedTopic != "catalog-published" ||
		cfg.Events.CatalogArchivedTopic != "catalog-archived" ||
		cfg.Events.CatalogConsumerGroup != "recommendation-catalog-test" ||
		cfg.Events.InteractionPreferenceTopic != "interaction-preferences" ||
		cfg.Events.InteractionConsumerGroup != "recommendation-interaction-test" ||
		cfg.Events.DeadLetterTopic != "events-dlq-test" {
		t.Fatalf("Events = %+v", cfg.Events)
	}
}

func TestLoadRejectsInvalidKafkaBroker(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "recommendation_db")
	t.Setenv("KAFKA_BROKERS", "missing-port")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestLoadRejectsInvalidRunTTL(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "recommendation_db")
	t.Setenv("RECOMMENDATION_RUN_TTL", "0s")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestLoadRejectsInvalidAllowedOrigin(t *testing.T) {
	t.Setenv("POSTGRES_USER", "recommendation_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_RECOMMENDATION_DB", "recommendation_db")
	t.Setenv("RECOMMENDATION_ALLOWED_ORIGINS", "http://localhost:5173/path")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
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
