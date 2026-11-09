ALTER TABLE content.releases
    DROP CONSTRAINT releases_published_at_status_check,
    ADD COLUMN archived_at timestamptz;

ALTER TABLE content.releases
    ADD CONSTRAINT releases_publication_lifecycle_check CHECK (
        (published_at IS NULL OR status IN ('published', 'archived'))
        AND ((status = 'published') = (published_at IS NOT NULL AND archived_at IS NULL))
        AND ((status = 'archived') = (archived_at IS NOT NULL))
    );

CREATE INDEX releases_archived_at_idx
    ON content.releases (archived_at DESC, id)
    WHERE status = 'archived';
