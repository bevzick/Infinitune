CREATE UNIQUE INDEX media_asset_readiness_external_track_key
    ON content.media_asset_readiness (content_track_id)
    WHERE source_upload_id IS NULL;

COMMENT ON INDEX content.media_asset_readiness_external_track_key IS
    'One ready externally imported media asset per Content release track.';
