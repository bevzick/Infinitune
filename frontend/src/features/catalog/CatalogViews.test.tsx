import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TrackList } from "./CatalogViews";
import type { Track } from "./catalogApi";

vi.mock("../player/PlayerContext", () => ({
  usePlayer: () => ({ currentTrack: null, status: "idle" }),
}));
vi.mock("../interactions/InteractionButton", () => ({
  InteractionButton: () => null,
}));

afterEach(cleanup);

function trackWithGenres(names: string[]): Track {
  return {
    id: "track-1",
    title: "Night Signal",
    cover_url: "",
    playback_url: "",
    duration_ms: 180000,
    track_number: 1,
    disc_number: 1,
    explicit: false,
    release_date: "2026-11-19",
    artists: [],
    genres: names.map((name, index) => ({
      id: `genre-${index}`,
      name,
      slug: name.toLowerCase(),
    })),
    moods: [],
    created_at: "2026-11-19T00:00:00Z",
    updated_at: "2026-11-19T00:00:00Z",
  };
}

function renderTracks(track: Track) {
  render(
    <MemoryRouter>
      <TrackList tracks={[track]} />
    </MemoryRouter>,
  );
}

describe("TrackList genre tags", () => {
  it("shows two genre names plus the number of hidden genres", () => {
    renderTracks(trackWithGenres(["Downtempo", "Dub", "Hip Hop", "Jazz"]));

    const tags = screen.getByText("Downtempo").parentElement;
    expect(tags).toHaveTextContent("DowntempoDub+2");
    expect(tags?.querySelector('[title="Hip Hop, Jazz"]')).toHaveTextContent(
      "+2",
    );
    expect(screen.queryByText("Hip Hop")).not.toBeInTheDocument();
  });

  it("does not show an overflow indicator for two genres", () => {
    renderTracks(trackWithGenres(["Downtempo", "Dub"]));

    expect(screen.getByText("Downtempo")).toBeInTheDocument();
    expect(screen.getByText("Dub")).toBeInTheDocument();
    expect(screen.queryByText(/^\+\d+$/)).not.toBeInTheDocument();
  });
});
