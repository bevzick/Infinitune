import type { Page, Request, Route } from "@playwright/test";

import { mockGateway } from "./gatewayMock";

export const historyTrackId = "00000000-0000-4000-8000-000000000101";
export const historyOtherTrackId = "00000000-0000-4000-8000-000000000102";

export type HistoryActor = "alice" | "bob";

type StoredEvent = {
  event_id: string;
  track_id: string;
  started_at: string;
  finished_at: string | null;
  listened_ms: number;
  completed: boolean;
  context_type:
    | "direct"
    | "search"
    | "album"
    | "playlist"
    | "artist"
    | "recommendation";
  context_id: string | null;
  device_type: "web" | "mobile" | "desktop" | "tv" | "other";
};

type RecordInput = Omit<StoredEvent, "completed"> & {
  finished_at: string;
  context_id?: string | null;
};

const gatewayBase = "http://localhost:8000";
const corsHeaders = {
  "access-control-allow-origin": "http://127.0.0.1:5173",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
  "access-control-allow-headers":
    "Accept, Authorization, Content-Type, X-Requested-With, X-CSRF-Token",
};
const userID = {
  alice: "00000000-0000-4000-8000-000000000031",
  bob: "00000000-0000-4000-8000-000000000032",
};
const tokenFor = (actor: HistoryActor) => `history-${actor}-token`;
const uuid =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function json(route: Route, payload: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    headers: corsHeaders,
    body: JSON.stringify(payload),
  });
}

function failure(route: Route, status: number, code: string) {
  return json(route, { error: { code, message: code } }, status);
}

// A real, decodable cursor like history-service's base64url JSON document.
function encodeCursor(event: StoredEvent) {
  return Buffer.from(
    JSON.stringify({ started_at: event.started_at, event_id: event.event_id }),
  ).toString("base64url");
}

function decodeCursor(value: string) {
  try {
    return JSON.parse(Buffer.from(value, "base64url").toString("utf8")) as {
      started_at: string;
      event_id: string;
    };
  } catch {
    return null;
  }
}

function silentWav(seconds = 2) {
  const samples = seconds * 8_000;
  const wav = Buffer.alloc(44 + samples * 2);
  wav.write("RIFF", 0);
  wav.writeUInt32LE(wav.length - 8, 4);
  wav.write("WAVEfmt ", 8);
  wav.writeUInt32LE(16, 16);
  wav.writeUInt16LE(1, 20);
  wav.writeUInt16LE(1, 22);
  wav.writeUInt32LE(8_000, 24);
  wav.writeUInt32LE(16_000, 28);
  wav.writeUInt16LE(2, 32);
  wav.writeUInt16LE(16, 34);
  wav.write("data", 36);
  wav.writeUInt32LE(samples * 2, 40);
  return wav;
}

const track = {
  id: historyTrackId,
  album_id: "00000000-0000-4000-8000-000000000201",
  playback_url: `/v1/media/tracks/${historyTrackId}/playback`,
  title: "Night Drive",
  duration_ms: 2_000,
  track_number: 1,
  disc_number: 1,
  explicit: false,
  release_date: "2026-11-10",
  artists: [
    {
      artist: {
        id: "00000000-0000-4000-8000-000000000301",
        name: "Luna Veil",
        image_url: "",
      },
      role: "primary",
      position: 1,
    },
  ],
  genres: [],
  moods: [],
  created_at: "2026-11-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

/** Browser-only contract mock. Never shares listening history across identities. */
export function createListeningHistoryGateway() {
  const events = new Map<HistoryActor, StoredEvent[]>([
    ["alice", []],
    ["bob", []],
  ]);
  const requests: Array<{ actor: HistoryActor; request: Request }> = [];
  const unavailableLists = new Set<HistoryActor>();

  const eventsFor = (actor: HistoryActor) => [...(events.get(actor) || [])];

  function seed(
    actor: HistoryActor,
    count: number,
    startAt = "2026-11-10T10:00:00Z",
  ) {
    const rows = events.get(actor);
    if (!rows) return;
    const offset = rows.length;
    for (let i = 0; i < count; i++) {
      rows.push({
        event_id: `00000000-0000-4000-8000-${String(1000 + offset + i).padStart(12, "0")}`,
        track_id: i % 2 ? historyOtherTrackId : historyTrackId,
        started_at: new Date(Date.parse(startAt) + i * 60_000).toISOString(),
        finished_at: new Date(
          Date.parse(startAt) + i * 60_000 + 2_000,
        ).toISOString(),
        listened_ms: i % 2 ? 1_200 : 2_000,
        completed: i % 2 === 0,
        context_type: i % 2 ? "search" : "direct",
        context_id: null,
        device_type: "web",
      });
    }
  }

  async function attach(page: Page, actor: HistoryActor) {
    await mockGateway(page);
    await page.route(`${gatewayBase}/v1/**`, async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const path = url.pathname;
      const method = request.method();
      requests.push({ actor, request });

      if (method === "OPTIONS") {
        return route.fulfill({ status: 204, headers: corsHeaders });
      }
      if (path === "/v1/auth/refresh" && method === "POST") {
        if (request.headers()["x-csrf-token"] !== "history-e2e-csrf") {
          return failure(route, 403, "invalid_csrf_token");
        }
        return json(route, {
          access_token: tokenFor(actor),
          token_type: "Bearer",
          expires_at: "2099-01-01T00:00:00Z",
        });
      }
      if (path === "/v1/auth/login" && method === "POST") {
        const body = request.postDataJSON() as { email?: string };
        if (body.email !== `${actor}@example.com`) {
          return failure(route, 401, "invalid_credentials");
        }
        return json(route, {
          access_token: tokenFor(actor),
          token_type: "Bearer",
          expires_at: "2099-01-01T00:00:00Z",
        });
      }
      if (path === "/v1/auth/identity" && method === "GET") {
        if (request.headers().authorization !== `Bearer ${tokenFor(actor)}`) {
          return failure(route, 401, "unauthorized");
        }
        return json(route, {
          user_id: userID[actor],
          roles: ["listener"],
          issued_at: "2026-11-10T00:00:00Z",
          expires_at: "2099-01-01T00:00:00Z",
        });
      }
      if (path === `/v1/catalog/tracks/${historyTrackId}` && method === "GET") {
        return json(route, track);
      }
      if (
        path === `/v1/catalog/tracks/${historyOtherTrackId}` &&
        method === "GET"
      ) {
        return json(route, {
          ...track,
          id: historyOtherTrackId,
          title: "Late Signal",
        });
      }
      if (
        path === `/v1/tracks/${historyTrackId}/comments` &&
        method === "GET"
      ) {
        return json(route, {
          items: [],
          pagination: { limit: 20, offset: 0, total: 0 },
        });
      }
      if (
        path === `/v1/media/tracks/${historyTrackId}/playback` &&
        method === "GET"
      ) {
        if (request.headers().authorization !== `Bearer ${tokenFor(actor)}`) {
          return failure(route, 401, "unauthorized");
        }
        return route.fulfill({
          status: 200,
          headers: { ...corsHeaders, "content-type": "audio/wav" },
          body: silentWav(),
        });
      }
      if (path !== "/v1/history/events") return route.fallback();
      if (request.headers().authorization !== `Bearer ${tokenFor(actor)}`) {
        return failure(route, 401, "unauthorized");
      }
      const rows = events.get(actor);
      if (!rows) return failure(route, 401, "unauthorized");

      if (method === "POST") {
        const input = request.postDataJSON() as RecordInput;
        if (
          !uuid.test(input.event_id || "") ||
          !uuid.test(input.track_id || "")
        ) {
          return failure(route, 400, "invalid_listening_event");
        }
        const existing = rows.find(
          (event) => event.event_id === input.event_id,
        );
        if (existing) {
          const { completed: _, ...stored } = existing;
          const original = {
            ...stored,
            context_id: stored.context_id ?? undefined,
          };
          const replay = {
            ...input,
            context_id: input.context_id ?? undefined,
          };
          if (JSON.stringify(original) !== JSON.stringify(replay)) {
            return failure(route, 409, "event_conflict");
          }
          return json(route, {
            event_id: input.event_id,
            recorded: true,
            completed: existing.completed,
          });
        }
        // Short tracks qualify after half of their duration (2s / 2 = 1s).
        const qualifies = input.listened_ms >= 1_000;
        const completed = input.listened_ms >= 1_800;
        if (qualifies) {
          rows.push({
            ...input,
            context_id: input.context_id ?? null,
            completed,
          });
        }
        return json(route, {
          event_id: input.event_id,
          recorded: qualifies,
          completed,
        });
      }

      if (method === "GET") {
        if (unavailableLists.has(actor)) {
          return failure(route, 503, "history_unavailable");
        }
        let filtered = [...rows].sort(
          (a, b) =>
            b.started_at.localeCompare(a.started_at) ||
            b.event_id.localeCompare(a.event_id),
        );
        for (const [key, matches] of [
          ["track_id", (e: StoredEvent, value: string) => e.track_id === value],
          [
            "completed",
            (e: StoredEvent, value: string) =>
              e.completed === (value === "true"),
          ],
          [
            "context_type",
            (e: StoredEvent, value: string) => e.context_type === value,
          ],
          [
            "device_type",
            (e: StoredEvent, value: string) => e.device_type === value,
          ],
          ["from", (e: StoredEvent, value: string) => e.started_at >= value],
          ["to", (e: StoredEvent, value: string) => e.started_at < value],
        ] as const) {
          const value = url.searchParams.get(key);
          if (value !== null)
            filtered = filtered.filter((event) => matches(event, value));
        }
        const cursorValue = url.searchParams.get("cursor");
        if (cursorValue) {
          const cursor = decodeCursor(cursorValue);
          if (!cursor || !uuid.test(cursor.event_id))
            return failure(route, 400, "invalid_history_query");
          filtered = filtered.filter(
            (event) =>
              event.started_at < cursor.started_at ||
              (event.started_at === cursor.started_at &&
                event.event_id < cursor.event_id),
          );
        }
        const limit = Math.min(
          100,
          Number(url.searchParams.get("limit") || "25"),
        );
        const page = filtered.slice(0, limit);
        const next_cursor =
          filtered.length > limit
            ? encodeCursor(page[page.length - 1])
            : undefined;
        return json(route, {
          items: page,
          ...(next_cursor ? { next_cursor } : {}),
        });
      }

      if (method === "DELETE") {
        const before = url.searchParams.get("before");
        const kept = before ? rows.filter((e) => e.started_at >= before) : [];
        const deleted = rows.length - kept.length;
        events.set(actor, kept);
        return json(route, { deleted });
      }
      return failure(route, 405, "method_not_allowed");
    });
  }

  return {
    attach,
    seed,
    eventsFor,
    requests,
    setListUnavailable(actor: HistoryActor, unavailable: boolean) {
      if (unavailable) unavailableLists.add(actor);
      else unavailableLists.delete(actor);
    },
  };
}
