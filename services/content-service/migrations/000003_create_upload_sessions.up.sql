CREATE TYPE content.upload_status AS ENUM (
    'pending',
    'uploading',
    'completed',
    'failed'
);

CREATE TABLE content.uploads (
    id uuid PRIMARY KEY,
    release_track_id uuid NOT NULL,
    requested_by_user_id uuid NOT NULL,
    idempotency_key uuid NOT NULL,
    object_key text NOT NULL,
    original_name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    size_bytes bigint NOT NULL,
    status content.upload_status NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uploads_release_track_id_fkey
        FOREIGN KEY (release_track_id)
        REFERENCES content.release_tracks (id)
        ON UPDATE NO ACTION
        ON DELETE CASCADE,
    CONSTRAINT uploads_object_key_key UNIQUE (object_key),
    CONSTRAINT uploads_request_idempotency_key
        UNIQUE (release_track_id, requested_by_user_id, idempotency_key),
    CONSTRAINT uploads_object_key_not_blank CHECK (btrim(object_key) <> ''),
    CONSTRAINT uploads_original_name_not_blank CHECK (btrim(original_name) <> ''),
    CONSTRAINT uploads_size_check CHECK (size_bytes > 0 AND size_bytes <= 524288000),
    CONSTRAINT uploads_mime_type_check CHECK (
        mime_type IN (
            'audio/wav',
            'audio/x-wav',
            'audio/flac',
            'audio/x-flac',
            'audio/mpeg',
            'audio/mp4',
            'audio/x-m4a'
        )
    )
);

COMMENT ON COLUMN content.uploads.release_track_id IS
    'Corrected spelling of release_tack_id from the source draft schema.';
COMMENT ON COLUMN content.uploads.requested_by_user_id IS
    'Cross-service Auth user UUID; intentionally has no database foreign key.';
COMMENT ON COLUMN content.uploads.object_key IS
    'Server-generated private object key; not a public object URL.';

CREATE INDEX uploads_release_track_created_at_idx
    ON content.uploads (release_track_id, created_at DESC, id);
CREATE INDEX uploads_status_updated_at_idx
    ON content.uploads (status, updated_at, id);
