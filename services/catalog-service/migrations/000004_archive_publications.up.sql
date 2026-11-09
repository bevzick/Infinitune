ALTER TABLE catalog.publications
    ADD COLUMN archived_at timestamptz;

CREATE INDEX publications_archived_at_idx
    ON catalog.publications (archived_at DESC, source_release_id)
    WHERE archived_at IS NOT NULL;
