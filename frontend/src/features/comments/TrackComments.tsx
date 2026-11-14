import {
  type FormEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { Link } from "react-router-dom";

import { ApiError } from "../../shared/api/client";
import { useSession } from "../session/SessionContext";
import { CommentCard } from "./CommentCard";
import {
  commentPageLimit,
  createTrackComment,
  deleteTrackComment,
  listLikedCommentIds,
  listTrackComments,
  maximumCommentLength,
  type ReportReason,
  reportTrackComment,
  setCommentLike,
  type TrackComment,
  updateTrackComment,
} from "./commentApi";

export function TrackComments({ trackId }: { trackId: string }) {
  const { accessToken, identity } = useSession();
  const [items, setItems] = useState<TrackComment[]>([]);
  const [offset, setOffset] = useState(0);
  const [total, setTotal] = useState(0);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [loadingMore, setLoadingMore] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [likesError, setLikesError] = useState("");
  const [likesReady, setLikesReady] = useState(false);
  const [likes, setLikes] = useState<Set<string>>(new Set());
  const [reported, setReported] = useState<Set<string>>(new Set());
  const [draft, setDraft] = useState("");
  const [replyTo, setReplyTo] = useState<string | null>(null);
  const sequence = useRef(0);
  const requestController = useRef<AbortController | null>(null);

  const reload = useCallback(() => {
    requestController.current?.abort();
    const controller = new AbortController();
    requestController.current = controller;
    setState("loading");
    setItems([]);
    setOffset(0);
    setTotal(0);
    setError("");
    setLikesReady(false);
    setLikes(new Set());
    setReported(new Set());
    setLikesError("");
    setReplyTo(null);
    void listTrackComments(trackId, 0, controller.signal)
      .then((page) => {
        if (controller.signal.aborted) return;
        setItems(page.items);
        setOffset(page.items.length);
        setTotal(page.page.total);
        setState("ready");
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setError(message(cause));
        setState("error");
      });
    if (accessToken) {
      void listLikedCommentIds(accessToken, controller.signal)
        .then((ids) => {
          if (controller.signal.aborted) return;
          setLikes(ids);
          setLikesReady(true);
        })
        .catch(() => {
          if (!controller.signal.aborted)
            setLikesError("Could not load your liked comments.");
        });
    }
  }, [trackId, accessToken]);

  useEffect(() => {
    reload();
    return () => requestController.current?.abort();
  }, [reload]);

  async function loadMore() {
    if (loadingMore || busy || state !== "ready") return;
    setLoadingMore(true);
    setError("");
    try {
      const page = await listTrackComments(trackId, offset);
      setItems((current) => [
        ...current,
        ...page.items.filter(
          (item) => !current.some((existing) => existing.id === item.id),
        ),
      ]);
      setOffset((current) => current + page.items.length);
      setTotal(page.page.total);
    } catch (cause) {
      setError(message(cause));
    } finally {
      setLoadingMore(false);
    }
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const content = draft.trim();
    if (!accessToken || !identity || !validText(content) || busy) return;
    const tempId = `pending:${++sequence.current}`;
    const optimistic: TrackComment = {
      id: tempId,
      track_id: trackId,
      user_id: identity.userId,
      parent_id: replyTo,
      content,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    setBusy(true);
    setError("");
    setItems((current) => [optimistic, ...current]);
    try {
      const saved = await createTrackComment(
        accessToken,
        trackId,
        content,
        replyTo,
      );
      setItems((current) =>
        current.map((item) => (item.id === tempId ? saved : item)),
      );
      setTotal((current) => current + 1);
      setOffset((current) => current + 1);
      setDraft("");
      setReplyTo(null);
    } catch (cause) {
      setItems((current) => current.filter((item) => item.id !== tempId));
      setError(message(cause));
    } finally {
      setBusy(false);
    }
  }

  async function edit(id: string, content: string): Promise<boolean> {
    if (!accessToken || busy || !validText(content)) return false;
    const original = items.find((item) => item.id === id);
    if (!original || original.user_id !== identity?.userId) return false;
    setBusy(true);
    setError("");
    setItems((current) =>
      current.map((item) => (item.id === id ? { ...item, content } : item)),
    );
    try {
      const saved = await updateTrackComment(accessToken, id, content);
      setItems((current) =>
        current.map((item) => (item.id === id ? saved : item)),
      );
      return true;
    } catch (cause) {
      setItems((current) =>
        current.map((item) => (item.id === id ? original : item)),
      );
      setError(message(cause));
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    if (!accessToken || busy) return;
    const previous = items;
    setBusy(true);
    setError("");
    setItems((current) => current.filter((item) => item.id !== id));
    try {
      await deleteTrackComment(accessToken, id);
      setTotal((current) => Math.max(0, current - 1));
      setOffset((current) => Math.max(0, current - 1));
    } catch (cause) {
      setItems(previous);
      setError(message(cause));
    } finally {
      setBusy(false);
    }
  }

  async function like(id: string) {
    if (!accessToken || !likesReady || busy) return;
    const original = likes.has(id);
    setBusy(true);
    setError("");
    setLikes((current) => toggleSet(current, id, !original));
    try {
      await setCommentLike(accessToken, id, !original);
    } catch (cause) {
      setLikes((current) => toggleSet(current, id, original));
      setError(message(cause));
    } finally {
      setBusy(false);
    }
  }

  async function report(id: string, reason: ReportReason, description: string) {
    if (!accessToken || busy || reported.has(id)) return false;
    setBusy(true);
    setError("");
    try {
      await reportTrackComment(accessToken, id, reason, description);
      setReported((current) => toggleSet(current, id, true));
      return true;
    } catch (cause) {
      setError(message(cause));
      return false;
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="track-comments" aria-label="Track comments">
      <header className="detail-section-heading">
        <div>
          <span className="eyebrow">Community</span>
          <h2>Comments</h2>
        </div>
        {state === "ready" ? <span>{total} comments</span> : null}
      </header>
      {state === "loading" ? <p role="status">Loading comments…</p> : null}
      {error ? (
        <p className="track-comments-error" role="alert">
          {error}
        </p>
      ) : null}
      {likesError ? (
        <p className="track-comments-error" role="status">
          {likesError}
        </p>
      ) : null}
      {state === "error" ? (
        <button type="button" onClick={reload}>
          Retry comments
        </button>
      ) : null}
      {accessToken && identity && state === "ready" ? (
        <form
          className="track-comment-form"
          onSubmit={(event) => void create(event)}
        >
          <label htmlFor="new-track-comment">
            {replyTo ? `Reply to ${replyTo.slice(0, 8)}` : "Write a comment"}
          </label>
          <textarea
            id="new-track-comment"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            maxLength={maximumCommentLength}
            placeholder="Share your thoughts about this track"
          />
          <div className="track-comment-actions">
            <button type="submit" disabled={busy || !validText(draft)}>
              {replyTo ? "Post reply" : "Post comment"}
            </button>
            {replyTo ? (
              <button
                type="button"
                disabled={busy}
                onClick={() => setReplyTo(null)}
              >
                Cancel reply
              </button>
            ) : null}
          </div>
        </form>
      ) : null}
      {!accessToken && state === "ready" ? (
        <p className="track-comments-login">
          <Link to="/login">Sign in</Link> to join the conversation.
        </p>
      ) : null}
      {state === "ready" && items.length === 0 ? <p>No comments yet.</p> : null}
      {state === "ready" ? (
        <div className="track-comment-list">
          {items.map((item) => (
            <CommentCard
              key={item.id}
              item={item}
              isOwn={item.user_id === identity?.userId}
              authenticated={Boolean(accessToken)}
              liked={likes.has(item.id)}
              likesReady={likesReady}
              busy={busy}
              reported={reported.has(item.id)}
              onReply={() => {
                setReplyTo(item.id);
                setDraft("");
              }}
              onLike={() => void like(item.id)}
              onEdit={(content) => edit(item.id, content)}
              onDelete={() => void remove(item.id)}
              onReport={(reason, description) =>
                report(item.id, reason, description)
              }
            />
          ))}
        </div>
      ) : null}
      {state === "ready" && offset < total ? (
        <button
          type="button"
          className="track-comments-more"
          disabled={busy || loadingMore}
          onClick={() => void loadMore()}
        >
          {loadingMore
            ? "Loading…"
            : `Load more comments (${Math.min(total - offset, commentPageLimit)} next)`}
        </button>
      ) : null}
    </section>
  );
}

function toggleSet(current: Set<string>, id: string, active: boolean) {
  const next = new Set(current);
  if (active) next.add(id);
  else next.delete(id);
  return next;
}

function validText(value: string) {
  const length = Array.from(value.trim()).length;
  return length >= 1 && length <= maximumCommentLength;
}

function message(cause: unknown) {
  return cause instanceof ApiError
    ? cause.message
    : "Operation failed. Please try again.";
}
