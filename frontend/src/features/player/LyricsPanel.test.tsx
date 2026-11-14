import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  fetchTrackLyrics,
  type Track,
  type TrackLyrics,
} from "../catalog/catalogApi";
import { activeLyricsLine, LyricsPanel } from "./LyricsPanel";

vi.mock("../catalog/catalogApi", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../catalog/catalogApi")>()),
  fetchTrackLyrics: vi.fn(),
}));

const track: Track = {
  id: "track-id",
  playback_url: "/v1/media/assets/asset-id/playback",
  title: "First Light Engine",
  duration_ms: 180_000,
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

const synchronized: TrackLyrics = {
  id: "lyrics-uk",
  track_id: track.id,
  lyrics_type: "synchronized",
  language: "uk",
  content: "",
  source_type: "publisher",
  source_name: "Publisher",
  copyright_notice: "Licensed",
  official: true,
  updated_at: "2026-11-11T00:00:00Z",
  lines: [
    { line_number: 1, start_ms: 0, end_ms: 1500, text: "Перший рядок" },
    { line_number: 2, start_ms: 1500, end_ms: 3000, text: "Другий рядок" },
  ],
};

describe("LyricsPanel", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("highlights playback timing and seeks when a line is selected", async () => {
    vi.mocked(fetchTrackLyrics).mockResolvedValue({ items: [synchronized] });
    const seek = vi.fn();
    const user = userEvent.setup();
    render(
      <LyricsPanel
        track={track}
        currentTime={1}
        onSeek={seek}
        onClose={vi.fn()}
      />,
    );

    const first = await screen.findByRole("button", { name: "Перший рядок" });
    expect(first).toHaveAttribute("aria-current", "true");
    await user.click(screen.getByRole("button", { name: "Другий рядок" }));
    expect(seek).toHaveBeenCalledWith(1.5);
  });

  it("falls back to plain lyrics and supports language selection", async () => {
    const english: TrackLyrics = {
      ...synchronized,
      id: "lyrics-en",
      lyrics_type: "plain",
      language: "en",
      content: "English lyrics",
      lines: [],
    };
    const ukrainian = {
      ...english,
      id: "lyrics-uk",
      language: "uk",
      content: "Український текст",
    };
    vi.mocked(fetchTrackLyrics).mockResolvedValue({
      items: [english, ukrainian],
    });
    const user = userEvent.setup();
    render(
      <LyricsPanel
        track={track}
        currentTime={0}
        onSeek={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(await screen.findByText("English lyrics")).toBeInTheDocument();
    await user.selectOptions(
      screen.getByRole("combobox", { name: "Language" }),
      "lyrics-uk",
    );
    expect(screen.getByText("Український текст")).toBeInTheDocument();
  });
});

describe("activeLyricsLine", () => {
  it("matches half-open playback intervals", () => {
    expect(activeLyricsLine(synchronized.lines, 1499)).toBe(1);
    expect(activeLyricsLine(synchronized.lines, 1500)).toBe(2);
    expect(activeLyricsLine(synchronized.lines, 3000)).toBeNull();
  });
});
