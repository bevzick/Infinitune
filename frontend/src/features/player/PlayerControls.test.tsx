import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { PlayerControls } from "./PlayerControls";

const player = vi.hoisted(() => ({
  status: "paused" as const,
  currentTrack: {
    id: "track-id",
    playback_url: "/v1/media/assets/asset-id/playback",
    title: "First Light Engine",
    duration_ms: 180_000,
    track_number: 1,
    disc_number: 1,
    explicit: false,
    release_date: "2026-11-10",
    artists: [
      {
        artist: { id: "artist-id", name: "Lumen Harbor", image_url: "" },
        role: "primary" as const,
        position: 1,
      },
    ],
    genres: [],
    moods: [],
    created_at: "2026-11-10T00:00:00Z",
    updated_at: "2026-11-10T00:00:00Z",
  },
  currentTime: 30,
  duration: 180,
  volume: 0.8,
  hasPrevious: true,
  hasNext: true,
  error: "",
  playQueue: vi.fn(),
  toggle: vi.fn(),
  previous: vi.fn(),
  next: vi.fn(),
  seek: vi.fn(),
  changeVolume: vi.fn(),
}));

vi.mock("./PlayerContext", () => ({ usePlayer: () => player }));

describe("PlayerControls", () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(cleanup);

  it("shows the current track and exposes transport controls", async () => {
    const user = userEvent.setup();
    render(<PlayerControls />);

    expect(screen.getByText("First Light Engine")).toBeInTheDocument();
    expect(screen.getByText("Lumen Harbor")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Lyrics" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Play" }));
    await user.click(screen.getByRole("button", { name: "Next track" }));

    expect(player.toggle).toHaveBeenCalledOnce();
    expect(player.next).toHaveBeenCalledOnce();
  });

  it("forwards seek and volume changes", () => {
    render(<PlayerControls />);

    fireEvent.change(screen.getByLabelText("Playback position"), {
      target: { value: "42" },
    });
    fireEvent.change(screen.getByLabelText("Volume"), {
      target: { value: "0.35" },
    });

    expect(player.seek).toHaveBeenCalledWith(42);
    expect(player.changeVolume).toHaveBeenCalledWith(0.35);
  });
});
