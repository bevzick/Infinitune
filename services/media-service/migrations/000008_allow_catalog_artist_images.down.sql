DELETE FROM media.artist_images WHERE creator_id IS NULL;

DROP INDEX media.artist_images_creator_id_key;

ALTER TABLE media.artist_images
    DROP CONSTRAINT artist_images_pkey,
    ALTER COLUMN creator_id SET NOT NULL,
    ADD CONSTRAINT artist_images_pkey PRIMARY KEY (creator_id),
    ADD CONSTRAINT artist_images_catalog_artist_id_key UNIQUE (catalog_artist_id);

