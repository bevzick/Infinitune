CREATE TABLE content.cover_uploads (
    id uuid PRIMARY KEY,
    release_id uuid NOT NULL REFERENCES content.releases(id) ON DELETE CASCADE,
    requested_by_user_id uuid NOT NULL,
    idempotency_key uuid NOT NULL,
    object_key text NOT NULL,
    original_name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    size_bytes bigint NOT NULL,
    status content.upload_status NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT cover_uploads_request_key UNIQUE (release_id, requested_by_user_id, idempotency_key),
    CONSTRAINT cover_uploads_object_key_key UNIQUE (object_key),
    CONSTRAINT cover_uploads_key_not_blank CHECK (btrim(object_key) <> ''),
    CONSTRAINT cover_uploads_name_not_blank CHECK (btrim(original_name) <> ''),
    CONSTRAINT cover_uploads_size_check CHECK (size_bytes BETWEEN 1 AND 10485760),
    CONSTRAINT cover_uploads_mime_check CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/webp'))
);

COMMENT ON COLUMN content.cover_uploads.requested_by_user_id IS
    'Cross-service Auth UUID; no database foreign key.';
COMMENT ON COLUMN content.cover_uploads.object_key IS
    'Server-generated immutable private object key; never returned by the browser session API.';

CREATE INDEX cover_uploads_release_created_at_idx
    ON content.cover_uploads (release_id, created_at DESC, id);
CREATE INDEX cover_uploads_status_updated_at_idx
    ON content.cover_uploads (status, updated_at, id);
