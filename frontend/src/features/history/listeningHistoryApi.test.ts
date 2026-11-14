import { afterEach, describe, expect, it, vi } from "vitest";

import {
  clearListeningHistory,
  listeningHistoryPath,
  listListeningHistory,
} from "./historyApi";

afterEach(() => vi.unstubAllGlobals());

describe("private listening history API", () => {
  it("serializes cursor and the filters expected by History Service", () => {
    const path = listeningHistoryPath(
      {
        trackId: "11111111-1111-4111-8111-111111111111",
        from: "2026-11-01T00:00:00.000Z",
        to: "2026-11-15T00:00:00.000Z",
        completed: false,
        contextType: "playlist",
        deviceType: "web",
      },
      "cursor+/value",
    );
    const url = new URL(path, "http://localhost");
    expect(url.pathname).toBe("/v1/history/events");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      limit: "20",
      track_id: "11111111-1111-4111-8111-111111111111",
      from: "2026-11-01T00:00:00.000Z",
      to: "2026-11-15T00:00:00.000Z",
      completed: "false",
      context_type: "playlist",
      device_type: "web",
      cursor: "cursor+/value",
    });
  });

  it("lists only the authenticated user's private events", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ items: [], next_cursor: "another" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();
    const page = await listListeningHistory(
      "private-token",
      { completed: true },
      "cursor",
      controller.signal,
    );
    expect(page.next_cursor).toBe("another");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toContain("/v1/history/events?");
    expect(new URL(url).searchParams.get("completed")).toBe("true");
    expect(init.signal).toBe(controller.signal);
    expect(new Headers(init.headers).get("Authorization")).toBe(
      "Bearer private-token",
    );
    expect(init.credentials).toBe("include");
  });

  it("clears all or only events before an RFC3339 cutoff", async () => {
    const fetchMock = vi.fn().mockImplementation(
      async () =>
        new Response(JSON.stringify({ deleted: 3 }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", fetchMock);
    expect(await clearListeningHistory("token")).toEqual({ deleted: 3 });
    expect(
      await clearListeningHistory("token", "2026-11-01T00:00:00.000Z"),
    ).toEqual({ deleted: 3 });
    expect(fetchMock.mock.calls[0][0]).toBe(
      "http://localhost:8000/v1/history/events",
    );
    expect(fetchMock.mock.calls[1][0]).toContain(
      "before=2026-11-01T00%3A00%3A00.000Z",
    );
    for (const [, init] of fetchMock.mock.calls) {
      expect(init.method).toBe("DELETE");
      expect(new Headers(init.headers).get("Authorization")).toBe(
        "Bearer token",
      );
    }
  });
});
