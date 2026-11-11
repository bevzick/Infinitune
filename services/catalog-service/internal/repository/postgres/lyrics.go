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

const qualifiedTrackLyricsColumns = `
	lyrics.id, lyrics.track_id, lyrics.lyrics_type, lyrics.language, lyrics.content,
	lyrics.source_type, lyrics.source_name, lyrics.copyright_notice, lyrics.official,
	lyrics.rights_authorized, lyrics.published, lyrics.created_at, lyrics.updated_at
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
		FROM catalog.track_lyrics AS lyrics
		WHERE lyrics.track_id = $1
		  AND lyrics.published
		  AND lyrics.rights_authorized
		  AND (
			lyrics.lyrics_type <> 'synchronized'
			OR EXISTS (
				SELECT 1 FROM catalog.lyrics_lines AS line WHERE line.lyrics_id = lyrics.id
			)
		  )
		ORDER BY lyrics.language, lyrics.id
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
	synchronizedIDs := make([]uuid.UUID, 0, len(result))
	for index := range result {
		result[index].Lines = make([]metadata.LyricsLine, 0)
		if result[index].Type == metadata.LyricsTypeSynchronized {
			synchronizedIDs = append(synchronizedIDs, result[index].ID)
		}
	}
	lines, err := repository.lyricsLines(ctx, synchronizedIDs)
	if err != nil {
		return nil, err
	}
	for index := range result {
		if values, exists := lines[result[index].ID]; exists {
			result[index].Lines = values
		}
	}
	return result, nil
}

func (repository *MetadataRepository) SaveTrackLyrics(
	ctx context.Context,
	input metadata.TrackLyricsInput,
) (metadata.TrackLyrics, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return metadata.TrackLyrics{}, fmt.Errorf("begin track lyrics transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := scanTrackLyrics(tx.QueryRow(ctx, `
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
	if result.Type == metadata.LyricsTypePlain {
		if _, err = tx.Exec(ctx, `DELETE FROM catalog.lyrics_lines WHERE lyrics_id = $1`, result.ID); err != nil {
			return metadata.TrackLyrics{}, fmt.Errorf("delete obsolete lyrics lines: %w", err)
		}
	}
	if result.Type == metadata.LyricsTypeSynchronized && result.Published {
		var hasLines bool
		if err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM catalog.lyrics_lines WHERE lyrics_id = $1
			)
		`, result.ID).Scan(&hasLines); err != nil {
			return metadata.TrackLyrics{}, fmt.Errorf("query synchronized lyrics lines: %w", err)
		}
		if !hasLines {
			return metadata.TrackLyrics{}, fmt.Errorf(
				"%w: synchronized lyrics require timing lines before publication",
				metadata.ErrInvalid,
			)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return metadata.TrackLyrics{}, fmt.Errorf("commit track lyrics transaction: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) TrackLyricsTiming(
	ctx context.Context,
	lyricsID uuid.UUID,
) (metadata.TrackLyrics, int, error) {
	var result metadata.TrackLyrics
	var durationMS int
	err := repository.database.QueryRow(ctx, `
		SELECT `+qualifiedTrackLyricsColumns+`, track.duration_ms
		FROM catalog.track_lyrics AS lyrics
		JOIN catalog.tracks AS track ON track.id = lyrics.track_id
		WHERE lyrics.id = $1
	`, lyricsID).Scan(
		&result.ID, &result.TrackID, &result.Type, &result.Language, &result.Content,
		&result.SourceType, &result.SourceName, &result.CopyrightNotice,
		&result.Official, &result.RightsAuthorized, &result.Published,
		&result.CreatedAt, &result.UpdatedAt, &durationMS,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return metadata.TrackLyrics{}, 0, metadata.ErrNotFound
	}
	if err != nil {
		return metadata.TrackLyrics{}, 0, fmt.Errorf("query track lyrics timing: %w", err)
	}
	return result, durationMS, nil
}

func (repository *MetadataRepository) ReplaceLyricsLines(
	ctx context.Context,
	lyricsID uuid.UUID,
	lines []metadata.LyricsLine,
) ([]metadata.LyricsLine, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin lyrics lines transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `DELETE FROM catalog.lyrics_lines WHERE lyrics_id = $1`, lyricsID); err != nil {
		return nil, fmt.Errorf("delete lyrics lines: %w", err)
	}
	for _, line := range lines {
		_, err = tx.Exec(ctx, `
			INSERT INTO catalog.lyrics_lines (
				lyrics_id, line_number, start_ms, end_ms, text_content
			)
			VALUES ($1, $2, $3, $4, $5)
		`, lyricsID, line.LineNumber, line.StartMS, line.EndMS, line.Text)
		if err != nil {
			if mapped := mapWriteError(err); mapped != nil {
				return nil, mapped
			}
			return nil, fmt.Errorf("insert lyrics line: %w", err)
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE catalog.track_lyrics SET updated_at = now() WHERE id = $1
	`, lyricsID)
	if err != nil {
		return nil, fmt.Errorf("update track lyrics timestamp: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return nil, metadata.ErrNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit lyrics lines transaction: %w", err)
	}
	return append([]metadata.LyricsLine(nil), lines...), nil
}

func (repository *MetadataRepository) lyricsLines(
	ctx context.Context,
	lyricsIDs []uuid.UUID,
) (map[uuid.UUID][]metadata.LyricsLine, error) {
	result := make(map[uuid.UUID][]metadata.LyricsLine, len(lyricsIDs))
	if len(lyricsIDs) == 0 {
		return result, nil
	}
	rows, err := repository.database.Query(ctx, `
		SELECT lyrics_id, line_number, start_ms, end_ms, text_content
		FROM catalog.lyrics_lines
		WHERE lyrics_id = ANY($1)
		ORDER BY lyrics_id, line_number
	`, lyricsIDs)
	if err != nil {
		return nil, fmt.Errorf("query lyrics lines: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var lyricsID uuid.UUID
		var line metadata.LyricsLine
		if err = rows.Scan(
			&lyricsID, &line.LineNumber, &line.StartMS, &line.EndMS, &line.Text,
		); err != nil {
			return nil, fmt.Errorf("scan lyrics line: %w", err)
		}
		result[lyricsID] = append(result[lyricsID], line)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lyrics lines: %w", err)
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
