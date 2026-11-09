package postgres

import (
	"context"
	"fmt"

	"github.com/bevzick/Infinitune/services/catalog-service/internal/metadata"
	"github.com/google/uuid"
)

func (repository *MetadataRepository) ExistingAlbumIDs(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]struct{}, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT a.id
		FROM catalog.albums AS a
		WHERE a.id = ANY($1::uuid[])
		  AND `+activeAlbumPredicate+`
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("query existing album ids: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]struct{}, len(ids))
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan existing album id: %w", err)
		}
		result[id] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing album ids: %w", err)
	}
	return result, nil
}

func (repository *MetadataRepository) TracksByIDs(
	ctx context.Context,
	ids []uuid.UUID,
) ([]metadata.Track, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT
			id, album_id, title, duration_ms, track_number, disc_number, explicit,
			release_date, media_asset_id, content_id, created_at, updated_at
		FROM catalog.tracks
		WHERE id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("query tracks by ids: %w", err)
	}

	byID := make(map[uuid.UUID]metadata.Track, len(ids))
	foundIDs := make([]uuid.UUID, 0, len(ids))
	for rows.Next() {
		var track metadata.Track
		if err = scanTrack(rows, &track); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan track by id: %w", err)
		}
		byID[track.ID] = track
		foundIDs = append(foundIDs, track.ID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, fmt.Errorf("iterate tracks by ids: %w", err)
	}

	artists, err := repository.trackArtistsByTrackIDs(ctx, foundIDs)
	if err != nil {
		return nil, fmt.Errorf("query batch track artists: %w", err)
	}
	genres, err := repository.trackGenresByTrackIDs(ctx, foundIDs)
	if err != nil {
		return nil, fmt.Errorf("query batch track genres: %w", err)
	}
	moods, err := repository.trackMoodsByTrackIDs(ctx, foundIDs)
	if err != nil {
		return nil, fmt.Errorf("query batch track moods: %w", err)
	}
	for id, track := range byID {
		track.Artists = artists[id]
		track.Genres = genres[id]
		track.Moods = moods[id]
		if track.Artists == nil {
			track.Artists = make([]metadata.TrackArtist, 0)
		}
		if track.Genres == nil {
			track.Genres = make([]metadata.Genre, 0)
		}
		if track.Moods == nil {
			track.Moods = make([]metadata.TrackMood, 0)
		}
		byID[id] = track
	}

	result := make([]metadata.Track, 0, len(foundIDs))
	for _, id := range ids {
		if track, exists := byID[id]; exists {
			result = append(result, track)
		}
	}
	return result, nil
}

func (repository *MetadataRepository) ExistingTrackIDs(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]struct{}, error) {
	rows, err := repository.database.Query(ctx, `
		SELECT t.id
		FROM catalog.tracks AS t
		WHERE t.id = ANY($1::uuid[])
		  AND `+activeTrackPredicate+`
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("query existing track ids: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]struct{}, len(ids))
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan existing track id: %w", err)
		}
		result[id] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing track ids: %w", err)
	}
	return result, nil
}
