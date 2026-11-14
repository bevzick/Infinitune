ALTER TABLE content.releases
    DROP CONSTRAINT IF EXISTS releases_published_at_status_check,
    ALTER COLUMN status DROP DEFAULT;

DROP INDEX IF EXISTS content.releases_creator_status_updated_at_idx;
DROP INDEX IF EXISTS content.releases_pending_review_submitted_at_idx;
DROP INDEX IF EXISTS content.releases_published_at_idx;

ALTER TYPE content.release_status RENAME TO release_status_with_archived;

CREATE TYPE content.release_status AS ENUM (
    'draft',
    'uploading',
    'processing',
    'pending_review',
    'approved',
    'rejected',
    'changes_requested',
    'published'
);

ALTER TABLE content.releases
    ALTER COLUMN status TYPE content.release_status
    USING status::text::content.release_status,
    ALTER COLUMN status SET DEFAULT 'draft';

DROP TYPE content.release_status_with_archived;

ALTER TABLE content.releases
    ADD CONSTRAINT releases_published_at_status_check
    CHECK ((status = 'published') = (published_at IS NOT NULL));

CREATE INDEX releases_creator_status_updated_at_idx
    ON content.releases (creator_id, status, updated_at DESC, id);

CREATE INDEX releases_pending_review_submitted_at_idx
    ON content.releases (submitted_at, id)
    WHERE status = 'pending_review';

CREATE INDEX releases_published_at_idx
    ON content.releases (published_at DESC, id)
    WHERE status = 'published';
