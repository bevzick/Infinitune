CREATE TABLE catalog.content_release_approvals (
    release_id uuid PRIMARY KEY,
    creator_id uuid NOT NULL,
    reviewer_user_id uuid NOT NULL,
    reviewed_submission_at timestamptz NOT NULL,
    event_id uuid NOT NULL UNIQUE,
    approved_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE catalog.content_release_approvals IS
    'Idempotent projection of Content moderation approvals required before publication.';
