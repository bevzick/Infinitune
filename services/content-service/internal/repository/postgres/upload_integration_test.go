//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/google/uuid"
)

func TestUploadSessionWorkflowIntegration(t *testing.T) {
	pool := openIntegrationDatabase(t)
	creatorRepository := NewCreatorRepository(pool)
	releaseRepository := NewReleaseRepository(pool)
	uploadRepository := NewUploadRepository(pool)
	ctx := context.Background()
	creatorID, ownerID, uploaderID := uuid.New(), uuid.New(), uuid.New()
	releaseID, trackID := uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })

	createUploadTarget(t, ctx, creatorRepository, releaseRepository,
		creatorID, ownerID, releaseID, trackID)
	if _, err := creatorRepository.AddMember(ctx, creator.Member{
		CreatorID: creatorID, UserID: uploaderID, Role: creator.RoleUploader,
	}); err != nil {
		t.Fatalf("add uploader member: %v", err)
	}
	service := upload.NewService(
		uploadRepository, creator.NewMemberService(creatorRepository, nil),
	)
	idempotencyKey := uuid.New()
	input := upload.CreateInput{
		ReleaseTrackID: trackID, IdempotencyKey: idempotencyKey,
		OriginalName: "master.flac", MIMEType: "audio/flac", SizeBytes: 1024,
	}
	created, err := service.CreateSession(ctx, uploaderID, input)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if created.Status != upload.StatusPending || created.CreatedAt.IsZero() ||
		created.UpdatedAt.IsZero() {
		t.Fatalf("CreateSession() = %+v", created)
	}
	idempotent, err := service.CreateSession(ctx, uploaderID, input)
	if err != nil || idempotent.ID != created.ID || idempotent.ObjectKey != created.ObjectKey {
		t.Fatalf("CreateSession(idempotent) = %+v, %v", idempotent, err)
	}
	if _, err = service.CreateSession(ctx, uuid.New(), upload.CreateInput{
		ReleaseTrackID: trackID, IdempotencyKey: uuid.New(),
		OriginalName: "foreign.wav", MIMEType: "audio/wav", SizeBytes: 100,
	}); !errors.Is(err, creator.ErrForbidden) {
		t.Fatalf("CreateSession(foreign creator) error = %v, want %v", err, creator.ErrForbidden)
	}
	uploading, err := service.Transition(ctx, created.ID, upload.StatusUploading)
	if err != nil || uploading.Status != upload.StatusUploading {
		t.Fatalf("Transition(uploading) = %+v, %v", uploading, err)
	}
	completed, err := service.Transition(ctx, created.ID, upload.StatusCompleted)
	if err != nil || completed.Status != upload.StatusCompleted ||
		!completed.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("Transition(completed) = %+v, %v", completed, err)
	}
}

func TestUploadRepositoryRejectsStaleStatusTransition(t *testing.T) {
	pool := openIntegrationDatabase(t)
	creatorRepository := NewCreatorRepository(pool)
	releaseRepository := NewReleaseRepository(pool)
	uploadRepository := NewUploadRepository(pool)
	ctx := context.Background()
	creatorID, ownerID := uuid.New(), uuid.New()
	releaseID, trackID := uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	createUploadTarget(t, ctx, creatorRepository, releaseRepository,
		creatorID, ownerID, releaseID, trackID)
	session, created, err := uploadRepository.Create(ctx, upload.Session{
		ID: uuid.New(), ReleaseTrackID: trackID, RequestedByUserID: ownerID,
		IdempotencyKey: uuid.New(), ObjectKey: "creator-content/stale/source.wav",
		OriginalName: "source.wav", MIMEType: "audio/wav", SizeBytes: 100,
		Status: upload.StatusPending,
	})
	if err != nil || !created {
		t.Fatalf("Create() = %+v, %v, created %v", session, err, created)
	}
	if _, err = uploadRepository.Transition(
		ctx, session.ID, upload.StatusUploading, upload.StatusCompleted,
	); !errors.Is(err, upload.ErrConflict) {
		t.Fatalf("Transition(stale) error = %v, want %v", err, upload.ErrConflict)
	}
}

func createUploadTarget(
	t *testing.T,
	ctx context.Context,
	creatorRepository *CreatorRepository,
	releaseRepository *ReleaseRepository,
	creatorID uuid.UUID,
	ownerID uuid.UUID,
	releaseID uuid.UUID,
	trackID uuid.UUID,
) {
	t.Helper()
	if _, err := creatorRepository.Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeArtist, DisplayName: "Upload Artist",
		VerificationStatus: creator.VerificationUnverified,
	}, creator.Member{
		CreatorID: creatorID, UserID: ownerID, Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator account: %v", err)
	}
	if _, err := releaseRepository.Create(ctx, release.Draft{
		Release: release.Release{
			ID: releaseID, CreatorID: creatorID, Title: "Upload Draft",
			Type: release.TypeSingle, ReleaseDate: time.Now(), Status: release.StatusDraft,
		},
		Tracks: []release.Track{{
			ID: trackID, ReleaseID: releaseID, Title: "Upload Track",
			DiscNumber: 1, TrackNumber: 1,
		}},
	}); err != nil {
		t.Fatalf("create release draft: %v", err)
	}
}
