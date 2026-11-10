CREATE TYPE comment.moderation_action AS ENUM (
    'start_review',
    'hide_comment',
    'restore_comment',
    'resolve_report',
    'reject_report'
);

CREATE TABLE comment.report_moderation_actions (
    id uuid PRIMARY KEY,
    report_id uuid NOT NULL,
    moderator_user_id uuid NOT NULL,
    action comment.moderation_action NOT NULL,
    note text NOT NULL,
    previous_report_status comment.report_status NOT NULL,
    resulting_report_status comment.report_status NOT NULL,
    previous_comment_status comment.comment_status NOT NULL,
    resulting_comment_status comment.comment_status NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT report_moderation_actions_note_check
        CHECK (char_length(btrim(note)) BETWEEN 1 AND 2000),
    CONSTRAINT report_moderation_actions_report_id_fkey
        FOREIGN KEY (report_id)
        REFERENCES comment.comment_reports (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON COLUMN comment.report_moderation_actions.moderator_user_id IS
    'Cross-service Auth user UUID from the verified moderator access token.';

CREATE INDEX report_moderation_actions_report_created_at_idx
    ON comment.report_moderation_actions (report_id, created_at, id);
CREATE INDEX report_moderation_actions_moderator_created_at_idx
    ON comment.report_moderation_actions (moderator_user_id, created_at DESC, id);
