package postgres

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/events"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ContentApprovalRepository struct{ pool *pgxpool.Pool }

func NewContentApprovalRepository(pool *pgxpool.Pool) *ContentApprovalRepository {
	return &ContentApprovalRepository{pool: pool}
}

func (repository *ContentApprovalRepository) UpsertContentApproval(ctx context.Context, approval events.ContentReleaseApproval) error {
	_, err := repository.pool.Exec(ctx, `
		INSERT INTO catalog.content_release_approvals (
			release_id, creator_id, reviewer_user_id, reviewed_submission_at, event_id, approved_at
		) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (release_id) DO UPDATE
		SET creator_id=EXCLUDED.creator_id, reviewer_user_id=EXCLUDED.reviewer_user_id,
		    reviewed_submission_at=EXCLUDED.reviewed_submission_at,
		    event_id=EXCLUDED.event_id, approved_at=EXCLUDED.approved_at,
		    received_at=now()
		WHERE EXCLUDED.approved_at > catalog.content_release_approvals.approved_at
	`, approval.ReleaseID, approval.CreatorID, approval.ReviewerUserID,
		approval.ReviewedSubmissionAt, approval.EventID, approval.ApprovedAt)
	if err != nil {
		return fmt.Errorf("upsert content release approval: %w", err)
	}
	return nil
}
