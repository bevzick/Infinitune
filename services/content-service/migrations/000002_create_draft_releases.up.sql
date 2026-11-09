CREATE TYPE content.release_type AS ENUM (
    'single',
    'ep',
    'album',
    'compilation'
);

CREATE TYPE content.release_status AS ENUM (
    'draft',
    'uploading',
    'processing',
    'pending_review',
    'approved',
    'rejected',
    'published'
);

CREATE TABLE content.releases (
    id uuid PRIMARY KEY,
    creator_id uuid NOT NULL,
    title varchar(200) NOT NULL,
    release_type content.release_type NOT NULL,
    release_date date NOT NULL,
    status content.release_status NOT NULL DEFAULT 'draft',
    cover_object_key text,
    catalog_album_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT releases_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT releases_cover_object_key_not_blank
        CHECK (cover_object_key IS NULL OR btrim(cover_object_key) <> ''),
    CONSTRAINT releases_creator_id_fkey
        FOREIGN KEY (creator_id)
        REFERENCES content.creator_accounts (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON COLUMN content.releases.catalog_album_id IS
    'Cross-service Catalog album UUID; intentionally has no database foreign key.';
COMMENT ON COLUMN content.releases.cover_object_key IS
    'Nullable while a release is an incomplete draft.';

CREATE TABLE content.release_tracks (
    id uuid PRIMARY KEY,
    release_id uuid NOT NULL,
    title varchar(200) NOT NULL,
    track_number integer NOT NULL,
    disc_number integer NOT NULL DEFAULT 1,
    explicit boolean NOT NULL DEFAULT false,
    catalog_track_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT release_tracks_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT release_tracks_track_number_positive CHECK (track_number > 0),
    CONSTRAINT release_tracks_disc_number_positive CHECK (disc_number > 0),
    CONSTRAINT release_tracks_release_id_fkey
        FOREIGN KEY (release_id)
        REFERENCES content.releases (id)
        ON UPDATE NO ACTION
        ON DELETE CASCADE,
    CONSTRAINT release_tracks_position_key
        UNIQUE (release_id, disc_number, track_number)
);

COMMENT ON COLUMN content.release_tracks.catalog_track_id IS
    'Cross-service Catalog track UUID; intentionally has no database foreign key.';

CREATE INDEX releases_creator_status_updated_at_idx
    ON content.releases (creator_id, status, updated_at DESC, id);
