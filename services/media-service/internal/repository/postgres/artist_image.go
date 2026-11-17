package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bevzick/Infinitune/services/media-service/internal/artistimage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repository *MediaRepository) Replace(
	ctx context.Context,
	image artistimage.Image,
) (*artistimage.Image, error) {
	var previousObjectKey *string
	err := repository.database.QueryRow(ctx, `
		WITH previous AS MATERIALIZED (
			SELECT object_key FROM media.artist_images
			WHERE creator_id = $1
			FOR UPDATE
		), saved AS (
		INSERT INTO media.artist_images (
			creator_id, catalog_artist_id, object_key, mime_type,
			size_bytes, updated_by_user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (creator_id) DO UPDATE SET
			catalog_artist_id = EXCLUDED.catalog_artist_id,
			object_key = EXCLUDED.object_key,
			mime_type = EXCLUDED.mime_type,
			size_bytes = EXCLUDED.size_bytes,
			updated_by_user_id = EXCLUDED.updated_by_user_id,
			updated_at = clock_timestamp()
		RETURNING created_at, updated_at
		)
		SELECT (SELECT object_key FROM previous), created_at, updated_at
		FROM saved
	`, image.CreatorID, image.CatalogArtistID, image.ObjectKey, image.MIMEType,
		image.SizeBytes, image.UpdatedByUserID,
	).Scan(&previousObjectKey, &image.CreatedAt, &image.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("replace artist image: %w", err)
	}
	if previousObjectKey == nil {
		return nil, nil
	}
	return &artistimage.Image{ObjectKey: *previousObjectKey}, nil
}

func (repository *MediaRepository) ByCatalogArtistID(
	ctx context.Context,
	artistID uuid.UUID,
) (artistimage.Image, error) {
	var image artistimage.Image
	err := repository.database.QueryRow(ctx, `
		SELECT creator_id, catalog_artist_id, object_key, mime_type, size_bytes,
		       updated_by_user_id, created_at, updated_at
		FROM media.artist_images
		WHERE catalog_artist_id = $1
	`, artistID).Scan(
		&image.CreatorID, &image.CatalogArtistID, &image.ObjectKey, &image.MIMEType,
		&image.SizeBytes, &image.UpdatedByUserID, &image.CreatedAt, &image.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return artistimage.Image{}, artistimage.ErrNotFound
	}
	if err != nil {
		return artistimage.Image{}, fmt.Errorf("find artist image: %w", err)
	}
	return image, nil
}
