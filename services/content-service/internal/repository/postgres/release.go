package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const releaseColumns = `
	id, creator_id, title, release_type, release_date, status,
	cover_object_key, catalog_album_id, submitted_at, reviewed_at, created_at, updated_at
`

const releaseTrackColumns = `
	id, release_id, title, track_number, disc_number, explicit,
	catalog_track_id, created_at
`

type ReleaseRepository struct {
	database *pgxpool.Pool
}

func NewReleaseRepository(database *pgxpool.Pool) *ReleaseRepository {
	return &ReleaseRepository{database: database}
}

func (repository *ReleaseRepository) Create(
	ctx context.Context,
	draft release.Draft,
) (release.Draft, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return release.Draft{}, fmt.Errorf("begin draft release transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	createdRelease, err := scanRelease(tx.QueryRow(ctx, `
		INSERT INTO content.releases (
			id, creator_id, title, release_type, release_date, status,
			cover_object_key, catalog_album_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+releaseColumns,
		draft.Release.ID, draft.Release.CreatorID, draft.Release.Title,
		draft.Release.Type, draft.Release.ReleaseDate, draft.Release.Status,
		draft.Release.CoverObjectKey, draft.Release.CatalogAlbumID,
	))
	if err != nil {
		return release.Draft{}, fmt.Errorf("insert draft release: %w", mapReleaseError(err))
	}

	createdTracks := make([]release.Track, 0, len(draft.Tracks))
	for _, track := range draft.Tracks {
		createdTrack, insertErr := scanReleaseTrack(tx.QueryRow(ctx, `
			INSERT INTO content.release_tracks (
				id, release_id, title, track_number, disc_number, explicit,
				catalog_track_id
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING `+releaseTrackColumns,
			track.ID, track.ReleaseID, track.Title, track.TrackNumber,
			track.DiscNumber, track.Explicit, track.CatalogTrackID,
		))
		if insertErr != nil {
			return release.Draft{}, fmt.Errorf(
				"insert draft release track: %w", mapReleaseError(insertErr),
			)
		}
		createdTracks = append(createdTracks, createdTrack)
	}
	if err = tx.Commit(ctx); err != nil {
		return release.Draft{}, fmt.Errorf("commit draft release transaction: %w", err)
	}
	return release.Draft{Release: createdRelease, Tracks: createdTracks}, nil
}

func (repository *ReleaseRepository) ByID(
	ctx context.Context,
	releaseID uuid.UUID,
) (release.Draft, error) {
	tx, err := repository.database.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return release.Draft{}, fmt.Errorf("begin read draft release transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	loadedRelease, err := scanRelease(tx.QueryRow(ctx, `
		SELECT `+releaseColumns+`
		FROM content.releases
		WHERE id = $1
	`, releaseID))
	if err != nil {
		return release.Draft{}, fmt.Errorf("find draft release: %w", mapReleaseError(err))
	}
	rows, err := tx.Query(ctx, `
		SELECT `+releaseTrackColumns+`
		FROM content.release_tracks
		WHERE release_id = $1
		ORDER BY disc_number, track_number, id
	`, releaseID)
	if err != nil {
		return release.Draft{}, fmt.Errorf("query draft release tracks: %w", err)
	}
	defer rows.Close()
	tracks := make([]release.Track, 0)
	for rows.Next() {
		track, scanErr := scanReleaseTrack(rows)
		if scanErr != nil {
			return release.Draft{}, fmt.Errorf("scan draft release track: %w", scanErr)
		}
		tracks = append(tracks, track)
	}
	if err = rows.Err(); err != nil {
		return release.Draft{}, fmt.Errorf("iterate draft release tracks: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return release.Draft{}, fmt.Errorf("commit read draft release transaction: %w", err)
	}
	return release.Draft{Release: loadedRelease, Tracks: tracks}, nil
}

func scanRelease(row pgx.Row) (release.Release, error) {
	var result release.Release
	var releaseType, status string
	err := row.Scan(
		&result.ID, &result.CreatorID, &result.Title, &releaseType,
		&result.ReleaseDate, &status, &result.CoverObjectKey,
		&result.CatalogAlbumID, &result.SubmittedAt, &result.ReviewedAt,
		&result.CreatedAt, &result.UpdatedAt,
	)
	result.Type = release.Type(releaseType)
	result.Status = release.Status(status)
	return result, err
}

func scanReleaseTrack(row pgx.Row) (release.Track, error) {
	var result release.Track
	err := row.Scan(
		&result.ID, &result.ReleaseID, &result.Title, &result.TrackNumber,
		&result.DiscNumber, &result.Explicit, &result.CatalogTrackID,
		&result.CreatedAt,
	)
	return result, err
}

func mapReleaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return release.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23503", "23505", "23514", "22P02":
			return release.ErrInvalid
		}
	}
	return err
}

var _ release.Store = (*ReleaseRepository)(nil)
