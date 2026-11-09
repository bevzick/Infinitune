package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/bevzick/Infinitune/services/content-service/internal/submission"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SubmissionRepository struct {
	database *pgxpool.Pool
}

func NewSubmissionRepository(database *pgxpool.Pool) *SubmissionRepository {
	return &SubmissionRepository{database: database}
}

func (repository *SubmissionRepository) SubmissionCandidate(
	ctx context.Context,
	releaseID uuid.UUID,
) (submission.Candidate, error) {
	tx, err := repository.database.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return submission.Candidate{}, fmt.Errorf("begin submission candidate read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	loadedRelease, err := scanRelease(tx.QueryRow(ctx, `
		SELECT `+releaseColumns+`
		FROM content.releases
		WHERE id = $1
	`, releaseID))
	if errors.Is(err, pgx.ErrNoRows) {
		return submission.Candidate{}, submission.ErrNotFound
	}
	if err != nil {
		return submission.Candidate{}, fmt.Errorf("find submitted release: %w", err)
	}
	rows, err := tx.Query(ctx, `
		SELECT
			track.id,
			(
				SELECT upload.id
				FROM content.uploads AS upload
				WHERE upload.release_track_id = track.id
				  AND upload.status = 'completed'
				ORDER BY upload.updated_at DESC, upload.id
				LIMIT 1
			),
			EXISTS (
				SELECT 1 FROM content.track_rights AS track_right
				WHERE track_right.release_track_id = track.id
				  AND track_right.territory_code = '001'
				  AND track_right.rights_type = 'streaming'
				  AND track_right.valid_from <= $2
				  AND (track_right.valid_until IS NULL OR track_right.valid_until >= $2)
			),
			EXISTS (
				SELECT 1 FROM content.track_rights AS track_right
				WHERE track_right.release_track_id = track.id
				  AND track_right.territory_code = '001'
				  AND track_right.rights_type = 'distribution'
				  AND track_right.valid_from <= $2
				  AND (track_right.valid_until IS NULL OR track_right.valid_until >= $2)
			)
		FROM content.release_tracks AS track
		WHERE track.release_id = $1
		ORDER BY track.disc_number, track.track_number, track.id
	`, releaseID, loadedRelease.ReleaseDate)
	if err != nil {
		return submission.Candidate{}, fmt.Errorf("query submission tracks: %w", err)
	}
	defer rows.Close()
	tracks := make([]submission.Track, 0)
	for rows.Next() {
		var track submission.Track
		if err = rows.Scan(
			&track.ID, &track.CompletedUploadID,
			&track.HasWorldwideStreaming, &track.HasWorldwideDistribution,
		); err != nil {
			return submission.Candidate{}, fmt.Errorf("scan submission track: %w", err)
		}
		tracks = append(tracks, track)
	}
	if err = rows.Err(); err != nil {
		return submission.Candidate{}, fmt.Errorf("iterate submission tracks: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return submission.Candidate{}, fmt.Errorf("commit submission candidate read: %w", err)
	}
	return submission.Candidate{Release: loadedRelease, Tracks: tracks}, nil
}

func (repository *SubmissionRepository) SubmitRelease(
	ctx context.Context,
	releaseID uuid.UUID,
) (release.Release, error) {
	result, err := scanRelease(repository.database.QueryRow(ctx, `
		UPDATE content.releases AS release
		SET status = 'pending_review', submitted_at = clock_timestamp(),
		    updated_at = clock_timestamp()
		WHERE release.id = $1
		  AND release.status IN ('draft', 'rejected')
		  AND release.cover_object_key IS NOT NULL
		  AND btrim(release.cover_object_key) <> ''
		  AND EXISTS (
		      SELECT 1 FROM content.release_tracks AS track
		      WHERE track.release_id = release.id
		  )
		  AND NOT EXISTS (
		      SELECT 1
		      FROM content.release_tracks AS track
		      WHERE track.release_id = release.id
		        AND (
		          NOT EXISTS (
		              SELECT 1 FROM content.uploads AS upload
		              WHERE upload.release_track_id = track.id
		                AND upload.status = 'completed'
		          )
		          OR NOT EXISTS (
		              SELECT 1 FROM content.track_rights AS track_right
		              WHERE track_right.release_track_id = track.id
		                AND track_right.territory_code = '001'
		                AND track_right.rights_type = 'streaming'
		                AND track_right.valid_from <= release.release_date
		                AND (track_right.valid_until IS NULL OR track_right.valid_until >= release.release_date)
		          )
		          OR NOT EXISTS (
		              SELECT 1 FROM content.track_rights AS track_right
		              WHERE track_right.release_track_id = track.id
		                AND track_right.territory_code = '001'
		                AND track_right.rights_type = 'distribution'
		                AND track_right.valid_from <= release.release_date
		                AND (track_right.valid_until IS NULL OR track_right.valid_until >= release.release_date)
		          )
		        )
		  )
		RETURNING `+releaseColumns,
		releaseID,
	))
	if err == nil {
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return release.Release{}, fmt.Errorf("transition release to pending review: %w", err)
	}
	current, readErr := scanRelease(repository.database.QueryRow(ctx, `
		SELECT `+releaseColumns+` FROM content.releases WHERE id = $1
	`, releaseID))
	if errors.Is(readErr, pgx.ErrNoRows) {
		return release.Release{}, submission.ErrNotFound
	}
	if readErr != nil {
		return release.Release{}, fmt.Errorf("read concurrent release submission: %w", readErr)
	}
	if current.Status == release.StatusPendingReview {
		return current, nil
	}
	if current.Status == release.StatusDraft || current.Status == release.StatusRejected {
		return release.Release{}, submission.ErrNotReady
	}
	return release.Release{}, submission.ErrConflict
}

var _ submission.Store = (*SubmissionRepository)(nil)
