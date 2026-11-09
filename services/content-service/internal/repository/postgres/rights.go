package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/rights"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const rightColumns = `
	id, release_track_id, territory_code, rights_type, valid_from,
	valid_until, granted_by_user_id, created_at, updated_at
`

type RightsRepository struct {
	database *pgxpool.Pool
}

func NewRightsRepository(database *pgxpool.Pool) *RightsRepository {
	return &RightsRepository{database: database}
}

func (repository *RightsRepository) TrackTarget(
	ctx context.Context,
	trackID uuid.UUID,
) (rights.TrackTarget, error) {
	var target rights.TrackTarget
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
		return rights.TrackTarget{}, fmt.Errorf("find track rights target: %w", mapRightsError(err))
	}
	target.ReleaseStatus = release.Status(status)
	return target, nil
}

func (repository *RightsRepository) RightTarget(
	ctx context.Context,
	rightID uuid.UUID,
) (rights.RightTarget, error) {
	var target rights.RightTarget
	var rightType, status string
	err := repository.database.QueryRow(ctx, `
		SELECT track_right.id, track_right.release_track_id, track_right.territory_code,
		       track_right.rights_type, track_right.valid_from, track_right.valid_until,
		       track_right.granted_by_user_id, track_right.created_at, track_right.updated_at,
		       track.release_id, release.creator_id, release.status
		FROM content.track_rights AS track_right
		JOIN content.release_tracks AS track ON track.id = track_right.release_track_id
		JOIN content.releases AS release ON release.id = track.release_id
		WHERE track_right.id = $1
	`, rightID).Scan(
		&target.Right.ID, &target.Right.ReleaseTrackID, &target.Right.TerritoryCode,
		&rightType, &target.Right.ValidFrom, &target.Right.ValidUntil,
		&target.Right.GrantedByUserID, &target.Right.CreatedAt, &target.Right.UpdatedAt,
		&target.Track.ReleaseID, &target.Track.CreatorID, &status,
	)
	if err != nil {
		return rights.RightTarget{}, fmt.Errorf("find track right: %w", mapRightsError(err))
	}
	target.Right.Type = rights.Type(rightType)
	target.Track.TrackID = target.Right.ReleaseTrackID
	target.Track.ReleaseStatus = release.Status(status)
	return target, nil
}

func (repository *RightsRepository) UpsertRight(
	ctx context.Context,
	right rights.Right,
) (rights.Right, error) {
	stored, err := scanRight(repository.database.QueryRow(ctx, `
		INSERT INTO content.track_rights (
			id, release_track_id, territory_code, rights_type,
			valid_from, valid_until, granted_by_user_id
		)
		SELECT $1, track.id, $3, $4, $5, $6, $7
		FROM content.release_tracks AS track
		JOIN content.releases AS release ON release.id = track.release_id
		WHERE track.id = $2
		  AND release.status IN ('draft', 'uploading', 'processing', 'rejected')
		ON CONFLICT (release_track_id, territory_code, rights_type) DO UPDATE
		SET valid_from = EXCLUDED.valid_from,
		    valid_until = EXCLUDED.valid_until,
		    granted_by_user_id = EXCLUDED.granted_by_user_id,
		    updated_at = now()
		RETURNING `+rightColumns,
		right.ID, right.ReleaseTrackID, right.TerritoryCode, right.Type,
		right.ValidFrom, right.ValidUntil, right.GrantedByUserID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		if _, targetErr := repository.TrackTarget(ctx, right.ReleaseTrackID); targetErr != nil {
			return rights.Right{}, targetErr
		}
		return rights.Right{}, rights.ErrImmutable
	}
	if err != nil {
		return rights.Right{}, fmt.Errorf("upsert track right: %w", mapRightsError(err))
	}
	return stored, nil
}

func (repository *RightsRepository) ListRights(
	ctx context.Context,
	trackID uuid.UUID,
) ([]rights.Right, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT `+rightColumns+`
		FROM content.track_rights
		WHERE release_track_id = $1
		ORDER BY territory_code, rights_type, id
	`, trackID)
	if err != nil {
		return nil, fmt.Errorf("query track rights: %w", err)
	}
	defer rows.Close()
	result := make([]rights.Right, 0)
	for rows.Next() {
		right, scanErr := scanRight(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan track right: %w", scanErr)
		}
		result = append(result, right)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate track rights: %w", err)
	}
	return result, nil
}

func (repository *RightsRepository) DeleteRight(
	ctx context.Context,
	rightID uuid.UUID,
) error {
	commandTag, err := repository.database.Exec(ctx, `
		DELETE FROM content.track_rights AS track_right
		USING content.release_tracks AS track, content.releases AS release
		WHERE track_right.id = $1
		  AND track.id = track_right.release_track_id
		  AND release.id = track.release_id
		  AND release.status IN ('draft', 'uploading', 'processing', 'rejected')
	`, rightID)
	if err != nil {
		return fmt.Errorf("delete track right: %w", err)
	}
	if commandTag.RowsAffected() == 1 {
		return nil
	}
	if _, targetErr := repository.RightTarget(ctx, rightID); targetErr != nil {
		return targetErr
	}
	return rights.ErrImmutable
}

func scanRight(row pgx.Row) (rights.Right, error) {
	var result rights.Right
	var rightType string
	err := row.Scan(
		&result.ID, &result.ReleaseTrackID, &result.TerritoryCode, &rightType,
		&result.ValidFrom, &result.ValidUntil, &result.GrantedByUserID,
		&result.CreatedAt, &result.UpdatedAt,
	)
	result.Type = rights.Type(rightType)
	return result, err
}

func mapRightsError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return rights.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23503":
			return rights.ErrNotFound
		case "23505", "23514", "22P02":
			return rights.ErrInvalid
		}
	}
	return err
}

var _ rights.Store = (*RightsRepository)(nil)
