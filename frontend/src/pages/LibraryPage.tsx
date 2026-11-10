import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

import { PlaylistForm } from "../features/playlists/PlaylistForm";
import {
  createPlaylist,
  listPlaylists,
  type PlaylistPage,
} from "../features/playlists/playlistApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";

const pageSize = 12;

export function LibraryPage() {
  const { accessToken } = useSession();
  const navigate = useNavigate();
  const [parameters] = useSearchParams();
  const pageNumber = positivePage(parameters.get("page"));
  const [page, setPage] = useState<PlaylistPage | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const [showCreate, setShowCreate] = useState(false);

  const load = useCallback(async () => {
    if (!accessToken) return;
    setStatus("loading");
    setError("");
    try {
      setPage(
        await listPlaylists(accessToken, pageSize, (pageNumber - 1) * pageSize),
      );
      setStatus("ready");
    } catch (reason) {
      setError(errorMessage(reason, "Could not load your playlists"));
      setStatus("error");
    }
  }, [accessToken, pageNumber]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <section className="library-page">
      <header className="library-heading">
        <div>
          <span className="hero-kicker">Your private collection</span>
          <h1>Library</h1>
          <p>Create a playlist, shape its order, and keep it ready to play.</p>
        </div>
        <button type="button" onClick={() => setShowCreate(true)}>
          + New playlist
        </button>
      </header>

      {showCreate ? (
        <section
          className="playlist-editor"
          aria-labelledby="new-playlist-title"
        >
          <div>
            <span className="eyebrow">Manual playlist</span>
            <h2 id="new-playlist-title">Create a new playlist</h2>
          </div>
          {error && status !== "error" ? (
            <p className="form-error" role="alert">
              {error}
            </p>
          ) : null}
          <PlaylistForm
            submitLabel="Create playlist"
            busy={creating}
            onCancel={() => setShowCreate(false)}
            onSubmit={async (input) => {
              if (!accessToken) return;
              setCreating(true);
              setError("");
              try {
                const created = await createPlaylist(accessToken, input);
                navigate(`/library/${created.id}`);
              } catch (reason) {
                setError(errorMessage(reason, "Could not create the playlist"));
              } finally {
                setCreating(false);
              }
            }}
          />
        </section>
      ) : null}

      {status === "loading" ? <LibraryLoading /> : null}
      {status === "error" ? (
        <LibraryError message={error} onRetry={() => void load()} />
      ) : null}
      {status === "ready" && page && !page.items.length ? (
        <div className="library-empty">
          <span className="library-empty-icon" aria-hidden="true">
            ＋
          </span>
          <h2>Your first playlist starts here.</h2>
          <p>Collect tracks from the catalog and arrange them in your order.</p>
          <button type="button" onClick={() => setShowCreate(true)}>
            Create playlist
          </button>
        </div>
      ) : null}
      {status === "ready" && page?.items.length ? (
        <>
          <div className="playlist-grid">
            {page.items.map((playlist) => (
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
              </Link>
            ))}
          </div>
          <LibraryPagination page={pageNumber} total={page.page.total} />
        </>
      ) : null}
    </section>
  );
}

function LibraryPagination({ page, total }: { page: number; total: number }) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  if (pages === 1 && page === 1) return null;
  return (
    <nav className="catalog-pagination" aria-label="Playlist pages">
      {page > 1 ? (
        <Link to={page > 2 ? `?page=${page - 1}` : "?"}>← Previous</Link>
      ) : (
        <span />
      )}
      <span>
        Page {page} of {pages}
      </span>
      {page < pages ? <Link to={`?page=${page + 1}`}>Next →</Link> : <span />}
    </nav>
  );
}

export function LibraryLoading() {
  return (
    <div className="catalog-state" aria-live="polite">
      <span className="loading-pulse" aria-hidden="true" />
      Loading your library…
    </div>
  );
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

function positivePage(value: string | null) {
  const page = Number(value ?? "1");
  return Number.isInteger(page) && page > 0 ? page : 1;
}
