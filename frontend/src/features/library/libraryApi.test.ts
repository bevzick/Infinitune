import { afterEach, describe, expect, it, vi } from "vitest";

import { loadLibrary } from "./libraryApi";

afterEach(() => vi.unstubAllGlobals());

describe("library API", () => {
  it("combines playlists, likes, follows, and catalog details", async () => {
    const fetchMock = vi.fn(
      (input: string | URL | Request, _init?: RequestInit) => {
        const url = String(input);
        if (url.includes("/v1/playlists?")) {
          return jsonResponse({
            items: [{ id: "playlist-id", name: "Night" }],
            page: page(1),
          });
        }
        if (url.includes("/tracks/likes?")) {
          return jsonResponse({
            items: [{ id: "track-id", liked_at: "2026-11-11T03:00:00Z" }],
            page: page(1),
          });
        }
        if (url.includes("/albums/likes?")) {
          return jsonResponse({
            items: [{ id: "album-id", liked_at: "2026-11-11T02:00:00Z" }],
            page: page(1),
          });
        }
        if (url.includes("/artists/follows?")) {
          return jsonResponse({
            items: [
              { artist_id: "artist-id", followed_at: "2026-11-11T01:00:00Z" },
            ],
            page: page(1),
          });
        }
        if (url.endsWith("/v1/catalog/tracks/track-id"))
          return jsonResponse({ id: "track-id", title: "Signal" });
        if (url.endsWith("/v1/catalog/albums/album-id"))
          return jsonResponse({ id: "album-id", title: "Orbit" });
        if (url.endsWith("/v1/catalog/artists/artist-id"))
          return jsonResponse({ id: "artist-id", name: "Lumen" });
        throw new Error(`Unexpected request: ${url}`);
      },
    );
    vi.stubGlobal("fetch", fetchMock);

    const result = await loadLibrary("access-token");

    expect(result.playlists[0].id).toBe("playlist-id");
    expect(result.tracks[0]).toMatchObject({
      item: { id: "track-id" },
      savedAt: "2026-11-11T03:00:00Z",
    });
    expect(result.albums[0].item.id).toBe("album-id");
    expect(result.artists[0].item.id).toBe("artist-id");
    const interactionCall = fetchMock.mock.calls.find(([url]) =>
      String(url).includes("/tracks/likes"),
    );
    expect(
      new Headers(interactionCall?.[1]?.headers).get("Authorization"),
    ).toBe("Bearer access-token");
  });
});

function page(total: number) {
  return { limit: 100, offset: 0, total };
}

function jsonResponse(body: unknown) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
}
