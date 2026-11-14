import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createCreatorAccount,
  createRelease,
  createTrackUploadSession,
  listDraftReleases,
  submitCreatorVerification,
  updateRelease,
  uploadAudioFile,
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
    await createTrackUploadSession("token", "track-1", {
      idempotency_key: "00000000-0000-4000-8000-000000000001",
      original_name: "signals.flac",
      mime_type: "audio/flac",
      size_bytes: 1024,
    });

    expect(fetchMock.mock.calls[0][1].method).toBe("POST");
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual(createInput);
    expect(fetchMock.mock.calls[1][0]).toBe(
      "http://localhost:8000/v1/releases/release-1",
    );
    expect(fetchMock.mock.calls[1][1].method).toBe("PUT");
    expect(fetchMock.mock.calls[2][0]).toBe(
      "http://localhost:8000/v1/release-tracks/track-1/uploads",
    );
    expect(fetchMock.mock.calls[2][1].method).toBe("POST");
    expect(JSON.parse(fetchMock.mock.calls[2][1].body)).toEqual({
      idempotency_key: "00000000-0000-4000-8000-000000000001",
      original_name: "signals.flac",
      mime_type: "audio/flac",
      size_bytes: 1024,
    });
  });

  it.each([
    {
      caseName: "structured error payload",
      response: {
        error: {
          code: "ingest_unavailable",
          message: "Media storage temporarily unavailable",
        },
      },
      expectedCode: "ingest_unavailable",
      expectedMessage: "Media storage temporarily unavailable",
    },
    {
      caseName: "missing error payload",
      response: null,
      expectedCode: "upload_failed",
      expectedMessage: "Audio upload failed with status 503",
    },
  ])(
    "rejects HTTP upload failures with $caseName",
    async ({ response, expectedCode, expectedMessage }) => {
      class FailedUploadRequest {
        status = 503;
        response = response;
        responseType = "";
        withCredentials = false;
        upload = { addEventListener: vi.fn() };
        open = vi.fn();
        setRequestHeader = vi.fn();
        private loadListener?: () => void;

        addEventListener(type: string, listener: () => void) {
          if (type === "load") this.loadListener = listener;
        }

        send() {
          this.loadListener?.();
        }
      }

      vi.stubGlobal("XMLHttpRequest", FailedUploadRequest);
      const onProgress = vi.fn();
      const file = new File(["audio"], "night-signal.mp3", {
        type: "audio/mpeg",
      });

      await expect(
        uploadAudioFile(
          "access-token",
          "/v1/media/uploads/upload-1",
          file,
          "audio/mpeg",
          onProgress,
        ),
      ).rejects.toMatchObject({
        status: 503,
        code: expectedCode,
        message: expectedMessage,
      });
      expect(onProgress).not.toHaveBeenCalled();
    },
  );
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
