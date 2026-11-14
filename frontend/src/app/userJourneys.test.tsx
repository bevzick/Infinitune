import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { FeedbackProvider } from "../features/feedback/FeedbackContext";
import { InteractionProvider } from "../features/interactions/InteractionContext";
import { LibraryPage } from "../pages/LibraryPage";
import { SearchPage } from "../pages/SearchPage";

const player = vi.hoisted(() => ({
  currentTrack: null,
  status: "idle",
  toggle: vi.fn(),
  playQueue: vi.fn(),
}));

vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({
    status: "authenticated",
    accessToken: "access-token",
    identity: { userId: "listener-id" },
  }),
}));

vi.mock("../features/player/PlayerContext", () => ({
  usePlayer: () => player,
}));

const track = {
  id: "track-id",
  playback_url: "/playback",
  title: "Night Drive",
  duration_ms: 228000,
  track_number: 1,
  disc_number: 1,
  explicit: false,
  release_date: "2026-11-10",
  artists: [
    {
      artist: { id: "artist-id", name: "Luna Veil", image_url: "" },
      role: "primary",
      position: 1,
    },
  ],
  genres: [],
  moods: [],
  created_at: "2026-11-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

describe("critical listener journeys", () => {
  let liked = false;
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    liked = false;
    fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
      const url = new URL(String(input));
      const method = init?.method ?? "GET";

      if (url.pathname === "/v1/interactions/tracks/likes") {
        return jsonResponse({
          items: liked
            ? [{ id: track.id, liked_at: "2026-11-12T12:00:00Z" }]
            : [],
          page: page(liked ? 1 : 0),
        });
      }
      if (url.pathname === "/v1/interactions/albums/likes") {
        return jsonResponse({ items: [], page: page(0) });
      }
      if (url.pathname === "/v1/interactions/artists/follows") {
        return jsonResponse({ items: [], page: page(0) });
      }
      if (url.pathname === `/v1/interactions/tracks/${track.id}/like`) {
        liked = method === "PUT";
        return method === "DELETE"
          ? Promise.resolve(new Response(null, { status: 204 }))
          : jsonResponse({ id: track.id, liked_at: "2026-11-12T12:00:00Z" });
      }
      if (url.pathname === "/v1/playlists") {
        return jsonResponse({ items: [], page: page(0) });
      }
      if (url.pathname === `/v1/catalog/tracks/${track.id}`) {
        return jsonResponse(track);
      }
      if (url.pathname === "/v1/catalog/tracks") {
        const matches = url.searchParams.get("title") === "night";
        return jsonResponse({
          items: matches ? [track] : [],
          pagination: { limit: 10, offset: 0, total: matches ? 1 : 0 },
        });
      }
      if (
        ["artists", "albums", "genres", "moods"].some(
          (resource) => url.pathname === `/v1/catalog/${resource}`,
        )
      ) {
        return jsonResponse({
          items: [],
          pagination: { limit: 100, offset: 0, total: 0 },
        });
      }
      throw new Error(`Unexpected request: ${method} ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("searches, starts playback, and likes the result", async () => {
    const user = userEvent.setup();
    renderJourney(
      <MemoryRouter initialEntries={["/search"]}>
        <SearchPage />
      </MemoryRouter>,
    );

    await user.type(
      screen.getByRole("searchbox", { name: "Track title" }),
      "night",
    );
    await user.click(screen.getByRole("button", { name: "Search" }));
    expect(await screen.findAllByText("Night Drive")).not.toHaveLength(0);

    await user.click(screen.getByRole("button", { name: "Play Night Drive" }));
    expect(player.playQueue).toHaveBeenCalledWith(
      [expect.objectContaining({ id: track.id })],
      0,
      { type: "search" },
    );

    const likeButton = screen.getByRole("button", { name: "Like Night Drive" });
    await waitFor(() => expect(likeButton).toBeEnabled());
    await user.click(likeButton);

    expect(await screen.findByRole("status")).toHaveTextContent(
      "Liked Night Drive.",
    );
    expect(liked).toBe(true);
    expect(authenticatedCall("PUT")).toBe(true);
  });

  it("unlikes a library track and refreshes the collection", async () => {
    liked = true;
    const user = userEvent.setup();
    renderJourney(
      <MemoryRouter initialEntries={["/library?view=tracks"]}>
        <LibraryPage />
      </MemoryRouter>,
    );

    expect(await screen.findByText("Night Drive")).toBeInTheDocument();
    const unlikeButton = screen.getByRole("button", {
      name: "Unlike Night Drive",
    });
    await waitFor(() => expect(unlikeButton).toBeEnabled());
    await user.click(unlikeButton);

    expect(await screen.findByText("No liked tracks yet.")).toBeInTheDocument();
    expect(liked).toBe(false);
    expect(authenticatedCall("DELETE")).toBe(true);
  });

  function authenticatedCall(method: string) {
    return fetchMock.mock.calls.some(([, init]) => {
      if (init?.method !== method) return false;
      return (
        new Headers(init.headers).get("Authorization") === "Bearer access-token"
      );
    });
  }
});

function renderJourney(children: React.ReactNode) {
  render(
    <FeedbackProvider>
      <InteractionProvider>{children}</InteractionProvider>
    </FeedbackProvider>,
  );
}

function page(total: number) {
  return { limit: 100, offset: 0, total };
}

function jsonResponse(body: unknown) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
}
