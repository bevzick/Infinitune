import { afterEach, describe, expect, it, vi } from "vitest";

import { listPlaylists, reorderPlaylistTracks } from "./playlistApi";

afterEach(() => vi.unstubAllGlobals());

describe("playlist API", () => {
  it("lists the verified user's playlists with pagination", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        items: [],
        page: { limit: 12, offset: 24, total: 0 },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await listPlaylists("access-token", 12, 24);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe("http://localhost:8000/v1/playlists?limit=12&offset=24");
    expect(new Headers(options.headers).get("Authorization")).toBe(
      "Bearer access-token",
    );
  });

  it("sends the complete ordered track ID list", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse([]));
    vi.stubGlobal("fetch", fetchMock);

    await reorderPlaylistTracks("access-token", "playlist-id", [
      "second-track",
      "first-track",
    ]);

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe(
      "http://localhost:8000/v1/playlists/playlist-id/tracks/order",
    );
    expect(options.method).toBe("PUT");
    expect(JSON.parse(options.body)).toEqual({
      track_ids: ["second-track", "first-track"],
    });
  });
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
