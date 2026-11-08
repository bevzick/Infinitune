CREATE SCHEMA media;

CREATE TYPE media.media_status AS ENUM (
    'uploaded',
    'processing',
    'ready',
    'failed'
);

CREATE TYPE media.processing_status AS ENUM (
    'pending',
    'running',
    'completed',
    'failed',
    'cancelled'
);

CREATE TABLE media.media_assets (
    id uuid PRIMARY KEY,
    track_id uuid NOT NULL,
    source_upload_id uuid NOT NULL,
    original_object_key text NOT NULL,
    original_filename text NOT NULL,
    duration_ms integer,
    status media.media_status NOT NULL DEFAULT 'uploaded',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT media_assets_object_key_not_blank CHECK (btrim(original_object_key) <> ''),
    CONSTRAINT media_assets_filename_not_blank CHECK (btrim(original_filename) <> ''),
    CONSTRAINT media_assets_duration_ms_positive CHECK (duration_ms IS NULL OR duration_ms > 0),
    CONSTRAINT media_assets_original_object_key_key UNIQUE (original_object_key)
);

COMMENT ON COLUMN media.media_assets.track_id IS
    'Cross-service Catalog track UUID; intentionally has no database foreign key.';
COMMENT ON COLUMN media.media_assets.source_upload_id IS
    'Cross-service Content upload UUID; intentionally has no database foreign key.';

CREATE TABLE media.audio_variants (
    id uuid PRIMARY KEY,
    media_asset_id uuid NOT NULL,
    object_key text NOT NULL,
    codec varchar(30) NOT NULL,
    container varchar(20) NOT NULL,
    bitrate_kbps integer NOT NULL,
    sample_rate integer NOT NULL,
    channels smallint NOT NULL,
    size_bytes bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT audio_variants_object_key_not_blank CHECK (btrim(object_key) <> ''),
    CONSTRAINT audio_variants_codec_not_blank CHECK (btrim(codec) <> ''),
    CONSTRAINT audio_variants_container_not_blank CHECK (btrim(container) <> ''),
    CONSTRAINT audio_variants_bitrate_positive CHECK (bitrate_kbps > 0),
    CONSTRAINT audio_variants_sample_rate_positive CHECK (sample_rate > 0),
    CONSTRAINT audio_variants_channels_positive CHECK (channels > 0),
    CONSTRAINT audio_variants_size_positive CHECK (size_bytes > 0),
    CONSTRAINT audio_variants_object_key_key UNIQUE (object_key),
    CONSTRAINT audio_variants_media_asset_id_fkey
        FOREIGN KEY (media_asset_id)
        REFERENCES media.media_assets (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE TABLE media.processing_jobs (
    id uuid PRIMARY KEY,
    media_asset_id uuid NOT NULL,
    job_type varchar(30) NOT NULL,
    status media.processing_status NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    error_message text,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT processing_jobs_type_not_blank CHECK (btrim(job_type) <> ''),
    CONSTRAINT processing_jobs_attempts_not_negative CHECK (attempts >= 0),
    CONSTRAINT processing_jobs_finished_after_start CHECK (
        finished_at IS NULL OR (started_at IS NOT NULL AND finished_at >= started_at)
    ),
    CONSTRAINT processing_jobs_media_asset_id_fkey
        FOREIGN KEY (media_asset_id)
        REFERENCES media.media_assets (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE INDEX media_assets_track_id_idx ON media.media_assets (track_id);
CREATE INDEX media_assets_status_created_at_idx ON media.media_assets (status, created_at, id);
CREATE INDEX audio_variants_media_asset_id_idx ON media.audio_variants (media_asset_id);
CREATE INDEX processing_jobs_media_asset_id_idx ON media.processing_jobs (media_asset_id);
CREATE INDEX processing_jobs_status_created_at_idx ON media.processing_jobs (status, created_at, id);
