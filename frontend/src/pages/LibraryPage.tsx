import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import {
  AlbumGrid,
  ArtistGrid,
  CatalogLoading,
  TrackList,
} from "../features/catalog/CatalogViews";
import type { Album, Artist, Track } from "../features/catalog/catalogApi";
import { interactionsChangedEvent } from "../features/interactions/InteractionContext";
import { type LibraryData, loadLibrary } from "../features/library/libraryApi";
import { DynamicPlaylistForm } from "../features/playlists/DynamicRuleForm";
import { PlaylistForm } from "../features/playlists/PlaylistForm";
import {
  createDynamicPlaylist,
  createPlaylist,
  type Playlist,
} from "../features/playlists/playlistApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";
import { Icon } from "../shared/ui/Icon";

const pageSize = 12;
type LibraryView = "playlists" | "tracks" | "albums" | "artists";
type LibrarySort = "recent" | "title";
type LibraryEntry =
  | {
      kind: "playlist";
      id: string;
      title: string;
      search: string;
      savedAt: string;
      item: Playlist;
    }
  | {
      kind: "track";
      id: string;
      title: string;
      search: string;
      savedAt: string;
      item: Track;
    }
  | {
      kind: "album";
      id: string;
      title: string;
      search: string;
      savedAt: string;
      item: Album;
    }
  | {
      kind: "artist";
      id: string;
      title: string;
      search: string;
      savedAt: string;
      item: Artist;
    };

const views: Array<{ value: LibraryView; label: string }> = [
  { value: "playlists", label: "Playlists" },
  { value: "tracks", label: "Liked tracks" },
  { value: "albums", label: "Liked albums" },
  { value: "artists", label: "Followed artists" },
];

export function LibraryPage() {
  const { accessToken } = useSession();
  const navigate = useNavigate();
  const [parameters, setParameters] = useSearchParams();
  const view = libraryView(parameters.get("view"));
  const query = parameters.get("q")?.trim() ?? "";
  const sort: LibrarySort =
    parameters.get("sort") === "title" ? "title" : "recent";
  const pageNumber = positivePage(parameters.get("page"));
  const [data, setData] = useState<LibraryData | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [creating, setCreating] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [createType, setCreateType] = useState<"manual" | "dynamic">("manual");

  useEffect(() => {
    if (!accessToken) return;
    const controller = new AbortController();
    setStatus("loading");
    setError("");
    if (attempt > 0) setData(null);
    loadLibrary(accessToken, controller.signal)
      .then((library) => {
        setData(library);
        setStatus("ready");
      })
      .catch((reason: unknown) => {
        if (controller.signal.aborted) return;
        setError(errorMessage(reason, "Could not load your library"));
        setStatus("error");
      });
    return () => controller.abort();
  }, [accessToken, attempt]);

  useEffect(() => {
    const refresh = () => setAttempt((value) => value + 1);
    window.addEventListener(interactionsChangedEvent, refresh);
    return () => window.removeEventListener(interactionsChangedEvent, refresh);
  }, []);

  const entries = useMemo(
    () => (data ? libraryEntries(data, view, query, sort) : []),
    [data, query, sort, view],
  );
  const currentPage = Math.min(
    pageNumber,
    Math.max(1, Math.ceil(entries.length / pageSize)),
  );
  const visibleEntries = entries.slice(
    (currentPage - 1) * pageSize,
    currentPage * pageSize,
  );

  const updateParameters = useCallback(
    (updates: Record<string, string>) => {
      const next = new URLSearchParams(parameters);
      for (const [key, value] of Object.entries(updates)) {
        if (
          !value ||
          (key === "view" && value === "playlists") ||
          (key === "sort" && value === "recent")
        )
          next.delete(key);
        else next.set(key, value);
      }
      next.delete("page");
      setParameters(next);
    },
    [parameters, setParameters],
  );

  return (
    <section className="library-page">
      <header className="library-heading">
        <div>
          <span className="hero-kicker">Everything you saved</span>
          <h1>Library</h1>
          <p>
            Playlists, likes, and followed artists in one private collection.
          </p>
        </div>
        <button
          type="button"
          onClick={() => {
            setShowCreate(true);
            updateParameters({ view: "playlists" });
          }}
        >
          <Icon name="library" size={17} />
          <span>+ New playlist</span>
        </button>
      </header>

      <nav className="library-tabs" aria-label="Library collections">
        {views.map((item) => (
          <button
            type="button"
            key={item.value}
            aria-current={view === item.value ? "page" : undefined}
            onClick={() => updateParameters({ view: item.value })}
          >
            {item.label}
            {data ? <span>{libraryCount(data, item.value)}</span> : null}
          </button>
        ))}
      </nav>

      <div className="library-toolbar">
        <label className="library-search">
          <span className="visually-hidden">Search this collection</span>
          <Icon name="search" size={18} />
          <input
            type="search"
            value={query}
            placeholder={`Search ${view}…`}
            onChange={(event) =>
              updateParameters({ q: event.currentTarget.value })
            }
          />
        </label>
        <label>
          <span>Sort</span>
          <select
            value={sort}
            onChange={(event) =>
              updateParameters({ sort: event.currentTarget.value })
            }
          >
            <option value="recent">Recently saved</option>
            <option value="title">Title A–Z</option>
          </select>
        </label>
      </div>

      {showCreate ? (
        <CreatePlaylistPanel
          accessToken={accessToken}
          type={createType}
          busy={creating}
          error={status !== "error" ? error : ""}
          onTypeChange={setCreateType}
          onBusyChange={setCreating}
          onError={setError}
          onClose={() => setShowCreate(false)}
          onCreated={(id) => navigate(`/library/${id}`)}
        />
      ) : null}

      {status === "loading" ? <LibraryLoading /> : null}
      {status === "error" ? (
        <LibraryError
          message={error}
          onRetry={() => setAttempt((value) => value + 1)}
        />
      ) : null}
      {status === "ready" && !entries.length ? (
        <LibraryEmpty
          view={view}
          filtered={Boolean(query)}
          onCreate={() => setShowCreate(true)}
        />
      ) : null}
      {status === "ready" && visibleEntries.length ? (
        <LibraryResults entries={visibleEntries} />
      ) : null}
      {status === "ready" ? (
        <LibraryPagination
          page={currentPage}
          total={entries.length}
          parameters={parameters}
        />
      ) : null}
    </section>
  );
}

function CreatePlaylistPanel({
  accessToken,
  type,
  busy,
  error,
  onTypeChange,
  onBusyChange,
  onError,
  onClose,
  onCreated,
}: {
  accessToken: string | null;
  type: "manual" | "dynamic";
  busy: boolean;
  error: string;
  onTypeChange(value: "manual" | "dynamic"): void;
  onBusyChange(value: boolean): void;
  onError(value: string): void;
  onClose(): void;
  onCreated(id: string): void;
}) {
  return (
    <section className="playlist-editor" aria-labelledby="new-playlist-title">
      <div>
        <span className="eyebrow">New collection</span>
        <h2 id="new-playlist-title">Create a new playlist</h2>
      </div>
      <fieldset className="playlist-type-picker">
        <legend>Playlist type</legend>
        {(["manual", "dynamic"] as const).map((value) => (
          <button
            type="button"
            key={value}
            aria-pressed={type === value}
            onClick={() => onTypeChange(value)}
          >
            <strong>{value === "manual" ? "Manual" : "Dynamic"}</strong>
            <span>
              {value === "manual"
                ? "You choose and order every track."
                : "Rules keep the track list up to date."}
            </span>
          </button>
        ))}
      </fieldset>
      {error ? (
        <p className="form-error" role="alert">
          {error}
        </p>
      ) : null}
      {type === "manual" ? (
        <PlaylistForm
          submitLabel="Create playlist"
          busy={busy}
          onCancel={onClose}
          onSubmit={async (input) => {
            if (!accessToken) return;
            onBusyChange(true);
            onError("");
            try {
              onCreated((await createPlaylist(accessToken, input)).id);
            } catch (reason) {
              onError(errorMessage(reason, "Could not create the playlist"));
            } finally {
              onBusyChange(false);
            }
          }}
        />
      ) : (
        <DynamicPlaylistForm
          busy={busy}
          onCancel={onClose}
          onSubmit={async (input) => {
            if (!accessToken) return;
            onBusyChange(true);
            onError("");
            try {
              onCreated(
                (await createDynamicPlaylist(accessToken, input)).playlist.id,
              );
            } catch (reason) {
              onError(
                errorMessage(reason, "Could not create the dynamic playlist"),
              );
            } finally {
              onBusyChange(false);
            }
          }}
        />
      )}
    </section>
  );
}

function LibraryResults({ entries }: { entries: LibraryEntry[] }) {
  const kind = entries[0].kind;
  if (kind === "track")
    return (
      <TrackList
        tracks={entries.flatMap((entry) =>
          entry.kind === "track" ? [entry.item] : [],
        )}
      />
    );
  if (kind === "album")
    return (
      <AlbumGrid
        albums={entries.flatMap((entry) =>
          entry.kind === "album" ? [entry.item] : [],
        )}
      />
    );
  if (kind === "artist")
    return (
      <ArtistGrid
        artists={entries.flatMap((entry) =>
          entry.kind === "artist" ? [entry.item] : [],
        )}
      />
    );
  return (
    <div className="playlist-grid">
      {entries
        .flatMap((entry) => (entry.kind === "playlist" ? [entry.item] : []))
        .map((playlist) => (
          <Link
            className="playlist-card"
            to={`/library/${playlist.id}`}
            key={playlist.id}
          >
            <span className="playlist-cover" aria-hidden="true">
              {playlist.type === "dynamic" ? "◇" : "≡"}
            </span>
            <div>
              <strong>{playlist.name}</strong>
              <span>{playlist.description || "No description yet"}</span>
            </div>
            <small>
              {playlist.type} · {playlist.visibility}
            </small>
            <span className="playlist-card-chevron">
              <Icon name="forward" size={19} />
            </span>
          </Link>
        ))}
    </div>
  );
}

function LibraryEmpty({
  view,
  filtered,
  onCreate,
}: {
  view: LibraryView;
  filtered: boolean;
  onCreate(): void;
}) {
  const labels: Record<LibraryView, string> = {
    playlists: "playlists",
    tracks: "liked tracks",
    albums: "liked albums",
    artists: "followed artists",
  };
  return (
    <div className="library-empty">
      <span className="library-empty-icon" aria-hidden="true">
        {filtered ? "⌕" : "＋"}
      </span>
      <h2>{filtered ? "No matching items." : `No ${labels[view]} yet.`}</h2>
      <p>
        {filtered
          ? "Try a broader search."
          : "Saved music and artists will appear here."}
      </p>
      {!filtered && view === "playlists" ? (
        <button type="button" onClick={onCreate}>
          Create playlist
        </button>
      ) : null}
    </div>
  );
}

function LibraryPagination({
  page,
  total,
  parameters,
}: {
  page: number;
  total: number;
  parameters: URLSearchParams;
}) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  if (pages === 1 && page === 1) return null;
  const url = (target: number) => {
    const next = new URLSearchParams(parameters);
    if (target <= 1) next.delete("page");
    else next.set("page", String(target));
    return `?${next}`;
  };
  return (
    <nav className="catalog-pagination" aria-label="Library pages">
      {page > 1 ? <Link to={url(page - 1)}>← Previous</Link> : <span />}
      <span>
        Page {page} of {pages}
      </span>
      {page < pages ? <Link to={url(page + 1)}>Next →</Link> : <span />}
    </nav>
  );
}

export function LibraryLoading() {
  return <CatalogLoading label="your library" />;
}

export function LibraryError({
  message,
  onRetry,
}: {
  message: string;
  onRetry(): void;
}) {
  return (
    <div className="catalog-state catalog-error" role="alert">
      <div>
        <strong>Library is unavailable</strong>
        <span>{message}</span>
      </div>
      <button type="button" onClick={onRetry}>
        Try again
      </button>
    </div>
  );
}

export function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof ApiError ? reason.message : fallback;
}

function libraryEntries(
  data: LibraryData,
  view: LibraryView,
  query: string,
  sort: LibrarySort,
): LibraryEntry[] {
  const entries: LibraryEntry[] =
    view === "playlists"
      ? data.playlists.map((item) => ({
          kind: "playlist",
          id: item.id,
          title: item.name,
          search: `${item.name} ${item.description}`,
          savedAt: item.updated_at,
          item,
        }))
      : view === "tracks"
        ? data.tracks.map(({ item, savedAt }) => ({
            kind: "track",
            id: item.id,
            title: item.title,
            search: `${item.title} ${item.artists.map(({ artist }) => artist.name).join(" ")}`,
            savedAt,
            item,
          }))
        : view === "albums"
          ? data.albums.map(({ item, savedAt }) => ({
              kind: "album",
              id: item.id,
              title: item.title,
              search: `${item.title} ${item.artists.map(({ artist }) => artist.name).join(" ")}`,
              savedAt,
              item,
            }))
          : data.artists.map(({ item, savedAt }) => ({
              kind: "artist",
              id: item.id,
              title: item.name,
              search: `${item.name} ${item.description}`,
              savedAt,
              item,
            }));
  const normalized = query.toLocaleLowerCase();
  return entries
    .filter(
      (entry) =>
        !normalized || entry.search.toLocaleLowerCase().includes(normalized),
    )
    .sort((left, right) =>
      sort === "title"
        ? left.title.localeCompare(right.title)
        : right.savedAt.localeCompare(left.savedAt) ||
          left.title.localeCompare(right.title),
    );
}

function libraryCount(data: LibraryData, view: LibraryView) {
  return data[view].length;
}

function libraryView(value: string | null): LibraryView {
  return views.some((view) => view.value === value)
    ? (value as LibraryView)
    : "playlists";
}

function positivePage(value: string | null) {
  const page = Number(value ?? "1");
  return Number.isInteger(page) && page > 0 ? page : 1;
}
