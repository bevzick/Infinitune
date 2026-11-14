import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DiscoverPage } from "./DiscoverPage";

const recommendationApi = vi.hoisted(() => ({ getRecommendedTracks: vi.fn() }));

vi.mock(
  "../features/recommendations/recommendationApi",
  () => recommendationApi,
);
vi.mock("../features/catalog/useCatalogPage", () => ({
  useCatalogPage: () => ({
    status: "ready",
    message: "",
    retry: vi.fn(),
    page: { items: [], pagination: { limit: 6, offset: 0, total: 0 } },
  }),
}));
vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({ accessToken: "access-token" }),
}));
vi.mock("../features/player/PlayerContext", () => ({
  usePlayer: () => ({
    currentTrack: null,
    status: "idle",
    toggle: vi.fn(),
    playQueue: vi.fn(),
  }),
}));

describe("DiscoverPage personalization", () => {
  beforeEach(() => {
    recommendationApi.getRecommendedTracks.mockResolvedValue([
      {
        track_id: "track-id",
        score: 2,
        rank: 1,
        reason: "Preferred genre",
        recommendationId: "run-id",
        track: {
          id: "track-id",
          playback_url: "/playback",
          title: "Midnight Signal",
          duration_ms: 180000,
          track_number: 1,
          disc_number: 1,
          explicit: false,
          release_date: "2026-11-11",
          artists: [],
          genres: [],
          moods: [],
          created_at: "2026-11-11T00:00:00Z",
          updated_at: "2026-11-11T00:00:00Z",
        },
      },
    ]);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows ranked recommendations for an authenticated listener", async () => {
    render(
      <MemoryRouter>
        <DiscoverPage />
      </MemoryRouter>,
    );

    expect(await screen.findByText("Made for you")).toBeInTheDocument();
    expect(screen.getByText("Midnight Signal")).toBeInTheDocument();
    expect(screen.getByText("Preferred genre")).toBeInTheDocument();
    expect(recommendationApi.getRecommendedTracks).toHaveBeenCalledWith(
      "access-token",
      6,
      expect.any(AbortSignal),
    );
  });
});
