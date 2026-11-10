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

func TestRecommendationSchemaConstraints(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	runID, userID := uuid.New(), uuid.New()
	generatedAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err := pool.Exec(ctx, `
		INSERT INTO recommendation.recommendation_runs
			(id, user_id, algorithm_version, generated_at, expires_at)
		VALUES ($1, $2, 'transparent-v1', $3, $4)
	`, runID, userID, generatedAt, generatedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("insert valid recommendation run: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM recommendation.recommendation_items WHERE run_id = $1`, runID)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM recommendation.recommendation_runs WHERE id = $1`, runID)
	})
	if _, err = pool.Exec(ctx, `
		INSERT INTO recommendation.recommendation_items
			(run_id, track_id, score, rank, reason)
		VALUES ($1, $2, 0.9, 1, 'preferred genre')
	`, runID, uuid.New()); err != nil {
		t.Fatalf("insert valid recommendation item: %v", err)
	}
	if _, err = pool.Exec(ctx, `
		INSERT INTO recommendation.recommendation_items
			(run_id, track_id, score, rank, reason)
		VALUES ($1, $2, 0.8, 1, 'duplicate rank')
	`, runID, uuid.New()); err == nil {
		t.Fatal("duplicate rank was accepted")
	}
	if _, err = pool.Exec(ctx, `
		INSERT INTO recommendation.recommendation_runs
			(id, user_id, algorithm_version, generated_at, expires_at)
		VALUES ($1, $2, 'v1', $3, $3)
	`, uuid.New(), userID, generatedAt); err == nil {
		t.Fatal("non-positive run TTL was accepted")
	}
	if _, err = pool.Exec(ctx, `
		INSERT INTO recommendation.recommendation_items
			(run_id, track_id, score, rank, reason)
		VALUES ($1, $2, 1, 1, 'orphan')
	`, uuid.New(), uuid.New()); err == nil {
		t.Fatal("orphan recommendation item was accepted")
	}
}

func TestRecommendationSchemaHasOnlyInternalForeignKeys(t *testing.T) {
	pool := openIntegrationDatabase(t)
	var internalCount, externalCount int
	err := pool.QueryRow(context.Background(), `
		SELECT
			count(*) FILTER (WHERE target_namespace.nspname = 'recommendation'),
			count(*) FILTER (WHERE target_namespace.nspname <> 'recommendation')
		FROM pg_constraint AS constraint_record
		JOIN pg_namespace AS source_namespace
		  ON source_namespace.oid = constraint_record.connamespace
		JOIN pg_class AS target_table
		  ON target_table.oid = constraint_record.confrelid
		JOIN pg_namespace AS target_namespace
		  ON target_namespace.oid = target_table.relnamespace
		WHERE source_namespace.nspname = 'recommendation'
		  AND constraint_record.contype = 'f'
	`).Scan(&internalCount, &externalCount)
	if err != nil {
		t.Fatal(err)
	}
	if internalCount != 1 || externalCount != 0 {
		t.Fatalf("foreign keys internal/external = %d/%d, want 1/0", internalCount, externalCount)
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
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_RECOMMENDATION_DB")
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
