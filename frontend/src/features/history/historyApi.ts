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
