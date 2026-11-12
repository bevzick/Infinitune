import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { PlaylistPage } from "./PlaylistPage";

const playlistApi = vi.hoisted(() => ({
  getPlaylist: vi.fn(),
  resolveDynamicTracks: vi.fn(),
  getCatalogTrack: vi.fn(),
}));
const player = vi.hoisted(() => ({ playQueue: vi.fn() }));

vi.mock("../features/playlists/playlistApi", async (importOriginal) => ({
  ...(await importOriginal()),
  ...playlistApi,
}));

vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({
    accessToken: "access-token",
    identity: { userId: "listener-id" },
  }),
}));

vi.mock("../features/player/PlayerContext", () => ({
  usePlayer: () => player,
}));

vi.mock("../features/catalog/CatalogViews", () => ({
  CatalogEmpty: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  CatalogLoading: () => <div>Loading</div>,
  TrackList: ({ tracks }: { tracks: Array<{ id: string }> }) => (
    <div>Queue order: {tracks.map((track) => track.id).join(", ")}</div>
  ),
}));

describe("PlaylistPage personalization", () => {
  beforeEach(() => {
    playlistApi.getPlaylist.mockResolvedValue({
      id: "playlist-id",
      owner_id: "owner-id",
      name: "Night sequence",
      description: "Dynamic listening",
      type: "dynamic",
      visibility: "public",
      created_at: "2026-11-11T00:00:00Z",
      updated_at: "2026-11-11T00:00:00Z",
    });
    playlistApi.resolveDynamicTracks.mockResolvedValue({
      items: [
        { track_id: "preferred-track", position: 1 },
        { track_id: "other-track", position: 2 },
      ],
      page: { limit: 100, offset: 0, total: 2 },
      resolution: { status: "ready" },
      personalized: true,
    });
    playlistApi.getCatalogTrack.mockImplementation((trackID: string) =>
      Promise.resolve(track(trackID)),
    );
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("explains personalized order and preserves the API sequence", async () => {
    renderPage();

    expect(await screen.findByText("Personalized order")).toBeInTheDocument();
    expect(
      screen.getByText("Queue order: preferred-track, other-track"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/captures this order for the active queue/),
    ).toBeInTheDocument();
  });

  it("labels a privacy or cold-start fallback as neutral", async () => {
    playlistApi.resolveDynamicTracks.mockResolvedValueOnce({
      items: [{ track_id: "other-track", position: 1 }],
      page: { limit: 100, offset: 0, total: 1 },
      resolution: { status: "ready" },
      personalized: false,
    });

    renderPage();

    expect(
      await screen.findByText("Neutral playlist order"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Personalization is off or there are not enough/),
    ).toBeInTheDocument();
  });

  it("starts playback with the resolved playlist order", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole("button", { name: "Play" }));

    expect(player.playQueue).toHaveBeenCalledWith(
      [
        expect.objectContaining({ id: "preferred-track" }),
        expect.objectContaining({ id: "other-track" }),
      ],
      0,
      { type: "playlist", id: "playlist-id" },
    );
  });
});

function renderPage() {
  render(
    <MemoryRouter initialEntries={["/library/playlist-id"]}>
      <Routes>
        <Route path="/library/:playlistId" element={<PlaylistPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

function track(id: string) {
  return {
    id,
    title: id,
    playback_url: `/tracks/${id}`,
    duration_ms: 180_000,
    track_number: 1,
    disc_number: 1,
    explicit: false,
    release_date: "2026-11-11",
    artists: [],
    genres: [],
    moods: [],
    created_at: "2026-11-11T00:00:00Z",
    updated_at: "2026-11-11T00:00:00Z",
  };
}
