import { apiRequest } from "../../shared/api/client";

export type PlaybackContext =
  | { type: "direct" | "search" }
  | {
      type: "album" | "playlist" | "artist" | "recommendation";
      id: string;
    };

export type ListeningEventInput = {
  event_id: string;
  track_id: string;
  started_at: string;
  finished_at: string;
  listened_ms: number;
  context_type: PlaybackContext["type"];
  context_id?: string;
  device_type: "web";
};

export type ListeningEventResult = {
  event_id: string;
  recorded: boolean;
  completed: boolean;
};

export function recordListeningEvent(
  accessToken: string,
  event: ListeningEventInput,
  keepalive = false,
) {
  return apiRequest<ListeningEventResult>("/v1/history/events", {
    method: "POST",
    accessToken,
    keepalive,
    body: JSON.stringify(event),
  });
}

/** A private listening event; the server resolves the user from the access token. */
export type ListeningHistoryEvent = {
  event_id: string;
  track_id: string;
  started_at: string;
  finished_at: string | null;
  listened_ms: number;
  completed: boolean;
  context_type: PlaybackContext["type"];
  context_id: string | null;
  device_type: "web" | "mobile" | "desktop" | "tv" | "other";
};

export type ListeningHistoryPage = {
  items: ListeningHistoryEvent[];
  next_cursor?: string;
};

export type ListeningHistoryFilters = {
  trackId?: string;
  from?: string; // RFC3339 inclusive
  to?: string; // RFC3339 exclusive
  completed?: boolean;
  contextType?: PlaybackContext["type"];
  deviceType?: ListeningHistoryEvent["device_type"];
};

export const listeningHistoryPageSize = 20;

export function listeningHistoryPath(
  filters: ListeningHistoryFilters = {},
  cursor?: string,
) {
  const params = new URLSearchParams({
    limit: String(listeningHistoryPageSize),
  });
  if (filters.trackId) params.set("track_id", filters.trackId);
  if (filters.from) params.set("from", filters.from);
  if (filters.to) params.set("to", filters.to);
  if (filters.completed !== undefined) {
    params.set("completed", String(filters.completed));
  }
  if (filters.contextType) params.set("context_type", filters.contextType);
  if (filters.deviceType) params.set("device_type", filters.deviceType);
  if (cursor) params.set("cursor", cursor);
  return `/v1/history/events?${params.toString()}`;
}

export function listListeningHistory(
  accessToken: string,
  filters: ListeningHistoryFilters = {},
  cursor?: string,
  signal?: AbortSignal,
) {
  return apiRequest<ListeningHistoryPage>(
    listeningHistoryPath(filters, cursor),
    {
      accessToken,
      signal,
    },
  );
}

export type ClearListeningHistoryResult = { deleted: number };

/** Without `before`, deletes all events belonging to the signed-in user. */
export function clearListeningHistory(accessToken: string, before?: string) {
  const query = before ? `?${new URLSearchParams({ before }).toString()}` : "";
  return apiRequest<ClearListeningHistoryResult>(`/v1/history/events${query}`, {
    method: "DELETE",
    accessToken,
  });
}
