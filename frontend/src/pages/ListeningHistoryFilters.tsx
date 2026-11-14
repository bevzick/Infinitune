import { type FormEvent, useState } from "react";

import type { ListeningHistoryFilters as HistoryFiltersInput } from "../features/history/historyApi";

type DraftFilters = {
  trackId: string;
  fromDate: string;
  toDate: string;
  completed: "all" | "true" | "false";
  contextType: string;
  deviceType: string;
};

const emptyDraft: DraftFilters = {
  trackId: "",
  fromDate: "",
  toDate: "",
  completed: "all",
  contextType: "",
  deviceType: "",
};

const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function localDayStart(date: string) {
  return new Date(`${date}T00:00:00`).toISOString();
}

/** Turns an inclusive end-date into the exclusive `to` expected by History Service. */
function localDayAfter(date: string) {
  const next = new Date(`${date}T00:00:00`);
  next.setDate(next.getDate() + 1);
  return next.toISOString();
}

function parseFilters(draft: DraftFilters): HistoryFiltersInput {
  const trackId = draft.trackId.trim();
  if (trackId && !uuidPattern.test(trackId)) {
    throw new Error("Track ID must be a UUID.");
  }
  if (draft.fromDate && draft.toDate && draft.fromDate > draft.toDate) {
    throw new Error("Start date cannot be after end date.");
  }
  return {
    ...(trackId ? { trackId } : {}),
    ...(draft.fromDate ? { from: localDayStart(draft.fromDate) } : {}),
    ...(draft.toDate ? { to: localDayAfter(draft.toDate) } : {}),
    ...(draft.completed !== "all"
      ? { completed: draft.completed === "true" }
      : {}),
    ...(draft.contextType
      ? {
          contextType: draft.contextType as HistoryFiltersInput["contextType"],
        }
      : {}),
    ...(draft.deviceType
      ? {
          deviceType: draft.deviceType as HistoryFiltersInput["deviceType"],
        }
      : {}),
  };
}

export function ListeningHistoryFilters({
  onApply,
}: {
  onApply(filters: HistoryFiltersInput): void;
}) {
  const [draft, setDraft] = useState<DraftFilters>(emptyDraft);
  const [filterError, setFilterError] = useState("");

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      onApply(parseFilters(draft));
      setFilterError("");
    } catch (cause) {
      setFilterError(
        cause instanceof Error ? cause.message : "Invalid filters.",
      );
    }
  }

  return (
    <form
      className="history-filters"
      aria-label="History filters"
      onSubmit={applyFilters}
    >
      <label>
        Track ID
        <input
          value={draft.trackId}
          onChange={(event) =>
            setDraft((old) => ({ ...old, trackId: event.target.value }))
          }
          placeholder="Track UUID (optional)"
        />
      </label>
      <label>
        From
        <input
          type="date"
          value={draft.fromDate}
          onChange={(event) =>
            setDraft((old) => ({ ...old, fromDate: event.target.value }))
          }
        />
      </label>
      <label>
        Through
        <input
          type="date"
          value={draft.toDate}
          onChange={(event) =>
            setDraft((old) => ({ ...old, toDate: event.target.value }))
          }
        />
      </label>
      <label>
        Playback
        <select
          value={draft.completed}
          onChange={(event) =>
            setDraft((old) => ({
              ...old,
              completed: event.target.value as DraftFilters["completed"],
            }))
          }
        >
          <option value="all">All</option>
          <option value="true">Completed</option>
          <option value="false">Not completed</option>
        </select>
      </label>
      <label>
        Context
        <select
          value={draft.contextType}
          onChange={(event) =>
            setDraft((old) => ({ ...old, contextType: event.target.value }))
          }
        >
          <option value="">All contexts</option>
          {(
            [
              "direct",
              "search",
              "album",
              "playlist",
              "artist",
              "recommendation",
            ] as const
          ).map((kind) => (
            <option value={kind} key={kind}>
              {kind}
            </option>
          ))}
        </select>
      </label>
      <label>
        Device
        <select
          value={draft.deviceType}
          onChange={(event) =>
            setDraft((old) => ({ ...old, deviceType: event.target.value }))
          }
        >
          <option value="">All devices</option>
          {(["web", "mobile", "desktop", "tv", "other"] as const).map(
            (device) => (
              <option value={device} key={device}>
                {device}
              </option>
            ),
          )}
        </select>
      </label>
      <div className="history-filter-actions">
        <button type="submit">Apply filters</button>
        <button
          type="button"
          className="history-secondary"
          onClick={() => {
            setDraft(emptyDraft);
            onApply({});
            setFilterError("");
          }}
        >
          Reset
        </button>
      </div>
      {filterError ? (
        <p role="alert" className="form-error">
          {filterError}
        </p>
      ) : null}
    </form>
  );
}
