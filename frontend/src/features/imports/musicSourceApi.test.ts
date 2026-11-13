import { afterEach, describe, expect, it, vi } from "vitest";

import {
  discoverSourceTracks,
  importSourceTrack,
  listDraftReleaseTracks,
  listMusicSourceProviders,
} from "./musicSourceApi";

afterEach(() => vi.unstubAllGlobals());

describe("music source API", () => {
  it("loads configured providers and encoded discovery results", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ items: [{ id: "jamendo" }] }))
      .mockResolvedValueOnce(
        jsonResponse({ items: [], limit: 20, offset: 0, total: 0 }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await expect(listMusicSourceProviders("token")).resolves.toEqual([
      "jamendo",
    ]);
    await discoverSourceTracks("token", "jamendo", "night city");

    expect(fetchMock.mock.calls[1][0]).toBe(
      "http://localhost:8000/v1/media/sources/jamendo/tracks?query=night+city&limit=20&offset=0",
    );
  });

  it("collects tracks from the user's draft releases", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          items: [{ id: "release-1", title: "Signals", tracks: [] }],
          page: { limit: 50, offset: 0, total: 1 },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          id: "release-1",
          title: "Signals",
          status: "draft",
          tracks: [{ id: "track-1", title: "Night Signal" }],
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await expect(listDraftReleaseTracks("token")).resolves.toEqual([
      {
        id: "track-1",
        title: "Night Signal",
        releaseId: "release-1",
        releaseTitle: "Signals",
      },
    ]);
  });

  it("starts an import for the selected release track", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        media_asset_id: "asset-1",
        content_track_id: "track-1",
        status: "uploaded",
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await importSourceTrack("token", "track-1", {
      provider: "internet_archive",
      external_id: "concert/audio/track.flac",
    });

    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      content_track_id: "track-1",
      provider: "internet_archive",
      external_id: "concert/audio/track.flac",
    });
  });
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
