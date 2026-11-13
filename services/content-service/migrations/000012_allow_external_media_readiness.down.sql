DELETE FROM content.media_asset_readiness
WHERE source_upload_id IS NULL;

ALTER TABLE content.media_asset_readiness
    ALTER COLUMN source_upload_id SET NOT NULL;
