import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { SearchPage } from "./SearchPage";

const track = {
  id: "track-id",
  playback_url: "/playback",
  title: "Night City",
  duration_ms: 228000,
  track_number: 1,
  disc_number: 1,
  explicit: false,
  release_date: "2026-11-10",
  artists: [
    {
      artist: { id: "artist-id", name: "Luna Veil", image_url: "" },
      role: "primary" as const,
      position: 1,
    },
  ],
  genres: [],
  moods: [],
  created_at: "2026-11-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

vi.mock("../features/catalog/useCatalogPage", () => ({
  useCatalogPage: (path: string) => ({
    status: "ready",
    page: {
      items: path.startsWith("/v1/catalog/tracks?") ? [track] : [],
      pagination: {
        limit: 10,
        offset: 0,
        total: path.includes("tracks") ? 1 : 0,
      },
    },
    message: "",
    retry: vi.fn(),
  }),
}));

vi.mock("../features/player/PlayerContext", () => ({
  usePlayer: () => ({
    currentTrack: null,
    status: "idle",
    toggle: vi.fn(),
    playQueue: vi.fn(),
  }),
}));

describe("SearchPage", () => {
  afterEach(cleanup);

  it("shows a best match and keeps the query in the URL", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter initialEntries={["/search"]}>
        <SearchPage />
        <LocationSearch />
      </MemoryRouter>,
    );

    expect(screen.getByText("Best match")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open track" })).toHaveAttribute(
      "href",
      "/tracks/track-id",
    );

    await user.type(
      screen.getByRole("searchbox", { name: "Track title" }),
      "after",
    );
    await user.click(screen.getByRole("button", { name: "Search" }));

    expect(screen.getByTestId("location-search")).toHaveTextContent("?q=after");
  });
});

function LocationSearch() {
  return <output data-testid="location-search">{useLocation().search}</output>;
}
