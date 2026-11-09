package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type metadataDatabase interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type MetadataRepository struct {
	database metadataDatabase
}

var _ metadata.Store = (*MetadataRepository)(nil)

func NewMetadataRepository(database metadataDatabase) *MetadataRepository {
	return &MetadataRepository{database: database}
}

func (repository *MetadataRepository) ArtistByID(
	ctx context.Context,
	id uuid.UUID,
) (metadata.Artist, error) {
	result, err := scanArtist(repository.database.QueryRow(ctx, `
		SELECT id, name, description, image_url, created_at, updated_at
		FROM catalog.artists
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return metadata.Artist{}, metadata.ErrNotFound
	}
	if err != nil {
		return metadata.Artist{}, fmt.Errorf("query artist by id: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) SaveArtist(
	ctx context.Context,
	input metadata.ArtistInput,
) (metadata.Artist, error) {
	result, err := scanArtist(repository.database.QueryRow(ctx, `
		INSERT INTO catalog.artists (id, name, description, image_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			image_url = EXCLUDED.image_url,
			updated_at = now()
		RETURNING id, name, description, image_url, created_at, updated_at
	`, input.ID, input.Name, input.Description, input.ImageURL))
	if err != nil {
		if mapped := mapWriteError(err); mapped != nil {
			return metadata.Artist{}, mapped
		}
		return metadata.Artist{}, fmt.Errorf("upsert artist: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) GenreByID(
	ctx context.Context,
	id uuid.UUID,
) (metadata.Genre, error) {
	result, err := scanGenre(repository.database.QueryRow(ctx, `
		SELECT id, name, slug
		FROM catalog.genres
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return metadata.Genre{}, metadata.ErrNotFound
	}
	if err != nil {
		return metadata.Genre{}, fmt.Errorf("query genre by id: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) SaveGenre(
	ctx context.Context,
	input metadata.GenreInput,
) (metadata.Genre, error) {
	result, err := scanGenre(repository.database.QueryRow(ctx, `
		INSERT INTO catalog.genres (id, name, slug)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, slug = EXCLUDED.slug
		RETURNING id, name, slug
	`, input.ID, input.Name, input.Slug))
	if err != nil {
		if mapped := mapWriteError(err); mapped != nil {
			return metadata.Genre{}, mapped
		}
		return metadata.Genre{}, fmt.Errorf("upsert genre: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) MoodByID(
	ctx context.Context,
	id uuid.UUID,
) (metadata.Mood, error) {
	result, err := scanMood(repository.database.QueryRow(ctx, `
		SELECT id, name, slug
		FROM catalog.moods
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return metadata.Mood{}, metadata.ErrNotFound
	}
	if err != nil {
		return metadata.Mood{}, fmt.Errorf("query mood by id: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) SaveMood(
	ctx context.Context,
	input metadata.MoodInput,
) (metadata.Mood, error) {
	result, err := scanMood(repository.database.QueryRow(ctx, `
		INSERT INTO catalog.moods (id, name, slug)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, slug = EXCLUDED.slug
		RETURNING id, name, slug
	`, input.ID, input.Name, input.Slug))
	if err != nil {
		if mapped := mapWriteError(err); mapped != nil {
			return metadata.Mood{}, mapped
		}
		return metadata.Mood{}, fmt.Errorf("upsert mood: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) AlbumByID(
	ctx context.Context,
	id uuid.UUID,
) (metadata.Album, error) {
	var result metadata.Album
	err := repository.database.QueryRow(ctx, `
		SELECT id, title, album_type, release_date, cover_url, created_at, updated_at
		FROM catalog.albums AS a
		WHERE id = $1 AND `+activeAlbumPredicate+`
	`, id).Scan(
		&result.ID,
		&result.Title,
		&result.Type,
		&result.ReleaseDate,
		&result.CoverURL,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return metadata.Album{}, metadata.ErrNotFound
	}
	if err != nil {
		return metadata.Album{}, fmt.Errorf("query album by id: %w", err)
	}

	result.Artists, err = repository.albumArtists(ctx, id)
	if err != nil {
		return metadata.Album{}, fmt.Errorf("query album artists: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) SaveAlbum(
	ctx context.Context,
	input metadata.AlbumInput,
) (metadata.Album, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return metadata.Album{}, fmt.Errorf("begin save album transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.albums (id, title, album_type, release_date, cover_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			album_type = EXCLUDED.album_type,
			release_date = EXCLUDED.release_date,
			cover_url = EXCLUDED.cover_url,
			updated_at = now()
	`, input.ID, input.Title, string(input.Type), input.ReleaseDate, input.CoverURL)
	if err != nil {
		return metadata.Album{}, repository.writeError("upsert album", err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog.album_artists WHERE album_id = $1`, input.ID); err != nil {
		return metadata.Album{}, fmt.Errorf("delete album artists: %w", err)
	}
	for _, artist := range input.Artists {
		_, err = tx.Exec(ctx, `
			INSERT INTO catalog.album_artists (album_id, artist_id, position)
			VALUES ($1, $2, $3)
		`, input.ID, artist.ArtistID, artist.Position)
		if err != nil {
			return metadata.Album{}, repository.writeError("insert album artist", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return metadata.Album{}, fmt.Errorf("commit save album transaction: %w", err)
	}

	result, err := repository.AlbumByID(ctx, input.ID)
	if err != nil {
		return metadata.Album{}, fmt.Errorf("read saved album: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) TrackByID(
	ctx context.Context,
	id uuid.UUID,
) (metadata.Track, error) {
	var result metadata.Track
	err := repository.database.QueryRow(ctx, `
		SELECT
			id, album_id, title, duration_ms, track_number, disc_number, explicit,
			release_date, media_asset_id, content_id, created_at, updated_at
		FROM catalog.tracks AS t
		WHERE id = $1 AND `+activeTrackPredicate+`
	`, id).Scan(
		&result.ID,
		&result.AlbumID,
		&result.Title,
		&result.DurationMS,
		&result.TrackNumber,
		&result.DiscNumber,
		&result.Explicit,
		&result.ReleaseDate,
		&result.MediaAssetID,
		&result.ContentID,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return metadata.Track{}, metadata.ErrNotFound
	}
	if err != nil {
		return metadata.Track{}, fmt.Errorf("query track by id: %w", err)
	}

	result.Artists, err = repository.trackArtists(ctx, id)
	if err != nil {
		return metadata.Track{}, fmt.Errorf("query track artists: %w", err)
	}
	result.Genres, err = repository.trackGenres(ctx, id)
	if err != nil {
		return metadata.Track{}, fmt.Errorf("query track genres: %w", err)
	}
	result.Moods, err = repository.trackMoods(ctx, id)
	if err != nil {
		return metadata.Track{}, fmt.Errorf("query track moods: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) SaveTrack(
	ctx context.Context,
	input metadata.TrackInput,
) (metadata.Track, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return metadata.Track{}, fmt.Errorf("begin save track transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.tracks (
			id, album_id, title, duration_ms, track_number, disc_number, explicit,
			release_date, media_asset_id, content_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			album_id = EXCLUDED.album_id,
			title = EXCLUDED.title,
			duration_ms = EXCLUDED.duration_ms,
			track_number = EXCLUDED.track_number,
			disc_number = EXCLUDED.disc_number,
			explicit = EXCLUDED.explicit,
			release_date = EXCLUDED.release_date,
			media_asset_id = EXCLUDED.media_asset_id,
			content_id = EXCLUDED.content_id,
			updated_at = now()
	`,
		input.ID,
		input.AlbumID,
		input.Title,
		input.DurationMS,
		input.TrackNumber,
		input.DiscNumber,
		input.Explicit,
		input.ReleaseDate,
		input.MediaAssetID,
		input.ContentID,
	)
	if err != nil {
		return metadata.Track{}, repository.writeError("upsert track", err)
	}
	for _, table := range []string{"track_artists", "track_genres", "track_moods"} {
		if _, err = tx.Exec(ctx, "DELETE FROM catalog."+table+" WHERE track_id = $1", input.ID); err != nil {
			return metadata.Track{}, fmt.Errorf("delete %s: %w", table, err)
		}
	}
	for _, artist := range input.Artists {
		_, err = tx.Exec(ctx, `
			INSERT INTO catalog.track_artists (track_id, artist_id, artist_role, position)
			VALUES ($1, $2, $3, $4)
		`, input.ID, artist.ArtistID, string(artist.Role), artist.Position)
		if err != nil {
			return metadata.Track{}, repository.writeError("insert track artist", err)
		}
	}
	for _, genreID := range input.GenreIDs {
		if _, err = tx.Exec(ctx, `
			INSERT INTO catalog.track_genres (track_id, genre_id) VALUES ($1, $2)
		`, input.ID, genreID); err != nil {
			return metadata.Track{}, repository.writeError("insert track genre", err)
		}
	}
	for _, mood := range input.Moods {
		if _, err = tx.Exec(ctx, `
			INSERT INTO catalog.track_moods (track_id, mood_id, weight) VALUES ($1, $2, $3)
		`, input.ID, mood.MoodID, mood.Weight); err != nil {
			return metadata.Track{}, repository.writeError("insert track mood", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return metadata.Track{}, fmt.Errorf("commit save track transaction: %w", err)
	}

	result, err := repository.TrackByID(ctx, input.ID)
	if err != nil {
		return metadata.Track{}, fmt.Errorf("read saved track: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) albumArtists(
	ctx context.Context,
	albumID uuid.UUID,
) ([]metadata.AlbumArtist, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT a.id, a.name, a.description, a.image_url, a.created_at, a.updated_at, aa.position
		FROM catalog.album_artists AS aa
		JOIN catalog.artists AS a ON a.id = aa.artist_id
		WHERE aa.album_id = $1
		ORDER BY aa.position, a.id
	`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]metadata.AlbumArtist, 0)
	for rows.Next() {
		var credit metadata.AlbumArtist
		if err = rows.Scan(
			&credit.Artist.ID,
			&credit.Artist.Name,
			&credit.Artist.Description,
			&credit.Artist.ImageURL,
			&credit.Artist.CreatedAt,
			&credit.Artist.UpdatedAt,
			&credit.Position,
		); err != nil {
			return nil, err
		}
		result = append(result, credit)
	}
	return result, rows.Err()
}

func (repository *MetadataRepository) trackArtists(
	ctx context.Context,
	trackID uuid.UUID,
) ([]metadata.TrackArtist, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT
			a.id, a.name, a.description, a.image_url, a.created_at, a.updated_at,
			ta.artist_role, ta.position
		FROM catalog.track_artists AS ta
		JOIN catalog.artists AS a ON a.id = ta.artist_id
		WHERE ta.track_id = $1
		ORDER BY
			CASE ta.artist_role
				WHEN 'primary' THEN 1
				WHEN 'featured' THEN 2
				WHEN 'producer' THEN 3
				WHEN 'composer' THEN 4
			END,
			ta.position,
			a.id
	`, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]metadata.TrackArtist, 0)
	for rows.Next() {
		var credit metadata.TrackArtist
		if err = rows.Scan(
			&credit.Artist.ID,
			&credit.Artist.Name,
			&credit.Artist.Description,
			&credit.Artist.ImageURL,
			&credit.Artist.CreatedAt,
			&credit.Artist.UpdatedAt,
			&credit.Role,
			&credit.Position,
		); err != nil {
			return nil, err
		}
		result = append(result, credit)
	}
	return result, rows.Err()
}

func (repository *MetadataRepository) trackGenres(
	ctx context.Context,
	trackID uuid.UUID,
) ([]metadata.Genre, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT g.id, g.name, g.slug
		FROM catalog.track_genres AS tg
		JOIN catalog.genres AS g ON g.id = tg.genre_id
		WHERE tg.track_id = $1
		ORDER BY lower(g.name), g.id
	`, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]metadata.Genre, 0)
	for rows.Next() {
		var genre metadata.Genre
		if err = rows.Scan(&genre.ID, &genre.Name, &genre.Slug); err != nil {
			return nil, err
		}
		result = append(result, genre)
	}
	return result, rows.Err()
}

func (repository *MetadataRepository) trackMoods(
	ctx context.Context,
	trackID uuid.UUID,
) ([]metadata.TrackMood, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT m.id, m.name, m.slug, tm.weight
		FROM catalog.track_moods AS tm
		JOIN catalog.moods AS m ON m.id = tm.mood_id
		WHERE tm.track_id = $1
		ORDER BY tm.weight DESC, m.id
	`, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]metadata.TrackMood, 0)
	for rows.Next() {
		var mood metadata.TrackMood
		if err = rows.Scan(&mood.Mood.ID, &mood.Mood.Name, &mood.Mood.Slug, &mood.Weight); err != nil {
			return nil, err
		}
		result = append(result, mood)
	}
	return result, rows.Err()
}

func (repository *MetadataRepository) writeError(operation string, err error) error {
	if mapped := mapWriteError(err); mapped != nil {
		return fmt.Errorf("%s: %w", operation, mapped)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

type scanner interface {
	Scan(...any) error
}

func scanArtist(row scanner) (metadata.Artist, error) {
	var result metadata.Artist
	err := row.Scan(
		&result.ID,
		&result.Name,
		&result.Description,
		&result.ImageURL,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	return result, err
}

func scanGenre(row scanner) (metadata.Genre, error) {
	var result metadata.Genre
	err := row.Scan(&result.ID, &result.Name, &result.Slug)
	return result, err
}

func scanMood(row scanner) (metadata.Mood, error) {
	var result metadata.Mood
	err := row.Scan(&result.ID, &result.Name, &result.Slug)
	return result, err
}
