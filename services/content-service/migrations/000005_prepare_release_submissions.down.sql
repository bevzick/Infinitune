DROP INDEX IF EXISTS content.releases_pending_review_submitted_at_idx;

ALTER TABLE content.releases
    DROP COLUMN IF EXISTS submitted_at;

