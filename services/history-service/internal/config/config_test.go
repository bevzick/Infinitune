package config

import "testing"

func TestLoadUsesHistoryDatabaseEnvironment(t *testing.T) {
	t.Setenv("POSTGRES_USER", "history_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_HISTORY_DB", "history_db")
	t.Setenv("HISTORY_HTTP_HOST", "127.0.0.1")
	t.Setenv("HISTORY_HTTP_PORT", "9080")
	t.Setenv("HISTORY_GRPC_HOST", "127.0.0.1")
	t.Setenv("HISTORY_GRPC_PORT", "9095")
	t.Setenv("CATALOG_GRPC_HOST", "catalog")
	t.Setenv("CATALOG_GRPC_PORT", "9091")
	t.Setenv("HISTORY_ALLOWED_ORIGINS", "https://app.example.test")
	t.Setenv("KAFKA_BROKERS", "kafka-a:9092,kafka-b:9092")
	t.Setenv("KAFKA_HISTORY_TOPIC", "history-events")
	t.Setenv("HISTORY_OUTBOX_POLL_INTERVAL", "250ms")
	t.Setenv("HISTORY_OUTBOX_BATCH_SIZE", "25")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddress != "127.0.0.1:9080" || cfg.GRPCAddress != "127.0.0.1:9095" {
		t.Fatalf("HTTPAddress/GRPCAddress = %q/%q", cfg.HTTPAddress, cfg.GRPCAddress)
	}
	if cfg.Database.User != "history_user" || cfg.Database.Name != "history_db" {
		t.Fatalf("Database = %+v", cfg.Database)
	}
	if cfg.CatalogAddress != "catalog:9091" || len(cfg.Origins) != 1 {
		t.Fatalf("Catalog/origins = %q/%v", cfg.CatalogAddress, cfg.Origins)
	}
	if len(cfg.Events.Brokers) != 2 || cfg.Events.Brokers[1] != "kafka-b:9092" ||
		cfg.Events.HistoryTopic != "history-events" ||
		cfg.Events.OutboxInterval.String() != "250ms" || cfg.Events.OutboxBatch != 25 {
		t.Fatalf("Events = %+v", cfg.Events)
	}
}

func TestLoadRejectsInvalidKafkaAndOutboxConfiguration(t *testing.T) {
	t.Setenv("POSTGRES_USER", "history_user")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_HISTORY_DB", "history_db")
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "broker", key: "KAFKA_BROKERS", value: "missing-port"},
		{name: "interval", key: "HISTORY_OUTBOX_POLL_INTERVAL", value: "never"},
		{name: "batch", key: "HISTORY_OUTBOX_BATCH_SIZE", value: "1001"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil for %s=%q", test.key, test.value)
			}
		})
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
