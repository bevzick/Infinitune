DROP INDEX IF EXISTS catalog.publications_archived_at_idx;

ALTER TABLE catalog.publications
    DROP COLUMN IF EXISTS archived_at;
