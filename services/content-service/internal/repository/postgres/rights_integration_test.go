//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/rights"
	"github.com/google/uuid"
)

func TestRightsRepositoryManagesOneIntervalPerTrackTerritoryAndType(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creatorID, ownerID, releaseID, trackID :=
		uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	if _, err := NewCreatorRepository(pool).Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeArtist, DisplayName: "Rights Artist",
		VerificationStatus: creator.VerificationVerified,
	}, creator.Member{
		CreatorID: creatorID, UserID: ownerID, Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator account: %v", err)
	}
	if _, err := NewReleaseRepository(pool).Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Territorial Release",
			Type: release.TypeSingle, ReleaseDate: time.Now(), Status: release.StatusDraft,
		},
		Tracks: []release.Track{{
			ID: trackID, ReleaseID: releaseID, Title: "Territorial Track",
			DiscNumber: 1, TrackNumber: 1,
		}},
	}); err != nil {
		t.Fatalf("create draft release: %v", err)
	}

	repository := NewRightsRepository(pool)
	target, err := repository.TrackTarget(ctx, trackID)
	if err != nil || target.CreatorID != creatorID || target.ReleaseID != releaseID ||
		target.ReleaseStatus != release.StatusDraft {
		t.Fatalf("TrackTarget() = %+v, %v", target, err)
	}
	validUntil := time.Date(2027, time.December, 31, 0, 0, 0, 0, time.UTC)
	first, err := repository.UpsertRight(ctx, rights.Right{
		ID: uuid.New(), ReleaseTrackID: trackID, TerritoryCode: "UA",
		Type:       rights.TypeStreaming,
		ValidFrom:  time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		ValidUntil: &validUntil, GrantedByUserID: ownerID,
	})
	if err != nil {
		t.Fatalf("UpsertRight() error = %v", err)
	}
	replacementID, replacementActor := uuid.New(), uuid.New()
	replaced, err := repository.UpsertRight(ctx, rights.Right{
		ID: replacementID, ReleaseTrackID: trackID, TerritoryCode: "UA",
		Type:            rights.TypeStreaming,
		ValidFrom:       time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC),
		GrantedByUserID: replacementActor,
	})
	if err != nil || replaced.ID != first.ID || replaced.ID == replacementID ||
		replaced.ValidUntil != nil || replaced.GrantedByUserID != replacementActor ||
		replaced.UpdatedAt.Before(first.UpdatedAt) {
		t.Fatalf("UpsertRight(replace) = %+v, %v; first = %+v", replaced, err, first)
	}
	listed, err := repository.ListRights(ctx, trackID)
	if err != nil || len(listed) != 1 || listed[0].ID != first.ID {
		t.Fatalf("ListRights() = %+v, %v", listed, err)
	}
	rightTarget, err := repository.RightTarget(ctx, first.ID)
	if err != nil || rightTarget.Track.CreatorID != creatorID ||
		rightTarget.Right.ReleaseTrackID != trackID {
		t.Fatalf("RightTarget() = %+v, %v", rightTarget, err)
	}

	if _, err = pool.Exec(ctx, `
		UPDATE content.releases SET status = 'pending_review' WHERE id = $1
	`, releaseID); err != nil {
		t.Fatalf("submit release fixture: %v", err)
	}
	if _, err = repository.UpsertRight(ctx, rights.Right{
		ID: uuid.New(), ReleaseTrackID: trackID, TerritoryCode: "US",
		Type: rights.TypeDistribution, ValidFrom: time.Now(),
		GrantedByUserID: ownerID,
	}); !errors.Is(err, rights.ErrImmutable) {
		t.Fatalf("UpsertRight(immutable) error = %v", err)
	}
	if err = repository.DeleteRight(ctx, first.ID); !errors.Is(err, rights.ErrImmutable) {
		t.Fatalf("DeleteRight(immutable) error = %v", err)
	}
	if _, err = pool.Exec(ctx, `
		UPDATE content.releases SET status = 'rejected' WHERE id = $1
	`, releaseID); err != nil {
		t.Fatalf("reject release fixture: %v", err)
	}
	if err = repository.DeleteRight(ctx, first.ID); err != nil {
		t.Fatalf("DeleteRight() error = %v", err)
	}
	if _, err = repository.RightTarget(ctx, first.ID); !errors.Is(err, rights.ErrNotFound) {
		t.Fatalf("RightTarget(deleted) error = %v", err)
	}
}

func TestRightsRepositoryMapsDatabaseConstraints(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creatorID, releaseID, trackID := uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	if _, err := NewCreatorRepository(pool).Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeLabel, DisplayName: "Rights Constraints",
		VerificationStatus: creator.VerificationVerified,
	}, creator.Member{
		CreatorID: creatorID, UserID: uuid.New(), Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator account: %v", err)
	}
	if _, err := NewReleaseRepository(pool).Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Rights Constraints",
			Type: release.TypeSingle, ReleaseDate: time.Now(), Status: release.StatusDraft,
		},
		Tracks: []release.Track{{
			ID: trackID, ReleaseID: releaseID, Title: "Track",
			DiscNumber: 1, TrackNumber: 1,
		}},
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	repository := NewRightsRepository(pool)
	invalid := []rights.Right{
		{ID: uuid.New(), ReleaseTrackID: trackID, TerritoryCode: "UKR",
			Type: rights.TypeStreaming, ValidFrom: time.Now(), GrantedByUserID: uuid.New()},
		{ID: uuid.New(), ReleaseTrackID: trackID, TerritoryCode: "UA",
			Type: "broadcast", ValidFrom: time.Now(), GrantedByUserID: uuid.New()},
	}
	for _, value := range invalid {
		if _, err := repository.UpsertRight(ctx, value); !errors.Is(err, rights.ErrInvalid) {
			t.Fatalf("UpsertRight(%+v) error = %v", value, err)
		}
	}
}
