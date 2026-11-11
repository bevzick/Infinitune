import { useEffect, useMemo, useRef, useState } from "react";

import {
  fetchTrackLyrics,
  type LyricsLine,
  type Track,
  type TrackLyrics,
} from "../catalog/catalogApi";

type LyricsState =
  | { status: "loading"; items: TrackLyrics[]; error: "" }
  | { status: "ready"; items: TrackLyrics[]; error: "" }
  | { status: "error"; items: TrackLyrics[]; error: string };

export function LyricsPanel({
  track,
  currentTime,
  onSeek,
  onClose,
}: {
  track: Track;
  currentTime: number;
  onSeek(seconds: number): void;
  onClose(): void;
}) {
  const [state, setState] = useState<LyricsState>({
    status: "loading",
    items: [],
    error: "",
  });
  const [selectedID, setSelectedID] = useState("");
  const lineElements = useRef(new Map<number, HTMLButtonElement>());

  useEffect(() => {
    const controller = new AbortController();
    setState({ status: "loading", items: [], error: "" });
    setSelectedID("");
    fetchTrackLyrics(track.id, controller.signal)
      .then(({ items }) => {
        if (controller.signal.aborted) return;
        setState({ status: "ready", items, error: "" });
        setSelectedID(items[0]?.id ?? "");
      })
      .catch((reason: unknown) => {
        if (controller.signal.aborted) return;
        setState({
          status: "error",
          items: [],
          error:
            reason instanceof Error
              ? reason.message
              : "Lyrics are temporarily unavailable.",
        });
      });
    return () => controller.abort();
  }, [track.id]);

  const selected =
    state.items.find(({ id }) => id === selectedID) ?? state.items[0];
  const activeLine = useMemo(
    () =>
      selected?.lyrics_type === "synchronized"
        ? activeLyricsLine(selected.lines, currentTime * 1000)
        : null,
    [currentTime, selected],
  );

  useEffect(() => {
    if (activeLine === null) return;
    lineElements.current.get(activeLine)?.scrollIntoView?.({
      block: "nearest",
      behavior: "smooth",
    });
  }, [activeLine]);

  return (
    <aside id="player-lyrics" className="lyrics-panel" aria-label="Lyrics">
      <header className="lyrics-header">
        <div>
          <span className="hero-kicker">Now singing</span>
          <h2>{track.title}</h2>
        </div>
        <button type="button" onClick={onClose} aria-label="Close lyrics">
          ×
        </button>
      </header>

      {state.status === "loading" ? (
        <p className="lyrics-state" aria-live="polite">
          Loading lyrics…
        </p>
      ) : null}
      {state.status === "error" ? (
        <p className="lyrics-state lyrics-error" role="alert">
          {state.error}
        </p>
      ) : null}
      {state.status === "ready" && !selected ? (
        <p className="lyrics-state">Lyrics are not available for this track.</p>
      ) : null}

      {selected ? (
        <>
          <div className="lyrics-toolbar">
            <label>
              <span>Language</span>
              <select
                value={selected.id}
                onChange={(event) => setSelectedID(event.currentTarget.value)}
              >
                {state.items.map((lyrics) => (
                  <option value={lyrics.id} key={lyrics.id}>
                    {languageName(lyrics.language)}
                  </option>
                ))}
              </select>
            </label>
            <span>
              {selected.official ? "Official lyrics" : "Licensed lyrics"}
            </span>
          </div>

          <div className="lyrics-content" aria-live="polite">
            {selected.lyrics_type === "synchronized" ? (
              <div className="lyrics-lines">
                {selected.lines.map((line) => (
                  <button
                    ref={(element) => {
                      if (element)
                        lineElements.current.set(line.line_number, element);
                      else lineElements.current.delete(line.line_number);
                    }}
                    className={
                      activeLine === line.line_number
                        ? "lyrics-line-active"
                        : ""
                    }
                    type="button"
                    key={line.line_number}
                    onClick={() => onSeek(line.start_ms / 1000)}
                    aria-current={
                      activeLine === line.line_number ? "true" : undefined
                    }
                  >
                    {line.text}
                  </button>
                ))}
              </div>
            ) : (
              <p className="lyrics-plain">{selected.content}</p>
            )}
          </div>
          <footer className="lyrics-attribution">
            <span>Source: {selected.source_name}</span>
            {selected.copyright_notice ? (
              <small>{selected.copyright_notice}</small>
            ) : null}
          </footer>
        </>
      ) : null}
    </aside>
  );
}

export function activeLyricsLine(lines: LyricsLine[], positionMS: number) {
  const line = lines.find(
    ({ start_ms, end_ms }) => positionMS >= start_ms && positionMS < end_ms,
  );
  return line?.line_number ?? null;
}

function languageName(language: string) {
  try {
    return (
      new Intl.DisplayNames(undefined, { type: "language" }).of(language) ??
      language
    );
  } catch {
    return language.toUpperCase();
  }
}
