import type { Track } from "../catalog/catalogApi";

// Playback owns a snapshot of the visible order. Later recommendation refreshes
// may replace the page data, but they must not reorder an active queue.
export function snapshotQueue(tracks: readonly Track[]): Track[] {
  return tracks.map((track) => ({
    ...track,
    artists: [...track.artists],
    genres: [...track.genres],
    moods: [...track.moods],
  }));
}
