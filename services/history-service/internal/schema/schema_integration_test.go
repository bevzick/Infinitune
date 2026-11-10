//go:build integration

package schema

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListeningEventsConstraints(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	eventID, userID, trackID := uuid.New(), uuid.New(), uuid.New()
	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	finishedAt := startedAt.Add(time.Minute)
	_, err := pool.Exec(ctx, `INSERT INTO history.listening_events
		(id,user_id,track_id,started_at,finished_at,listened_ms,completed,context_type,context_id,device_type)
		VALUES ($1,$2,$3,$4,$5,30000,false,'direct',NULL,'web')`, eventID, userID, trackID, startedAt, finishedAt)
	if err != nil {
		t.Fatalf("insert valid event: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO history.listening_events
		(id,user_id,track_id,started_at,listened_ms,completed,context_type,device_type)
		VALUES ($1,$2,$3,$4,-1,false,'direct','web')`, uuid.New(), userID, trackID, startedAt); err == nil {
		t.Fatal("negative listened_ms was accepted")
	}
}

func TestHistorySchemaHasNoCrossDatabaseForeignKeys(t *testing.T) {
	pool := openIntegrationDatabase(t)
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_constraint c JOIN pg_namespace n ON n.oid=c.connamespace WHERE n.nspname='history' AND c.contype='f'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("foreign key count = %d, want 0", count)
	}
}

func openIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1")
	}
	port, err := strconv.ParseUint(envOrDefault("POSTGRES_PORT", "5432"), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Host = envOrDefault("POSTGRES_HOST", "127.0.0.1")
	config.ConnConfig.Port = uint16(port)
	config.ConnConfig.User = requiredEnv(t, "POSTGRES_USER")
	config.ConnConfig.Password = requiredEnv(t, "POSTGRES_PASSWORD")
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_HISTORY_DB")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
func requiredEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}
func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
