DROP INDEX IF EXISTS media.media_assets_external_content_track_key;

DELETE FROM media.event_outbox
WHERE aggregate_id IN (
    SELECT id FROM media.media_assets WHERE source_upload_id IS NULL
);

DELETE FROM media.processing_jobs
WHERE media_asset_id IN (
    SELECT id FROM media.media_assets WHERE source_upload_id IS NULL
);

DELETE FROM media.audio_variants
WHERE media_asset_id IN (
    SELECT id FROM media.media_assets WHERE source_upload_id IS NULL
);

DELETE FROM media.media_assets
WHERE source_upload_id IS NULL;

ALTER TABLE media.media_assets
    ALTER COLUMN source_upload_id SET NOT NULL;
