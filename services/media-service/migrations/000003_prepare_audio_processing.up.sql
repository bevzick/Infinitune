ALTER TABLE media.processing_jobs
    ADD COLUMN available_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE media.processing_jobs
    ADD CONSTRAINT processing_jobs_asset_type_key
        UNIQUE (media_asset_id, job_type);

CREATE UNIQUE INDEX audio_variants_asset_profile_key
    ON media.audio_variants (media_asset_id, codec, container, bitrate_kbps);

CREATE INDEX processing_jobs_available_idx
    ON media.processing_jobs (available_at, created_at, id)
    WHERE status = 'pending';

