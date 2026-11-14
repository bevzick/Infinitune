CREATE TABLE content.media_asset_readiness (
    media_asset_id uuid PRIMARY KEY,
    source_upload_id uuid NOT NULL UNIQUE,
    content_track_id uuid NOT NULL,
    duration_ms integer NOT NULL,
    event_id uuid NOT NULL UNIQUE,
    occurred_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT media_asset_readiness_duration_positive CHECK (duration_ms > 0)
);

CREATE INDEX media_asset_readiness_content_track_idx
    ON content.media_asset_readiness (content_track_id);

COMMENT ON TABLE content.media_asset_readiness IS
    'Kafka projection of ready creator media assets used by submission and publication.';
