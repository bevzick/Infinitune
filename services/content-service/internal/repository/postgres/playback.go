package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/bevzick/Infinitune/services/content-service/internal/playback"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PlaybackRepository struct {
	database *pgxpool.Pool
}

func NewPlaybackRepository(database *pgxpool.Pool) *PlaybackRepository {
	return &PlaybackRepository{database: database}
}

func (repository *PlaybackRepository) IsTrackPlayable(
	ctx context.Context,
	trackID uuid.UUID,
	at time.Time,
) (bool, error) {
	var allowed bool
	err := repository.database.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM content.release_tracks AS track
			JOIN content.releases AS release ON release.id = track.release_id
			JOIN content.track_rights AS track_right
			  ON track_right.release_track_id = track.id
			WHERE track.id = $1
			  AND track.catalog_track_id IS NOT NULL
			  AND release.status = 'published'
			  AND release.published_at IS NOT NULL
			  AND release.release_date <= $2::date
			  AND track_right.territory_code = '001'
			  AND track_right.rights_type = 'streaming'
			  AND track_right.valid_from <= $2::date
			  AND (track_right.valid_until IS NULL OR track_right.valid_until >= $2::date)
		)
	`, trackID, at.UTC().Format(time.DateOnly)).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("query track playback access: %w", err)
	}
	return allowed, nil
}

var _ playback.Store = (*PlaybackRepository)(nil)
