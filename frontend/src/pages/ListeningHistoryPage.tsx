import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";

import { fetchCatalogItem, type Track } from "../features/catalog/catalogApi";
import {
  clearListeningHistory,
  type ListeningHistoryFilters as HistoryFiltersInput,
  type ListeningHistoryEvent,
  listListeningHistory,
} from "../features/history/historyApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";
import { ListeningHistoryFilters } from "./ListeningHistoryFilters";
import { ListeningHistoryResults } from "./ListeningHistoryResults";

function localDayStart(date: string) {
  return new Date(`${date}T00:00:00`).toISOString();
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError || error instanceof Error) return error.message;
  return "Could not load listening history.";
}

export function ListeningHistoryPage() {
  const { accessToken, identity } = useSession();
  const [query, setQuery] = useState<{
    filters: HistoryFiltersInput;
    revision: number;
  }>({
    filters: {},
    revision: 0,
  });
  const [items, setItems] = useState<ListeningHistoryEvent[]>([]);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState("");
  const [moreBusy, setMoreBusy] = useState(false);
  const [clearBusy, setClearBusy] = useState(false);
  const [clearBefore, setClearBefore] = useState("");
  const [notice, setNotice] = useState("");
  const [titles, setTitles] = useState<Record<string, string>>({});
  const titleCache = useRef(new Map<string, string>());
  const requestController = useRef<AbortController | null>(null);
  const generation = useRef(0);

  useEffect(() => {
    if (!accessToken) return;
    const seq = ++generation.current;
    requestController.current?.abort();
    const controller = new AbortController();
    requestController.current = controller;
    setState("loading");
    setError("");
    setItems([]);
    setNextCursor(undefined);
    setMoreBusy(false);
    void listListeningHistory(
      accessToken,
      query.filters,
      undefined,
      controller.signal,
    )
      .then((page) => {
        if (controller.signal.aborted || seq !== generation.current) return;
        setItems(page.items);
        setNextCursor(page.next_cursor);
        setState("ready");
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted || seq !== generation.current) return;
        setError(errorMessage(cause));
        setState("error");
      });
    return () => controller.abort();
  }, [accessToken, query]);

  // Resolve only distinct, currently visible tracks. History itself never depends on Catalog.
  useEffect(() => {
    const ids = [...new Set(items.map((item) => item.track_id))].filter(
      (id) => !titleCache.current.has(id),
    );
    if (ids.length === 0) return;
    const controller = new AbortController();
    void Promise.all(
      ids.map(async (id) => {
        try {
          const track = await fetchCatalogItem<Track>(
            "tracks",
            id,
            controller.signal,
          );
          return [id, track.title] as const;
        } catch {
          return [id, ""] as const;
        }
      }),
    ).then((resolved) => {
      if (controller.signal.aborted) return;
      for (const [id, title] of resolved) titleCache.current.set(id, title);
      setTitles(Object.fromEntries(titleCache.current));
    });
    return () => controller.abort();
  }, [items]);

  // Prevent a previous user's titles from remaining in a new session.
  useEffect(() => {
    if (!identity?.userId) return;
    titleCache.current.clear();
    setTitles({});
  }, [identity?.userId]);

  async function loadMore() {
    if (!accessToken || !nextCursor || moreBusy || state !== "ready") return;
    const seq = generation.current;
    setMoreBusy(true);
    setError("");
    try {
      const page = await listListeningHistory(
        accessToken,
        query.filters,
        nextCursor,
        requestController.current?.signal,
      );
      if (
        seq !== generation.current ||
        requestController.current?.signal.aborted
      )
        return;
      setItems((current) => [
        ...current,
        ...page.items.filter(
          (item) => !current.some((entry) => entry.event_id === item.event_id),
        ),
      ]);
      setNextCursor(page.next_cursor);
    } catch (cause) {
      if (
        seq === generation.current &&
        !requestController.current?.signal.aborted
      ) {
        setError(errorMessage(cause));
      }
    } finally {
      if (seq === generation.current) setMoreBusy(false);
    }
  }

  async function clearHistory() {
    if (!accessToken || clearBusy) return;
    const before = clearBefore ? localDayStart(clearBefore) : undefined;
    if (before && new Date(before).getTime() > Date.now()) {
      setError("Choose a date that is not in the future.");
      return;
    }
    const description = clearBefore
      ? `Delete all listening events before ${clearBefore}? This cannot be undone.`
      : "Delete your entire listening history? This cannot be undone.";
    if (!window.confirm(description)) return;
    setClearBusy(true);
    setError("");
    setNotice("");
    try {
      const result = await clearListeningHistory(accessToken, before);
      setNotice(`${result.deleted} listening event(s) deleted.`);
      setQuery((current) => ({ ...current, revision: current.revision + 1 }));
    } catch (cause) {
      setError(errorMessage(cause));
    } finally {
      setClearBusy(false);
    }
  }

  return (
    <section className="history-page" aria-labelledby="listening-history-title">
      <header className="history-heading">
        <span className="eyebrow">Your listening activity</span>
        <h1 id="listening-history-title">Listening history</h1>
        <p>
          Only you can open this private history. The "Show listening activity"
          preference controls sharing where supported, not this private page.
          Manage it in <Link to="/profile">Profile settings</Link>.
        </p>
      </header>

      <ListeningHistoryFilters
        onApply={(filters) => {
          setNotice("");
          setQuery((current) => ({ filters, revision: current.revision + 1 }));
        }}
      />

      <ListeningHistoryResults
        state={state}
        items={items}
        titles={titles}
        error={error}
        hasMore={Boolean(nextCursor)}
        moreBusy={moreBusy}
        onRetry={() =>
          setQuery((current) => ({
            ...current,
            revision: current.revision + 1,
          }))
        }
        onLoadMore={() => void loadMore()}
      />

      <section className="history-clear" aria-labelledby="clear-history-title">
        <h2 id="clear-history-title">Clear listening history</h2>
        <p>Clearing removes your stored events. It cannot be undone.</p>
        <label>
          Delete events before date (optional; leave blank to clear all)
          <input
            type="date"
            value={clearBefore}
            onChange={(event) => setClearBefore(event.target.value)}
          />
        </label>
        <button
          type="button"
          className="history-danger"
          disabled={clearBusy || state === "loading"}
          onClick={() => void clearHistory()}
        >
          {clearBusy ? "Clearing…" : "Clear history"}
        </button>
        {notice ? <p role="status">{notice}</p> : null}
      </section>
    </section>
  );
}
