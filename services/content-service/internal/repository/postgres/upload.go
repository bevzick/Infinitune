package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/upload"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uploadColumns = `
	id, release_track_id, requested_by_user_id, idempotency_key,
	object_key, original_name, mime_type, size_bytes, status,
	created_at, updated_at
`

type UploadRepository struct {
	database *pgxpool.Pool
}

func NewUploadRepository(database *pgxpool.Pool) *UploadRepository {
	return &UploadRepository{database: database}
}

func (repository *UploadRepository) TrackTarget(
	ctx context.Context,
	trackID uuid.UUID,
) (upload.TrackTarget, error) {
	var target upload.TrackTarget
	var status string
	err := repository.database.QueryRow(ctx, `
		SELECT track.id, track.release_id, release.creator_id, release.status
		FROM content.release_tracks AS track
		JOIN content.releases AS release ON release.id = track.release_id
		WHERE track.id = $1
	`, trackID).Scan(
		&target.TrackID, &target.ReleaseID, &target.CreatorID, &status,
	)
	if err != nil {
		return upload.TrackTarget{}, fmt.Errorf("find upload track target: %w", mapUploadError(err))
	}
	target.ReleaseStatus = release.Status(status)
	return target, nil
}

func (repository *UploadRepository) Create(
	ctx context.Context,
	session upload.Session,
) (upload.Session, bool, error) {
	created, err := scanUpload(repository.database.QueryRow(ctx, `
		INSERT INTO content.uploads (
			id, release_track_id, requested_by_user_id, idempotency_key,
			object_key, original_name, mime_type, size_bytes, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (
			release_track_id, requested_by_user_id, idempotency_key
		) DO NOTHING
		RETURNING `+uploadColumns,
		session.ID, session.ReleaseTrackID, session.RequestedByUserID,
		session.IdempotencyKey, session.ObjectKey, session.OriginalName,
		session.MIMEType, session.SizeBytes, session.Status,
	))
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return upload.Session{}, false, fmt.Errorf("insert upload session: %w", mapUploadError(err))
	}
	existing, err := scanUpload(repository.database.QueryRow(ctx, `
		SELECT `+uploadColumns+`
		FROM content.uploads
		WHERE release_track_id = $1
		  AND requested_by_user_id = $2
		  AND idempotency_key = $3
	`, session.ReleaseTrackID, session.RequestedByUserID, session.IdempotencyKey))
	if err != nil {
		return upload.Session{}, false, fmt.Errorf("find idempotent upload session: %w", mapUploadError(err))
	}
	return existing, false, nil
}

func (repository *UploadRepository) ByRequest(
	ctx context.Context,
	releaseTrackID uuid.UUID,
	requestedByUserID uuid.UUID,
	idempotencyKey uuid.UUID,
) (upload.Session, error) {
	session, err := scanUpload(repository.database.QueryRow(ctx, `
		SELECT `+uploadColumns+`
		FROM content.uploads
		WHERE release_track_id = $1
		  AND requested_by_user_id = $2
		  AND idempotency_key = $3
	`, releaseTrackID, requestedByUserID, idempotencyKey))
	if err != nil {
		return upload.Session{}, mapUploadError(err)
	}
	return session, nil
}

func (repository *UploadRepository) ByID(
	ctx context.Context,
	uploadID uuid.UUID,
) (upload.Session, error) {
	session, err := scanUpload(repository.database.QueryRow(ctx, `
		SELECT `+uploadColumns+`
		FROM content.uploads
		WHERE id = $1
	`, uploadID))
	if err != nil {
		return upload.Session{}, fmt.Errorf("find upload session: %w", mapUploadError(err))
	}
	return session, nil
}

func (repository *UploadRepository) Transition(
	ctx context.Context,
	uploadID uuid.UUID,
	expected upload.Status,
	target upload.Status,
) (upload.Session, error) {
	session, err := scanUpload(repository.database.QueryRow(ctx, `
		UPDATE content.uploads
		SET status = $3, updated_at = clock_timestamp()
		WHERE id = $1 AND status = $2
		RETURNING `+uploadColumns,
		uploadID, expected, target,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return upload.Session{}, upload.ErrConflict
	}
	if err != nil {
		return upload.Session{}, fmt.Errorf("update upload status: %w", mapUploadError(err))
	}
	return session, nil
}

func scanUpload(row pgx.Row) (upload.Session, error) {
	var session upload.Session
	var status string
	err := row.Scan(
		&session.ID, &session.ReleaseTrackID, &session.RequestedByUserID,
		&session.IdempotencyKey, &session.ObjectKey, &session.OriginalName,
		&session.MIMEType, &session.SizeBytes, &status,
		&session.CreatedAt, &session.UpdatedAt,
	)
	session.Status = upload.Status(status)
	return session, err
}

func mapUploadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return upload.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23503", "23514", "22P02":
			return upload.ErrInvalid
		case "23505":
			return upload.ErrConflict
		}
	}
	return err
}

var _ upload.Store = (*UploadRepository)(nil)
