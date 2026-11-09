DROP INDEX media.processing_jobs_available_idx;
DROP INDEX media.audio_variants_asset_profile_key;

ALTER TABLE media.processing_jobs
    DROP CONSTRAINT processing_jobs_asset_type_key,
    DROP COLUMN available_at;

