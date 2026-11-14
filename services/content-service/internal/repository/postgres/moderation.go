package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/events"
	"github.com/bevzick/Infinitune/services/content-service/internal/moderation"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ModerationRepository struct {
	database      *pgxpool.Pool
	approvedTopic string
}

func NewModerationRepository(database *pgxpool.Pool, configuredTopic ...string) *ModerationRepository {
	topic := events.ReleaseApprovedV1
	if len(configuredTopic) > 0 {
		topic = configuredTopic[0]
	}
	return &ModerationRepository{database: database, approvedTopic: topic}
}

func (repository *ModerationRepository) RecordReview(
	ctx context.Context,
	review moderation.Review,
) (moderation.Review, error) {
	tx, err := repository.database.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return moderation.Review{}, fmt.Errorf("begin moderation review: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var creatorID uuid.UUID
	var releaseStatus string
	var submittedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT creator_id, status, submitted_at
		FROM content.releases
		WHERE id = $1
		FOR UPDATE
	`, review.ReleaseID).Scan(&creatorID, &releaseStatus, &submittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return moderation.Review{}, moderation.ErrNotFound
	}
	if err != nil {
		return moderation.Review{}, fmt.Errorf("lock reviewed release: %w", err)
	}
	if submittedAt != nil {
		existing, existingErr := scanModerationReview(tx.QueryRow(ctx, `
			SELECT id, release_id, reviewer_user_id, decision,
			       COALESCE(comment, ''), reviewed_submission_at, created_at
			FROM content.moderation_reviews
			WHERE release_id = $1 AND reviewed_submission_at = $2
		`, review.ReleaseID, *submittedAt))
		if existingErr == nil {
			existing.ReleaseStatus = statusForDecision(existing.Decision)
			if sameReview(existing, review) {
				if err = tx.Commit(ctx); err != nil {
					return moderation.Review{}, fmt.Errorf("commit idempotent review: %w", err)
				}
				return existing, nil
			}
			return moderation.Review{}, moderation.ErrConflict
		}
		if !errors.Is(existingErr, pgx.ErrNoRows) {
			return moderation.Review{}, fmt.Errorf("read existing moderation review: %w", existingErr)
		}
	}
	if release.Status(releaseStatus) != release.StatusPendingReview || submittedAt == nil {
		return moderation.Review{}, moderation.ErrConflict
	}
	var reviewerIsMember bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM content.creator_members
			WHERE creator_id = $1 AND user_id = $2
		)
	`, creatorID, review.ReviewerUserID).Scan(&reviewerIsMember)
	if err != nil {
		return moderation.Review{}, fmt.Errorf("check reviewer membership: %w", err)
	}
	if reviewerIsMember {
		return moderation.Review{}, moderation.ErrSelfReview
	}
	recorded, err := scanModerationReview(tx.QueryRow(ctx, `
		INSERT INTO content.moderation_reviews (
			id, release_id, reviewer_user_id, decision, comment, reviewed_submission_at
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
		RETURNING id, release_id, reviewer_user_id, decision,
		          COALESCE(comment, ''), reviewed_submission_at, created_at
	`, review.ID, review.ReleaseID, review.ReviewerUserID, review.Decision,
		review.Comment, *submittedAt))
	if err != nil {
		return moderation.Review{}, fmt.Errorf("insert moderation review: %w", mapModerationError(err))
	}
	recorded.ReleaseStatus = statusForDecision(recorded.Decision)
	commandTag, err := tx.Exec(ctx, `
		UPDATE content.releases
		SET status = $3, reviewed_at = $4, updated_at = $4
		WHERE id = $1 AND status = 'pending_review' AND submitted_at = $2
	`, review.ReleaseID, *submittedAt, recorded.ReleaseStatus, recorded.CreatedAt)
	if err != nil {
		return moderation.Review{}, fmt.Errorf("apply moderation decision: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return moderation.Review{}, moderation.ErrConflict
	}
	if recorded.Decision == moderation.DecisionApproved {
		message, messageErr := events.NewReleaseApproved(recorded, creatorID, repository.approvedTopic)
		if messageErr != nil {
			return moderation.Review{}, fmt.Errorf("encode release approved event: %w", messageErr)
		}
		_, messageErr = tx.Exec(ctx, `
			INSERT INTO content.event_outbox (
				id, aggregate_id, event_type, topic, partition_key, payload, occurred_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, message.ID, message.AggregateID, message.Type, message.Topic,
			message.Key, message.Payload, message.OccurredAt)
		if messageErr != nil {
			return moderation.Review{}, fmt.Errorf("enqueue release approved event: %w", messageErr)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return moderation.Review{}, fmt.Errorf("commit moderation review: %w", mapModerationError(err))
	}
	return recorded, nil
}

func (repository *ModerationRepository) ListReviews(
	ctx context.Context,
	page moderation.PageRequest,
) (moderation.Page, error) {
	var total int64
	if err := repository.database.QueryRow(ctx,
		`SELECT count(*) FROM content.moderation_reviews`).Scan(&total); err != nil {
		return moderation.Page{}, fmt.Errorf("count moderation reviews: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT id, release_id, reviewer_user_id, decision,
		       COALESCE(comment, ''), reviewed_submission_at, created_at
		FROM content.moderation_reviews
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, page.Limit, page.Offset)
	if err != nil {
		return moderation.Page{}, fmt.Errorf("query moderation reviews: %w", err)
	}
	defer rows.Close()
	items := make([]moderation.Review, 0, page.Limit)
	for rows.Next() {
		item, scanErr := scanModerationReview(rows)
		if scanErr != nil {
			return moderation.Page{}, fmt.Errorf("scan moderation review: %w", scanErr)
		}
		item.ReleaseStatus = statusForDecision(item.Decision)
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return moderation.Page{}, fmt.Errorf("iterate moderation reviews: %w", err)
	}
	return moderation.Page{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func scanModerationReview(row pgx.Row) (moderation.Review, error) {
	var result moderation.Review
	var decision string
	err := row.Scan(
		&result.ID, &result.ReleaseID, &result.ReviewerUserID, &decision,
		&result.Comment, &result.ReviewedSubmissionAt, &result.CreatedAt,
	)
	result.Decision = moderation.Decision(decision)
	return result, err
}

func sameReview(existing, requested moderation.Review) bool {
	return existing.ReleaseID == requested.ReleaseID &&
		existing.ReviewerUserID == requested.ReviewerUserID &&
		existing.Decision == requested.Decision && existing.Comment == requested.Comment
}

func statusForDecision(decision moderation.Decision) release.Status {
	switch decision {
	case moderation.DecisionApproved:
		return release.StatusApproved
	case moderation.DecisionRejected:
		return release.StatusRejected
	case moderation.DecisionChangesRequested:
		return release.StatusChangesRequested
	default:
		return ""
	}
}

func mapModerationError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505", "40001":
			return moderation.ErrConflict
		case "23503":
			return moderation.ErrNotFound
		case "23514", "22P02":
			return moderation.ErrInvalid
		}
	}
	return err
}

var _ moderation.Store = (*ModerationRepository)(nil)
