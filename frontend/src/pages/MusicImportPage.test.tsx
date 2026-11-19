import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MusicImportPage } from "./MusicImportPage";

const musicSourceApi = vi.hoisted(() => ({
  backfillSourceArtwork: vi.fn(),
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
          artwork_url: "https://example.com/artwork/42.jpg",
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
    musicSourceApi.backfillSourceArtwork.mockResolvedValue({
      scanned: 1,
      repaired: 1,
      unchanged: 0,
      skipped: 0,
      failed: 0,
      next_offset: 1,
      has_more: false,
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
    expect(document.querySelector(".music-source-art img")).toHaveAttribute(
      "src",
      "https://example.com/artwork/42.jpg",
    );
    expect(musicSourceApi.discoverSourceTracks).toHaveBeenCalledWith(
      "access-token",
      "jamendo",
      "night city",
      0,
      expect.any(AbortSignal),
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

  it("navigates search pages without including unsupported tracks", async () => {
    const user = userEvent.setup();
    musicSourceApi.discoverSourceTracks
      .mockResolvedValueOnce({
        items: [],
        limit: 20,
        offset: 0,
        total: 42,
      })
      .mockResolvedValueOnce({
        items: [
          {
            provider: "jamendo",
            external_id: "99",
            title: "After Us",
            artist: "Example Artist",
            page_url: "https://example.com/99",
            artwork_url: "",
            license: {
              name: "CC BY",
              url: "https://creativecommons.org/licenses/by/4.0/",
              attribution: "Example Artist",
              allows_streaming: true,
              allows_redistribution: true,
            },
            metadata: {},
          },
        ],
        limit: 20,
        offset: 20,
        total: 42,
      });
    render(<MusicImportPage />);
    await screen.findByRole("heading", {
      name: "Import music from a licensed source.",
    });
    await user.type(screen.getByLabelText("Track name"), "After Us");
    await user.click(screen.getByRole("button", { name: "Search source" }));
    expect(
      await screen.findByText(/No importable tracks on this page/),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Next page" }));
    expect(await screen.findByText("Example Artist")).toBeInTheDocument();
    expect(screen.getByText("Page 2 of 3")).toBeInTheDocument();
    expect(musicSourceApi.discoverSourceTracks).toHaveBeenLastCalledWith(
      "access-token",
      "jamendo",
      "After Us",
      20,
      expect.any(AbortSignal),
    );

    await user.clear(screen.getByLabelText("Track name"));
    expect(screen.queryByText("Example Artist")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Next page" }),
    ).not.toBeInTheDocument();
  });

  it("explains when no draft tracks are available", async () => {
    musicSourceApi.listDraftReleaseTracks.mockResolvedValue([]);
    render(<MusicImportPage />);
    expect(
      await screen.findByText("Create a draft release first"),
    ).toBeInTheDocument();
  });

  it("repairs artwork for existing editable imports", async () => {
    const user = userEvent.setup();
    render(<MusicImportPage />);
    await screen.findByRole("heading", {
      name: "Import music from a licensed source.",
    });

    await user.click(screen.getByRole("button", { name: "Repair artwork" }));

    expect(await screen.findByRole("status")).toHaveTextContent(
      "1 updated, 0 already complete",
    );
    expect(musicSourceApi.backfillSourceArtwork).toHaveBeenCalledWith(
      "access-token",
      50,
      0,
    );
  });
});
