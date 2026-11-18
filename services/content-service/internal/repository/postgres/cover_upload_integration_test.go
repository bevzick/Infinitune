//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/bevzick/Infinitune/services/content-service/internal/coverupload"
	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	completed, err := service.Complete(ctx, first.ID)
	if err != nil || completed.Status != coverupload.StatusCompleted {
		t.Fatalf("completed: %+v %v", completed, err)
	}
	var attachedKey string
	if err := pool.QueryRow(ctx, `SELECT cover_object_key FROM content.releases WHERE id=$1`, releaseID).Scan(&attachedKey); err != nil || attachedKey != completed.ObjectKey {
		t.Fatalf("release cover key=%q err=%v", attachedKey, err)
	}
	if _, err := service.Complete(ctx, first.ID); err != nil {
		t.Fatalf("idempotent completion: %v", err)
	}
}

func TestImportedCoverNeverReplacesCreatorCoverIntegration(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creators := NewCreatorRepository(pool)
	releases := NewReleaseRepository(pool)
	covers := NewCoverUploadRepository(pool)
	creatorID, actorID, releaseID, trackID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	createUploadTarget(t, ctx, creators, releases, creatorID, actorID, releaseID, trackID)
	service := coverupload.NewService(covers, creator.NewMemberService(creators, nil))

	provider := createUploadingCover(t, ctx, service, actorID, releaseID, "provider.jpg", "image/jpeg")
	provider, err := service.CompleteIfMissing(ctx, provider.ID)
	if err != nil || provider.Status != coverupload.StatusCompleted {
		t.Fatalf("complete initial provider cover: %+v, %v", provider, err)
	}
	assertReleaseCover(t, ctx, pool, releaseID, provider.ObjectKey)

	creatorCover := createUploadingCover(t, ctx, service, actorID, releaseID, "creator.png", "image/png")
	creatorCover, err = service.Complete(ctx, creatorCover.ID)
	if err != nil || creatorCover.Status != coverupload.StatusCompleted {
		t.Fatalf("complete creator cover: %+v, %v", creatorCover, err)
	}
	assertReleaseCover(t, ctx, pool, releaseID, creatorCover.ObjectKey)

	retry := createUploadingCover(t, ctx, service, actorID, releaseID, "provider-retry.webp", "image/webp")
	if _, err = service.CompleteIfMissing(ctx, retry.ID); !errors.Is(err, coverupload.ErrConflict) {
		t.Fatalf("provider retry error = %v, want %v", err, coverupload.ErrConflict)
	}
	assertReleaseCover(t, ctx, pool, releaseID, creatorCover.ObjectKey)
	stored, err := covers.ByID(ctx, retry.ID)
	if err != nil || stored.Status != coverupload.StatusUploading {
		t.Fatalf("provider retry session = %+v, %v", stored, err)
	}
}

func createUploadingCover(
	t *testing.T,
	ctx context.Context,
	service *coverupload.Service,
	actorID, releaseID uuid.UUID,
	name, mimeType string,
) coverupload.Session {
	t.Helper()
	session, err := service.CreateSession(ctx, actorID, coverupload.CreateInput{
		ReleaseID: releaseID, IdempotencyKey: uuid.New(), OriginalName: name,
		MIMEType: mimeType, SizeBytes: 100,
	})
	if err != nil {
		t.Fatalf("create %s cover: %v", name, err)
	}
	session, err = service.Transition(ctx, session.ID, coverupload.StatusUploading)
	if err != nil {
		t.Fatalf("start %s cover: %v", name, err)
	}
	return session
}

func assertReleaseCover(
	t *testing.T,
	ctx context.Context,
	pool interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	releaseID uuid.UUID,
	want string,
) {
	t.Helper()
	var got string
	if err := pool.QueryRow(ctx,
		`SELECT cover_object_key FROM content.releases WHERE id=$1`, releaseID,
	).Scan(&got); err != nil || got != want {
		t.Fatalf("release cover key = %q, want %q, error = %v", got, want, err)
	}
}
