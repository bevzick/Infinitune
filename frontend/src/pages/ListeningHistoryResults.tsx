import { Link } from "react-router-dom";

import {
  type ListeningHistoryEvent,
  listeningHistoryPageSize,
} from "../features/history/historyApi";

function friendlyDate(value: string) {
  const time = new Date(value);
  return Number.isNaN(time.getTime()) ? value : time.toLocaleString();
}

function listeningDuration(milliseconds: number) {
  const totalSeconds = Math.floor(Math.max(0, milliseconds) / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

export function ListeningHistoryResults({
  state,
  items,
  titles,
  error,
  hasMore,
  moreBusy,
  onRetry,
  onLoadMore,
}: {
  state: "loading" | "ready" | "error";
  items: ListeningHistoryEvent[];
  titles: Record<string, string>;
  error: string;
  hasMore: boolean;
  moreBusy: boolean;
  onRetry(): void;
  onLoadMore(): void;
}) {
  return (
    <section className="history-results" aria-label="Listening events">
      <div className="history-section-heading">
        <h2>Recent listens</h2>
        {state === "ready" ? (
          <span>
            {items.length} shown · up to {listeningHistoryPageSize} per page
          </span>
        ) : null}
      </div>
      {state === "loading" ? (
        <p role="status">Loading listening history…</p>
      ) : null}
      {state === "error" ? (
        <div role="alert" className="history-inline-error">
          <p>{error}</p>
          <button type="button" onClick={onRetry}>
            Try again
          </button>
        </div>
      ) : null}
      {state === "ready" && items.length === 0 ? (
        <p>No listening events match these filters.</p>
      ) : null}
      {state === "ready" && items.length > 0 ? (
        <ol className="history-list">
          {items.map((item) => (
            <li key={item.event_id} className="history-event">
              <div>
                <Link to={`/tracks/${encodeURIComponent(item.track_id)}`}>
                  {titles[item.track_id] ||
                    `Track ${item.track_id.slice(0, 8)}…`}
                </Link>
                <small title={item.started_at}>
                  {friendlyDate(item.started_at)}
                </small>
              </div>
              <div className="history-event-details">
                <span>{listeningDuration(item.listened_ms)} listened</span>
                <span>{item.completed ? "Completed" : "Partial"}</span>
                <span>
                  {item.context_type} · {item.device_type}
                </span>
              </div>
            </li>
          ))}
        </ol>
      ) : null}
      {state === "ready" && hasMore ? (
        <button
          type="button"
          className="history-more"
          disabled={moreBusy}
          onClick={onLoadMore}
        >
          {moreBusy ? "Loading…" : "Load more"}
        </button>
      ) : null}
      {state === "ready" && error ? (
        <p role="alert" className="form-error">
          {error}
        </p>
      ) : null}
    </section>
  );
}
