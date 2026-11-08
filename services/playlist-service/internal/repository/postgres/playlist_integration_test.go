//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/database"
	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPlaylistRepositoryLifecycle(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewPlaylistRepository(pool)
	service := playlist.NewService(repository)
	ctx := context.Background()
	ownerID := uuid.New()

	created, err := service.Create(ctx, ownerID, playlist.CreateInput{
		Name:        "Integration playlist",
		Description: "created by repository integration test",
		Visibility:  playlist.VisibilityUnlisted,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1`, created.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlists WHERE id = $1`, created.ID)
	})
	if created.OwnerID != ownerID || created.Type != playlist.TypeManual ||
		created.Visibility != playlist.VisibilityUnlisted || created.CreatedAt.IsZero() {
		t.Fatalf("Create() = %+v", created)
	}

	read, err := service.ByID(ctx, uuid.Nil, created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if read != created {
		t.Fatalf("ByID() = %+v, want %+v", read, created)
	}

	name := "Updated integration playlist"
	visibility := playlist.VisibilityPrivate
	updated, err := service.Update(ctx, ownerID, created.ID, playlist.UpdateInput{
		Name:       &name,
		Visibility: &visibility,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.OwnerID != ownerID || updated.Name != name ||
		updated.Visibility != playlist.VisibilityPrivate {
		t.Fatalf("Update() = %+v", updated)
	}

	trackID := uuid.New()
	if _, err = pool.Exec(ctx, `
		INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by)
		VALUES ($1, $2, 1, $3)
	`, created.ID, trackID, ownerID); err != nil {
		t.Fatalf("insert playlist track before Delete(): %v", err)
	}
	if err = service.Delete(ctx, ownerID, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err = repository.ByID(ctx, created.ID); !errors.Is(err, playlist.ErrNotFound) {
		t.Fatalf("ByID() after Delete() error = %v, want %v", err, playlist.ErrNotFound)
	}
	var trackCount int
	if err = pool.QueryRow(ctx, `
		SELECT count(*) FROM playlist.playlist_tracks WHERE playlist_id = $1
	`, created.ID).Scan(&trackCount); err != nil {
		t.Fatalf("count tracks after Delete(): %v", err)
	}
	if trackCount != 0 {
		t.Fatalf("track count after Delete() = %d, want 0", trackCount)
	}
}

func TestPlaylistRepositoryRequiresMatchingOwnerForMutation(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewPlaylistRepository(pool)
	ctx := context.Background()
	ownerID := uuid.New()
	created, err := repository.Create(ctx, playlist.Playlist{
		ID: uuid.New(), OwnerID: ownerID, Name: "Owned playlist",
		Type: playlist.TypeManual, Visibility: playlist.VisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1`, created.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM playlist.playlists WHERE id = $1`, created.ID)
	})

	created.OwnerID = uuid.New()
	created.Name = "Unauthorized update"
	trackID := uuid.New()
	if _, err = pool.Exec(ctx, `
		INSERT INTO playlist.playlist_tracks (playlist_id, track_id, position, added_by)
		VALUES ($1, $2, 1, $3)
	`, created.ID, trackID, ownerID); err != nil {
		t.Fatalf("insert playlist track: %v", err)
	}
	if _, err = repository.Update(ctx, created); !errors.Is(err, playlist.ErrNotFound) {
		t.Fatalf("Update() with wrong owner error = %v, want %v", err, playlist.ErrNotFound)
	}
	if err = repository.Delete(ctx, created.ID, created.OwnerID); !errors.Is(err, playlist.ErrNotFound) {
		t.Fatalf("Delete() with wrong owner error = %v, want %v", err, playlist.ErrNotFound)
	}

	read, err := repository.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if read.OwnerID != ownerID || read.Name != "Owned playlist" {
		t.Fatalf("playlist changed after unauthorized mutations: %+v", read)
	}
	var trackCount int
	if err = pool.QueryRow(ctx, `
		SELECT count(*) FROM playlist.playlist_tracks WHERE playlist_id = $1
	`, created.ID).Scan(&trackCount); err != nil {
		t.Fatalf("count tracks after unauthorized Delete(): %v", err)
	}
	if trackCount != 1 {
		t.Fatalf("track count after unauthorized Delete() = %d, want 1", trackCount)
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, database.Config{
		Host:     envOrDefault("POSTGRES_HOST", "127.0.0.1"),
		Port:     uint16(port),
		User:     requiredEnvironment(t, "POSTGRES_USER"),
		Password: requiredEnvironment(t, "POSTGRES_PASSWORD"),
		Name:     requiredEnvironment(t, "POSTGRES_PLAYLIST_DB"),
	})
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func requiredEnvironment(t *testing.T, key string) string {
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
