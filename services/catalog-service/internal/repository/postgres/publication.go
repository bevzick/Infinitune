package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/events"
	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repository *MetadataRepository) PublishRelease(
	ctx context.Context,
	input metadata.PublicationInput,
	fingerprint string,
) (metadata.Publication, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return metadata.Publication{}, fmt.Errorf("begin catalog publication: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		input.SourceReleaseID.String()); err != nil {
		return metadata.Publication{}, fmt.Errorf("lock catalog publication: %w", err)
	}
	existing, existingFingerprint, err := publicationBySource(ctx, tx, input.SourceReleaseID)
	if err == nil {
		if existingFingerprint != fingerprint {
			return metadata.Publication{}, metadata.ErrConflict
		}
		message, messageErr := events.NewReleasePublished(
			existing, repository.topics.ReleasePublished,
		)
		if messageErr != nil {
			return metadata.Publication{}, fmt.Errorf("encode catalog publication event: %w", messageErr)
		}
		if err = enqueueEvent(ctx, tx, message); err != nil {
			return metadata.Publication{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return metadata.Publication{}, fmt.Errorf("commit idempotent catalog publication: %w", err)
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return metadata.Publication{}, fmt.Errorf("read catalog publication: %w", err)
	}
	var approved bool
	if err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM catalog.content_release_approvals WHERE release_id=$1
		)
	`, input.SourceReleaseID).Scan(&approved); err != nil {
		return metadata.Publication{}, fmt.Errorf("check content release approval: %w", err)
	}
	if !approved {
		return metadata.Publication{}, metadata.ErrNotApproved
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO catalog.artists (id, name, description, image_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
	`, input.Artist.ID, input.Artist.Name, input.Artist.Description, input.Artist.ImageURL); err != nil {
		return metadata.Publication{}, publicationWriteError("upsert publication artist", err)
	}
	if input.Album != nil {
		if _, err = tx.Exec(ctx, `
			INSERT INTO catalog.albums (id, title, album_type, release_date, cover_url)
			VALUES ($1, $2, $3, $4, $5)
		`, input.Album.ID, input.Album.Title, input.Album.Type,
			input.Album.ReleaseDate, input.Album.CoverURL); err != nil {
			return metadata.Publication{}, publicationWriteError("insert publication album", err)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO catalog.album_artists (album_id, artist_id, position)
			VALUES ($1, $2, 1)
		`, input.Album.ID, input.Artist.ID); err != nil {
			return metadata.Publication{}, publicationWriteError("insert publication album artist", err)
		}
	}
	for _, track := range input.Tracks {
		if _, err = tx.Exec(ctx, `
			INSERT INTO catalog.tracks (
				id, album_id, title, duration_ms, track_number, disc_number,
				explicit, release_date, media_asset_id, content_id
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, track.ID, track.AlbumID, track.Title, track.DurationMS,
			track.TrackNumber, track.DiscNumber, track.Explicit, track.ReleaseDate,
			track.MediaAssetID, track.ContentID); err != nil {
			return metadata.Publication{}, publicationWriteError("insert publication track", err)
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO catalog.track_artists (track_id, artist_id, artist_role, position)
			VALUES ($1, $2, 'primary', 1)
		`, track.ID, input.Artist.ID); err != nil {
			return metadata.Publication{}, publicationWriteError("insert publication track artist", err)
		}
	}
	var result metadata.Publication
	result.SourceReleaseID = input.SourceReleaseID
	result.ArtistID = input.Artist.ID
	if input.Album != nil {
		albumID := input.Album.ID
		result.AlbumID = &albumID
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO catalog.publications (
			source_release_id, payload_fingerprint, artist_id, album_id
		)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`, input.SourceReleaseID, fingerprint, input.Artist.ID, result.AlbumID).Scan(&result.PublishedAt)
	if err != nil {
		return metadata.Publication{}, publicationWriteError("insert catalog publication", err)
	}
	result.Tracks = make([]metadata.PublishedTrack, 0, len(input.Tracks))
	for _, track := range input.Tracks {
		if _, err = tx.Exec(ctx, `
			INSERT INTO catalog.publication_tracks (
				source_release_id, content_track_id, catalog_track_id
			)
			VALUES ($1, $2, $3)
		`, input.SourceReleaseID, track.ContentID, track.ID); err != nil {
			return metadata.Publication{}, publicationWriteError("insert publication track mapping", err)
		}
		result.Tracks = append(result.Tracks, metadata.PublishedTrack{
			ContentTrackID: track.ContentID, CatalogTrackID: track.ID,
		})
	}
	message, err := events.NewReleasePublished(result, repository.topics.ReleasePublished)
	if err != nil {
		return metadata.Publication{}, fmt.Errorf("encode catalog publication event: %w", err)
	}
	if err = enqueueEvent(ctx, tx, message); err != nil {
		return metadata.Publication{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return metadata.Publication{}, publicationWriteError("commit catalog publication", err)
	}
	return result, nil
}

func (repository *MetadataRepository) ArchiveRelease(
	ctx context.Context,
	sourceReleaseID uuid.UUID,
) (metadata.Archive, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return metadata.Archive{}, fmt.Errorf("begin catalog archive: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		sourceReleaseID.String()); err != nil {
		return metadata.Archive{}, fmt.Errorf("lock catalog archive: %w", err)
	}
	publication, _, err := publicationBySource(ctx, tx, sourceReleaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return metadata.Archive{}, metadata.ErrNotFound
	}
	if err != nil {
		return metadata.Archive{}, fmt.Errorf("read catalog publication for archive: %w", err)
	}
	var result metadata.Archive
	err = tx.QueryRow(ctx, `
		UPDATE catalog.publications
		SET archived_at = COALESCE(archived_at, clock_timestamp())
		WHERE source_release_id = $1
		RETURNING source_release_id, archived_at
	`, sourceReleaseID).Scan(&result.SourceReleaseID, &result.ArchivedAt)
	if err != nil {
		return metadata.Archive{}, fmt.Errorf("archive catalog publication: %w", err)
	}
	message, err := events.NewReleaseArchived(
		publication, result.ArchivedAt, repository.topics.ReleaseArchived,
	)
	if err != nil {
		return metadata.Archive{}, fmt.Errorf("encode catalog archive event: %w", err)
	}
	if err = enqueueEvent(ctx, tx, message); err != nil {
		return metadata.Archive{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return metadata.Archive{}, fmt.Errorf("commit catalog archive: %w", err)
	}
	return result, nil
}

func enqueueEvent(ctx context.Context, tx pgx.Tx, message events.Message) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO catalog.event_outbox (
			id, aggregate_id, event_type, topic, partition_key, payload, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (aggregate_id, event_type) DO NOTHING
	`, message.ID, message.AggregateID, message.Type, message.Topic,
		message.Key, message.Payload, message.OccurredAt)
	if err != nil {
		return fmt.Errorf("enqueue catalog event: %w", err)
	}
	return nil
}

func publicationBySource(
	ctx context.Context,
	tx pgx.Tx,
	sourceReleaseID uuid.UUID,
) (metadata.Publication, string, error) {
	var result metadata.Publication
	var fingerprint string
	err := tx.QueryRow(ctx, `
		SELECT source_release_id, payload_fingerprint, artist_id, album_id, created_at
		FROM catalog.publications
		WHERE source_release_id = $1
	`, sourceReleaseID).Scan(
		&result.SourceReleaseID, &fingerprint, &result.ArtistID,
		&result.AlbumID, &result.PublishedAt,
	)
	if err != nil {
		return metadata.Publication{}, "", err
	}
	rows, err := tx.Query(ctx, `
		SELECT content_track_id, catalog_track_id
		FROM catalog.publication_tracks
		WHERE source_release_id = $1
		ORDER BY content_track_id
	`, sourceReleaseID)
	if err != nil {
		return metadata.Publication{}, "", err
	}
	defer rows.Close()
	result.Tracks = make([]metadata.PublishedTrack, 0)
	for rows.Next() {
		var track metadata.PublishedTrack
		if err = rows.Scan(&track.ContentTrackID, &track.CatalogTrackID); err != nil {
			return metadata.Publication{}, "", err
		}
		result.Tracks = append(result.Tracks, track)
	}
	return result, fingerprint, rows.Err()
}

func publicationWriteError(operation string, err error) error {
	if mapped := mapWriteError(err); mapped != nil {
		return fmt.Errorf("%s: %w", operation, mapped)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
