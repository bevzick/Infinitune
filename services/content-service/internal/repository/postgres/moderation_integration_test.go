//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/creator"
	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestModerationRepositoryRecordsDecisionIdempotently(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creatorID, ownerID, releaseID := uuid.New(), uuid.New(), uuid.New()
	reviewerID, reviewID := uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	createPendingReviewFixture(t, pool, creatorID, ownerID, releaseID)

	repository := NewModerationRepository(pool)
	requested := moderation.Review{
		ID: reviewID, ReleaseID: releaseID, ReviewerUserID: reviewerID,
		Decision: moderation.DecisionApproved, Comment: "ready",
	}
	recorded, err := repository.RecordReview(ctx, requested)
	if err != nil || recorded.ID != reviewID ||
		recorded.ReleaseStatus != release.StatusApproved ||
		recorded.ReviewedSubmissionAt.IsZero() || recorded.CreatedAt.IsZero() {
		t.Fatalf("RecordReview() = %+v, %v", recorded, err)
	}
	loaded, err := NewReleaseRepository(pool).ByID(ctx, releaseID)
	if err != nil || loaded.Release.Status != release.StatusApproved ||
		loaded.Release.ReviewedAt == nil ||
		!loaded.Release.ReviewedAt.Equal(recorded.CreatedAt) {
		t.Fatalf("reviewed release = %+v, %v", loaded.Release, err)
	}
	again, err := repository.RecordReview(ctx, moderation.Review{
		ID: uuid.New(), ReleaseID: releaseID, ReviewerUserID: reviewerID,
		Decision: moderation.DecisionApproved, Comment: "ready",
	})
	if err != nil || again.ID != recorded.ID || !again.CreatedAt.Equal(recorded.CreatedAt) {
		t.Fatalf("RecordReview(idempotent) = %+v, %v", again, err)
	}
	var approvalEvents int
	if err = pool.QueryRow(ctx, `
		SELECT count(*) FROM content.event_outbox
		WHERE aggregate_id=$1 AND event_type='content.release.approved.v1'
	`, releaseID).Scan(&approvalEvents); err != nil || approvalEvents != 1 {
		t.Fatalf("approval outbox events = %d, %v", approvalEvents, err)
	}
	_, err = repository.RecordReview(ctx, moderation.Review{
		ID: uuid.New(), ReleaseID: releaseID, ReviewerUserID: reviewerID,
		Decision: moderation.DecisionRejected, Comment: "changed mind",
	})
	if !errors.Is(err, moderation.ErrConflict) {
		t.Fatalf("RecordReview(conflict) error = %v", err)
	}
}

func TestModerationRepositoryPreventsSelfReview(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creatorID, ownerID, releaseID := uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	createPendingReviewFixture(t, pool, creatorID, ownerID, releaseID)

	_, err := NewModerationRepository(pool).RecordReview(ctx, moderation.Review{
		ID: uuid.New(), ReleaseID: releaseID, ReviewerUserID: ownerID,
		Decision: moderation.DecisionApproved,
	})
	if !errors.Is(err, moderation.ErrSelfReview) {
		t.Fatalf("RecordReview(self) error = %v", err)
	}
	loaded, loadErr := NewReleaseRepository(pool).ByID(ctx, releaseID)
	if loadErr != nil || loaded.Release.Status != release.StatusPendingReview ||
		loaded.Release.ReviewedAt != nil {
		t.Fatalf("release after self review = %+v, %v", loaded.Release, loadErr)
	}
}

func TestModerationRepositoryRequestsChanges(t *testing.T) {
	pool := openIntegrationDatabase(t)
	ctx := context.Background()
	creatorID, ownerID, releaseID := uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { cleanupReleaseAggregate(t, pool, creatorID, releaseID) })
	createPendingReviewFixture(t, pool, creatorID, ownerID, releaseID)

	recorded, err := NewModerationRepository(pool).RecordReview(ctx, moderation.Review{
		ID: uuid.New(), ReleaseID: releaseID, ReviewerUserID: uuid.New(),
		Decision: moderation.DecisionChangesRequested, Comment: "replace cover",
	})
	if err != nil || recorded.ReleaseStatus != release.StatusChangesRequested {
		t.Fatalf("RecordReview() = %+v, %v", recorded, err)
	}
	loaded, loadErr := NewReleaseRepository(pool).ByID(ctx, releaseID)
	if loadErr != nil || loaded.Release.Status != release.StatusChangesRequested {
		t.Fatalf("changed release = %+v, %v", loaded.Release, loadErr)
	}
	var events int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM content.event_outbox WHERE aggregate_id=$1`, releaseID).Scan(&events); err != nil || events != 0 {
		t.Fatalf("changes requested outbox events = %d, %v", events, err)
	}
}

func createPendingReviewFixture(
	t *testing.T,
	pool *pgxpool.Pool,
	creatorID uuid.UUID,
	ownerID uuid.UUID,
	releaseID uuid.UUID,
) {
	t.Helper()
	ctx := context.Background()
	if _, err := NewCreatorRepository(pool).Create(ctx, creator.Account{
		ID: creatorID, Type: creator.TypeArtist, DisplayName: "Moderation Artist",
		VerificationStatus: creator.VerificationVerified,
	}, creator.Member{
		CreatorID: creatorID, UserID: ownerID, Role: creator.RoleOwner,
	}); err != nil {
		t.Fatalf("create creator: %v", err)
	}
	if _, err := NewReleaseRepository(pool).Create(ctx, release.Draft{Release: release.Release{
		ID: releaseID, CreatorID: creatorID, Title: "Review Me",
		Type: release.TypeSingle, ReleaseDate: time.Now().UTC().AddDate(0, 1, 0),
		Status: release.StatusDraft,
	}}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE content.releases
		SET status = 'pending_review', submitted_at = now()
		WHERE id = $1
	`, releaseID); err != nil {
		t.Fatalf("submit fixture release: %v", err)
	}
}
