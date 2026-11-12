package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
)

const activeTrackPredicate = `NOT EXISTS (
	SELECT 1
	FROM catalog.publication_tracks AS visibility_track
	JOIN catalog.publications AS visibility_publication
	  ON visibility_publication.source_release_id = visibility_track.source_release_id
	WHERE visibility_track.catalog_track_id = t.id
	  AND visibility_publication.archived_at IS NOT NULL
)`

const activeAlbumPredicate = `NOT EXISTS (
	SELECT 1 FROM catalog.publications AS visibility_publication
	WHERE visibility_publication.album_id = a.id
	  AND visibility_publication.archived_at IS NOT NULL
)`

const artistDiscographySQL = `
	WITH discography AS (
		SELECT DISTINCT
			'album'::text AS item_kind,
			a.id AS item_id,
			a.release_date,
			lower(a.title) AS item_title
		FROM catalog.albums AS a
		WHERE ` + activeAlbumPredicate + `
		  AND (
			EXISTS (
				SELECT 1 FROM catalog.album_artists AS aa
				WHERE aa.album_id = a.id AND aa.artist_id = $1
			)
			OR EXISTS (
				SELECT 1
				FROM catalog.tracks AS t
				JOIN catalog.track_artists AS ta ON ta.track_id = t.id
				WHERE t.album_id = a.id
				  AND ta.artist_id = $1
				  AND ` + activeTrackPredicate + `
			)
		  )
		UNION ALL
		SELECT
			'track'::text AS item_kind,
			t.id AS item_id,
			t.release_date,
			lower(t.title) AS item_title
		FROM catalog.tracks AS t
		WHERE t.album_id IS NULL
		  AND ` + activeTrackPredicate + `
		  AND EXISTS (
			SELECT 1 FROM catalog.track_artists AS ta
			WHERE ta.track_id = t.id AND ta.artist_id = $1
		  )
	)`

type discographyKey struct {
	kind metadata.DiscographyKind
	id   uuid.UUID
}

func (repository *MetadataRepository) ListArtistDiscography(
	ctx context.Context,
	artistID uuid.UUID,
	page metadata.PageRequest,
) (metadata.Page[metadata.DiscographyItem], error) {
	var total int64
	if err := repository.database.QueryRow(
		ctx, artistDiscographySQL+` SELECT count(*) FROM discography`, artistID,
	).Scan(&total); err != nil {
		return metadata.Page[metadata.DiscographyItem]{}, fmt.Errorf("count artist discography: %w", err)
	}
	rows, err := repository.database.Query(ctx, artistDiscographySQL+`
		SELECT item_kind, item_id
		FROM discography
		ORDER BY release_date DESC, item_title, item_id
		LIMIT $2 OFFSET $3
	`, artistID, page.Limit, page.Offset)
	if err != nil {
		return metadata.Page[metadata.DiscographyItem]{}, fmt.Errorf("query artist discography: %w", err)
	}
	keys := make([]discographyKey, 0, page.Limit)
	albumIDs := make([]uuid.UUID, 0, page.Limit)
	trackIDs := make([]uuid.UUID, 0, page.Limit)
	for rows.Next() {
		var key discographyKey
		if err = rows.Scan(&key.kind, &key.id); err != nil {
			rows.Close()
			return metadata.Page[metadata.DiscographyItem]{}, fmt.Errorf("scan artist discography: %w", err)
		}
		keys = append(keys, key)
		switch key.kind {
		case metadata.DiscographyKindAlbum:
			albumIDs = append(albumIDs, key.id)
		case metadata.DiscographyKindTrack:
			trackIDs = append(trackIDs, key.id)
		default:
			rows.Close()
			return metadata.Page[metadata.DiscographyItem]{}, fmt.Errorf("unsupported discography kind %q", key.kind)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return metadata.Page[metadata.DiscographyItem]{}, fmt.Errorf("iterate artist discography: %w", err)
	}
	albums, err := repository.albumsByIDs(ctx, albumIDs)
	if err != nil {
		return metadata.Page[metadata.DiscographyItem]{}, fmt.Errorf("query discography albums: %w", err)
	}
	tracks, err := repository.TracksByIDs(ctx, trackIDs)
	if err != nil {
		return metadata.Page[metadata.DiscographyItem]{}, fmt.Errorf("query discography tracks: %w", err)
	}
	albumByID := make(map[uuid.UUID]metadata.Album, len(albums))
	for _, album := range albums {
		albumByID[album.ID] = album
	}
	trackByID := make(map[uuid.UUID]metadata.Track, len(tracks))
	for _, track := range tracks {
		trackByID[track.ID] = track
	}
	items := make([]metadata.DiscographyItem, 0, len(keys))
	for _, key := range keys {
		switch key.kind {
		case metadata.DiscographyKindAlbum:
			album, exists := albumByID[key.id]
			if !exists {
				return metadata.Page[metadata.DiscographyItem]{}, fmt.Errorf("hydrate discography album %s: %w", key.id, metadata.ErrNotFound)
			}
			items = append(items, metadata.DiscographyItem{Kind: key.kind, Album: &album})
		case metadata.DiscographyKindTrack:
			track, exists := trackByID[key.id]
			if !exists {
				return metadata.Page[metadata.DiscographyItem]{}, fmt.Errorf("hydrate discography track %s: %w", key.id, metadata.ErrNotFound)
			}
			items = append(items, metadata.DiscographyItem{Kind: key.kind, Track: &track})
		}
	}
	return metadata.Page[metadata.DiscographyItem]{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func (repository *MetadataRepository) albumsByIDs(
	ctx context.Context,
	ids []uuid.UUID,
) ([]metadata.Album, error) {
	if len(ids) == 0 {
		return []metadata.Album{}, nil
	}
	rows, err := repository.database.Query(ctx, `
		SELECT id, title, album_type, release_date, cover_url, created_at, updated_at
		FROM catalog.albums
		WHERE id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]metadata.Album, len(ids))
	foundIDs := make([]uuid.UUID, 0, len(ids))
	for rows.Next() {
		var album metadata.Album
		if err = rows.Scan(
			&album.ID, &album.Title, &album.Type, &album.ReleaseDate, &album.CoverURL,
			&album.CreatedAt, &album.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}
		byID[album.ID] = album
		foundIDs = append(foundIDs, album.ID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	artists, err := repository.albumArtistsByAlbumIDs(ctx, foundIDs)
	if err != nil {
		return nil, err
	}
	for id, album := range byID {
		album.Artists = artists[id]
		if album.Artists == nil {
			album.Artists = make([]metadata.AlbumArtist, 0)
		}
		byID[id] = album
	}
	result := make([]metadata.Album, 0, len(foundIDs))
	for _, id := range ids {
		if album, exists := byID[id]; exists {
			result = append(result, album)
		}
	}
	return result, nil
}

func (repository *MetadataRepository) ListArtists(
	ctx context.Context,
	page metadata.PageRequest,
) (metadata.Page[metadata.Artist], error) {
	total, err := repository.count(ctx, `SELECT count(*) FROM catalog.artists`)
	if err != nil {
		return metadata.Page[metadata.Artist]{}, fmt.Errorf("count artists: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT id, name, description, image_url, created_at, updated_at
		FROM catalog.artists
		ORDER BY lower(name), id
		LIMIT $1 OFFSET $2
	`, page.Limit, page.Offset)
	if err != nil {
		return metadata.Page[metadata.Artist]{}, fmt.Errorf("query artists: %w", err)
	}
	defer rows.Close()

	items := make([]metadata.Artist, 0, page.Limit)
	for rows.Next() {
		artist, scanErr := scanArtist(rows)
		if scanErr != nil {
			return metadata.Page[metadata.Artist]{}, fmt.Errorf("scan artist: %w", scanErr)
		}
		items = append(items, artist)
	}
	if err = rows.Err(); err != nil {
		return metadata.Page[metadata.Artist]{}, fmt.Errorf("iterate artists: %w", err)
	}
	return metadata.Page[metadata.Artist]{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func (repository *MetadataRepository) ListAlbums(
	ctx context.Context,
	page metadata.PageRequest,
) (metadata.Page[metadata.Album], error) {
	total, err := repository.count(ctx, `
		SELECT count(*) FROM catalog.albums AS a WHERE `+activeAlbumPredicate)
	if err != nil {
		return metadata.Page[metadata.Album]{}, fmt.Errorf("count albums: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT id, title, album_type, release_date, cover_url, created_at, updated_at
		FROM catalog.albums AS a
		WHERE `+activeAlbumPredicate+`
		ORDER BY release_date DESC, lower(title), id
		LIMIT $1 OFFSET $2
	`, page.Limit, page.Offset)
	if err != nil {
		return metadata.Page[metadata.Album]{}, fmt.Errorf("query albums: %w", err)
	}

	items := make([]metadata.Album, 0, page.Limit)
	ids := make([]uuid.UUID, 0, page.Limit)
	for rows.Next() {
		var album metadata.Album
		if err = rows.Scan(
			&album.ID,
			&album.Title,
			&album.Type,
			&album.ReleaseDate,
			&album.CoverURL,
			&album.CreatedAt,
			&album.UpdatedAt,
		); err != nil {
			rows.Close()
			return metadata.Page[metadata.Album]{}, fmt.Errorf("scan album: %w", err)
		}
		items = append(items, album)
		ids = append(ids, album.ID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return metadata.Page[metadata.Album]{}, fmt.Errorf("iterate albums: %w", err)
	}

	artists, err := repository.albumArtistsByAlbumIDs(ctx, ids)
	if err != nil {
		return metadata.Page[metadata.Album]{}, fmt.Errorf("query album page artists: %w", err)
	}
	for index := range items {
		items[index].Artists = artists[items[index].ID]
		if items[index].Artists == nil {
			items[index].Artists = make([]metadata.AlbumArtist, 0)
		}
	}
	return metadata.Page[metadata.Album]{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func (repository *MetadataRepository) SearchTracks(
	ctx context.Context,
	query metadata.TrackQuery,
) (metadata.Page[metadata.Track], error) {
	orderBy, err := trackOrderBy(query.Sort)
	if err != nil {
		return metadata.Page[metadata.Track]{}, err
	}
	filterSQL := `
		FROM catalog.tracks AS t
		WHERE ` + activeTrackPredicate + `
			AND ($1::text = '' OR t.title ILIKE ('%' || $1 || '%') ESCAPE '\')
			AND ($2::uuid IS NULL OR EXISTS (
				SELECT 1 FROM catalog.track_artists AS ta
				WHERE ta.track_id = t.id AND ta.artist_id = $2
			))
			AND ($3::uuid IS NULL OR t.album_id = $3)
			AND ($4::uuid IS NULL OR EXISTS (
				SELECT 1 FROM catalog.track_genres AS tg
				WHERE tg.track_id = t.id AND tg.genre_id = $4
			))
			AND ($5::uuid IS NULL OR EXISTS (
				SELECT 1 FROM catalog.track_moods AS tm
				WHERE tm.track_id = t.id AND tm.mood_id = $5
			))
			AND ($6::date IS NULL OR (t.release_date >= $6 AND t.release_date < $7))
			AND ($8::boolean IS NULL OR t.explicit = $8)
	`
	filterArguments := trackFilterArguments(query)
	return repository.filteredTrackPage(
		ctx, filterSQL, filterArguments, query.Page, orderBy,
	)
}

func trackOrderBy(sort metadata.TrackSort) (string, error) {
	switch sort {
	case metadata.TrackSortNewest:
		return "t.release_date DESC, lower(t.title) ASC, t.id ASC", nil
	case metadata.TrackSortOldest:
		return "t.release_date ASC, lower(t.title) ASC, t.id ASC", nil
	case metadata.TrackSortTitleAsc:
		return "lower(t.title) ASC, t.id ASC", nil
	case metadata.TrackSortTitleDesc:
		return "lower(t.title) DESC, t.id ASC", nil
	default:
		return "", fmt.Errorf("unsupported track sort %q: %w", sort, metadata.ErrInvalid)
	}
}

func trackFilterArguments(query metadata.TrackQuery) []any {
	var releaseStart, releaseEnd *time.Time
	if query.ReleaseYear != nil {
		start := time.Date(*query.ReleaseYear, time.January, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(1, 0, 0)
		releaseStart = &start
		releaseEnd = &end
	}
	return []any{
		escapeLike(query.Title), query.ArtistID, query.AlbumID, query.GenreID,
		query.MoodID, releaseStart, releaseEnd, query.Explicit,
	}
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func (repository *MetadataRepository) ListGenres(
	ctx context.Context,
	page metadata.PageRequest,
) (metadata.Page[metadata.Genre], error) {
	total, err := repository.count(ctx, `SELECT count(*) FROM catalog.genres`)
	if err != nil {
		return metadata.Page[metadata.Genre]{}, fmt.Errorf("count genres: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT id, name, slug
		FROM catalog.genres
		ORDER BY lower(name), id
		LIMIT $1 OFFSET $2
	`, page.Limit, page.Offset)
	if err != nil {
		return metadata.Page[metadata.Genre]{}, fmt.Errorf("query genres: %w", err)
	}
	defer rows.Close()

	items := make([]metadata.Genre, 0, page.Limit)
	for rows.Next() {
		genre, scanErr := scanGenre(rows)
		if scanErr != nil {
			return metadata.Page[metadata.Genre]{}, fmt.Errorf("scan genre: %w", scanErr)
		}
		items = append(items, genre)
	}
	if err = rows.Err(); err != nil {
		return metadata.Page[metadata.Genre]{}, fmt.Errorf("iterate genres: %w", err)
	}
	return metadata.Page[metadata.Genre]{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func (repository *MetadataRepository) ListMoods(
	ctx context.Context,
	page metadata.PageRequest,
) (metadata.Page[metadata.Mood], error) {
	total, err := repository.count(ctx, `SELECT count(*) FROM catalog.moods`)
	if err != nil {
		return metadata.Page[metadata.Mood]{}, fmt.Errorf("count moods: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT id, name, slug
		FROM catalog.moods
		ORDER BY lower(name), id
		LIMIT $1 OFFSET $2
	`, page.Limit, page.Offset)
	if err != nil {
		return metadata.Page[metadata.Mood]{}, fmt.Errorf("query moods: %w", err)
	}
	defer rows.Close()

	items := make([]metadata.Mood, 0, page.Limit)
	for rows.Next() {
		mood, scanErr := scanMood(rows)
		if scanErr != nil {
			return metadata.Page[metadata.Mood]{}, fmt.Errorf("scan mood: %w", scanErr)
		}
		items = append(items, mood)
	}
	if err = rows.Err(); err != nil {
		return metadata.Page[metadata.Mood]{}, fmt.Errorf("iterate moods: %w", err)
	}
	return metadata.Page[metadata.Mood]{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func (repository *MetadataRepository) count(ctx context.Context, query string) (int64, error) {
	var total int64
	if err := repository.database.QueryRow(ctx, query).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (repository *MetadataRepository) albumArtistsByAlbumIDs(
	ctx context.Context,
	albumIDs []uuid.UUID,
) (map[uuid.UUID][]metadata.AlbumArtist, error) {
	result := make(map[uuid.UUID][]metadata.AlbumArtist, len(albumIDs))
	if len(albumIDs) == 0 {
		return result, nil
	}
	rows, err := repository.database.Query(ctx, `
		SELECT
			aa.album_id, a.id, a.name, a.description, a.image_url,
			a.created_at, a.updated_at, aa.position
		FROM catalog.album_artists AS aa
		JOIN catalog.artists AS a ON a.id = aa.artist_id
		WHERE aa.album_id = ANY($1::uuid[])
		ORDER BY aa.album_id, aa.position, a.id
	`, albumIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var albumID uuid.UUID
		var credit metadata.AlbumArtist
		if err = rows.Scan(
			&albumID,
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
		result[albumID] = append(result[albumID], credit)
	}
	return result, rows.Err()
}

func (repository *MetadataRepository) trackArtistsByTrackIDs(
	ctx context.Context,
	trackIDs []uuid.UUID,
) (map[uuid.UUID][]metadata.TrackArtist, error) {
	result := make(map[uuid.UUID][]metadata.TrackArtist, len(trackIDs))
	if len(trackIDs) == 0 {
		return result, nil
	}
	rows, err := repository.database.Query(ctx, `
		SELECT
			ta.track_id, a.id, a.name, a.description, a.image_url, a.created_at, a.updated_at,
			ta.artist_role, ta.position
		FROM catalog.track_artists AS ta
		JOIN catalog.artists AS a ON a.id = ta.artist_id
		WHERE ta.track_id = ANY($1::uuid[])
		ORDER BY
			ta.track_id,
			CASE ta.artist_role
				WHEN 'primary' THEN 1
				WHEN 'featured' THEN 2
				WHEN 'producer' THEN 3
				WHEN 'composer' THEN 4
			END,
			ta.position,
			a.id
	`, trackIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var trackID uuid.UUID
		var credit metadata.TrackArtist
		if err = rows.Scan(
			&trackID,
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
		result[trackID] = append(result[trackID], credit)
	}
	return result, rows.Err()
}

func (repository *MetadataRepository) trackGenresByTrackIDs(
	ctx context.Context,
	trackIDs []uuid.UUID,
) (map[uuid.UUID][]metadata.Genre, error) {
	result := make(map[uuid.UUID][]metadata.Genre, len(trackIDs))
	if len(trackIDs) == 0 {
		return result, nil
	}
	rows, err := repository.database.Query(ctx, `
		SELECT tg.track_id, g.id, g.name, g.slug
		FROM catalog.track_genres AS tg
		JOIN catalog.genres AS g ON g.id = tg.genre_id
		WHERE tg.track_id = ANY($1::uuid[])
		ORDER BY tg.track_id, lower(g.name), g.id
	`, trackIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var trackID uuid.UUID
		var genre metadata.Genre
		if err = rows.Scan(&trackID, &genre.ID, &genre.Name, &genre.Slug); err != nil {
			return nil, err
		}
		result[trackID] = append(result[trackID], genre)
	}
	return result, rows.Err()
}

func (repository *MetadataRepository) trackMoodsByTrackIDs(
	ctx context.Context,
	trackIDs []uuid.UUID,
) (map[uuid.UUID][]metadata.TrackMood, error) {
	result := make(map[uuid.UUID][]metadata.TrackMood, len(trackIDs))
	if len(trackIDs) == 0 {
		return result, nil
	}
	rows, err := repository.database.Query(ctx, `
		SELECT tm.track_id, m.id, m.name, m.slug, tm.weight
		FROM catalog.track_moods AS tm
		JOIN catalog.moods AS m ON m.id = tm.mood_id
		WHERE tm.track_id = ANY($1::uuid[])
		ORDER BY tm.track_id, tm.weight DESC, m.id
	`, trackIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var trackID uuid.UUID
		var mood metadata.TrackMood
		if err = rows.Scan(
			&trackID,
			&mood.Mood.ID,
			&mood.Mood.Name,
			&mood.Mood.Slug,
			&mood.Weight,
		); err != nil {
			return nil, err
		}
		result[trackID] = append(result[trackID], mood)
	}
	return result, rows.Err()
}

func scanTrack(row scanner, result *metadata.Track) error {
	return row.Scan(
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
}
