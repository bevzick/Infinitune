package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/playlist-service/internal/playlist"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (repository *PlaylistRepository) Tracks(
	ctx context.Context,
	playlistID uuid.UUID,
	page playlist.PageRequest,
) (playlist.TrackPage, error) {
	var total int64
	if err := repository.database.QueryRow(ctx, `
		SELECT count(*) FROM playlist.playlist_tracks WHERE playlist_id = $1
	`, playlistID).Scan(&total); err != nil {
		return playlist.TrackPage{}, fmt.Errorf("count playlist tracks: %w", err)
	}
	rows, err := repository.database.Query(ctx, `
		SELECT playlist_id, track_id, position, added_by, added_at
		FROM playlist.playlist_tracks
		WHERE playlist_id = $1
		ORDER BY position
		LIMIT $2 OFFSET $3
	`, playlistID, page.Limit, page.Offset)
	if err != nil {
		return playlist.TrackPage{}, fmt.Errorf("query playlist tracks: %w", err)
	}
	defer rows.Close()
	items, err := scanTracks(rows)
	if err != nil {
		return playlist.TrackPage{}, err
	}
	return playlist.TrackPage{
		Items: items, Limit: page.Limit, Offset: page.Offset, Total: total,
	}, nil
}

func (repository *PlaylistRepository) AddTrack(
	ctx context.Context,
	playlistID uuid.UUID,
	ownerID uuid.UUID,
	trackID uuid.UUID,
	addedBy uuid.UUID,
) (playlist.Track, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return playlist.Track{}, fmt.Errorf("begin add track transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockManualPlaylist(ctx, tx, playlistID, ownerID); err != nil {
		return playlist.Track{}, err
	}

	var position int
	if err = tx.QueryRow(ctx, `
		SELECT COALESCE(max(position), 0) + 1
		FROM playlist.playlist_tracks
		WHERE playlist_id = $1
	`, playlistID).Scan(&position); err != nil {
		return playlist.Track{}, fmt.Errorf("find next playlist position: %w", err)
	}
	var added playlist.Track
	err = tx.QueryRow(ctx, `
		INSERT INTO playlist.playlist_tracks (
			playlist_id, track_id, position, added_by
		)
		VALUES ($1, $2, $3, $4)
		RETURNING playlist_id, track_id, position, added_by, added_at
	`, playlistID, trackID, position, addedBy).Scan(
		&added.PlaylistID,
		&added.TrackID,
		&added.Position,
		&added.AddedBy,
		&added.AddedAt,
	)
	if err != nil {
		if isConstraint(err, "playlist_tracks_pkey") {
			return playlist.Track{}, playlist.ErrTrackExists
		}
		return playlist.Track{}, fmt.Errorf("insert playlist track: %w", err)
	}
	if err = touchPlaylist(ctx, tx, playlistID); err != nil {
		return playlist.Track{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return playlist.Track{}, fmt.Errorf("commit add track transaction: %w", err)
	}
	return added, nil
}

func (repository *PlaylistRepository) RemoveTrack(
	ctx context.Context,
	playlistID uuid.UUID,
	ownerID uuid.UUID,
	trackID uuid.UUID,
) error {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove track transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockManualPlaylist(ctx, tx, playlistID, ownerID); err != nil {
		return err
	}
	current, err := lockedTracks(ctx, tx, playlistID)
	if err != nil {
		return err
	}
	remaining := make([]playlist.Track, 0, len(current))
	found := false
	for _, track := range current {
		if track.TrackID == trackID {
			found = true
			continue
		}
		remaining = append(remaining, track)
	}
	if !found {
		return playlist.ErrTrackNotFound
	}
	if err = replaceTracks(ctx, tx, playlistID, remaining); err != nil {
		return fmt.Errorf("compact playlist positions: %w", err)
	}
	if err = touchPlaylist(ctx, tx, playlistID); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit remove track transaction: %w", err)
	}
	return nil
}

func (repository *PlaylistRepository) ReorderTracks(
	ctx context.Context,
	playlistID uuid.UUID,
	ownerID uuid.UUID,
	trackIDs []uuid.UUID,
) ([]playlist.Track, error) {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin reorder tracks transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockManualPlaylist(ctx, tx, playlistID, ownerID); err != nil {
		return nil, err
	}
	current, err := lockedTracks(ctx, tx, playlistID)
	if err != nil {
		return nil, err
	}
	ordered, changed, err := orderedTracks(current, trackIDs)
	if err != nil {
		return nil, err
	}
	if !changed {
		return current, nil
	}
	if err = replaceTracks(ctx, tx, playlistID, ordered); err != nil {
		return nil, fmt.Errorf("replace playlist track order: %w", err)
	}
	if err = touchPlaylist(ctx, tx, playlistID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit reorder tracks transaction: %w", err)
	}
	for index := range ordered {
		ordered[index].Position = index + 1
	}
	return ordered, nil
}

func lockManualPlaylist(
	ctx context.Context,
	tx pgx.Tx,
	playlistID uuid.UUID,
	ownerID uuid.UUID,
) error {
	var playlistType string
	err := tx.QueryRow(ctx, `
		SELECT playlist_type
		FROM playlist.playlists
		WHERE id = $1 AND owner_id = $2
		FOR UPDATE
	`, playlistID, ownerID).Scan(&playlistType)
	if errors.Is(err, pgx.ErrNoRows) {
		return playlist.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock playlist: %w", err)
	}
	if playlist.Type(playlistType) != playlist.TypeManual {
		return playlist.ErrManualOnly
	}
	return nil
}

func lockedTracks(
	ctx context.Context,
	tx pgx.Tx,
	playlistID uuid.UUID,
) ([]playlist.Track, error) {
	rows, err := tx.Query(ctx, `
		SELECT playlist_id, track_id, position, added_by, added_at
		FROM playlist.playlist_tracks
		WHERE playlist_id = $1
		ORDER BY position
		FOR UPDATE
	`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("lock playlist tracks: %w", err)
	}
	defer rows.Close()
	tracks, err := scanTracks(rows)
	if err != nil {
		return nil, fmt.Errorf("scan locked playlist tracks: %w", err)
	}
	return tracks, nil
}

func scanTracks(rows pgx.Rows) ([]playlist.Track, error) {
	result := make([]playlist.Track, 0)
	for rows.Next() {
		var track playlist.Track
		if err := rows.Scan(
			&track.PlaylistID,
			&track.TrackID,
			&track.Position,
			&track.AddedBy,
			&track.AddedAt,
		); err != nil {
			return nil, fmt.Errorf("scan playlist track: %w", err)
		}
		result = append(result, track)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate playlist tracks: %w", err)
	}
	return result, nil
}

func orderedTracks(
	current []playlist.Track,
	trackIDs []uuid.UUID,
) ([]playlist.Track, bool, error) {
	if len(current) != len(trackIDs) {
		return nil, false, invalidTrackOrder()
	}
	byID := make(map[uuid.UUID]playlist.Track, len(current))
	for _, track := range current {
		byID[track.TrackID] = track
	}
	ordered := make([]playlist.Track, 0, len(trackIDs))
	changed := false
	for index, trackID := range trackIDs {
		track, exists := byID[trackID]
		if !exists {
			return nil, false, invalidTrackOrder()
		}
		if current[index].TrackID != trackID {
			changed = true
		}
		ordered = append(ordered, track)
		delete(byID, trackID)
	}
	if len(byID) != 0 {
		return nil, false, invalidTrackOrder()
	}
	return ordered, changed, nil
}

func replaceTracks(
	ctx context.Context,
	tx pgx.Tx,
	playlistID uuid.UUID,
	tracks []playlist.Track,
) error {
	if _, err := tx.Exec(ctx, `
		DELETE FROM playlist.playlist_tracks WHERE playlist_id = $1
	`, playlistID); err != nil {
		return fmt.Errorf("delete old playlist track order: %w", err)
	}
	for index, track := range tracks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO playlist.playlist_tracks (
				playlist_id, track_id, position, added_by, added_at
			)
			VALUES ($1, $2, $3, $4, $5)
		`, playlistID, track.TrackID, index+1, track.AddedBy, track.AddedAt); err != nil {
			return fmt.Errorf("insert reordered playlist track: %w", err)
		}
	}
	return nil
}

func touchPlaylist(ctx context.Context, tx pgx.Tx, playlistID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `
		UPDATE playlist.playlists SET updated_at = now() WHERE id = $1
	`, playlistID); err != nil {
		return fmt.Errorf("update playlist timestamp: %w", err)
	}
	return nil
}

func invalidTrackOrder() error {
	return fmt.Errorf("%w: track order must contain every playlist track exactly once",
		playlist.ErrInvalidArgument)
}

func isConstraint(err error, name string) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.ConstraintName == name
}

var _ playlist.TrackStore = (*PlaylistRepository)(nil)
