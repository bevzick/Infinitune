import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MusicImportPage } from "./MusicImportPage";

const musicSourceApi = vi.hoisted(() => ({
  listMusicSourceProviders: vi.fn(),
  listDraftReleaseTracks: vi.fn(),
  discoverSourceTracks: vi.fn(),
  importSourceTrack: vi.fn(),
}));

vi.mock("../features/imports/musicSourceApi", () => musicSourceApi);
vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({ accessToken: "access-token" }),
}));

describe("MusicImportPage", () => {
  beforeEach(() => {
    musicSourceApi.listMusicSourceProviders.mockResolvedValue([
      "jamendo",
      "internet_archive",
    ]);
    musicSourceApi.listDraftReleaseTracks.mockResolvedValue([
      {
        id: "release-track-1",
        title: "Night Signal",
        releaseId: "release-1",
        releaseTitle: "Signals",
      },
    ]);
    musicSourceApi.discoverSourceTracks.mockResolvedValue({
      items: [
        {
          provider: "jamendo",
          external_id: "42",
          title: "Night Signal",
          artist: "Luna Veil",
          page_url: "https://example.com/tracks/42",
          license: {
            name: "CC BY 4.0",
            url: "https://creativecommons.org/licenses/by/4.0/",
            attribution: "Luna Veil",
            allows_streaming: true,
            allows_redistribution: true,
          },
          metadata: {},
        },
      ],
      limit: 20,
      offset: 0,
      total: 1,
    });
    musicSourceApi.importSourceTrack.mockResolvedValue({
      media_asset_id: "asset-1",
      content_track_id: "release-track-1",
      status: "uploaded",
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("discovers and imports a provider track into a draft release", async () => {
    const user = userEvent.setup();
    render(<MusicImportPage />);

    expect(
      await screen.findByRole("heading", {
        name: "Import music from a licensed source.",
      }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Draft release track")).toHaveValue(
      "release-track-1",
    );

    await user.type(screen.getByLabelText("Track name"), "night city");
    await user.click(screen.getByRole("button", { name: "Search source" }));

    expect(await screen.findByText("Luna Veil")).toBeInTheDocument();
    expect(musicSourceApi.discoverSourceTracks).toHaveBeenCalledWith(
      "access-token",
      "jamendo",
      "night city",
    );

    await user.click(screen.getByRole("button", { name: "Import track" }));
    expect(await screen.findByRole("status")).toHaveTextContent(
      "is being imported",
    );
    expect(musicSourceApi.importSourceTrack).toHaveBeenCalledWith(
      "access-token",
      "release-track-1",
      expect.objectContaining({ external_id: "42" }),
    );
  });

  it("explains when no draft tracks are available", async () => {
    musicSourceApi.listDraftReleaseTracks.mockResolvedValue([]);
    render(<MusicImportPage />);
    expect(
      await screen.findByText("Create a draft release first"),
    ).toBeInTheDocument();
  });
});
