import { describe, expect, it } from "vitest";

import type { Track } from "../catalog/catalogApi";
import { snapshotQueue } from "./queue";

describe("playback queue snapshot", () => {
  it("keeps the captured order and track data after page results change", () => {
    const first = track("first", "First");
    const second = track("second", "Second");
    const source = [first, second];

    const queue = snapshotQueue(source);
    source.reverse();
    first.title = "Changed after refresh";

    expect(queue.map((item) => item.id)).toEqual(["first", "second"]);
    expect(queue[0]?.title).toBe("First");
  });
});

function track(id: string, title: string): Track {
  return {
    id,
    title,
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
