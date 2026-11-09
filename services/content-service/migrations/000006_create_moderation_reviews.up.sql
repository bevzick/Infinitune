ALTER TYPE content.release_status
    ADD VALUE IF NOT EXISTS 'changes_requested' AFTER 'rejected';

ALTER TABLE content.releases
    ADD COLUMN reviewed_at timestamptz;

CREATE TYPE content.moderation_decision AS ENUM (
    'approved',
    'rejected',
    'changes_requested'
);

CREATE TABLE content.moderation_reviews (
    id uuid PRIMARY KEY,
    release_id uuid NOT NULL,
    reviewer_user_id uuid NOT NULL,
    decision content.moderation_decision NOT NULL,
    comment text,
    reviewed_submission_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT moderation_reviews_comment_check CHECK (
        char_length(comment) <= 2000
        AND (decision = 'approved' OR (comment IS NOT NULL AND btrim(comment) <> ''))
    ),
    CONSTRAINT moderation_reviews_release_id_fkey
        FOREIGN KEY (release_id)
        REFERENCES content.releases (id)
        ON UPDATE NO ACTION
        ON DELETE CASCADE,
    CONSTRAINT moderation_reviews_release_submission_key
        UNIQUE (release_id, reviewed_submission_at)
);

COMMENT ON COLUMN content.moderation_reviews.reviewer_user_id IS
    'Cross-service User UUID from the verified moderator access token.';
COMMENT ON COLUMN content.moderation_reviews.reviewed_submission_at IS
    'Identifies the exact release submission attempt reviewed by this decision.';

CREATE INDEX moderation_reviews_reviewer_created_at_idx
    ON content.moderation_reviews (reviewer_user_id, created_at DESC, id);

