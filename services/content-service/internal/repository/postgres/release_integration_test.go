//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReleaseRepositoryCreatesAndLoadsOrderedDraft(t *testing.T) {
	pool := openIntegrationDatabase(t)
	creatorRepository := NewCreatorRepository(pool)
	releaseRepository := NewReleaseRepository(pool)
	ctx := context.Background()
	creatorID, ownerID, releaseID := uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })

	if _, err := creatorRepository.Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeArtist, DisplayName: "Draft Artist",
		VerificationStatus: creator.VerificationUnverified,
	}, creator.Member{
		CreatorID: creatorID, UserID: ownerID, Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator account: %v", err)
	}
	draft := release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Two Sides",
			Type:        release.TypeAlbum,
			ReleaseDate: time.Date(2027, time.September, 4, 0, 0, 0, 0, time.UTC),
			Status:      release.StatusDraft,
		},
		Tracks: []release.Track{
			{ID: uuid.New(), ReleaseID: releaseID, Title: "Second Disc", DiscNumber: 2, TrackNumber: 1},
			{ID: uuid.New(), ReleaseID: releaseID, Title: "Opening", DiscNumber: 1, TrackNumber: 1},
			{ID: uuid.New(), ReleaseID: releaseID, Title: "Closing", DiscNumber: 1, TrackNumber: 2, Explicit: true},
		},
	}
	created, err := releaseRepository.Create(ctx, draft)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Release.CreatedAt.IsZero() || created.Release.UpdatedAt.IsZero() ||
		len(created.Tracks) != 3 {
		t.Fatalf("Create() = %+v", created)
	}
	loaded, err := releaseRepository.ByID(ctx, releaseID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if len(loaded.Tracks) != 3 || loaded.Tracks[0].Title != "Opening" ||
		loaded.Tracks[1].Title != "Closing" || loaded.Tracks[2].Title != "Second Disc" {
		t.Fatalf("ByID() tracks = %+v", loaded.Tracks)
	}
}

func TestReleaseRepositoryRollsBackInvalidTrackPositions(t *testing.T) {
	pool := openIntegrationDatabase(t)
	creatorRepository := NewCreatorRepository(pool)
	releaseRepository := NewReleaseRepository(pool)
	ctx := context.Background()
	creatorID, releaseID := uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	if _, err := creatorRepository.Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeLabel, DisplayName: "Rollback Label",
		VerificationStatus: creator.VerificationUnverified,
	}, creator.Member{
		CreatorID: creatorID, UserID: uuid.New(), Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator account: %v", err)
	}
	_, err := releaseRepository.Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Invalid Positions",
			Type: release.TypeEP, ReleaseDate: time.Now(), Status: release.StatusDraft,
		},
		Tracks: []release.Track{
			{ID: uuid.New(), ReleaseID: releaseID, Title: "One", DiscNumber: 1, TrackNumber: 1},
			{ID: uuid.New(), ReleaseID: releaseID, Title: "Duplicate", DiscNumber: 1, TrackNumber: 1},
		},
	})
	if !errors.Is(err, release.ErrInvalid) {
		t.Fatalf("Create() error = %v, want %v", err, release.ErrInvalid)
	}
	if _, err = releaseRepository.ByID(ctx, releaseID); !errors.Is(err, release.ErrNotFound) {
		t.Fatalf("ByID() error = %v, want %v", err, release.ErrNotFound)
	}
}

func cleanupReleaseAggregate(
	t *testing.T,
	pool *pgxpool.Pool,
	creatorID uuid.UUID,
	releaseID uuid.UUID,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `DELETE FROM content.releases WHERE id = $1`, releaseID); err != nil {
		t.Errorf("cleanup release: %v", err)
	}
	cleanupCreatorAccount(t, pool, creatorID)
}
