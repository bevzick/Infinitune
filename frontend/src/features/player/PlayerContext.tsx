import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { ApiError, apiBlobRequest } from "../../shared/api/client";
import type { Track } from "../catalog/catalogApi";
import { useSession } from "../session/SessionContext";

export type PlayerStatus = "idle" | "loading" | "playing" | "paused" | "error";

type PlayerContextValue = {
  status: PlayerStatus;
  currentTrack: Track | null;
  currentTime: number;
  duration: number;
  volume: number;
  hasPrevious: boolean;
  hasNext: boolean;
  error: string;
  playQueue(tracks: Track[], index: number): Promise<void>;
  toggle(): Promise<void>;
  previous(): Promise<void>;
  next(): Promise<void>;
  seek(seconds: number): void;
  changeVolume(volume: number): void;
};

const PlayerContext = createContext<PlayerContextValue | null>(null);

export function PlayerProvider({ children }: PropsWithChildren) {
  const { accessToken } = useSession();
  const audio = useRef<HTMLAudioElement | null>(null);
  const request = useRef<AbortController | null>(null);
  const objectUrl = useRef<string | null>(null);
  const previousAccessToken = useRef(accessToken);
  const [queue, setQueue] = useState<Track[]>([]);
  const [currentIndex, setCurrentIndex] = useState(-1);
  const [status, setStatus] = useState<PlayerStatus>("idle");
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(1);
  const [error, setError] = useState("");
  const currentTrack = queue[currentIndex] ?? null;

  const releaseObjectUrl = useCallback(() => {
    if (!objectUrl.current) return;
    URL.revokeObjectURL(objectUrl.current);
    objectUrl.current = null;
  }, []);

  const clear = useCallback(() => {
    request.current?.abort();
    request.current = null;
    audio.current?.pause();
    if (audio.current) {
      audio.current.removeAttribute("src");
      audio.current.load();
    }
    releaseObjectUrl();
    setQueue([]);
    setCurrentIndex(-1);
    setCurrentTime(0);
    setDuration(0);
    setError("");
    setStatus("idle");
  }, [releaseObjectUrl]);

  useEffect(() => {
    if (previousAccessToken.current && !accessToken) clear();
    previousAccessToken.current = accessToken;
  }, [accessToken, clear]);

  useEffect(() => {
    return () => {
      request.current?.abort();
      releaseObjectUrl();
    };
  }, [releaseObjectUrl]);

  const playQueue = useCallback(
    async (tracks: Track[], index: number) => {
      const track = tracks[index];
      if (!track) return;
      setQueue(tracks);
      setCurrentIndex(index);
      setCurrentTime(0);
      setDuration(track.duration_ms / 1000);
      setError("");

      if (!accessToken) {
        setStatus("error");
        setError("Sign in to play this track.");
        return;
      }
      if (!audio.current) return;

      request.current?.abort();
      const controller = new AbortController();
      request.current = controller;
      audio.current.pause();
      setStatus("loading");

      try {
        const blob = await apiBlobRequest(track.playback_url, {
          accessToken,
          signal: controller.signal,
        });
        if (controller.signal.aborted || !audio.current) return;
        releaseObjectUrl();
        objectUrl.current = URL.createObjectURL(blob);
        audio.current.src = objectUrl.current;
        audio.current.load();
        try {
          await audio.current.play();
          setStatus("playing");
        } catch {
          setStatus("paused");
        }
      } catch (reason) {
        if (controller.signal.aborted) return;
        setStatus("error");
        setError(
          reason instanceof ApiError
            ? reason.message
            : "Playback is temporarily unavailable.",
        );
      }
    },
    [accessToken, releaseObjectUrl],
  );

  const toggle = useCallback(async () => {
    if (!audio.current || !currentTrack || status === "loading") return;
    if (audio.current.paused) {
      try {
        await audio.current.play();
        setStatus("playing");
        setError("");
      } catch {
        setStatus("error");
        setError("Your browser blocked playback. Try again.");
      }
    } else {
      audio.current.pause();
      setStatus("paused");
    }
  }, [currentTrack, status]);

  const next = useCallback(async () => {
    if (currentIndex >= 0 && currentIndex < queue.length - 1) {
      await playQueue(queue, currentIndex + 1);
    } else {
      setStatus("paused");
    }
  }, [currentIndex, playQueue, queue]);

  const previous = useCallback(async () => {
    if (audio.current && audio.current.currentTime > 3) {
      audio.current.currentTime = 0;
      setCurrentTime(0);
      return;
    }
    if (currentIndex > 0) await playQueue(queue, currentIndex - 1);
  }, [currentIndex, playQueue, queue]);

  const seek = useCallback((seconds: number) => {
    if (!audio.current || !Number.isFinite(seconds)) return;
    audio.current.currentTime = seconds;
    setCurrentTime(seconds);
  }, []);

  const changeVolume = useCallback((nextVolume: number) => {
    const normalized = Math.min(1, Math.max(0, nextVolume));
    if (audio.current) audio.current.volume = normalized;
    setVolume(normalized);
  }, []);

  const value = useMemo<PlayerContextValue>(
    () => ({
      status,
      currentTrack,
      currentTime,
      duration,
      volume,
      hasPrevious: currentIndex > 0,
      hasNext: currentIndex >= 0 && currentIndex < queue.length - 1,
      error,
      playQueue,
      toggle,
      previous,
      next,
      seek,
      changeVolume,
    }),
    [
      status,
      currentTrack,
      currentTime,
      duration,
      volume,
      currentIndex,
      queue.length,
      error,
      playQueue,
      toggle,
      previous,
      next,
      seek,
      changeVolume,
    ],
  );

  return (
    <PlayerContext.Provider value={value}>
      {children}
      {/* biome-ignore lint/a11y/useMediaCaption: Catalog audio is music-only and has no caption track. */}
      <audio
        ref={audio}
        preload="metadata"
        onTimeUpdate={(event) =>
          setCurrentTime(event.currentTarget.currentTime)
        }
        onDurationChange={(event) => {
          if (Number.isFinite(event.currentTarget.duration)) {
            setDuration(event.currentTarget.duration);
          }
        }}
        onPlay={() => setStatus("playing")}
        onPause={() =>
          setStatus((current) => (current === "playing" ? "paused" : current))
        }
        onEnded={() => void next()}
      />
    </PlayerContext.Provider>
  );
}

export function usePlayer() {
  const value = useContext(PlayerContext);
  if (!value) throw new Error("usePlayer must be used inside PlayerProvider");
  return value;
}
