ALTER TABLE content.media_asset_readiness
    ALTER COLUMN source_upload_id DROP NOT NULL;

COMMENT ON COLUMN content.media_asset_readiness.source_upload_id IS
    'Content upload UUID for local files; NULL for authorized external imports.';
