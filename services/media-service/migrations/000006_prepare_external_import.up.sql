ALTER TABLE media.media_assets
    ALTER COLUMN source_upload_id DROP NOT NULL;

CREATE UNIQUE INDEX media_assets_external_content_track_key
    ON media.media_assets (content_track_id)
    WHERE source_upload_id IS NULL;

COMMENT ON INDEX media.media_assets_external_content_track_key IS
    'Allows one idempotent provider import for a Content release track.';
