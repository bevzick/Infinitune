package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const trackLyricsColumns = `
	id, track_id, lyrics_type, language, content, source_type, source_name,
	copyright_notice, official, rights_authorized, published, created_at, updated_at
`

func (repository *MetadataRepository) PublicTrackLyrics(
	ctx context.Context,
	trackID uuid.UUID,
) ([]metadata.TrackLyrics, error) {
	var active bool
	err := repository.database.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM catalog.tracks AS t
			WHERE t.id = $1 AND `+activeTrackPredicate+`
		)
	`, trackID).Scan(&active)
	if err != nil {
		return nil, fmt.Errorf("query track availability: %w", err)
	}
	if !active {
		return nil, metadata.ErrNotFound
	}

	rows, err := repository.database.Query(ctx, `
		SELECT `+trackLyricsColumns+`
		FROM catalog.track_lyrics
		WHERE track_id = $1
		  AND published
		  AND rights_authorized
		ORDER BY language, id
	`, trackID)
	if err != nil {
		return nil, fmt.Errorf("query public track lyrics: %w", err)
	}
	defer rows.Close()

	result := make([]metadata.TrackLyrics, 0)
	for rows.Next() {
		value, scanErr := scanTrackLyrics(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan public track lyrics: %w", scanErr)
		}
		result = append(result, value)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public track lyrics: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) SaveTrackLyrics(
	ctx context.Context,
	input metadata.TrackLyricsInput,
) (metadata.TrackLyrics, error) {
	result, err := scanTrackLyrics(repository.database.QueryRow(ctx, `
		INSERT INTO catalog.track_lyrics (
			id, track_id, lyrics_type, language, content, source_type, source_name,
			copyright_notice, official, rights_authorized, published
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (track_id, language) DO UPDATE SET
			lyrics_type = EXCLUDED.lyrics_type,
			content = EXCLUDED.content,
			source_type = EXCLUDED.source_type,
			source_name = EXCLUDED.source_name,
			copyright_notice = EXCLUDED.copyright_notice,
			official = EXCLUDED.official,
			rights_authorized = EXCLUDED.rights_authorized,
			published = EXCLUDED.published,
			updated_at = now()
		RETURNING `+trackLyricsColumns+`
	`, input.ID, input.TrackID, input.Type, input.Language, input.Content,
		input.SourceType, input.SourceName, input.CopyrightNotice, input.Official,
		input.RightsAuthorized, input.Published))
	if err != nil {
		if mapped := mapWriteError(err); mapped != nil {
			return metadata.TrackLyrics{}, mapped
		}
		return metadata.TrackLyrics{}, fmt.Errorf("upsert track lyrics: %w", err)
	}
	return result, nil
}

type lyricsScanner interface {
	Scan(...any) error
}

func scanTrackLyrics(row lyricsScanner) (metadata.TrackLyrics, error) {
	var result metadata.TrackLyrics
	err := row.Scan(
		&result.ID, &result.TrackID, &result.Type, &result.Language, &result.Content,
		&result.SourceType, &result.SourceName, &result.CopyrightNotice,
		&result.Official, &result.RightsAuthorized, &result.Published,
		&result.CreatedAt, &result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return metadata.TrackLyrics{}, metadata.ErrNotFound
	}
	return result, err
}
