package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/coverupload"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const coverUploadColumns = `
    id, release_id, requested_by_user_id, idempotency_key,
    object_key, original_name, mime_type, size_bytes, status,
    created_at, updated_at
`

type CoverUploadRepository struct{ database *pgxpool.Pool }

func NewCoverUploadRepository(database *pgxpool.Pool) *CoverUploadRepository {
	return &CoverUploadRepository{database: database}
}

func (repository *CoverUploadRepository) ReleaseTarget(ctx context.Context, releaseID uuid.UUID) (coverupload.ReleaseTarget, error) {
	var target coverupload.ReleaseTarget
	var status string
	err := repository.database.QueryRow(ctx, `
        SELECT id, creator_id, status FROM content.releases WHERE id = $1
    `, releaseID).Scan(&target.ReleaseID, &target.CreatorID, &status)
	if err != nil {
		return coverupload.ReleaseTarget{}, fmt.Errorf("find cover release target: %w", mapCoverUploadError(err))
	}
	target.Status = release.Status(status)
	return target, nil
}

func (repository *CoverUploadRepository) Create(ctx context.Context, session coverupload.Session) (coverupload.Session, bool, error) {
	created, err := scanCoverUpload(repository.database.QueryRow(ctx, `
        INSERT INTO content.cover_uploads (
            id, release_id, requested_by_user_id, idempotency_key,
            object_key, original_name, mime_type, size_bytes, status
        )
        SELECT $1, release.id, $3, $4, $5, $6, $7, $8, $9
        FROM content.releases AS release
        WHERE release.id = $2
          AND release.status IN ('draft', 'rejected', 'changes_requested')
        ON CONFLICT (release_id, requested_by_user_id, idempotency_key) DO NOTHING
        RETURNING `+coverUploadColumns,
		session.ID, session.ReleaseID, session.RequestedByUserID, session.IdempotencyKey,
		session.ObjectKey, session.OriginalName, session.MIMEType, session.SizeBytes, session.Status,
	))
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return coverupload.Session{}, false, fmt.Errorf("insert cover upload: %w", mapCoverUploadError(err))
	}
	existing, err := repository.ByRequest(ctx, session.ReleaseID, session.RequestedByUserID, session.IdempotencyKey)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, coverupload.ErrNotFound) {
		return coverupload.Session{}, false, err
	}
	return coverupload.Session{}, false, coverupload.ErrReleaseLocked
}

func (repository *CoverUploadRepository) ByRequest(ctx context.Context, releaseID, requestedBy, key uuid.UUID) (coverupload.Session, error) {
	result, err := scanCoverUpload(repository.database.QueryRow(ctx, `
        SELECT `+coverUploadColumns+` FROM content.cover_uploads
        WHERE release_id = $1 AND requested_by_user_id = $2 AND idempotency_key = $3
    `, releaseID, requestedBy, key))
	if err != nil {
		return coverupload.Session{}, mapCoverUploadError(err)
	}
	return result, nil
}

func (repository *CoverUploadRepository) ByID(ctx context.Context, id uuid.UUID) (coverupload.Session, error) {
	result, err := scanCoverUpload(repository.database.QueryRow(ctx, `
        SELECT `+coverUploadColumns+` FROM content.cover_uploads WHERE id = $1
    `, id))
	if err != nil {
		return coverupload.Session{}, mapCoverUploadError(err)
	}
	return result, nil
}

func (repository *CoverUploadRepository) Transition(ctx context.Context, id uuid.UUID, expected, next coverupload.Status) (coverupload.Session, error) {
	result, err := scanCoverUpload(repository.database.QueryRow(ctx, `
        UPDATE content.cover_uploads SET status = $3, updated_at = clock_timestamp()
        WHERE id = $1 AND status = $2
        RETURNING `+coverUploadColumns,
		id, expected, next))
	if errors.Is(err, pgx.ErrNoRows) {
		return coverupload.Session{}, coverupload.ErrConflict
	}
	if err != nil {
		return coverupload.Session{}, fmt.Errorf("update cover upload status: %w", mapCoverUploadError(err))
	}
	return result, nil
}

func scanCoverUpload(row pgx.Row) (coverupload.Session, error) {
	var session coverupload.Session
	var status string
	err := row.Scan(
		&session.ID, &session.ReleaseID, &session.RequestedByUserID,
		&session.IdempotencyKey, &session.ObjectKey, &session.OriginalName,
		&session.MIMEType, &session.SizeBytes, &status,
		&session.CreatedAt, &session.UpdatedAt,
	)
	session.Status = coverupload.Status(status)
	return session, err
}

func mapCoverUploadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return coverupload.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23503", "23514", "22P02":
			return coverupload.ErrInvalid
		case "23505":
			return coverupload.ErrConflict
		}
	}
	return err
}

var _ coverupload.Store = (*CoverUploadRepository)(nil)

// Complete updates the uploaded cover and the release atomically, preventing
// a failed or unverified upload from making a release submission-ready.
func (repository *CoverUploadRepository) Complete(ctx context.Context, uploadID uuid.UUID) (coverupload.Session, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return coverupload.Session{}, fmt.Errorf("begin cover completion: %w", err)
	}
	defer tx.Rollback(ctx)

	current, err := scanCoverUpload(tx.QueryRow(ctx,
		`SELECT `+coverUploadColumns+` FROM content.cover_uploads WHERE id = $1 FOR UPDATE`, uploadID))
	if err != nil {
		return coverupload.Session{}, mapCoverUploadError(err)
	}
	if current.Status == coverupload.StatusCompleted {
		return current, tx.Commit(ctx)
	}
	if current.Status != coverupload.StatusUploading {
		return coverupload.Session{}, coverupload.ErrInvalidTransition
	}

	var releaseID uuid.UUID
	err = tx.QueryRow(ctx, `UPDATE content.releases SET cover_object_key = $2
        WHERE id = $1 AND status IN ('draft', 'rejected', 'changes_requested')
        RETURNING id`, current.ReleaseID, current.ObjectKey).Scan(&releaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return coverupload.Session{}, coverupload.ErrReleaseLocked
	}
	if err != nil {
		return coverupload.Session{}, fmt.Errorf("attach uploaded cover: %w", mapCoverUploadError(err))
	}

	completed, err := scanCoverUpload(tx.QueryRow(ctx, `UPDATE content.cover_uploads
        SET status = 'completed', updated_at = clock_timestamp()
        WHERE id = $1 AND status = 'uploading'
        RETURNING `+coverUploadColumns, uploadID))
	if errors.Is(err, pgx.ErrNoRows) {
		return coverupload.Session{}, coverupload.ErrConflict
	}
	if err != nil {
		return coverupload.Session{}, fmt.Errorf("mark cover completed: %w", mapCoverUploadError(err))
	}
	if err := tx.Commit(ctx); err != nil {
		return coverupload.Session{}, fmt.Errorf("commit cover completion: %w", err)
	}
	return completed, nil
}
