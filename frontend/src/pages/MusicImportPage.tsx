import {
  type FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  type ArtworkBackfillReport,
  backfillSourceArtwork,
  discoverSourceTracks,
  importSourceTrack,
  listDraftReleaseTracks,
  listMusicSourceProviders,
  type MusicSourceProvider,
  type ReleaseTrackOption,
  type SourceTrack,
  type SourceTrackPage,
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
  const [searchPage, setSearchPage] = useState<SourceTrackPage | null>(null);
  const searchController = useRef<AbortController | null>(null);
  const [status, setStatus] = useState<
    "loading" | "ready" | "searching" | "error"
  >("loading");
  const [error, setError] = useState("");
  const [importing, setImporting] = useState("");
  const [imported, setImported] = useState<SourceTrack | null>(null);
  const [hasSearched, setHasSearched] = useState(false);
  const [repairing, setRepairing] = useState(false);
  const [repairNotice, setRepairNotice] = useState("");

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

  useEffect(() => () => searchController.current?.abort(), []);

  const selectedReleaseTrack = useMemo(
    () => releaseTracks.find(({ id }) => id === contentTrackId),
    [contentTrackId, releaseTracks],
  );

  function resetSearch() {
    searchController.current?.abort();
    setSearchPage(null);
    setHasSearched(false);
    setStatus("ready");
    setError("");
  }

  async function searchTracks(offset: number) {
    if (!accessToken || !provider || !query.trim()) return;
    searchController.current?.abort();
    const controller = new AbortController();
    searchController.current = controller;
    setStatus("searching");
    setError("");
    setImported(null);
    setSearchPage(null);
    setHasSearched(true);
    try {
      const page = await discoverSourceTracks(
        accessToken,
        provider,
        query.trim(),
        offset,
        controller.signal,
      );
      if (controller.signal.aborted) return;
      setSearchPage(page);
      setStatus("ready");
    } catch (reason) {
      if (controller.signal.aborted) return;
      setHasSearched(false);
      setError(errorMessage(reason, "Could not search this music source"));
      setStatus("ready");
    }
  }

  function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void searchTracks(0);
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

  async function repairImportedArtwork() {
    if (!accessToken || repairing) return;
    setRepairing(true);
    setRepairNotice("");
    setError("");
    const total: ArtworkBackfillReport = {
      scanned: 0,
      repaired: 0,
      unchanged: 0,
      skipped: 0,
      failed: 0,
      next_offset: 0,
      has_more: false,
    };
    try {
      do {
        const page = await backfillSourceArtwork(
          accessToken,
          50,
          total.next_offset,
        );
        total.scanned += page.scanned;
        total.repaired += page.repaired;
        total.unchanged += page.unchanged;
        total.skipped += page.skipped;
        total.failed += page.failed;
        total.next_offset = page.next_offset;
        total.has_more = page.has_more;
      } while (total.has_more);
      setRepairNotice(
        `${total.repaired} updated, ${total.unchanged} already complete, ` +
          `${total.skipped} not editable, ${total.failed} failed.`,
      );
    } catch (reason) {
      setError(errorMessage(reason, "Could not repair imported artwork"));
    } finally {
      setRepairing(false);
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

      {providers.length ? (
        <section
          className="music-import-repair"
          aria-labelledby="artwork-repair-title"
        >
          <div>
            <span className="eyebrow">Existing imports</span>
            <h2 id="artwork-repair-title">Repair missing artwork</h2>
            <p>
              Restore provider covers and artist images for imported tracks in
              releases you can still edit.
            </p>
          </div>
          <button
            type="button"
            disabled={repairing}
            onClick={() => void repairImportedArtwork()}
          >
            {repairing ? "Repairing…" : "Repair artwork"}
          </button>
          {repairNotice ? <p role="status">{repairNotice}</p> : null}
        </section>
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
                      resetSearch();
                      setImported(null);
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
                  onChange={(event) => {
                    setQuery(event.currentTarget.value);
                    resetSearch();
                  }}
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
            page={searchPage}
            searched={hasSearched && status !== "searching"}
            searching={status === "searching"}
            importing={importing}
            disabled={!contentTrackId}
            onImport={startImport}
            onPageChange={(offset) => void searchTracks(offset)}
          />
        </>
      ) : null}
    </section>
  );
}

function SourceResults({
  page,
  searched,
  searching,
  importing,
  disabled,
  onImport,
  onPageChange,
}: {
  page: SourceTrackPage | null;
  searched: boolean;
  searching: boolean;
  importing: string;
  disabled: boolean;
  onImport(track: SourceTrack): void;
  onPageChange(offset: number): void;
}) {
  if (!page) return null;
  const { items: tracks, limit, offset, total } = page;
  const hasPrevious = offset > 0;
  const hasNext = offset + limit < total;
  if (!tracks.length && !hasPrevious && !hasNext) {
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
        <span>{tracks.length} importable on this page</span>
      </div>
      {!tracks.length ? (
        <p className="music-source-empty-page">
          No importable tracks on this page. Some provider results may be
          unavailable for download or have unsupported licenses.
        </p>
      ) : null}
      <div className="music-source-list">
        {tracks.map((track) => (
          <article
            className="music-source-card"
            key={`${track.provider}:${track.external_id}`}
          >
            <span className="music-source-art" aria-hidden="true">
              {track.artwork_url ? (
                <img src={track.artwork_url} alt="" loading="lazy" />
              ) : (
                "◎"
              )}
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
      {hasPrevious || hasNext ? (
        <nav
          className="music-source-pagination"
          aria-label="Search result pages"
        >
          <button
            type="button"
            disabled={!hasPrevious || searching}
            onClick={() => onPageChange(Math.max(0, offset - limit))}
          >
            Previous page
          </button>
          <span>
            Page {Math.floor(offset / limit) + 1} of {Math.ceil(total / limit)}
          </span>
          <button
            type="button"
            disabled={!hasNext || searching}
            onClick={() => onPageChange(offset + limit)}
          >
            Next page
          </button>
        </nav>
      ) : null}
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
