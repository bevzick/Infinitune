CREATE TYPE content.track_right_type AS ENUM (
    'streaming',
    'download',
    'distribution'
);

CREATE TABLE content.track_rights (
    id uuid PRIMARY KEY,
    release_track_id uuid NOT NULL,
    territory_code varchar(3) NOT NULL,
    rights_type content.track_right_type NOT NULL,
    valid_from date NOT NULL,
    valid_until date,
    granted_by_user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT track_rights_territory_code_check CHECK (
        territory_code = '001' OR territory_code ~ '^[A-Z]{2}$'
    ),
    CONSTRAINT track_rights_validity_check CHECK (
        valid_until IS NULL OR valid_until >= valid_from
    ),
    CONSTRAINT track_rights_release_track_id_fkey
        FOREIGN KEY (release_track_id)
        REFERENCES content.release_tracks (id)
        ON UPDATE NO ACTION
        ON DELETE CASCADE,
    CONSTRAINT track_rights_track_territory_type_key
        UNIQUE (release_track_id, territory_code, rights_type)
);

COMMENT ON COLUMN content.track_rights.territory_code IS
    'ISO 3166-1 alpha-2 territory code, or UN M49 001 for worldwide rights.';
COMMENT ON COLUMN content.track_rights.granted_by_user_id IS
    'Cross-service User UUID of the verified creator member who set the rights.';

CREATE INDEX track_rights_release_track_validity_idx
    ON content.track_rights (release_track_id, valid_from, valid_until, id);

