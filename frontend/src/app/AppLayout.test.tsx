import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AppLayout, pageTitle } from "./AppLayout";

vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({
    status: "anonymous",
    identity: null,
    logout: vi.fn(),
  }),
}));

vi.mock("../features/player/PlayerControls", () => ({
  PlayerControls: () => <div data-testid="player" />,
}));

vi.mock("../features/theme/ThemeToggle", () => ({
  ThemeToggle: () => <button type="button">Theme</button>,
}));

describe("AppLayout accessibility", () => {
  afterEach(() => {
    cleanup();
    document.title = "Infinitune";
  });

  it("moves focus to the main content and updates the document title", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter initialEntries={["/search"]}>
        <Routes>
          <Route path="/" element={<AppLayout />}>
            <Route path="search" element={<h1>Search content</h1>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => expect(document.title).toBe("Search · Infinitune"));
    await user.click(screen.getByRole("button", { name: "Skip to content" }));

    await waitFor(() => expect(screen.getByRole("main")).toHaveFocus());
  });

  it("reports offline recovery without hiding the current page", async () => {
    render(
      <MemoryRouter initialEntries={["/search"]}>
        <Routes>
          <Route path="/" element={<AppLayout />}>
            <Route path="search" element={<h1>Search content</h1>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    act(() => window.dispatchEvent(new Event("offline")));
    expect(screen.getByRole("status")).toHaveTextContent("You are offline");
    expect(
      screen.getByRole("heading", { name: "Search content" }),
    ).toBeVisible();
    act(() => window.dispatchEvent(new Event("online")));
    expect(screen.queryByText(/You are offline/)).not.toBeInTheDocument();
  });

  it("keeps sign-in accessible from the topbar on narrow layouts", () => {
    render(
      <MemoryRouter initialEntries={["/search"]}>
        <Routes>
          <Route path="/" element={<AppLayout />}>
            <Route path="search" element={<h1>Search content</h1>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
    expect(
      screen.getByRole("banner").getElementsByClassName("sign-in-link"),
    ).toHaveLength(1);
    expect(
      screen.getByRole("banner").querySelector("a[href='/login']"),
    ).toHaveTextContent("Sign in");
  });

  it("maps collection and detail routes to concise titles", () => {
    expect(pageTitle("/albums")).toBe("Albums");
    expect(pageTitle("/albums/album-id")).toBe("Album");
    expect(pageTitle("/library/playlist-id")).toBe("Playlist");
    expect(pageTitle("/history")).toBe("Listening history");
    expect(pageTitle("/creator/releases")).toBe("Creator releases");
    expect(pageTitle("/missing")).toBe("Infinitune");
  });
});
