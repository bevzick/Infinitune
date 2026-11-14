CREATE SCHEMA playlist;

CREATE TYPE playlist.playlist_type AS ENUM (
    'manual',
    'dynamic'
);

CREATE TYPE playlist.playlist_visibility AS ENUM (
    'private',
    'unlisted',
    'public'
);

CREATE TABLE playlist.playlists (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    name varchar(150) NOT NULL,
    description text NOT NULL DEFAULT '',
    playlist_type playlist.playlist_type NOT NULL DEFAULT 'manual',
    visibility playlist.playlist_visibility NOT NULL DEFAULT 'private',
    rule_set_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT playlists_name_not_blank CHECK (btrim(name) <> '')
);

COMMENT ON COLUMN playlist.playlists.owner_id IS
    'Cross-service Auth user UUID; intentionally has no database foreign key.';
COMMENT ON COLUMN playlist.playlists.rule_set_id IS
    'Cross-domain Rule Engine UUID; intentionally has no database foreign key.';

CREATE TABLE playlist.playlist_tracks (
    playlist_id uuid NOT NULL,
    track_id uuid NOT NULL,
    position integer NOT NULL,
    added_by uuid NOT NULL,
    added_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (playlist_id, track_id),
    CONSTRAINT playlist_tracks_position_positive CHECK (position > 0),
    CONSTRAINT playlist_tracks_playlist_position_key UNIQUE (playlist_id, position),
    CONSTRAINT playlist_tracks_playlist_id_fkey
        FOREIGN KEY (playlist_id)
        REFERENCES playlist.playlists (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON COLUMN playlist.playlist_tracks.track_id IS
    'Cross-service Catalog track UUID; intentionally has no database foreign key.';
COMMENT ON COLUMN playlist.playlist_tracks.added_by IS
    'Cross-service Auth user UUID; intentionally has no database foreign key.';

CREATE INDEX playlists_owner_id_updated_at_idx
    ON playlist.playlists (owner_id, updated_at DESC, id);
CREATE INDEX playlists_visibility_updated_at_idx
    ON playlist.playlists (visibility, updated_at DESC, id);
CREATE INDEX playlist_tracks_track_id_idx
    ON playlist.playlist_tracks (track_id);
