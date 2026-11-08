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

type playlistDatabase interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PlaylistRepository struct {
	database playlistDatabase
}

func NewPlaylistRepository(database playlistDatabase) *PlaylistRepository {
	return &PlaylistRepository{database: database}
}

func (repository *PlaylistRepository) Create(
	ctx context.Context,
	value playlist.Playlist,
) (playlist.Playlist, error) {
	return scanPlaylist(repository.database.QueryRow(ctx, `
		INSERT INTO playlist.playlists (
			id, owner_id, name, description, playlist_type, visibility, rule_set_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id, owner_id, name, description, playlist_type, visibility,
			rule_set_id, created_at, updated_at
	`, value.ID, value.OwnerID, value.Name, value.Description, value.Type,
		value.Visibility, value.RuleSetID))
}

func (repository *PlaylistRepository) ByID(
	ctx context.Context,
	id uuid.UUID,
) (playlist.Playlist, error) {
	result, err := scanPlaylist(repository.database.QueryRow(ctx, `
		SELECT
			id, owner_id, name, description, playlist_type, visibility,
			rule_set_id, created_at, updated_at
		FROM playlist.playlists
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return playlist.Playlist{}, playlist.ErrNotFound
	}
	if err != nil {
		return playlist.Playlist{}, fmt.Errorf("query playlist by id: %w", err)
	}
	return result, nil
}

func (repository *PlaylistRepository) Update(
	ctx context.Context,
	value playlist.Playlist,
) (playlist.Playlist, error) {
	result, err := scanPlaylist(repository.database.QueryRow(ctx, `
		UPDATE playlist.playlists
		SET name = $3, description = $4, visibility = $5, updated_at = now()
		WHERE id = $1 AND owner_id = $2
		RETURNING
			id, owner_id, name, description, playlist_type, visibility,
			rule_set_id, created_at, updated_at
	`, value.ID, value.OwnerID, value.Name, value.Description, value.Visibility))
	if errors.Is(err, pgx.ErrNoRows) {
		return playlist.Playlist{}, playlist.ErrNotFound
	}
	if err != nil {
		return playlist.Playlist{}, fmt.Errorf("update playlist: %w", err)
	}
	return result, nil
}

func (repository *PlaylistRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	ownerID uuid.UUID,
) error {
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete playlist transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `
		DELETE FROM playlist.playlist_tracks
		WHERE playlist_id = $1
	`, id); err != nil {
		return fmt.Errorf("delete playlist tracks: %w", err)
	}
	commandTag, err := tx.Exec(ctx, `
		DELETE FROM playlist.playlists
		WHERE id = $1 AND owner_id = $2
	`, id, ownerID)
	if err != nil {
		return fmt.Errorf("delete playlist: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return playlist.ErrNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete playlist transaction: %w", err)
	}
	return nil
}

func scanPlaylist(row pgx.Row) (playlist.Playlist, error) {
	var result playlist.Playlist
	var playlistType string
	var visibility string
	err := row.Scan(
		&result.ID,
		&result.OwnerID,
		&result.Name,
		&result.Description,
		&playlistType,
		&visibility,
		&result.RuleSetID,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	result.Type = playlist.Type(playlistType)
	result.Visibility = playlist.Visibility(visibility)
	return result, err
}

var _ playlist.Store = (*PlaylistRepository)(nil)
