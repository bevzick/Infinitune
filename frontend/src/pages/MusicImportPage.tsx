import {
  type FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import {
  discoverSourceTracks,
  importSourceTrack,
  listDraftReleaseTracks,
  listMusicSourceProviders,
  type MusicSourceProvider,
  type ReleaseTrackOption,
  type SourceTrack,
} from "../features/imports/musicSourceApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";
import { Icon } from "../shared/ui/Icon";

const providerLabels: Record<MusicSourceProvider, string> = {
  jamendo: "Jamendo",
  internet_archive: "Internet Archive",
};

export function MusicImportPage() {
  const { accessToken } = useSession();
  const [providers, setProviders] = useState<MusicSourceProvider[]>([]);
  const [releaseTracks, setReleaseTracks] = useState<ReleaseTrackOption[]>([]);
  const [provider, setProvider] = useState<MusicSourceProvider | "">("");
  const [contentTrackId, setContentTrackId] = useState("");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SourceTrack[]>([]);
  const [status, setStatus] = useState<
    "loading" | "ready" | "searching" | "error"
  >("loading");
  const [error, setError] = useState("");
  const [importing, setImporting] = useState("");
  const [imported, setImported] = useState<SourceTrack | null>(null);
  const [hasSearched, setHasSearched] = useState(false);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!accessToken) return;
      setStatus("loading");
      setError("");
      try {
        const [availableProviders, tracks] = await Promise.all([
          listMusicSourceProviders(accessToken, signal),
          listDraftReleaseTracks(accessToken, signal),
        ]);
        setProviders(availableProviders);
        setReleaseTracks(tracks);
        setProvider((current) => current || availableProviders[0] || "");
        setContentTrackId((current) => current || tracks[0]?.id || "");
        setStatus("ready");
      } catch (reason) {
        if (signal?.aborted) return;
        setError(errorMessage(reason, "Could not prepare music import"));
        setStatus("error");
      }
    },
    [accessToken],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const selectedReleaseTrack = useMemo(
    () => releaseTracks.find(({ id }) => id === contentTrackId),
    [contentTrackId, releaseTracks],
  );

  async function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken || !provider || !query.trim()) return;
    setStatus("searching");
    setError("");
    setImported(null);
    setHasSearched(true);
    try {
      const page = await discoverSourceTracks(
        accessToken,
        provider,
        query.trim(),
      );
      setResults(page.items);
      setStatus("ready");
    } catch (reason) {
      setResults([]);
      setHasSearched(false);
      setError(errorMessage(reason, "Could not search this music source"));
      setStatus("ready");
    }
  }

  async function startImport(track: SourceTrack) {
    if (!accessToken || !contentTrackId) return;
    setImporting(track.external_id);
    setImported(null);
    setError("");
    try {
      await importSourceTrack(accessToken, contentTrackId, track);
      setImported(track);
    } catch (reason) {
      setError(errorMessage(reason, "Could not import this track"));
    } finally {
      setImporting("");
    }
  }

  if (status === "loading") {
    return <ImportState label="Loading creator tracks and music sources…" />;
  }
  if (status === "error") {
    return (
      <ImportState label={error} error>
        <button type="button" onClick={() => void load()}>
          Try again
        </button>
      </ImportState>
    );
  }

  return (
    <section className="music-import-page">
      <header className="music-import-hero">
        <div>
          <span className="hero-kicker">Creator studio</span>
          <h1>Import music from a licensed source.</h1>
          <p>
            Find provider audio, inspect its license, and attach it to a track
            in one of your draft releases.
          </p>
        </div>
        <span className="music-import-mark" aria-hidden="true">
          ↙
        </span>
      </header>

      {!providers.length ? (
        <ImportNotice title="No music sources are configured">
          Configure Jamendo or Internet Archive in Media service before using
          external import.
        </ImportNotice>
      ) : null}
      {!releaseTracks.length ? (
        <ImportNotice title="Create a draft release first">
          External audio must be attached to an existing track in a draft
          release.
        </ImportNotice>
      ) : null}

      {providers.length && releaseTracks.length ? (
        <>
          <form className="music-import-form" onSubmit={search}>
            <div className="music-import-step">
              <span>1</span>
              <div>
                <strong>Destination track</strong>
                <small>
                  The imported media will be processed for this track.
                </small>
              </div>
            </div>
            <label>
              Draft release track
              <select
                value={contentTrackId}
                onChange={(event) => {
                  setContentTrackId(event.currentTarget.value);
                  setImported(null);
                }}
              >
                {releaseTracks.map((track) => (
                  <option value={track.id} key={track.id}>
                    {track.releaseTitle} — {track.title}
                  </option>
                ))}
              </select>
            </label>

            <div className="music-import-step">
              <span>2</span>
              <div>
                <strong>Music source</strong>
                <small>Only sources configured by this server are shown.</small>
              </div>
            </div>
            <fieldset className="music-source-tabs">
              <legend className="visually-hidden">Music source</legend>
              {providers.map((item) => (
                <label key={item}>
                  <input
                    type="radio"
                    name="music-source"
                    value={item}
                    checked={provider === item}
                    onChange={() => {
                      setProvider(item);
                      setResults([]);
                      setImported(null);
                      setHasSearched(false);
                    }}
                  />
                  <span>{providerLabels[item]}</span>
                </label>
              ))}
            </fieldset>
            <label>
              {provider === "internet_archive"
                ? "Internet Archive item identifier"
                : "Track name"}
              <span className="music-import-search">
                <Icon name="search" size={18} />
                <input
                  type="search"
                  value={query}
                  maxLength={200}
                  placeholder={
                    provider === "internet_archive"
                      ? "e.g. live-concert-2026"
                      : "Search Jamendo tracks…"
                  }
                  onChange={(event) => setQuery(event.currentTarget.value)}
                  required
                />
                <button type="submit" disabled={status === "searching"}>
                  {status === "searching" ? "Searching…" : "Search source"}
                </button>
              </span>
            </label>
          </form>

          {error ? (
            <p className="music-import-error" role="alert">
              {error}
            </p>
          ) : null}
          {imported && selectedReleaseTrack ? (
            <p className="music-import-success" role="status">
              <strong>{imported.title}</strong> is being imported for “
              {selectedReleaseTrack.title}”. Media processing continues in the
              background.
            </p>
          ) : null}

          <SourceResults
            tracks={results}
            searched={hasSearched && status !== "searching"}
            importing={importing}
            disabled={!contentTrackId}
            onImport={startImport}
          />
        </>
      ) : null}
    </section>
  );
}

function SourceResults({
  tracks,
  searched,
  importing,
  disabled,
  onImport,
}: {
  tracks: SourceTrack[];
  searched: boolean;
  importing: string;
  disabled: boolean;
  onImport(track: SourceTrack): void;
}) {
  if (!tracks.length) {
    return searched ? (
      <ImportNotice title="No importable tracks found">
        Try another query or verify the Internet Archive item identifier.
      </ImportNotice>
    ) : null;
  }
  return (
    <section
      className="music-source-results"
      aria-labelledby="source-results-title"
    >
      <div className="music-source-results-heading">
        <div>
          <span className="eyebrow">Licensed audio</span>
          <h2 id="source-results-title">Search results</h2>
        </div>
        <span>{tracks.length} available</span>
      </div>
      <div className="music-source-list">
        {tracks.map((track) => (
          <article
            className="music-source-card"
            key={`${track.provider}:${track.external_id}`}
          >
            <span className="music-source-art" aria-hidden="true">
              ◎
            </span>
            <div className="music-source-copy">
              <strong>{track.title}</strong>
              <span>{track.artist}</span>
              <small>
                {track.license.name} · Attribution:{" "}
                {track.license.attribution || track.artist}
              </small>
              <a href={track.page_url} target="_blank" rel="noreferrer">
                View source details
              </a>
            </div>
            <button
              type="button"
              disabled={disabled || Boolean(importing)}
              onClick={() => onImport(track)}
            >
              {importing === track.external_id ? "Importing…" : "Import track"}
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}

function ImportState({
  label,
  error = false,
  children,
}: {
  label: string;
  error?: boolean;
  children?: React.ReactNode;
}) {
  return (
    <section className="music-import-state" role={error ? "alert" : "status"}>
      <span className="loading-pulse" aria-hidden="true" />
      <strong>{label}</strong>
      {children}
    </section>
  );
}

function ImportNotice({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="music-import-notice">
      <strong>{title}</strong>
      <p>{children}</p>
    </section>
  );
}

function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof ApiError ? reason.message : fallback;
}
