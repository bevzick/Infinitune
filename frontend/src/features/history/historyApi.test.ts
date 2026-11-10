import { afterEach, describe, expect, it, vi } from "vitest";

import { recordListeningEvent } from "./historyApi";

afterEach(() => vi.unstubAllGlobals());

describe("history API", () => {
  it("records an authenticated idempotent event with keepalive", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          event_id: "event-id",
          recorded: true,
          completed: false,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);
    const event = {
      event_id: "event-id",
      track_id: "track-id",
      started_at: "2026-11-10T12:00:00.000Z",
      finished_at: "2026-11-10T12:00:35.000Z",
      listened_ms: 35_000,
      context_type: "search" as const,
      device_type: "web" as const,
    };

    await recordListeningEvent("access-token", event, true);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe("http://localhost:8000/v1/history/events");
    expect(options.method).toBe("POST");
    expect(options.keepalive).toBe(true);
    expect(new Headers(options.headers).get("Authorization")).toBe(
      "Bearer access-token",
    );
    expect(JSON.parse(options.body)).toEqual(event);
  });
});
