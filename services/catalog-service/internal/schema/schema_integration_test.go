//go:build integration

package schema

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCatalogSchemaSupportsSingleAndStandaloneTrack(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	artistID := uuid.New()
	albumID := uuid.New()
	trackID := uuid.New()
	genreID := uuid.New()
	moodID := uuid.New()

	_, err := tx.Exec(ctx, `
		INSERT INTO catalog.artists (id, name)
		VALUES ($1, 'Integration Artist')
	`, artistID)
	if err != nil {
		t.Fatalf("insert artist: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.albums (id, title, album_type, release_date)
		VALUES ($1, 'Integration Single', 'single', '2026-01-02')
	`, albumID)
	if err != nil {
		t.Fatalf("insert single album: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.album_artists (album_id, artist_id, position)
		VALUES ($1, $2, 1)
	`, albumID, artistID)
	if err != nil {
		t.Fatalf("link album artist: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.tracks (
			id, album_id, title, duration_ms, track_number, release_date,
			media_asset_id, content_id
		)
		VALUES ($1, NULL, 'Standalone Track', 180000, 1, '2026-01-02', $2, $3)
	`, trackID, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("insert standalone track with cross-service IDs: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.track_artists (track_id, artist_id, artist_role, position)
		VALUES ($1, $2, 'primary', 1)
	`, trackID, artistID)
	if err != nil {
		t.Fatalf("link track artist: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.genres (id, name, slug)
		VALUES ($1, 'Integration Genre', 'integration-genre')
	`, genreID)
	if err != nil {
		t.Fatalf("insert genre: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.moods (id, name, slug)
		VALUES ($1, 'Integration Mood', 'integration-mood')
	`, moodID)
	if err != nil {
		t.Fatalf("insert mood: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.track_genres (track_id, genre_id) VALUES ($1, $2)
	`, trackID, genreID)
	if err != nil {
		t.Fatalf("link track genre: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.track_moods (track_id, mood_id, weight) VALUES ($1, $2, 0.750)
	`, trackID, moodID)
	if err != nil {
		t.Fatalf("link track mood: %v", err)
	}

	var albumReference *uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT album_id FROM catalog.tracks WHERE id = $1`, trackID).
		Scan(&albumReference); err != nil {
		t.Fatalf("read track: %v", err)
	}
	if albumReference != nil {
		t.Fatalf("album_id = %s, want NULL", *albumReference)
	}
}

func TestCatalogSchemaConstraints(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, pgx.Tx) error
	}{
		{
			name: "blank artist name",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO catalog.artists (id, name) VALUES ($1, '   ')`, uuid.New())
				return err
			},
		},
		{
			name: "duplicate genre name ignoring case",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO catalog.genres (id, name, slug) VALUES
						($1, 'Synthwave', 'synthwave'),
						($2, 'SYNTHWAVE', 'retro-wave')
				`, uuid.New(), uuid.New())
				return err
			},
		},
		{
			name: "duplicate mood slug",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO catalog.moods (id, name, slug) VALUES
						($1, 'Focused', 'focused'),
						($2, 'Concentrated', 'focused')
				`, uuid.New(), uuid.New())
				return err
			},
		},
		{
			name: "invalid genre slug",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO catalog.genres (id, name, slug)
					VALUES ($1, 'Invalid Genre', 'Invalid Genre')
				`, uuid.New())
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := beginIntegrationTransaction(t)
			if err := test.run(context.Background(), tx); err == nil {
				t.Fatal("constraint accepted invalid data")
			}
		})
	}
}

func TestCatalogSchemaUsesCorrectedColumnNames(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()

	for table, columns := range map[string][]string{
		"albums":        {"updated_at"},
		"track_artists": {"artist_id", "artist_role"},
	} {
		for _, column := range columns {
			var exists bool
			err := pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM information_schema.columns
					WHERE table_schema = 'catalog' AND table_name = $1 AND column_name = $2
				)
			`, table, column).Scan(&exists)
			if err != nil {
				t.Fatalf("check %s.%s: %v", table, column, err)
			}
			if !exists {
				t.Errorf("catalog.%s.%s does not exist", table, column)
			}
		}
	}
}

func TestCatalogSearchIndexesExist(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()

	for _, index := range []string{
		"tracks_title_trgm_idx",
		"tracks_title_order_idx",
		"tracks_release_date_title_idx",
		"tracks_explicit_release_date_idx",
	} {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE schemaname = 'catalog' AND indexname = $1
			)
		`, index).Scan(&exists)
		if err != nil {
			t.Fatalf("check index %s: %v", index, err)
		}
		if !exists {
			t.Errorf("index catalog.%s does not exist", index)
		}
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
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
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
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_CATALOG_DB")

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
