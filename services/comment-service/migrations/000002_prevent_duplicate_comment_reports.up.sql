CREATE UNIQUE INDEX comment_reports_comment_reporter_uidx
    ON comment.comment_reports (comment_id, reporter_user_id);
