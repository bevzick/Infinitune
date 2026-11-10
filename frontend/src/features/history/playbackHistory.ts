import type { ListeningEventInput, PlaybackContext } from "./historyApi";

type ActivePlayback = {
  eventID: string;
  trackID: string;
  durationMS: number;
  context: PlaybackContext;
  startedAt: Date;
  segmentStartedAt: Date | null;
  listenedMS: number;
};

export class PlaybackHistoryTracker {
  private active: ActivePlayback | null = null;

  constructor(
    private readonly newEventID: () => string = () => crypto.randomUUID(),
  ) {}

  resume(
    trackID: string,
    durationMS: number,
    context: PlaybackContext,
    now = new Date(),
  ) {
    if (!this.active) {
      this.active = {
        eventID: this.newEventID(),
        trackID,
        durationMS,
        context,
        startedAt: now,
        segmentStartedAt: now,
        listenedMS: 0,
      };
      return;
    }
    if (this.active.trackID !== trackID) {
      throw new Error(
        "Finish the active playback before starting another track",
      );
    }
    if (!this.active.segmentStartedAt) this.active.segmentStartedAt = now;
  }

  suspend(now = new Date()) {
    this.accumulate(now);
  }

  finish(now = new Date()): ListeningEventInput | null {
    if (!this.active) return null;
    this.accumulate(now);
    const playback = this.active;
    this.active = null;
    return {
      event_id: playback.eventID,
      track_id: playback.trackID,
      started_at: playback.startedAt.toISOString(),
      finished_at: now.toISOString(),
      listened_ms: Math.min(
        playback.durationMS,
        Math.max(0, Math.round(playback.listenedMS)),
      ),
      context_type: playback.context.type,
      ...(requiresContextID(playback.context)
        ? { context_id: playback.context.id }
        : {}),
      device_type: "web",
    };
  }

  private accumulate(now: Date) {
    if (!this.active?.segmentStartedAt) return;
    this.active.listenedMS += Math.max(
      0,
      now.getTime() - this.active.segmentStartedAt.getTime(),
    );
    this.active.segmentStartedAt = null;
  }
}

type EventSender = (
  accessToken: string,
  event: ListeningEventInput,
  keepalive?: boolean,
) => Promise<unknown>;

export class ListeningHistoryReporter {
  private readonly pending = new Map<string, ListeningEventInput>();
  private readonly inFlight = new Set<string>();

  constructor(private readonly send: EventSender) {}

  enqueue(event: ListeningEventInput) {
    this.pending.set(event.event_id, event);
  }

  flush(accessToken: string, keepalive = false) {
    return Promise.all(
      [...this.pending.values()].map((event) =>
        this.sendOne(accessToken, event, keepalive),
      ),
    );
  }

  private async sendOne(
    accessToken: string,
    event: ListeningEventInput,
    keepalive: boolean,
  ) {
    if (this.inFlight.has(event.event_id)) return;
    this.inFlight.add(event.event_id);
    try {
      await this.send(accessToken, event, keepalive);
      this.pending.delete(event.event_id);
    } catch {
      // Retain the immutable payload; retrying the same event ID is idempotent.
    } finally {
      this.inFlight.delete(event.event_id);
    }
  }
}

function requiresContextID(
  context: PlaybackContext,
): context is Extract<PlaybackContext, { id: string }> {
  return "id" in context;
}
