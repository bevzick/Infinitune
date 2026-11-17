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
	tx, err := repository.database.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin replacing artist image: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize replacements even when the artist does not have an image yet.
	// A row lock alone cannot protect that case and could leave an overwritten
	// object behind when two first uploads race each other.
	if _, err = tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))
	`, image.CatalogArtistID); err != nil {
		return nil, fmt.Errorf("lock artist image: %w", err)
	}

	var previousObjectKey *string
	var existingObjectKey string
	err = tx.QueryRow(ctx, `
		SELECT object_key FROM media.artist_images
		WHERE catalog_artist_id = $1
	`, image.CatalogArtistID).Scan(&existingObjectKey)
	if err == nil {
		previousObjectKey = &existingObjectKey
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find previous artist image: %w", err)
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO media.artist_images (
			creator_id, catalog_artist_id, object_key, mime_type,
			size_bytes, updated_by_user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (catalog_artist_id) DO UPDATE SET
			creator_id = COALESCE(EXCLUDED.creator_id, artist_images.creator_id),
			object_key = EXCLUDED.object_key,
			mime_type = EXCLUDED.mime_type,
			size_bytes = EXCLUDED.size_bytes,
			updated_by_user_id = EXCLUDED.updated_by_user_id,
			updated_at = clock_timestamp()
		RETURNING created_at, updated_at
	`, image.CreatorID, image.CatalogArtistID, image.ObjectKey, image.MIMEType,
		image.SizeBytes, image.UpdatedByUserID,
	).Scan(&image.CreatedAt, &image.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("replace artist image: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit artist image replacement: %w", err)
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
	var creatorID *uuid.UUID
	err := repository.database.QueryRow(ctx, `
		SELECT creator_id, catalog_artist_id, object_key, mime_type, size_bytes,
		       updated_by_user_id, created_at, updated_at
		FROM media.artist_images
		WHERE catalog_artist_id = $1
	`, artistID).Scan(
		&creatorID, &image.CatalogArtistID, &image.ObjectKey, &image.MIMEType,
		&image.SizeBytes, &image.UpdatedByUserID, &image.CreatedAt, &image.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return artistimage.Image{}, artistimage.ErrNotFound
	}
	if err != nil {
		return artistimage.Image{}, fmt.Errorf("find artist image: %w", err)
	}
	image.CreatorID = creatorID
	return image, nil
}
