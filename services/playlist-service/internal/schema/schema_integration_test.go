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

func TestPlaylistSchemaStoresManualPlaylistWithoutCrossDatabaseForeignKeys(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	playlistID := uuid.New()
	ownerID := uuid.New()
	trackID := uuid.New()

	_, err := tx.Exec(ctx, `
		INSERT INTO playlist.playlists (id, owner_id, name)
		VALUES ($1, $2, 'Integration Playlist')
	`, playlistID, ownerID)
	if err != nil {
		t.Fatalf("insert playlist with cross-service owner UUID: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by)
		VALUES ($1, $2, 1, $3)
	`, playlistID, trackID, ownerID)
	if err != nil {
		t.Fatalf("insert track with cross-service UUIDs: %v", err)
	}

	var playlistType, visibility string
	if err = tx.QueryRow(ctx, `
		SELECT playlist_type, visibility FROM playlist.playlists WHERE id = $1
	`, playlistID).Scan(&playlistType, &visibility); err != nil {
		t.Fatalf("read playlist defaults: %v", err)
	}
	if playlistType != "manual" || visibility != "private" {
		t.Fatalf("playlist defaults = type %q, visibility %q", playlistType, visibility)
	}
}

func TestPlaylistTrackCompositeKeyPreventsDuplicates(t *testing.T) {
	tx := beginIntegrationTransaction(t)
	ctx := context.Background()
	playlistID := uuid.New()
	trackID := uuid.New()
	ownerID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO playlist.playlists (id, owner_id, name)
		VALUES ($1, $2, 'No Duplicates')
	`, playlistID, ownerID)
	if err != nil {
		t.Fatalf("insert playlist: %v", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by)
		VALUES ($1, $2, 1, $3)
	`, playlistID, trackID, ownerID)
	if err != nil {
		t.Fatalf("insert first playlist track: %v", err)
	}
	if _, err = tx.Exec(ctx, "SAVEPOINT duplicate_track"); err != nil {
		t.Fatalf("create savepoint: %v", err)
	}
	_, duplicateErr := tx.Exec(ctx, `
		INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by)
		VALUES ($1, $2, 2, $3)
	`, playlistID, trackID, ownerID)
	if duplicateErr == nil {
		t.Fatal("composite primary key accepted duplicate track")
	}
	if _, err = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT duplicate_track"); err != nil {
		t.Fatalf("rollback duplicate insert: %v", err)
	}
	var count int
	if err = tx.QueryRow(ctx, `
		SELECT count(*) FROM playlist.playlist_tracks
		WHERE playlist_id = $1 AND track_id = $2
	`, playlistID, trackID).Scan(&count); err != nil {
		t.Fatalf("count playlist tracks: %v", err)
	}
	if count != 1 {
		t.Fatalf("playlist track count = %d, want 1", count)
	}
}

func TestPlaylistSchemaConstraints(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, pgx.Tx) error
	}{
		{
			name: "blank playlist name",
			run: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO playlist.playlists (id, owner_id, name)
					VALUES ($1, $2, '   ')
				`, uuid.New(), uuid.New())
				return err
			},
		},
		{
			name: "non-positive position",
			run: func(ctx context.Context, tx pgx.Tx) error {
				playlistID := uuid.New()
				ownerID := uuid.New()
				if _, err := tx.Exec(ctx, `
					INSERT INTO playlist.playlists (id, owner_id, name)
					VALUES ($1, $2, 'Position Check')
				`, playlistID, ownerID); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `
					INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by)
					VALUES ($1, $2, 0, $3)
				`, playlistID, uuid.New(), ownerID)
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
	config.ConnConfig.Database = requiredEnv(t, "POSTGRES_PLAYLIST_DB")
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
