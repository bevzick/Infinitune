import { afterEach, describe, expect, it, vi } from "vitest";

import { getRecommendedTracks } from "./recommendationApi";

afterEach(() => vi.unstubAllGlobals());

describe("recommendation API", () => {
  it("hydrates ranked track IDs without changing their order", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          run_id: "run-id",
          algorithm_version: "weighted-v1",
          generated_at: "2026-11-11T00:00:00Z",
          expires_at: "2026-11-11T01:00:00Z",
          items: [
            {
              track_id: "track-2",
              score: 2,
              rank: 1,
              reason: "Preferred genre",
            },
            {
              track_id: "track-1",
              score: 1,
              rank: 2,
              reason: "Recent listening",
            },
          ],
          total: 2,
          limit: 6,
          offset: 0,
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ id: "track-2", title: "Second" }))
      .mockResolvedValueOnce(jsonResponse({ id: "track-1", title: "First" }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await getRecommendedTracks("access-token", 6);

    expect(result.map(({ track }) => track.id)).toEqual(["track-2", "track-1"]);
    expect(result[0].recommendationId).toBe("run-id");
    expect(fetchMock.mock.calls[0][0]).toBe(
      "http://localhost:8000/v1/recommendations?limit=6&offset=0",
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
