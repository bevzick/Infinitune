import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { LibraryPage } from "./LibraryPage";

const libraryApi = vi.hoisted(() => ({
  loadLibrary: vi.fn(),
}));
const playlistApi = vi.hoisted(() => ({
  createPlaylist: vi.fn(),
  createDynamicPlaylist: vi.fn(),
}));

vi.mock("../features/library/libraryApi", () => libraryApi);

vi.mock("../features/playlists/playlistApi", async (importOriginal) => ({
  ...(await importOriginal()),
  ...playlistApi,
}));

vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({ accessToken: "access-token" }),
}));

describe("LibraryPage", () => {
  beforeEach(() => {
    libraryApi.loadLibrary.mockResolvedValue({
      playlists: [
        {
          id: "playlist-id",
          owner_id: "user-id",
          name: "Night signals",
          description: "Quiet electronic tracks",
          type: "manual",
          visibility: "private",
          created_at: "2026-11-10T00:00:00Z",
          updated_at: "2026-11-10T00:00:00Z",
        },
      ],
      tracks: [],
      albums: [],
      artists: [
        {
          item: {
            id: "artist-id",
            name: "Lumen Harbor",
            image_url: "",
            description: "Electronic artist",
            created_at: "2026-11-01T00:00:00Z",
            updated_at: "2026-11-01T00:00:00Z",
          },
          savedAt: "2026-11-10T00:00:00Z",
        },
      ],
    });
  });
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("renders owned playlists and opens the create form", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <LibraryPage />
      </MemoryRouter>,
    );

    expect(await screen.findByText("Night signals")).toBeInTheDocument();
    expect(libraryApi.loadLibrary).toHaveBeenCalledWith(
      "access-token",
      expect.any(AbortSignal),
    );
    await user.click(screen.getByRole("button", { name: "+ New playlist" }));
    expect(
      screen.getByRole("heading", { name: "Create a new playlist" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
  });

  it("switches collections and filters followed artists", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <LibraryPage />
      </MemoryRouter>,
    );

    await screen.findByText("Night signals");
    await user.click(screen.getByRole("button", { name: /Followed artists/ }));
    expect(await screen.findByText("Lumen Harbor")).toBeInTheDocument();

    await user.type(screen.getByRole("searchbox"), "missing");
    expect(screen.getByText("No matching items.")).toBeInTheDocument();
    expect(screen.queryByText("Lumen Harbor")).not.toBeInTheDocument();
  });

  it("switches the create flow to dynamic playlist rules", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <LibraryPage />
      </MemoryRouter>,
    );

    await screen.findByText("Night signals");
    await user.click(screen.getByRole("button", { name: "+ New playlist" }));
    await user.click(screen.getByRole("button", { name: /Dynamic/ }));

    expect(
      screen.getByRole("group", { name: "Automatic selection rules" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "+ Add condition" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create dynamic playlist" }),
    ).toBeInTheDocument();
  });
});
