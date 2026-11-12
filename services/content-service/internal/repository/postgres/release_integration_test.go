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
	status := release.StatusDraft
	owned, err := releaseRepository.ListForUser(ctx, ownerID, release.PageRequest{
		CreatorID: &creatorID, Status: &status, Limit: 10,
	})
	if err != nil || owned.Total != 1 || len(owned.Items) != 1 || owned.Items[0].ID != releaseID {
		t.Fatalf("ListForUser() = %+v, %v", owned, err)
	}
	queue, err := releaseRepository.ListForModeration(ctx, uuid.New(), release.PageRequest{
		Status: &status, Limit: 10,
	})
	if err != nil || queue.Total != 1 || len(queue.Items) != 1 {
		t.Fatalf("ListForModeration() = %+v, %v", queue, err)
	}
	selfQueue, err := releaseRepository.ListForModeration(ctx, ownerID, release.PageRequest{
		Status: &status, Limit: 10,
	})
	if err != nil || selfQueue.Total != 0 || len(selfQueue.Items) != 0 {
		t.Fatalf("ListForModeration(owner) = %+v, %v", selfQueue, err)
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

func TestReleaseRepositoryUpdatesWithdrawsAndArchivesWithoutDeletingTracks(t *testing.T) {
	pool := openIntegrationDatabase(t)
	repository := NewReleaseRepository(pool)
	ctx := context.Background()
	creatorID, releaseID, firstTrackID, secondTrackID :=
		uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	if _, err := NewCreatorRepository(pool).Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeArtist, DisplayName: "Lifecycle Artist",
		VerificationStatus: creator.VerificationVerified,
	}, creator.Member{
		CreatorID: creatorID, UserID: uuid.New(), Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator: %v", err)
	}
	created, err := repository.Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Before",
			Type: release.TypeAlbum, ReleaseDate: time.Now(), Status: release.StatusDraft,
		},
		Tracks: []release.Track{
			{ID: firstTrackID, ReleaseID: releaseID, Title: "First", DiscNumber: 1, TrackNumber: 1},
			{ID: secondTrackID, ReleaseID: releaseID, Title: "Second", DiscNumber: 1, TrackNumber: 2},
		},
	})
	if err != nil {
		t.Fatalf("create release: %v", err)
	}
	created.Release.Title = "After"
	created.Tracks[0].TrackNumber = 2
	created.Tracks[1].TrackNumber = 1
	updated, err := repository.Update(ctx, created)
	if err != nil || updated.Release.Title != "After" || len(updated.Tracks) != 2 {
		t.Fatalf("Update() = %+v, %v", updated, err)
	}
	if _, err = pool.Exec(ctx, `
		UPDATE content.releases
		SET status = 'pending_review', submitted_at = now()
		WHERE id = $1
	`, releaseID); err != nil {
		t.Fatalf("prepare pending release: %v", err)
	}
	withdrawn, err := repository.Withdraw(ctx, releaseID)
	if err != nil || withdrawn.Status != release.StatusDraft || withdrawn.SubmittedAt != nil {
		t.Fatalf("Withdraw() = %+v, %v", withdrawn, err)
	}
	archived, err := repository.Archive(ctx, releaseID)
	if err != nil || archived.Status != release.StatusArchived || archived.ArchivedAt == nil {
		t.Fatalf("Archive() = %+v, %v", archived, err)
	}
	loaded, err := repository.ByID(ctx, releaseID)
	if err != nil || len(loaded.Tracks) != 2 || loaded.Release.Status != release.StatusArchived {
		t.Fatalf("ByID(archived) = %+v, %v", loaded, err)
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
