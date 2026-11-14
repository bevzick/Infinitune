ALTER TABLE content.releases
    ADD COLUMN submitted_at timestamptz;

CREATE INDEX releases_pending_review_submitted_at_idx
    ON content.releases (submitted_at, id)
    WHERE status = 'pending_review';

