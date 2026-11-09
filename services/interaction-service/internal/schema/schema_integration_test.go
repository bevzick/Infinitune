//go:build integration

package schema

import (
	"context"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInteractionTablesUseCompositePrimaryKeys(t *testing.T) {
	pool := openIntegrationDatabase(t)
	want := map[string][]string{
		"track_likes":    {"user_id", "track_id"},
		"album_likes":    {"user_id", "album_id"},
		"artist_follows": {"user_id", "artist_id"},
		"comment_likes":  {"user_id", "comment_id"},
	}
	for table, wantColumns := range want {
		var columns []string
		err := pool.QueryRow(context.Background(), `
			SELECT array_agg(attribute.attname ORDER BY key.ordinality)
			FROM pg_constraint AS con
			JOIN pg_class AS relation ON relation.oid = con.conrelid
			JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
			CROSS JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS key(number, ordinality)
			JOIN pg_attribute AS attribute
			  ON attribute.attrelid = relation.oid AND attribute.attnum = key.number
			WHERE namespace.nspname = 'interaction'
			  AND relation.relname = $1
			  AND con.contype = 'p'
		`, table).Scan(&columns)
		if err != nil {
			t.Fatalf("read %s primary key: %v", table, err)
		}
		if !reflect.DeepEqual(columns, wantColumns) {
			t.Fatalf("%s primary key = %v, want %v", table, columns, wantColumns)
		}
	}
}

func TestArtistFollowsAllowMultipleArtistsAndRejectDuplicates(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	userID, firstArtistID, secondArtistID := uuid.New(), uuid.New(), uuid.New()
	for _, artistID := range []uuid.UUID{firstArtistID, secondArtistID} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO interaction.artist_follows (user_id, artist_id)
			VALUES ($1, $2)
		`, userID, artistID); err != nil {
			t.Fatalf("insert artist follow: %v", err)
		}
	}
	if _, err := tx.Exec(ctx, "SAVEPOINT duplicate_follow"); err != nil {
		t.Fatalf("create savepoint: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO interaction.artist_follows (user_id, artist_id)
		VALUES ($1, $2)
	`, userID, firstArtistID); err == nil {
		t.Fatal("composite primary key accepted a duplicate artist follow")
	}
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT duplicate_follow"); err != nil {
		t.Fatalf("rollback duplicate insert: %v", err)
	}
	var count int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM interaction.artist_follows WHERE user_id = $1
	`, userID).Scan(&count); err != nil {
		t.Fatalf("count artist follows: %v", err)
	}
	if count != 2 {
		t.Fatalf("artist follow count = %d, want 2", count)
	}
}

func TestInteractionSchemaAcceptsCrossServiceIdentifiersWithoutForeignKeys(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	userID := uuid.New()
	tables := []struct {
		name   string
		target string
	}{
		{name: "track_likes", target: "track_id"},
		{name: "album_likes", target: "album_id"},
		{name: "artist_follows", target: "artist_id"},
		{name: "comment_likes", target: "comment_id"},
	}
	for _, table := range tables {
		query := "INSERT INTO interaction." + table.name +
			" (user_id, " + table.target + ") VALUES ($1, $2) RETURNING created_at"
		var createdAt time.Time
		if err := tx.QueryRow(ctx, query, userID, uuid.New()).Scan(&createdAt); err != nil {
			t.Fatalf("insert into %s: %v", table.name, err)
		}
		if createdAt.IsZero() {
			t.Fatalf("%s created_at is zero", table.name)
		}
	}
	var foreignKeyCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_constraint AS con
		JOIN pg_namespace AS namespace ON namespace.oid = con.connamespace
		WHERE namespace.nspname = 'interaction' AND con.contype = 'f'
	`).Scan(&foreignKeyCount); err != nil {
		t.Fatalf("count foreign keys: %v", err)
	}
	if foreignKeyCount != 0 {
		t.Fatalf("interaction foreign key count = %d, want 0", foreignKeyCount)
	}
}

func beginIntegrationTransaction(t *testing.T) pgx.Tx {
	t.Helper()
	pool := openIntegrationDatabase(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	})
	return tx
}

func openIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	port, err := strconv.ParseUint(envOrDefault("POSTGRES_PORT", "5432"), 10, 16)
	if err != nil {
		t.Fatalf("parse POSTGRES_PORT: %v", err)
	}
	config, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}
	config.ConnConfig.Host = envOrDefault("POSTGRES_HOST", "127.0.0.1")
	config.ConnConfig.Port = uint16(port)
	config.ConnConfig.User = requiredEnv(t, "POSTGRES_USER")
	config.ConnConfig.Password = requiredEnv(t, "POSTGRES_PASSWORD")
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_INTERACTION_DB")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping postgres: %v", err)
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
