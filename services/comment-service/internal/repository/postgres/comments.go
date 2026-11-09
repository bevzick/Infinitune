package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bevzick/Infinitune/services/comment-service/internal/comment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maximumCommentsPerMinute = 5

type CommentRepository struct {
	database *pgxpool.Pool
}

func NewCommentRepository(database *pgxpool.Pool) *CommentRepository {
	return &CommentRepository{database: database}
}

func (repository *CommentRepository) Create(
	ctx context.Context,
	value comment.Comment,
) (comment.Comment, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return comment.Comment{}, fmt.Errorf("begin create comment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))
	`, value.UserID.String()); err != nil {
		return comment.Comment{}, fmt.Errorf("lock comment author: %w", err)
	}
	var recentCount int
	if err = tx.QueryRow(ctx, `
		SELECT count(*)
		FROM comment.comments
		WHERE user_id = $1 AND created_at >= now() - interval '1 minute'
	`, value.UserID).Scan(&recentCount); err != nil {
		return comment.Comment{}, fmt.Errorf("count recent comments: %w", err)
	}
	if recentCount >= maximumCommentsPerMinute {
		return comment.Comment{}, comment.ErrRateLimited
	}
	if value.ParentID != nil {
		var parentID *uuid.UUID
		var status comment.Status
		err = tx.QueryRow(ctx, `
			SELECT parent_id, status
			FROM comment.comments
			WHERE id = $1 AND track_id = $2
			FOR SHARE
		`, *value.ParentID, value.TrackID).Scan(&parentID, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return comment.Comment{}, comment.ErrParentInvalid
		}
		if err != nil {
			return comment.Comment{}, fmt.Errorf("read comment parent: %w", err)
		}
		if parentID != nil || status != comment.StatusPublished {
			return comment.Comment{}, comment.ErrParentInvalid
		}
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO comment.comments (
			id, user_id, track_id, parent_id, content
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, track_id, parent_id, content, status,
		          created_at, updated_at, deleted_at
	`, value.ID, value.UserID, value.TrackID, value.ParentID, value.Content).Scan(
		&value.ID, &value.UserID, &value.TrackID, &value.ParentID, &value.Content,
		&value.Status, &value.CreatedAt, &value.UpdatedAt, &value.DeletedAt,
	)
	if err != nil {
		if parentConstraint(err) {
			return comment.Comment{}, comment.ErrParentInvalid
		}
		return comment.Comment{}, fmt.Errorf("insert comment: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return comment.Comment{}, fmt.Errorf("commit create comment: %w", err)
	}
	return value, nil
}

func (repository *CommentRepository) List(
	ctx context.Context,
	trackID uuid.UUID,
	page comment.PageRequest,
) (comment.Page, error) {
	visibility := `c.track_id = $1 AND c.status = 'published' AND (
		c.parent_id IS NULL OR EXISTS (
			SELECT 1 FROM comment.comments AS parent
			WHERE parent.id = c.parent_id AND parent.status = 'published'
		)
	)`
	var total int64
	if err := repository.database.QueryRow(ctx, `
		SELECT count(*) FROM comment.comments AS c WHERE `+visibility,
		trackID).Scan(&total); err != nil {
		return comment.Page{}, fmt.Errorf("count comments: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT c.id, c.user_id, c.track_id, c.parent_id, c.content, c.status,
		       c.created_at, c.updated_at, c.deleted_at
		FROM comment.comments AS c
		WHERE `+visibility+`
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $2 OFFSET $3
	`, trackID, page.Limit, page.Offset)
	if err != nil {
		return comment.Page{}, fmt.Errorf("query comments: %w", err)
	}
	defer rows.Close()
	items := make([]comment.Comment, 0, page.Limit)
	for rows.Next() {
		value, scanErr := scanComment(rows)
		if scanErr != nil {
			return comment.Page{}, fmt.Errorf("scan comment: %w", scanErr)
		}
		items = append(items, value)
	}
	if err = rows.Err(); err != nil {
		return comment.Page{}, fmt.Errorf("iterate comments: %w", err)
	}
	return comment.Page{Items: items, Limit: page.Limit, Offset: page.Offset, Total: total}, nil
}

func (repository *CommentRepository) UpdateOwned(
	ctx context.Context,
	commentID uuid.UUID,
	userID uuid.UUID,
	content string,
) (comment.Comment, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return comment.Comment{}, fmt.Errorf("begin update comment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = authorizeEditable(ctx, tx, commentID, userID); err != nil {
		return comment.Comment{}, err
	}
	var result comment.Comment
	err = tx.QueryRow(ctx, `
		UPDATE comment.comments
		SET content = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, track_id, parent_id, content, status,
		          created_at, updated_at, deleted_at
	`, commentID, userID, content).Scan(
		&result.ID, &result.UserID, &result.TrackID, &result.ParentID, &result.Content,
		&result.Status, &result.CreatedAt, &result.UpdatedAt, &result.DeletedAt,
	)
	if err != nil {
		return comment.Comment{}, fmt.Errorf("update comment: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return comment.Comment{}, fmt.Errorf("commit update comment: %w", err)
	}
	return result, nil
}

func (repository *CommentRepository) DeleteOwned(
	ctx context.Context,
	commentID uuid.UUID,
	userID uuid.UUID,
) error {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete comment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	alreadyDeleted, err := authorizeDelete(ctx, tx, commentID, userID)
	if err != nil {
		return err
	}
	if alreadyDeleted {
		return nil
	}
	if _, err = tx.Exec(ctx, `
		UPDATE comment.comments
		SET status = 'deleted', deleted_at = now(), updated_at = now()
		WHERE id = $1 AND user_id = $2
	`, commentID, userID); err != nil {
		return fmt.Errorf("soft delete comment: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete comment: %w", err)
	}
	return nil
}

func authorizeDelete(ctx context.Context, tx pgx.Tx, commentID, userID uuid.UUID) (bool, error) {
	var ownerID uuid.UUID
	var status comment.Status
	err := tx.QueryRow(ctx, `
		SELECT user_id, status FROM comment.comments WHERE id = $1 FOR UPDATE
	`, commentID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, comment.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("lock comment: %w", err)
	}
	if ownerID != userID {
		return false, comment.ErrForbidden
	}
	return status == comment.StatusDeleted, nil
}

func authorizeEditable(ctx context.Context, tx pgx.Tx, commentID, userID uuid.UUID) error {
	var ownerID uuid.UUID
	var status comment.Status
	err := tx.QueryRow(ctx, `
		SELECT user_id, status FROM comment.comments WHERE id = $1 FOR UPDATE
	`, commentID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return comment.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock comment: %w", err)
	}
	if ownerID != userID {
		return comment.ErrForbidden
	}
	if status != comment.StatusPublished {
		return comment.ErrNotEditable
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanComment(row rowScanner) (comment.Comment, error) {
	var result comment.Comment
	err := row.Scan(
		&result.ID, &result.UserID, &result.TrackID, &result.ParentID, &result.Content,
		&result.Status, &result.CreatedAt, &result.UpdatedAt, &result.DeletedAt,
	)
	return result, err
}

func parentConstraint(err error) bool {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return false
	}
	return postgresError.ConstraintName == "comments_parent_track_fkey" ||
		strings.Contains(postgresError.Message, "comment parent") ||
		strings.Contains(postgresError.Message, "comment replies")
}

var _ comment.Store = (*CommentRepository)(nil)
