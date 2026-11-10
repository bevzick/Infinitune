import { describe, expect, it, vi } from "vitest";

import {
  ListeningHistoryReporter,
  PlaybackHistoryTracker,
} from "./playbackHistory";

describe("PlaybackHistoryTracker", () => {
  it("counts playing intervals without counting pauses or seeks", () => {
    const tracker = new PlaybackHistoryTracker(() => "event-id");
    tracker.resume(
      "track-id",
      180_000,
      { type: "playlist", id: "playlist-id" },
      at(0),
    );
    tracker.suspend(at(10_000));
    tracker.resume(
      "track-id",
      180_000,
      { type: "playlist", id: "playlist-id" },
      at(40_000),
    );

    expect(tracker.finish(at(65_000))).toEqual({
      event_id: "event-id",
      track_id: "track-id",
      started_at: at(0).toISOString(),
      finished_at: at(65_000).toISOString(),
      listened_ms: 35_000,
      context_type: "playlist",
      context_id: "playlist-id",
      device_type: "web",
    });
  });

  it("uses one stable event ID and clamps listening time to track duration", () => {
    const tracker = new PlaybackHistoryTracker(() => "stable-event-id");
    tracker.resume("short-track", 10_000, { type: "direct" }, at(0));

    expect(tracker.finish(at(20_000))).toMatchObject({
      event_id: "stable-event-id",
      listened_ms: 10_000,
      context_type: "direct",
    });
    expect(tracker.finish(at(30_000))).toBeNull();
  });
});

describe("ListeningHistoryReporter", () => {
  it("retries the identical event after a network error", async () => {
    const send = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("offline"))
      .mockResolvedValueOnce({ recorded: true });
    const reporter = new ListeningHistoryReporter(send);
    const event = {
      event_id: "stable-event-id",
      track_id: "track-id",
      started_at: at(0).toISOString(),
      finished_at: at(35_000).toISOString(),
      listened_ms: 35_000,
      context_type: "direct" as const,
      device_type: "web" as const,
    };
    reporter.enqueue(event);

    await reporter.flush("first-token");
    await reporter.flush("refreshed-token");

    expect(send).toHaveBeenCalledTimes(2);
    expect(send.mock.calls[0][1]).toBe(event);
    expect(send.mock.calls[1][1]).toBe(event);
    expect(send.mock.calls[1][0]).toBe("refreshed-token");
  });
});

function at(milliseconds: number) {
  return new Date(Date.UTC(2026, 10, 10, 12, 0, 0, milliseconds));
}
