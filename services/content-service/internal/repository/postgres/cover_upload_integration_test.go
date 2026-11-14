//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/bevzick/Infinitune/services/content-service/internal/coverupload"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
)

func TestCoverUploadSessionPersistenceIntegration(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creators := NewCreatorRepository(pool)
	releases := NewReleaseRepository(pool)
	covers := NewCoverUploadRepository(pool)
	creatorID, actorID, releaseID, trackID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	createUploadTarget(t, ctx, creators, releases, creatorID, actorID, releaseID, trackID)
	service := coverupload.NewService(covers, creator.NewMemberService(creators, nil))
	input := coverupload.CreateInput{
		ReleaseID: releaseID, IdempotencyKey: uuid.New(),
		OriginalName: "front.png", MIMEType: "image/png", SizeBytes: 100,
	}
	first, err := service.CreateSession(ctx, actorID, input)
	if err != nil || first.CreatedAt.IsZero() || first.Status != coverupload.StatusPending {
		t.Fatalf("create: %+v, %v", first, err)
	}
	replay, err := service.CreateSession(ctx, actorID, input)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("replay: %+v, %v", replay, err)
	}
	input.SizeBytes++
	if _, err = service.CreateSession(ctx, actorID, input); !errors.Is(err, coverupload.ErrIdempotencyConflict) {
		t.Fatalf("collision: %v", err)
	}
	input.IdempotencyKey = uuid.New()
	if _, err = service.CreateSession(ctx, uuid.New(), input); !errors.Is(err, creator.ErrForbidden) {
		t.Fatalf("foreign user: %v", err)
	}
	uploading, err := service.Transition(ctx, first.ID, coverupload.StatusUploading)
	if err != nil || uploading.Status != coverupload.StatusUploading {
		t.Fatalf("uploading: %+v %v", uploading, err)
	}
	if _, err = covers.Transition(ctx, first.ID, coverupload.StatusPending, coverupload.StatusCompleted); !errors.Is(err, coverupload.ErrConflict) {
		t.Fatalf("stale transition: %v", err)
	}
	completed, err := service.Transition(ctx, first.ID, coverupload.StatusCompleted)
	if err != nil || completed.Status != coverupload.StatusCompleted {
		t.Fatalf("completed: %+v %v", completed, err)
	}
}
