import { afterEach, describe, expect, it, vi } from "vitest";

import { listInteractionIDs, setInteraction } from "./interactionApi";

afterEach(() => vi.unstubAllGlobals());

describe("interaction API", () => {
  it("loads followed artist identifiers", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        items: [{ artist_id: "artist-id", followed_at: "2026-11-12" }],
        page: { limit: 100, offset: 0, total: 1 },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(listInteractionIDs("access-token", "artist")).resolves.toEqual(
      ["artist-id"],
    );
    expect(fetchMock.mock.calls[0][0]).toBe(
      "http://localhost:8000/v1/interactions/artists/follows?limit=100&offset=0",
    );
  });

  it("uses authenticated PUT and DELETE requests for likes", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ id: "track-id" }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await setInteraction("access-token", "track", "track/id", true);
    await setInteraction("access-token", "track", "track/id", false);

    expect(fetchMock.mock.calls.map(([, options]) => options.method)).toEqual([
      "PUT",
      "DELETE",
    ]);
    expect(fetchMock.mock.calls[0][0]).toBe(
      "http://localhost:8000/v1/interactions/tracks/track%2Fid/like",
    );
    expect(
      new Headers(fetchMock.mock.calls[0][1].headers).get("Authorization"),
    ).toBe("Bearer access-token");
  });
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
