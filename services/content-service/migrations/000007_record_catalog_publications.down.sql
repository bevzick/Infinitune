DROP INDEX IF EXISTS content.releases_published_at_idx;
DROP INDEX IF EXISTS content.releases_catalog_album_id_key;
DROP INDEX IF EXISTS content.release_tracks_catalog_track_id_key;

ALTER TABLE content.releases
    DROP CONSTRAINT IF EXISTS releases_published_at_status_check,
    DROP COLUMN IF EXISTS published_at;
