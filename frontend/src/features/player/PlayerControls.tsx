import { usePlayer } from "./PlayerContext";

export function PlayerControls() {
  const player = usePlayer();
  const track = player.currentTrack;
  const disabled = !track || player.status === "loading";

  return (
    <section className="player-shell" aria-label="Audio player">
      <div className="player-art" aria-hidden="true">
        {track ? track.title.slice(0, 1).toUpperCase() : "∞"}
      </div>
      <div className="player-copy">
        <strong>{track?.title ?? "Nothing playing yet"}</strong>
        <span>
          {player.error ||
            (track ? artistNames(track) : "Choose a track to begin")}
        </span>
      </div>
      <div className="player-transport">
        <button
          type="button"
          onClick={() => void player.previous()}
          disabled={!player.hasPrevious && player.currentTime <= 3}
          aria-label="Previous track"
        >
          ‹
        </button>
        <button
          className="play-button"
          type="button"
          onClick={() => void player.toggle()}
          disabled={disabled}
          aria-label={player.status === "playing" ? "Pause" : "Play"}
        >
          {player.status === "loading"
            ? "…"
            : player.status === "playing"
              ? "Ⅱ"
              : "▶"}
        </button>
        <button
          type="button"
          onClick={() => void player.next()}
          disabled={!player.hasNext}
          aria-label="Next track"
        >
          ›
        </button>
      </div>
      <label className="player-progress">
        <span className="visually-hidden">Playback position</span>
        <input
          aria-label="Playback position"
          type="range"
          min="0"
          max={Math.max(player.duration, 0)}
          step="0.1"
          value={Math.min(player.currentTime, player.duration || 0)}
          onChange={(event) => player.seek(Number(event.currentTarget.value))}
          disabled={!track || !player.duration}
          style={
            {
              "--player-progress": `${progress(player.currentTime, player.duration)}%`,
            } as React.CSSProperties
          }
        />
      </label>
      <span className="player-time">
        {formatTime(player.currentTime)} / {formatTime(player.duration)}
      </span>
      <label className="player-volume">
        <span aria-hidden="true">◖</span>
        <span className="visually-hidden">Volume</span>
        <input
          aria-label="Volume"
          type="range"
          min="0"
          max="1"
          step="0.05"
          value={player.volume}
          onChange={(event) =>
            player.changeVolume(Number(event.currentTarget.value))
          }
          style={
            {
              "--player-progress": `${player.volume * 100}%`,
            } as React.CSSProperties
          }
        />
      </label>
    </section>
  );
}

function artistNames(
  track: NonNullable<ReturnType<typeof usePlayer>["currentTrack"]>,
) {
  const primary = track.artists.filter(({ role }) => role === "primary");
  return (primary.length ? primary : track.artists)
    .map(({ artist }) => artist.name)
    .join(", ");
}

function formatTime(seconds: number) {
  const safe = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0;
  return `${Math.floor(safe / 60)}:${String(safe % 60).padStart(2, "0")}`;
}

function progress(current: number, duration: number) {
  return duration > 0 ? Math.min(100, (current / duration) * 100) : 0;
}
