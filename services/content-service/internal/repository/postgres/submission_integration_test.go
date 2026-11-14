//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/events"
	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/rights"
	"github.com/bevzick/Infinitune/services/content-service/internal/submission"
	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/google/uuid"
)

func TestSubmissionRepositoryLoadsReadinessAndTransitionsIdempotently(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creatorID, ownerID, releaseID, trackID, uploadID :=
		uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	if _, err := NewCreatorRepository(pool).Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeArtist, DisplayName: "Submission Artist",
		VerificationStatus: creator.VerificationVerified,
	}, creator.Member{
		CreatorID: creatorID, UserID: ownerID, Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator account: %v", err)
	}
	cover := "creator-content/" + releaseID.String() + "/cover.jpg"
	releaseDate := time.Date(2027, time.June, 10, 0, 0, 0, 0, time.UTC)
	if _, err := NewReleaseRepository(pool).Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Ready Release",
			Type: release.TypeSingle, ReleaseDate: releaseDate,
			Status: release.StatusDraft, CoverObjectKey: &cover,
		},
		Tracks: []release.Track{{
			ID: trackID, ReleaseID: releaseID, Title: "Ready Track",
			DiscNumber: 1, TrackNumber: 1,
		}},
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	uploadRepository := NewUploadRepository(pool)
	createdUpload, _, err := uploadRepository.Create(ctx, upload.Session{
		ID: uploadID, ReleaseTrackID: trackID, RequestedByUserID: ownerID,
		IdempotencyKey: uuid.New(),
		ObjectKey:      "creator-content/" + releaseID.String() + "/source.flac",
		OriginalName:   "source.flac", MIMEType: "audio/flac", SizeBytes: 1024,
		Status: upload.StatusPending,
	})
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}
	createdUpload, err = uploadRepository.Transition(
		ctx, createdUpload.ID, upload.StatusPending, upload.StatusUploading,
	)
	if err != nil {
		t.Fatalf("start upload: %v", err)
	}
	if _, err = uploadRepository.Transition(
		ctx, createdUpload.ID, upload.StatusUploading, upload.StatusCompleted,
	); err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	rightsRepository := NewRightsRepository(pool)
	var distributionRight rights.Right
	for _, rightType := range []rights.Type{rights.TypeStreaming, rights.TypeDistribution} {
		storedRight, storeErr := rightsRepository.UpsertRight(ctx, rights.Right{
			ID: uuid.New(), ReleaseTrackID: trackID,
			TerritoryCode: rights.TerritoryWorld, Type: rightType,
			ValidFrom: releaseDate.AddDate(0, 0, -1), GrantedByUserID: ownerID,
		})
		if storeErr != nil {
			t.Fatalf("create %s rights: %v", rightType, storeErr)
		}
		if rightType == rights.TypeDistribution {
			distributionRight = storedRight
		}
	}

	repository := NewSubmissionRepository(pool)
	candidate, err := repository.SubmissionCandidate(ctx, releaseID)
	if err != nil || len(candidate.Tracks) != 1 ||
		candidate.Tracks[0].CompletedUploadID == nil ||
		*candidate.Tracks[0].CompletedUploadID != uploadID ||
		!candidate.Tracks[0].HasWorldwideStreaming ||
		!candidate.Tracks[0].HasWorldwideDistribution {
		t.Fatalf("SubmissionCandidate() = %+v, %v", candidate, err)
	}
	if err = rightsRepository.DeleteRight(ctx, distributionRight.ID); err != nil {
		t.Fatalf("delete distribution rights fixture: %v", err)
	}
	if _, err = repository.SubmitRelease(ctx, releaseID); !errors.Is(err, submission.ErrNotReady) {
		t.Fatalf("SubmitRelease(changed readiness) error = %v", err)
	}
	if _, err = rightsRepository.UpsertRight(ctx, rights.Right{
		ID: uuid.New(), ReleaseTrackID: trackID,
		TerritoryCode: rights.TerritoryWorld, Type: rights.TypeDistribution,
		ValidFrom: releaseDate.AddDate(0, 0, -1), GrantedByUserID: ownerID,
	}); err != nil {
		t.Fatalf("restore distribution rights: %v", err)
	}
	submitted, err := repository.SubmitRelease(ctx, releaseID)
	if err != nil || submitted.Status != release.StatusPendingReview ||
		submitted.SubmittedAt == nil {
		t.Fatalf("SubmitRelease() = %+v, %v", submitted, err)
	}
	again, err := repository.SubmitRelease(ctx, releaseID)
	if err != nil || again.SubmittedAt == nil ||
		!again.SubmittedAt.Equal(*submitted.SubmittedAt) {
		t.Fatalf("SubmitRelease(idempotent) = %+v, %v", again, err)
	}
	reviewed, err := NewModerationRepository(pool).RecordReview(ctx, moderation.Review{
		ID: uuid.New(), ReleaseID: releaseID, ReviewerUserID: uuid.New(),
		Decision: moderation.DecisionChangesRequested, Comment: "replace metadata",
	})
	if err != nil || reviewed.ReleaseStatus != release.StatusChangesRequested {
		t.Fatalf("RecordReview(changes requested) = %+v, %v", reviewed, err)
	}
	resubmitted, err := repository.SubmitRelease(ctx, releaseID)
	if err != nil || resubmitted.Status != release.StatusPendingReview ||
		resubmitted.SubmittedAt == nil || resubmitted.ReviewedAt != nil ||
		!resubmitted.SubmittedAt.After(*submitted.SubmittedAt) {
		t.Fatalf("SubmitRelease(corrected) = %+v, %v", resubmitted, err)
	}
	if _, err = pool.Exec(ctx, `
		UPDATE content.releases SET status = 'approved' WHERE id = $1
	`, releaseID); err != nil {
		t.Fatalf("approve fixture release: %v", err)
	}
	if _, err = repository.SubmitRelease(ctx, releaseID); !errors.Is(err, submission.ErrConflict) {
		t.Fatalf("SubmitRelease(approved) error = %v", err)
	}
}

func TestSubmissionRepositoryAcceptsReadyExternalMedia(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creatorID, ownerID, releaseID, trackID, assetID :=
		uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM content.media_asset_readiness WHERE media_asset_id=$1`, assetID)
		cleanupReleaseAggregate(t, pool, creatorID, releaseID)
	})
	if _, err := NewCreatorRepository(pool).Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeArtist, DisplayName: "External Artist",
		VerificationStatus: creator.VerificationVerified,
	}, creator.Member{
		CreatorID: creatorID, UserID: ownerID, Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator: %v", err)
	}
	cover := "creator-content/" + releaseID.String() + "/cover.jpg"
	releaseDate := time.Date(2027, time.July, 1, 0, 0, 0, 0, time.UTC)
	if _, err := NewReleaseRepository(pool).Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Imported Release",
			Type: release.TypeSingle, ReleaseDate: releaseDate,
			Status: release.StatusDraft, CoverObjectKey: &cover,
		},
		Tracks: []release.Track{{
			ID: trackID, ReleaseID: releaseID, Title: "Imported Track",
			DiscNumber: 1, TrackNumber: 1,
		}},
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	for _, rightType := range []rights.Type{rights.TypeStreaming, rights.TypeDistribution} {
		if _, err := NewRightsRepository(pool).UpsertRight(ctx, rights.Right{
			ID: uuid.New(), ReleaseTrackID: trackID, TerritoryCode: rights.TerritoryWorld,
			Type: rightType, ValidFrom: releaseDate.AddDate(0, 0, -1),
			GrantedByUserID: ownerID,
		}); err != nil {
			t.Fatalf("create %s rights: %v", rightType, err)
		}
	}
	if err := NewMediaReadinessRepository(pool).UpsertReady(ctx, events.ReadyAsset{
		MediaAssetID: assetID, ContentTrackID: trackID, DurationMS: 180000,
		EventID: uuid.New(), OccurredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("store external media readiness: %v", err)
	}
	repository := NewSubmissionRepository(pool)
	candidate, err := repository.SubmissionCandidate(ctx, releaseID)
	if err != nil || candidate.Tracks[0].CompletedUploadID != nil {
		t.Fatalf("SubmissionCandidate() = %+v, %v", candidate, err)
	}
	submitted, err := repository.SubmitRelease(ctx, releaseID)
	if err != nil || submitted.Status != release.StatusPendingReview {
		t.Fatalf("SubmitRelease() = %+v, %v", submitted, err)
	}
}

func TestSubmissionRepositoryRejectsExpiredWorldwideRights(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creatorID, releaseID, trackID := uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	if _, err := NewCreatorRepository(pool).Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeLabel, DisplayName: "Expired Rights",
		VerificationStatus: creator.VerificationVerified,
	}, creator.Member{
		CreatorID: creatorID, UserID: uuid.New(), Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator: %v", err)
	}
	releaseDate := time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC)
	if _, err := NewReleaseRepository(pool).Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Future Release",
			Type: release.TypeSingle, ReleaseDate: releaseDate, Status: release.StatusDraft,
		},
		Tracks: []release.Track{{
			ID: trackID, ReleaseID: releaseID, Title: "Track", DiscNumber: 1, TrackNumber: 1,
		}},
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	expired := releaseDate.AddDate(0, 0, -1)
	if _, err := NewRightsRepository(pool).UpsertRight(ctx, rights.Right{
		ID: uuid.New(), ReleaseTrackID: trackID,
		TerritoryCode: rights.TerritoryWorld, Type: rights.TypeStreaming,
		ValidFrom: releaseDate.AddDate(-1, 0, 0), ValidUntil: &expired,
		GrantedByUserID: uuid.New(),
	}); err != nil {
		t.Fatalf("create expired rights: %v", err)
	}
	candidate, err := NewSubmissionRepository(pool).SubmissionCandidate(ctx, releaseID)
	if err != nil || len(candidate.Tracks) != 1 ||
		candidate.Tracks[0].HasWorldwideStreaming {
		t.Fatalf("SubmissionCandidate() = %+v, %v", candidate, err)
	}
}
