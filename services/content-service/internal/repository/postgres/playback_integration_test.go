//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/rights"
	"github.com/google/uuid"
)

func TestPlaybackRepositoryRequiresPublishedReleasedTrackAndCurrentStreamingRights(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	creatorID, ownerID, releaseID, trackID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })

	if _, err := NewCreatorRepository(pool).Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeArtist, DisplayName: "Playback Artist",
		VerificationStatus: creator.VerificationVerified,
	}, creator.Member{
		CreatorID: creatorID, UserID: ownerID, Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator: %v", err)
	}
	if _, err := NewReleaseRepository(pool).Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Playback Release",
			Type: release.TypeSingle, ReleaseDate: now.AddDate(0, 0, -1),
			Status: release.StatusDraft,
		},
		Tracks: []release.Track{{
			ID: trackID, ReleaseID: releaseID, Title: "Playable Track",
			DiscNumber: 1, TrackNumber: 1,
		}},
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	if _, err := NewRightsRepository(pool).UpsertRight(ctx, rights.Right{
		ID: uuid.New(), ReleaseTrackID: trackID, TerritoryCode: rights.TerritoryWorld,
		Type: rights.TypeStreaming, ValidFrom: now.AddDate(0, 0, -2),
		GrantedByUserID: ownerID,
	}); err != nil {
		t.Fatalf("create streaming right: %v", err)
	}

	repository := NewPlaybackRepository(pool)
	allowed, err := repository.IsTrackPlayable(ctx, trackID, now)
	if err != nil || allowed {
		t.Fatalf("draft IsTrackPlayable() = %v, %v", allowed, err)
	}

	if _, err = pool.Exec(ctx, `
		UPDATE content.release_tracks SET catalog_track_id = $2 WHERE id = $1
	`, trackID, uuid.New()); err != nil {
		t.Fatalf("link catalog track: %v", err)
	}
	if _, err = pool.Exec(ctx, `
		UPDATE content.releases
		SET status = 'published', published_at = $2, updated_at = $2
		WHERE id = $1
	`, releaseID, now); err != nil {
		t.Fatalf("publish release: %v", err)
	}
	allowed, err = repository.IsTrackPlayable(ctx, trackID, now)
	if err != nil || !allowed {
		t.Fatalf("published IsTrackPlayable() = %v, %v", allowed, err)
	}
	allowed, err = repository.IsTrackPlayable(ctx, trackID, now.AddDate(0, 0, -2))
	if err != nil || allowed {
		t.Fatalf("before release date IsTrackPlayable() = %v, %v", allowed, err)
	}

	allowed, err = repository.IsTrackPlayable(ctx, trackID, now.AddDate(1, 0, 0))
	if err != nil || !allowed {
		t.Fatalf("open-ended right IsTrackPlayable() = %v, %v", allowed, err)
	}
	if _, err = pool.Exec(ctx, `
		UPDATE content.track_rights SET valid_until = $2 WHERE release_track_id = $1
	`, trackID, now); err != nil {
		t.Fatalf("expire streaming right: %v", err)
	}
	allowed, err = repository.IsTrackPlayable(ctx, trackID, now.AddDate(0, 0, 1))
	if err != nil || allowed {
		t.Fatalf("expired IsTrackPlayable() = %v, %v", allowed, err)
	}
}
