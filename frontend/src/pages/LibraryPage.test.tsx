import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { LibraryPage } from "./LibraryPage";

const playlistApi = vi.hoisted(() => ({
  listPlaylists: vi.fn(),
  createPlaylist: vi.fn(),
}));

vi.mock("../features/playlists/playlistApi", async (importOriginal) => ({
  ...(await importOriginal()),
  ...playlistApi,
}));

vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({ accessToken: "access-token" }),
}));

describe("LibraryPage", () => {
  beforeEach(() => {
    playlistApi.listPlaylists.mockResolvedValue({
      items: [
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
      page: { limit: 12, offset: 0, total: 1 },
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
    expect(playlistApi.listPlaylists).toHaveBeenCalledWith(
      "access-token",
      12,
      0,
    );
    await user.click(screen.getByRole("button", { name: "+ New playlist" }));
    expect(
      screen.getByRole("heading", { name: "Create a new playlist" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
  });
});
