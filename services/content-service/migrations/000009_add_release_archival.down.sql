DROP INDEX IF EXISTS content.releases_archived_at_idx;

ALTER TABLE content.releases
    DROP CONSTRAINT IF EXISTS releases_publication_lifecycle_check;

UPDATE content.releases
SET status = CASE
        WHEN published_at IS NULL THEN 'draft'::content.release_status
        ELSE 'published'::content.release_status
    END,
    archived_at = NULL
WHERE status = 'archived';

ALTER TABLE content.releases
    DROP COLUMN IF EXISTS archived_at;

ALTER TABLE content.releases
    ADD CONSTRAINT releases_published_at_status_check
    CHECK ((status = 'published') = (published_at IS NOT NULL));
