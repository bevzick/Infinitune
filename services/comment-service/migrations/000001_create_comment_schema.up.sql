CREATE SCHEMA comment;

CREATE TYPE comment.comment_status AS ENUM (
    'published',
    'hidden',
    'deleted'
);

CREATE TYPE comment.report_status AS ENUM (
    'pending',
    'reviewing',
    'resolved',
    'rejected'
);

CREATE TABLE comment.comments (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    track_id uuid NOT NULL,
    parent_id uuid,
    content text NOT NULL,
    status comment.comment_status NOT NULL DEFAULT 'published',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT comments_id_track_id_key UNIQUE (id, track_id),
    CONSTRAINT comments_content_length_check
        CHECK (char_length(btrim(content)) BETWEEN 1 AND 2000),
    CONSTRAINT comments_parent_not_self_check
        CHECK (parent_id IS NULL OR parent_id <> id),
    CONSTRAINT comments_updated_at_check
        CHECK (updated_at >= created_at),
    CONSTRAINT comments_deleted_state_check
        CHECK ((status = 'deleted') = (deleted_at IS NOT NULL)),
    CONSTRAINT comments_deleted_at_check
        CHECK (deleted_at IS NULL OR deleted_at >= created_at),
    CONSTRAINT comments_parent_track_fkey
        FOREIGN KEY (parent_id, track_id)
        REFERENCES comment.comments (id, track_id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON COLUMN comment.comments.user_id IS
    'Cross-service Auth user UUID; intentionally has no database foreign key.';
COMMENT ON COLUMN comment.comments.track_id IS
    'Cross-service Catalog track UUID; intentionally has no database foreign key.';

CREATE FUNCTION comment.enforce_comment_tree_depth()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    parent_parent_id uuid;
BEGIN
    IF TG_OP = 'UPDATE' AND (
        NEW.track_id IS DISTINCT FROM OLD.track_id OR
        NEW.parent_id IS DISTINCT FROM OLD.parent_id
    ) THEN
        RAISE EXCEPTION 'comment track_id and parent_id are immutable'
            USING ERRCODE = '23514';
    END IF;

    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT parent.parent_id
    INTO parent_parent_id
    FROM comment.comments AS parent
    WHERE parent.id = NEW.parent_id
      AND parent.track_id = NEW.track_id
    FOR KEY SHARE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'comment parent must exist on the same track'
            USING ERRCODE = '23503';
    END IF;

    IF parent_parent_id IS NOT NULL THEN
        RAISE EXCEPTION 'comment replies may not have replies'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER comments_tree_depth_trigger
BEFORE INSERT OR UPDATE OF parent_id, track_id
ON comment.comments
FOR EACH ROW
EXECUTE FUNCTION comment.enforce_comment_tree_depth();

CREATE TABLE comment.comment_reports (
    id uuid PRIMARY KEY,
    comment_id uuid NOT NULL,
    reporter_user_id uuid NOT NULL,
    reason varchar(50) NOT NULL,
    description text NOT NULL,
    status comment.report_status NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz,
    CONSTRAINT comment_reports_reason_length_check
        CHECK (char_length(btrim(reason)) BETWEEN 1 AND 50),
    CONSTRAINT comment_reports_description_length_check
        CHECK (char_length(btrim(description)) BETWEEN 1 AND 2000),
    CONSTRAINT comment_reports_updated_at_check
        CHECK (updated_at IS NULL OR updated_at >= created_at),
    CONSTRAINT comment_reports_comment_id_fkey
        FOREIGN KEY (comment_id)
        REFERENCES comment.comments (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON COLUMN comment.comment_reports.reporter_user_id IS
    'Cross-service Auth user UUID; intentionally has no database foreign key.';

CREATE INDEX comments_track_status_created_at_idx
    ON comment.comments (track_id, status, created_at DESC, id);
CREATE INDEX comments_parent_created_at_idx
    ON comment.comments (parent_id, created_at, id)
    WHERE parent_id IS NOT NULL;
CREATE INDEX comments_user_updated_at_idx
    ON comment.comments (user_id, updated_at DESC, id);
CREATE INDEX comment_reports_comment_created_at_idx
    ON comment.comment_reports (comment_id, created_at DESC, id);
CREATE INDEX comment_reports_reporter_created_at_idx
    ON comment.comment_reports (reporter_user_id, created_at DESC, id);
CREATE INDEX comment_reports_status_created_at_idx
    ON comment.comment_reports (status, created_at, id);
