CREATE TABLE catalog.publications (
    source_release_id uuid PRIMARY KEY,
    payload_fingerprint char(64) NOT NULL,
    artist_id uuid NOT NULL,
    album_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT publications_artist_id_fkey
        FOREIGN KEY (artist_id) REFERENCES catalog.artists (id)
        ON UPDATE NO ACTION ON DELETE NO ACTION,
    CONSTRAINT publications_album_id_fkey
        FOREIGN KEY (album_id) REFERENCES catalog.albums (id)
        ON UPDATE NO ACTION ON DELETE NO ACTION,
    CONSTRAINT publications_fingerprint_format
        CHECK (payload_fingerprint ~ '^[0-9a-f]{64}$')
);

CREATE TABLE catalog.publication_tracks (
    source_release_id uuid NOT NULL,
    content_track_id uuid NOT NULL,
    catalog_track_id uuid NOT NULL,
    PRIMARY KEY (source_release_id, content_track_id),
    CONSTRAINT publication_tracks_catalog_track_id_key UNIQUE (catalog_track_id),
    CONSTRAINT publication_tracks_source_release_id_fkey
        FOREIGN KEY (source_release_id) REFERENCES catalog.publications (source_release_id)
        ON UPDATE NO ACTION ON DELETE CASCADE,
    CONSTRAINT publication_tracks_catalog_track_id_fkey
        FOREIGN KEY (catalog_track_id) REFERENCES catalog.tracks (id)
        ON UPDATE NO ACTION ON DELETE NO ACTION
);

CREATE UNIQUE INDEX tracks_content_id_key ON catalog.tracks (content_id);

COMMENT ON TABLE catalog.publications IS
    'Idempotency and audit records for approved releases received from Content Service.';
