ALTER TABLE media.media_assets
    DROP CONSTRAINT IF EXISTS media_assets_track_reference_check,
    DROP CONSTRAINT IF EXISTS media_assets_source_upload_id_key,
    DROP COLUMN IF EXISTS content_track_id,
    ALTER COLUMN track_id SET NOT NULL;

COMMENT ON COLUMN media.media_assets.track_id IS
    'Cross-service Catalog track UUID; intentionally has no database foreign key.';
