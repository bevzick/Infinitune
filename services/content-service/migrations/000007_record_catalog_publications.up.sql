ALTER TABLE content.releases
    ADD COLUMN published_at timestamptz;

ALTER TABLE content.releases
    ADD CONSTRAINT releases_published_at_status_check
    CHECK ((status = 'published') = (published_at IS NOT NULL));

CREATE INDEX releases_published_at_idx
    ON content.releases (published_at DESC, id)
    WHERE status = 'published';

CREATE UNIQUE INDEX releases_catalog_album_id_key
    ON content.releases (catalog_album_id)
    WHERE catalog_album_id IS NOT NULL;

CREATE UNIQUE INDEX release_tracks_catalog_track_id_key
    ON content.release_tracks (catalog_track_id)
    WHERE catalog_track_id IS NOT NULL;
