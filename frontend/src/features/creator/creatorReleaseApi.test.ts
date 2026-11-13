import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createCreatorAccount,
  createRelease,
  listDraftReleases,
  submitCreatorVerification,
  updateRelease,
} from "./creatorReleaseApi";

afterEach(() => vi.unstubAllGlobals());

describe("creator release API", () => {
  it("creates and submits a creator account for verification", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(async () =>
        jsonResponse({ id: "creator-1", verification_status: "unverified" }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await createCreatorAccount("token", {
      type: "artist",
      display_name: "Luna Veil",
    });
    await submitCreatorVerification("token", "creator-1");

    expect(fetchMock.mock.calls[0][0]).toBe(
      "http://localhost:8000/v1/creators",
    );
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      type: "artist",
      display_name: "Luna Veil",
    });
    expect(fetchMock.mock.calls[1][0]).toBe(
      "http://localhost:8000/v1/creators/creator-1/verification-submissions",
    );
    expect(fetchMock.mock.calls[1][1].method).toBe("POST");
  });

  it("hydrates draft release tracks", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          items: [{ id: "release-1", title: "Signals", tracks: [] }],
          page: { limit: 100, offset: 0, total: 1 },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          id: "release-1",
          title: "Signals",
          tracks: [{ id: "track-1", title: "Night Signal" }],
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const releases = await listDraftReleases("token");

    expect(releases[0].tracks[0].id).toBe("track-1");
    expect(fetchMock.mock.calls[0][0]).toContain(
      "/v1/releases?status=draft&limit=100&offset=0",
    );
  });

  it("serializes create and update operations", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(async () =>
        jsonResponse({ id: "release-1", tracks: [] }),
      );
    vi.stubGlobal("fetch", fetchMock);
    const createInput = {
      creator_id: "creator-1",
      title: "Signals",
      release_type: "single" as const,
      release_date: "2026-11-13",
      tracks: [
        {
          title: "Signals",
          track_number: 1,
          disc_number: 1,
          explicit: false,
        },
      ],
    };

    await createRelease("token", createInput);
    await updateRelease("token", "release-1", {
      title: createInput.title,
      release_type: createInput.release_type,
      release_date: createInput.release_date,
      cover_object_key: null,
      tracks: [{ id: "track-1", ...createInput.tracks[0] }],
    });

    expect(fetchMock.mock.calls[0][1].method).toBe("POST");
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual(createInput);
    expect(fetchMock.mock.calls[1][0]).toBe(
      "http://localhost:8000/v1/releases/release-1",
    );
    expect(fetchMock.mock.calls[1][1].method).toBe("PUT");
  });
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
