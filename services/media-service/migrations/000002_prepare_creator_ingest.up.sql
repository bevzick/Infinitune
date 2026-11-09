ALTER TABLE media.media_assets
    ALTER COLUMN track_id DROP NOT NULL,
    ADD COLUMN content_track_id uuid,
    ADD CONSTRAINT media_assets_source_upload_id_key UNIQUE (source_upload_id),
    ADD CONSTRAINT media_assets_track_reference_check
        CHECK (track_id IS NOT NULL OR content_track_id IS NOT NULL);

COMMENT ON COLUMN media.media_assets.track_id IS
    'Nullable cross-service Catalog track UUID assigned during publication.';
COMMENT ON COLUMN media.media_assets.content_track_id IS
    'Cross-service Content release-track UUID; intentionally has no database foreign key.';
