import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { CreatorReleasesPage } from "./CreatorReleasesPage";

const creatorApi = vi.hoisted(() => ({
  listCreatorAccounts: vi.fn(),
  listDraftReleases: vi.fn(),
  createRelease: vi.fn(),
  updateRelease: vi.fn(),
  listTrackRights: vi.fn(),
  setTrackRight: vi.fn(),
  deleteTrackRight: vi.fn(),
  createTrackUploadSession: vi.fn(),
  uploadAudioFile: vi.fn(),
}));

vi.mock("../features/creator/creatorReleaseApi", () => creatorApi);
vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({ accessToken: "access-token" }),
}));

const release = {
  id: "release-1",
  creator_id: "creator-1",
  title: "Signals",
  release_type: "single",
  release_date: "2026-11-13",
  status: "draft",
  cover_object_key: null,
  tracks: [
    {
      id: "track-1",
      title: "Night Signal",
      track_number: 1,
      disc_number: 1,
      explicit: false,
    },
  ],
  created_at: "2026-11-13T00:00:00Z",
  updated_at: "2026-11-13T00:00:00Z",
};

describe("CreatorReleasesPage", () => {
  beforeEach(() => {
    creatorApi.listCreatorAccounts.mockResolvedValue([
      {
        id: "creator-1",
        type: "artist",
        display_name: "Luna Veil",
        verification_status: "verified",
      },
    ]);
    creatorApi.listDraftReleases.mockResolvedValue([release]);
    creatorApi.createRelease.mockImplementation(
      async (_token: string, input: Record<string, unknown>) => ({
        ...release,
        id: "release-2",
        title: input.title,
      }),
    );
    creatorApi.updateRelease.mockImplementation(
      async (_token: string, _id: string, input: Record<string, unknown>) => ({
        ...release,
        title: input.title,
      }),
    );
    creatorApi.listTrackRights.mockResolvedValue([]);
    creatorApi.setTrackRight.mockImplementation(
      async (
        _token: string,
        trackId: string,
        input: Record<string, unknown>,
      ) => ({
        id: `right-${String(input.rights_type)}`,
        release_track_id: trackId,
        ...input,
      }),
    );
    creatorApi.deleteTrackRight.mockResolvedValue(undefined);
    creatorApi.createTrackUploadSession.mockResolvedValue({
      id: "upload-1",
      upload_url: "/v1/media/uploads/upload-1",
      status: "pending",
    });
    creatorApi.uploadAudioFile.mockImplementation(
      async (
        _token: string,
        _url: string,
        _file: File,
        _mimeType: string,
        onProgress: (progress: number) => void,
      ) => {
        onProgress(50);
        onProgress(100);
        return {
          media_asset_id: "asset-1",
          content_track_id: "track-1",
          status: "uploaded",
        };
      },
    );
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("creates a release with its initial track", async () => {
    const user = userEvent.setup();
    render(<CreatorReleasesPage />);

    await user.click(
      await screen.findByRole("button", { name: "+ New release" }),
    );
    await user.type(screen.getByLabelText("Release title"), "New Signal");
    await user.type(screen.getByLabelText("Title"), "Opening Track");
    await user.click(screen.getByRole("button", { name: "Create draft" }));

    expect(creatorApi.createRelease).toHaveBeenCalledWith("access-token", {
      creator_id: "creator-1",
      title: "New Signal",
      release_type: "single",
      release_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
      tracks: [
        {
          title: "Opening Track",
          track_number: 1,
          disc_number: 1,
          explicit: false,
        },
      ],
    });
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Draft “New Signal” was created.",
    );
  });

  it("updates metadata while preserving track IDs", async () => {
    const user = userEvent.setup();
    render(<CreatorReleasesPage />);

    await user.click(await screen.findByRole("button", { name: "Edit draft" }));
    const title = screen.getByLabelText("Release title");
    await user.clear(title);
    await user.type(title, "Signals Remastered");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(creatorApi.updateRelease).toHaveBeenCalledWith(
      "access-token",
      "release-1",
      expect.objectContaining({
        title: "Signals Remastered",
        tracks: [expect.objectContaining({ id: "track-1" })],
      }),
    );
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Draft “Signals Remastered” was updated.",
    );
  });

  it("manages required worldwide track rights", async () => {
    const user = userEvent.setup();
    creatorApi.listTrackRights.mockResolvedValue([
      {
        id: "right-streaming",
        release_track_id: "track-1",
        territory_code: "001",
        rights_type: "streaming",
        valid_from: "2026-11-13",
        valid_until: null,
      },
    ]);
    render(<CreatorReleasesPage />);

    await user.click(
      await screen.findByRole("button", { name: "Manage rights" }),
    );
    const streaming = await screen.findByRole("checkbox", {
      name: "Streaming",
    });
    const distribution = screen.getByRole("checkbox", {
      name: "Distribution",
    });
    expect(streaming).toBeChecked();
    expect(distribution).not.toBeChecked();

    await user.click(distribution);
    expect(creatorApi.setTrackRight).toHaveBeenCalledWith(
      "access-token",
      "track-1",
      {
        territory_code: "001",
        rights_type: "distribution",
        valid_from: "2026-11-13",
        valid_until: null,
      },
    );

    await user.click(streaming);
    expect(creatorApi.deleteTrackRight).toHaveBeenCalledWith(
      "access-token",
      "right-streaming",
    );
  });

  it("uploads local audio for a release track", async () => {
    const user = userEvent.setup();
    render(<CreatorReleasesPage />);

    await user.click(
      await screen.findByRole("button", { name: "Upload audio" }),
    );
    const file = new File(["audio-data"], "night-signal.mp3", {
      type: "audio/mpeg",
    });
    await user.upload(
      screen.getByLabelText("Audio file for Night Signal"),
      file,
    );
    await user.click(
      screen.getByRole("button", { name: "Upload Night Signal" }),
    );

    expect(creatorApi.createTrackUploadSession).toHaveBeenCalledWith(
      "access-token",
      "track-1",
      {
        idempotency_key: expect.stringMatching(
          /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
        ),
        original_name: "night-signal.mp3",
        mime_type: "audio/mpeg",
        size_bytes: 10,
      },
    );
    expect(creatorApi.uploadAudioFile).toHaveBeenCalledWith(
      "access-token",
      "/v1/media/uploads/upload-1",
      file,
      "audio/mpeg",
      expect.any(Function),
    );
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Uploaded. Audio processing has been queued.",
    );
  });
});
