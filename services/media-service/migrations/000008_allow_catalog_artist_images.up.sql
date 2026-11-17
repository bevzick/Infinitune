ALTER TABLE media.artist_images
    DROP CONSTRAINT artist_images_pkey,
    DROP CONSTRAINT artist_images_catalog_artist_id_key,
    ALTER COLUMN creator_id DROP NOT NULL,
    ADD CONSTRAINT artist_images_pkey PRIMARY KEY (catalog_artist_id);

CREATE UNIQUE INDEX artist_images_creator_id_key
    ON media.artist_images (creator_id)
    WHERE creator_id IS NOT NULL;

COMMENT ON COLUMN media.artist_images.creator_id IS
    'Optional Content creator UUID. Imported catalog artists may not have a creator account.';

