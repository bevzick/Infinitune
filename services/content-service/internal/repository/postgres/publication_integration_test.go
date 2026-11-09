//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	contentcatalog "github.com/bevzick/Infinitune/services/content-service/internal/catalog"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/publication"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/google/uuid"
)

func TestPublicationRepositoryLoadsAndRecordsPublicationIdempotently(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creatorID, ownerID, releaseID := uuid.New(), uuid.New(), uuid.New()
	trackOneID, trackTwoID := uuid.New(), uuid.New()
	uploadOneID, uploadTwoID := uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	if _, err := NewCreatorRepository(pool).Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeArtist, DisplayName: "Publishing Artist",
		VerificationStatus: creator.VerificationVerified,
	}, creator.Member{CreatorID: creatorID, UserID: ownerID, Role: creator.RoleOwner}); err != nil {
		t.Fatalf("create creator: %v", err)
	}
	if _, err := NewReleaseRepository(pool).Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Approved Release",
			Type: release.TypeAlbum, ReleaseDate: time.Date(2027, 5, 6, 0, 0, 0, 0, time.UTC),
			Status: release.StatusDraft,
		},
		Tracks: []release.Track{
			{ID: trackOneID, ReleaseID: releaseID, Title: "One", TrackNumber: 1, DiscNumber: 1},
			{ID: trackTwoID, ReleaseID: releaseID, Title: "Two", TrackNumber: 2, DiscNumber: 1},
		},
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	uploadRepository := NewUploadRepository(pool)
	for uploadID, trackID := range map[uuid.UUID]uuid.UUID{
		uploadOneID: trackOneID, uploadTwoID: trackTwoID,
	} {
		created, _, err := uploadRepository.Create(ctx, upload.Session{
			ID: uploadID, ReleaseTrackID: trackID, RequestedByUserID: ownerID,
			IdempotencyKey: uuid.New(), ObjectKey: "creator-content/" + uploadID.String() + "/source.flac",
			OriginalName: "source.flac", MIMEType: "audio/flac", SizeBytes: 100,
			Status: upload.StatusPending,
		})
		if err != nil {
			t.Fatalf("create upload: %v", err)
		}
		if _, err = uploadRepository.Transition(ctx, created.ID, upload.StatusPending, upload.StatusUploading); err != nil {
			t.Fatalf("start upload: %v", err)
		}
		if _, err = uploadRepository.Transition(ctx, created.ID, upload.StatusUploading, upload.StatusCompleted); err != nil {
			t.Fatalf("complete upload: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE content.releases SET status = 'approved' WHERE id = $1`, releaseID); err != nil {
		t.Fatalf("approve fixture: %v", err)
	}
	repository := NewPublicationRepository(pool)
	candidate, err := repository.PublicationCandidate(ctx, releaseID)
	if err != nil || candidate.CreatorName != "Publishing Artist" ||
		candidate.Release.Status != release.StatusApproved || len(candidate.Tracks) != 2 ||
		candidate.Tracks[0].CompletedUploadID == nil ||
		*candidate.Tracks[0].CompletedUploadID != uploadOneID {
		t.Fatalf("PublicationCandidate() = %+v, %v", candidate, err)
	}
	albumID := uuid.New()
	publishedAt := time.Now().UTC().Truncate(time.Microsecond)
	record := publication.Record{
		ReleaseID: releaseID, AlbumID: &albumID, PublishedAt: publishedAt,
		Tracks: []contentcatalog.PublishedTrack{
			{ContentTrackID: trackOneID, CatalogTrackID: uuid.New()},
			{ContentTrackID: trackTwoID, CatalogTrackID: uuid.New()},
		},
	}
	published, err := repository.MarkPublished(ctx, record)
	if err != nil || published.Status != release.StatusPublished ||
		published.CatalogAlbumID == nil || *published.CatalogAlbumID != albumID ||
		published.PublishedAt == nil || !published.PublishedAt.Equal(publishedAt) {
		t.Fatalf("MarkPublished() = %+v, %v", published, err)
	}
	again, err := repository.MarkPublished(ctx, record)
	if err != nil || again.Status != release.StatusPublished {
		t.Fatalf("MarkPublished(idempotent) = %+v, %v", again, err)
	}
	conflicting := record
	conflicting.Tracks = append([]contentcatalog.PublishedTrack(nil), record.Tracks...)
	conflicting.Tracks[0].CatalogTrackID = uuid.New()
	if _, err = repository.MarkPublished(ctx, conflicting); !errors.Is(err, publication.ErrConflict) {
		t.Fatalf("MarkPublished(conflict) error = %v", err)
	}
}
