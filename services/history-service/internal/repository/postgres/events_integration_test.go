//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/history-service/internal/history"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRecordIsIdempotentAndRejectsChangedPayload(t *testing.T) {
	pool := openDatabase(t)
	repository := NewEventRepository(pool)
	finishedAt := time.Now().UTC().Truncate(time.Microsecond)
	event := history.Event{ID: uuid.New(), UserID: uuid.New(), TrackID: uuid.New(), StartedAt: finishedAt.Add(-time.Minute), FinishedAt: &finishedAt, ListenedMS: 60000, Completed: false, ContextType: "direct", DeviceType: "web"}
	if _, err := repository.Record(context.Background(), event); err != nil {
		t.Fatalf("first Record: %v", err)
	}
	if _, err := repository.Record(context.Background(), event); err != nil {
		t.Fatalf("idempotent Record: %v", err)
	}
	changed := event
	changed.ListenedMS++
	if _, err := repository.Record(context.Background(), changed); !errors.Is(err, history.ErrEventConflict) {
		t.Fatalf("changed Record error = %v", err)
	}
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM history.listening_events WHERE id=$1", event.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("event count = %d, want 1", count)
	}
}

func openDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1")
	}
	port, err := strconv.ParseUint(env("POSTGRES_PORT", "5432"), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Host = env("POSTGRES_HOST", "127.0.0.1")
	cfg.ConnConfig.Port = uint16(port)
	cfg.ConnConfig.User = os.Getenv("POSTGRES_USER")
	cfg.ConnConfig.Password = os.Getenv("POSTGRES_PASSWORD")
	cfg.ConnConfig.Database = os.Getenv("POSTGRES_HISTORY_DB")
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
