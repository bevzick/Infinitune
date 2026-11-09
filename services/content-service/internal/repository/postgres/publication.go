package postgres

import (
	"context"
	"errors"
	"fmt"

	contentcatalog "github.com/bevzick/Infinitune/services/content-service/internal/catalog"
	"github.com/bevzick/Infinitune/services/content-service/internal/publication"
	"github.com/bevzick/Infinitune/services/content-service/internal/release"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PublicationRepository struct {
	database *pgxpool.Pool
}

func NewPublicationRepository(database *pgxpool.Pool) *PublicationRepository {
	return &PublicationRepository{database: database}
}

func (repository *PublicationRepository) PublicationCandidate(
	ctx context.Context,
	releaseID uuid.UUID,
) (publication.Candidate, error) {
	tx, err := repository.database.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return publication.Candidate{}, fmt.Errorf("begin publication candidate read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var candidate publication.Candidate
	var releaseType, status string
	err = tx.QueryRow(ctx, `
		SELECT
			release.id, release.creator_id, release.title, release.release_type,
			release.release_date, release.status, release.cover_object_key,
			release.catalog_album_id, release.submitted_at, release.reviewed_at,
			release.published_at, release.created_at, release.updated_at,
			creator.display_name
		FROM content.releases AS release
		JOIN content.creator_accounts AS creator ON creator.id = release.creator_id
		WHERE release.id = $1
	`, releaseID).Scan(
		&candidate.Release.ID, &candidate.Release.CreatorID, &candidate.Release.Title,
		&releaseType, &candidate.Release.ReleaseDate, &status,
		&candidate.Release.CoverObjectKey, &candidate.Release.CatalogAlbumID,
		&candidate.Release.SubmittedAt, &candidate.Release.ReviewedAt,
		&candidate.Release.PublishedAt, &candidate.Release.CreatedAt,
		&candidate.Release.UpdatedAt, &candidate.CreatorName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return publication.Candidate{}, publication.ErrNotFound
	}
	if err != nil {
		return publication.Candidate{}, fmt.Errorf("query publication candidate: %w", err)
	}
	candidate.Release.Type = release.Type(releaseType)
	candidate.Release.Status = release.Status(status)
	rows, err := tx.Query(ctx, `
		SELECT
			track.id, track.release_id, track.title, track.track_number,
			track.disc_number, track.explicit, track.catalog_track_id, track.created_at,
			(
				SELECT upload.id
				FROM content.uploads AS upload
				WHERE upload.release_track_id = track.id AND upload.status = 'completed'
				ORDER BY upload.updated_at DESC, upload.id
				LIMIT 1
			)
		FROM content.release_tracks AS track
		WHERE track.release_id = $1
		ORDER BY track.disc_number, track.track_number, track.id
	`, releaseID)
	if err != nil {
		return publication.Candidate{}, fmt.Errorf("query publication tracks: %w", err)
	}
	defer rows.Close()
	candidate.Tracks = make([]publication.Track, 0)
	for rows.Next() {
		var track publication.Track
		if err = rows.Scan(
			&track.ID, &track.ReleaseID, &track.Title, &track.TrackNumber,
			&track.DiscNumber, &track.Explicit, &track.CatalogTrackID,
			&track.CreatedAt, &track.CompletedUploadID,
		); err != nil {
			return publication.Candidate{}, fmt.Errorf("scan publication track: %w", err)
		}
		candidate.Tracks = append(candidate.Tracks, track)
	}
	if err = rows.Err(); err != nil {
		return publication.Candidate{}, fmt.Errorf("iterate publication tracks: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return publication.Candidate{}, fmt.Errorf("commit publication candidate read: %w", err)
	}
	return candidate, nil
}

func (repository *PublicationRepository) MarkPublished(
	ctx context.Context,
	record publication.Record,
) (release.Release, error) {
	if err := validatePublicationRecord(record); err != nil {
		return release.Release{}, err
	}
	tx, err := repository.database.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return release.Release{}, fmt.Errorf("begin publication record: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := scanRelease(tx.QueryRow(ctx, `
		SELECT `+releaseColumns+`
		FROM content.releases
		WHERE id = $1
		FOR UPDATE
	`, record.ReleaseID))
	if errors.Is(err, pgx.ErrNoRows) {
		return release.Release{}, publication.ErrNotFound
	}
	if err != nil {
		return release.Release{}, fmt.Errorf("lock published release: %w", err)
	}
	if current.Status == release.StatusPublished {
		tracksMatch, matchErr := publicationTracksMatch(ctx, tx, record.ReleaseID, record.Tracks)
		if matchErr != nil {
			return release.Release{}, fmt.Errorf("verify published track mappings: %w", matchErr)
		}
		if !sameOptionalID(current.CatalogAlbumID, record.AlbumID) ||
			current.PublishedAt == nil || !current.PublishedAt.Equal(record.PublishedAt) ||
			!tracksMatch {
			return release.Release{}, publication.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return release.Release{}, fmt.Errorf("commit idempotent publication record: %w", err)
		}
		return current, nil
	}
	if current.Status != release.StatusApproved {
		return release.Release{}, publication.ErrConflict
	}
	var trackCount int
	if err = tx.QueryRow(ctx, `
		SELECT count(*) FROM content.release_tracks WHERE release_id = $1
	`, record.ReleaseID).Scan(&trackCount); err != nil {
		return release.Release{}, fmt.Errorf("count published tracks: %w", err)
	}
	if trackCount != len(record.Tracks) {
		return release.Release{}, publication.ErrConflict
	}
	for _, track := range record.Tracks {
		command, updateErr := tx.Exec(ctx, `
			UPDATE content.release_tracks
			SET catalog_track_id = $3
			WHERE id = $1 AND release_id = $2
		`, track.ContentTrackID, record.ReleaseID, track.CatalogTrackID)
		if updateErr != nil {
			return release.Release{}, fmt.Errorf("record published track: %w", mapPublicationError(updateErr))
		}
		if command.RowsAffected() != 1 {
			return release.Release{}, publication.ErrConflict
		}
	}
	result, err := scanRelease(tx.QueryRow(ctx, `
		UPDATE content.releases
		SET status = 'published', catalog_album_id = $2, published_at = $3,
		    updated_at = clock_timestamp()
		WHERE id = $1 AND status = 'approved'
		RETURNING `+releaseColumns,
		record.ReleaseID, record.AlbumID, record.PublishedAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return release.Release{}, publication.ErrConflict
	}
	if err != nil {
		return release.Release{}, fmt.Errorf("record published release: %w", mapPublicationError(err))
	}
	if err = tx.Commit(ctx); err != nil {
		return release.Release{}, fmt.Errorf("commit publication record: %w", mapPublicationError(err))
	}
	return result, nil
}

func validatePublicationRecord(record publication.Record) error {
	if record.ReleaseID == uuid.Nil || record.PublishedAt.IsZero() || len(record.Tracks) == 0 {
		return publication.ErrInvalid
	}
	seenContent := make(map[uuid.UUID]struct{}, len(record.Tracks))
	seenCatalog := make(map[uuid.UUID]struct{}, len(record.Tracks))
	for _, track := range record.Tracks {
		if track.ContentTrackID == uuid.Nil || track.CatalogTrackID == uuid.Nil {
			return publication.ErrInvalid
		}
		if _, exists := seenContent[track.ContentTrackID]; exists {
			return publication.ErrInvalid
		}
		if _, exists := seenCatalog[track.CatalogTrackID]; exists {
			return publication.ErrInvalid
		}
		seenContent[track.ContentTrackID] = struct{}{}
		seenCatalog[track.CatalogTrackID] = struct{}{}
	}
	return nil
}

func publicationTracksMatch(
	ctx context.Context,
	tx pgx.Tx,
	releaseID uuid.UUID,
	tracks []contentcatalog.PublishedTrack,
) (bool, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, catalog_track_id
		FROM content.release_tracks
		WHERE release_id = $1
	`, releaseID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	expected := make(map[uuid.UUID]uuid.UUID, len(tracks))
	for _, track := range tracks {
		expected[track.ContentTrackID] = track.CatalogTrackID
	}
	count := 0
	for rows.Next() {
		var contentID uuid.UUID
		var catalogID *uuid.UUID
		if scanErr := rows.Scan(&contentID, &catalogID); scanErr != nil {
			return false, scanErr
		}
		if catalogID == nil || expected[contentID] != *catalogID {
			return false, nil
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return false, err
	}
	return count == len(expected), nil
}

func sameOptionalID(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func mapPublicationError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505", "40001":
			return publication.ErrConflict
		case "23503":
			return publication.ErrNotFound
		case "23514", "22P02":
			return publication.ErrInvalid
		}
	}
	return err
}

var _ publication.Store = (*PublicationRepository)(nil)
