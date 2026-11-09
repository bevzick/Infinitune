package submission

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	contentmedia "github.com/bevzick/Infinitune/services/content-service/internal/media"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
)

func TestServiceSubmitsCompleteRelease(t *testing.T) {
	actorID, creatorID, releaseID, trackID, uploadID :=
		uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	cover := "creator-content/cover.jpg"
	store := &storeStub{candidate: Candidate{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Status: release.StatusDraft,
			CoverObjectKey: &cover,
		},
		Tracks: []Track{{
			ID: trackID, CompletedUploadID: &uploadID,
			HasWorldwideStreaming: true, HasWorldwideDistribution: true,
		}},
	}}
	access := &accessStub{decision: creator.AccessDecision{
		Allowed: true, Role: creator.RoleOwner,
		VerificationStatus: creator.VerificationVerified,
	}}
	mediaAssets := &mediaStub{asset: contentmedia.Asset{
		ID: uuid.New(), SourceUploadID: uploadID, ContentTrackID: trackID,
		Status: contentmedia.StatusReady,
	}}
	result, err := NewService(store, access, mediaAssets).Submit(
		context.Background(), actorID, releaseID,
	)
	if err != nil || result.Status != release.StatusPendingReview ||
		store.submitCalls != 1 || mediaAssets.calls != 1 {
		t.Fatalf("Submit() = %+v, %v; submit calls %d, media calls %d",
			result, err, store.submitCalls, mediaAssets.calls)
	}
	if access.permission != creator.PermissionContentPublish ||
		access.actorID != actorID || access.creatorID != creatorID {
		t.Fatalf("access = %+v", access)
	}
}

func TestServiceReturnsPendingReviewIdempotently(t *testing.T) {
	releaseID := uuid.New()
	store := &storeStub{candidate: Candidate{Release: release.Release{
		ID: releaseID, CreatorID: uuid.New(), Status: release.StatusPendingReview,
	}}}
	service := NewService(store, &accessStub{decision: creator.AccessDecision{Allowed: true}}, &mediaStub{})
	result, err := service.Submit(context.Background(), uuid.New(), releaseID)
	if err != nil || result.Status != release.StatusPendingReview || store.submitCalls != 0 {
		t.Fatalf("Submit() = %+v, %v; calls %d", result, err, store.submitCalls)
	}
}

func TestServiceResubmitsReleaseAfterRequestedChanges(t *testing.T) {
	releaseID, trackID, uploadID := uuid.New(), uuid.New(), uuid.New()
	cover := "corrected-cover.jpg"
	store := &storeStub{candidate: Candidate{
		Release: release.Release{
			ID: releaseID, CreatorID: uuid.New(), Status: release.StatusChangesRequested,
			CoverObjectKey: &cover,
		},
		Tracks: []Track{{
			ID: trackID, CompletedUploadID: &uploadID,
			HasWorldwideStreaming: true, HasWorldwideDistribution: true,
		}},
	}}
	mediaAssets := &mediaStub{asset: contentmedia.Asset{
		ID: uuid.New(), SourceUploadID: uploadID, ContentTrackID: trackID,
		Status: contentmedia.StatusReady,
	}}
	result, err := NewService(store,
		&accessStub{decision: creator.AccessDecision{Allowed: true}}, mediaAssets,
	).Submit(context.Background(), uuid.New(), releaseID)
	if err != nil || result.Status != release.StatusPendingReview || store.submitCalls != 1 {
		t.Fatalf("Submit() = %+v, %v; calls = %d", result, err, store.submitCalls)
	}
}

func TestServiceRejectsIncompleteRelease(t *testing.T) {
	trackID, uploadID := uuid.New(), uuid.New()
	cover := "cover.jpg"
	base := Candidate{
		Release: release.Release{
			ID: uuid.New(), CreatorID: uuid.New(), Status: release.StatusDraft,
			CoverObjectKey: &cover,
		},
		Tracks: []Track{{
			ID: trackID, CompletedUploadID: &uploadID,
			HasWorldwideStreaming: true, HasWorldwideDistribution: true,
		}},
	}
	tests := []struct {
		name      string
		mutate    func(*Candidate)
		media     contentmedia.Asset
		mediaErr  error
		wantError error
	}{
		{name: "missing cover", mutate: func(value *Candidate) { value.Release.CoverObjectKey = nil }, wantError: ErrNotReady},
		{name: "missing upload", mutate: func(value *Candidate) { value.Tracks[0].CompletedUploadID = nil }, wantError: ErrNotReady},
		{name: "missing streaming rights", mutate: func(value *Candidate) { value.Tracks[0].HasWorldwideStreaming = false }, wantError: ErrNotReady},
		{name: "missing distribution rights", mutate: func(value *Candidate) { value.Tracks[0].HasWorldwideDistribution = false }, wantError: ErrNotReady},
		{name: "media processing", media: contentmedia.Asset{ID: uuid.New(), SourceUploadID: uploadID, ContentTrackID: trackID, Status: contentmedia.StatusProcessing}, wantError: ErrNotReady},
		{name: "media missing", mediaErr: contentmedia.ErrNotFound, wantError: ErrNotReady},
		{name: "media unavailable", mediaErr: contentmedia.ErrUnavailable, wantError: ErrMediaUnavailable},
		{name: "media track mismatch", media: contentmedia.Asset{ID: uuid.New(), SourceUploadID: uploadID, ContentTrackID: uuid.New(), Status: contentmedia.StatusReady}, wantError: ErrInconsistentMedia},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := base
			candidate.Tracks = append([]Track(nil), base.Tracks...)
			if test.mutate != nil {
				test.mutate(&candidate)
			}
			store := &storeStub{candidate: candidate}
			mediaAsset := test.media
			if mediaAsset.ID == uuid.Nil && test.mediaErr == nil {
				mediaAsset = contentmedia.Asset{
					ID: uuid.New(), SourceUploadID: uploadID,
					ContentTrackID: trackID, Status: contentmedia.StatusReady,
				}
			}
			_, err := NewService(store,
				&accessStub{decision: creator.AccessDecision{Allowed: true}},
				&mediaStub{asset: mediaAsset, err: test.mediaErr},
			).Submit(context.Background(), uuid.New(), candidate.Release.ID)
			if !errors.Is(err, test.wantError) || store.submitCalls != 0 {
				t.Fatalf("Submit() error = %v, want %v; calls %d",
					err, test.wantError, store.submitCalls)
			}
		})
	}
}

func TestServiceRequiresVerifiedPublisher(t *testing.T) {
	store := &storeStub{candidate: Candidate{Release: release.Release{
		ID: uuid.New(), CreatorID: uuid.New(), Status: release.StatusDraft,
	}}}
	_, err := NewService(store, &accessStub{decision: creator.AccessDecision{
		Role: creator.RoleManager, VerificationStatus: creator.VerificationUnverified,
	}}, &mediaStub{}).Submit(context.Background(), uuid.New(), store.candidate.Release.ID)
	if !errors.Is(err, creator.ErrVerificationRequired) || store.submitCalls != 0 {
		t.Fatalf("Submit() error = %v, calls %d", err, store.submitCalls)
	}
}

type storeStub struct {
	candidate   Candidate
	err         error
	submitCalls int
}

func (stub *storeStub) SubmissionCandidate(context.Context, uuid.UUID) (Candidate, error) {
	return stub.candidate, stub.err
}

func (stub *storeStub) SubmitRelease(
	_ context.Context,
	releaseID uuid.UUID,
) (release.Release, error) {
	stub.submitCalls++
	result := stub.candidate.Release
	result.ID, result.Status = releaseID, release.StatusPendingReview
	now := time.Now()
	result.SubmittedAt = &now
	return result, stub.err
}

type accessStub struct {
	decision   creator.AccessDecision
	err        error
	actorID    uuid.UUID
	creatorID  uuid.UUID
	permission creator.Permission
}

func (stub *accessStub) Check(
	_ context.Context,
	actorID uuid.UUID,
	creatorID uuid.UUID,
	permission creator.Permission,
) (creator.AccessDecision, error) {
	stub.actorID, stub.creatorID, stub.permission = actorID, creatorID, permission
	return stub.decision, stub.err
}

type mediaStub struct {
	asset contentmedia.Asset
	err   error
	calls int
}

func (stub *mediaStub) AssetByUpload(context.Context, uuid.UUID) (contentmedia.Asset, error) {
	stub.calls++
	return stub.asset, stub.err
}

var _ Store = (*storeStub)(nil)
var _ AccessChecker = (*accessStub)(nil)
var _ MediaAssets = (*mediaStub)(nil)
