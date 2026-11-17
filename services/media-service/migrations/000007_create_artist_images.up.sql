CREATE TABLE media.artist_images (
    creator_id uuid PRIMARY KEY,
    catalog_artist_id uuid NOT NULL UNIQUE,
    object_key text NOT NULL UNIQUE,
    mime_type varchar(32) NOT NULL,
    size_bytes bigint NOT NULL,
    updated_by_user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT artist_images_object_key_not_blank CHECK (btrim(object_key) <> ''),
    CONSTRAINT artist_images_mime_type_check
        CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/webp')),
    CONSTRAINT artist_images_size_check CHECK (size_bytes BETWEEN 1 AND 10485760)
);

COMMENT ON COLUMN media.artist_images.catalog_artist_id IS
    'Stable Catalog artist UUID derived from the Content creator UUID.';

