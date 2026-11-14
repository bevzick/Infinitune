import { type FormEvent, useState } from "react";

import {
  maximumCommentLength,
  type ReportReason,
  type TrackComment,
} from "./commentApi";

const reasons: Array<{ value: ReportReason; label: string }> = [
  { value: "spam", label: "Spam" },
  { value: "harassment", label: "Harassment" },
  { value: "hate_speech", label: "Hate speech" },
  { value: "sexual_content", label: "Sexual content" },
  { value: "violence", label: "Violence" },
  { value: "misinformation", label: "Misinformation" },
  { value: "other", label: "Other" },
];

type Props = {
  item: TrackComment;
  isOwn: boolean;
  authenticated: boolean;
  liked: boolean;
  likesReady: boolean;
  busy: boolean;
  reported: boolean;
  onReply(): void;
  onLike(): void;
  onEdit(content: string): Promise<boolean>;
  onDelete(): void;
  onReport(reason: ReportReason, description: string): Promise<boolean>;
};

export function CommentCard({
  item,
  isOwn,
  authenticated,
  liked,
  likesReady,
  busy,
  reported,
  onReply,
  onLike,
  onEdit,
  onDelete,
  onReport,
}: Props) {
  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState(item.content);
  const [reporting, setReporting] = useState(false);
  const [reason, setReason] = useState<ReportReason>("spam");
  const [description, setDescription] = useState("");
  const editValid = isValidText(editText);
  const reportValid = isValidText(description);

  async function saveEdit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editValid) return;
    if (await onEdit(editText.trim())) setEditing(false);
  }

  async function sendReport(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!reportValid) return;
    if (await onReport(reason, description.trim())) setReporting(false);
  }

  return (
    <article
      className={`track-comment${item.parent_id ? " track-comment-reply" : ""}`}
      aria-label={`Comment by ${isOwn ? "you" : item.user_id.slice(0, 8)}`}
    >
      <header className="track-comment-header">
        <strong>
          {isOwn ? "You" : `Listener ${item.user_id.slice(0, 8)}`}
        </strong>
        <time dateTime={item.created_at}>
          {new Date(item.created_at).toLocaleDateString()}
        </time>
      </header>
      {item.parent_id ? (
        <span className="track-comment-parent">
          Reply to {item.parent_id.slice(0, 8)}
        </span>
      ) : null}
      {editing ? (
        <form
          className="track-comment-form"
          onSubmit={(event) => void saveEdit(event)}
        >
          <label htmlFor={`edit-comment-${item.id}`}>Edit your comment</label>
          <textarea
            id={`edit-comment-${item.id}`}
            value={editText}
            onChange={(event) => setEditText(event.target.value)}
            maxLength={maximumCommentLength}
          />
          <div className="track-comment-actions">
            <button type="submit" disabled={busy || !editValid}>
              Save edit
            </button>
            <button
              type="button"
              disabled={busy}
              onClick={() => setEditing(false)}
            >
              Cancel edit
            </button>
          </div>
        </form>
      ) : (
        <p className="track-comment-text">{item.content}</p>
      )}
      {item.id.startsWith("pending:") ? (
        <span role="status">Sending…</span>
      ) : null}
      {authenticated && !item.id.startsWith("pending:") ? (
        <div className="track-comment-actions">
          <button
            type="button"
            aria-label={liked ? "Unlike comment" : "Like comment"}
            aria-pressed={liked}
            disabled={busy || !likesReady}
            onClick={onLike}
          >
            {liked ? "Liked" : "Like"}
          </button>
          <button type="button" disabled={busy} onClick={onReply}>
            Reply
          </button>
          {isOwn ? (
            <>
              {!editing ? (
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    setEditText(item.content);
                    setEditing(true);
                  }}
                >
                  Edit
                </button>
              ) : null}
              <button
                type="button"
                disabled={busy}
                onClick={() => {
                  if (window.confirm("Delete this comment?")) onDelete();
                }}
              >
                Delete
              </button>
            </>
          ) : (
            <button
              type="button"
              disabled={busy || reported}
              onClick={() => setReporting((value) => !value)}
            >
              {reported ? "Reported" : "Report"}
            </button>
          )}
        </div>
      ) : null}
      {reporting && !reported ? (
        <form
          className="track-comment-form"
          onSubmit={(event) => void sendReport(event)}
        >
          <label htmlFor={`reason-${item.id}`}>Report reason</label>
          <select
            id={`reason-${item.id}`}
            value={reason}
            onChange={(event) => setReason(event.target.value as ReportReason)}
          >
            {reasons.map((option) => (
              <option value={option.value} key={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <label htmlFor={`description-${item.id}`}>Report details</label>
          <textarea
            id={`description-${item.id}`}
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            maxLength={maximumCommentLength}
          />
          <div className="track-comment-actions">
            <button disabled={busy || !reportValid} type="submit">
              Send report
            </button>
            <button
              disabled={busy}
              type="button"
              onClick={() => setReporting(false)}
            >
              Cancel report
            </button>
          </div>
        </form>
      ) : null}
    </article>
  );
}

function isValidText(value: string) {
  const length = Array.from(value.trim()).length;
  return length >= 1 && length <= maximumCommentLength;
}
