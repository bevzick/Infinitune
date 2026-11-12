import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AlbumPage } from "./CatalogDetailPage";

const player = vi.hoisted(() => ({
  playQueue: vi.fn(),
  toggle: vi.fn(),
  currentTrack: null,
  status: "idle",
}));

const album = {
  id: "album-id",
  title: "Night City",
  album_type: "album" as const,
  release_date: "2026-11-10",
  cover_url: "",
  artists: [
    {
      artist: { id: "artist-id", name: "Luna Veil", image_url: "" },
      position: 1,
    },
  ],
  created_at: "2026-11-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

const track = {
  id: "track-id",
  album_id: album.id,
  playback_url: "/playback",
  title: "After Midnight",
  duration_ms: 228000,
  track_number: 1,
  disc_number: 1,
  explicit: false,
  release_date: "2026-11-10",
  artists: [],
  genres: [],
  moods: [],
  created_at: "2026-11-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

vi.mock("../features/catalog/useCatalogItem", () => ({
  useCatalogItem: () => ({
    status: "ready",
    item: album,
    message: "",
    retry: vi.fn(),
  }),
}));

vi.mock("../features/catalog/useCatalogPage", () => ({
  useCatalogPage: () => ({
    status: "ready",
    page: {
      items: [track],
      pagination: { limit: 100, offset: 0, total: 1 },
    },
    message: "",
    retry: vi.fn(),
  }),
}));

vi.mock("../features/player/PlayerContext", () => ({
  usePlayer: () => player,
}));

describe("AlbumPage", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("renders release metadata and starts its playback queue", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter initialEntries={["/albums/album-id"]}>
        <Routes>
          <Route path="/albums/:albumId" element={<AlbumPage />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(
      screen.getByRole("heading", { name: "Night City" }),
    ).toBeInTheDocument();
    expect(screen.getByText("After Midnight")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Play release" }));

    expect(player.playQueue).toHaveBeenCalledWith([track], 0, {
      type: "album",
      id: "album-id",
    });
  });
});
