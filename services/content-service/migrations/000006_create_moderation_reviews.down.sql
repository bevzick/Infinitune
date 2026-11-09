DROP TABLE IF EXISTS content.moderation_reviews;
DROP TYPE IF EXISTS content.moderation_decision;

ALTER TABLE content.releases
    DROP COLUMN IF EXISTS reviewed_at;

UPDATE content.releases
SET status = 'rejected'
WHERE status = 'changes_requested';

ALTER TABLE content.releases
    ALTER COLUMN status DROP DEFAULT;

DROP INDEX IF EXISTS content.releases_pending_review_submitted_at_idx;

ALTER TYPE content.release_status RENAME TO release_status_with_changes;

CREATE TYPE content.release_status AS ENUM (
    'draft',
    'uploading',
    'processing',
    'pending_review',
    'approved',
    'rejected',
    'published'
);

ALTER TABLE content.releases
    ALTER COLUMN status TYPE content.release_status
    USING status::text::content.release_status,
    ALTER COLUMN status SET DEFAULT 'draft';

CREATE INDEX releases_pending_review_submitted_at_idx
    ON content.releases (submitted_at, id)
    WHERE status = 'pending_review';

DROP TYPE content.release_status_with_changes;

