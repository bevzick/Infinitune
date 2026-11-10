import { Link } from "react-router-dom";
import { usePlayer } from "../player/PlayerContext";
import type { Album, Artist, Track } from "./catalogApi";

export function TrackList({ tracks }: { tracks: Track[] }) {
  const player = usePlayer();
  return (
    <div className="track-list">
      {tracks.map((track, index) => (
        <article className="track-row" key={track.id}>
          <span className="track-index" aria-hidden="true">
            {String(index + 1).padStart(2, "0")}
          </span>
          <button
            className={`track-art track-play${player.currentTrack?.id === track.id ? " track-play-active" : ""}`}
            type="button"
            aria-label={`${player.currentTrack?.id === track.id && player.status === "playing" ? "Pause" : "Play"} ${track.title}`}
            onClick={() => {
              if (player.currentTrack?.id === track.id) void player.toggle();
              else void player.playQueue(tracks, index);
            }}
          >
            {track.title.slice(0, 1).toUpperCase()}
          </button>
          <div className="track-main">
            <strong>{track.title}</strong>
            <span>{primaryArtists(track)}</span>
          </div>
          <div className="track-tags">
            {track.genres.slice(0, 2).map((genre) => (
              <span key={genre.id}>{genre.name}</span>
            ))}
          </div>
          {track.explicit ? <span className="explicit-mark">E</span> : null}
          <time dateTime={`PT${Math.round(track.duration_ms / 1000)}S`}>
            {formatDuration(track.duration_ms)}
          </time>
        </article>
      ))}
    </div>
  );
}

export function AlbumGrid({ albums }: { albums: Album[] }) {
  return (
    <div className="album-grid">
      {albums.map((album) => (
        <Link
          className="album-card"
          to={`/search?album=${album.id}`}
          key={album.id}
        >
          <Artwork src={album.cover_url} label={album.title} shape="square" />
          <strong>{album.title}</strong>
          <span>
            {album.artists.map(({ artist }) => artist.name).join(", ")}
          </span>
          <small>
            {album.album_type.toUpperCase()} · {album.release_date.slice(0, 4)}
          </small>
        </Link>
      ))}
    </div>
  );
}

export function ArtistGrid({ artists }: { artists: Artist[] }) {
  return (
    <div className="artist-grid">
      {artists.map((artist) => (
        <Link
          className="artist-card"
          to={`/search?artist=${artist.id}`}
          key={artist.id}
        >
          <Artwork src={artist.image_url} label={artist.name} shape="round" />
          <strong>{artist.name}</strong>
          <span>{artist.description || "Artist on Infinitune"}</span>
        </Link>
      ))}
    </div>
  );
}

export function CatalogLoading({ label }: { label: string }) {
  return (
    <div className="catalog-state" aria-live="polite">
      <span className="loading-pulse" aria-hidden="true" />
      Loading {label}…
    </div>
  );
}

export function CatalogError({
  message,
  onRetry,
}: {
  message: string;
  onRetry(): void;
}) {
  return (
    <div className="catalog-state catalog-error" role="alert">
      <div>
        <strong>Could not load the catalog</strong>
        <span>{message}</span>
      </div>
      <button type="button" onClick={onRetry}>
        Try again
      </button>
    </div>
  );
}

export function CatalogEmpty({ children }: { children: React.ReactNode }) {
  return (
    <div className="catalog-state catalog-empty">
      <span aria-hidden="true">∅</span>
      <strong>{children}</strong>
      <small>
        Change the filters or check back after new music is published.
      </small>
    </div>
  );
}

export function CatalogPagination({
  page,
  pageSize,
  total,
  pageUrl,
}: {
  page: number;
  pageSize: number;
  total: number;
  pageUrl(page: number): string;
}) {
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  if (pageCount <= 1 && page <= 1) return null;
  const previousPage = Math.min(page - 1, pageCount);
  return (
    <nav className="catalog-pagination" aria-label="Catalog pages">
      {page > 1 ? <Link to={pageUrl(previousPage)}>← Previous</Link> : <span />}
      <span>
        Page {page} of {pageCount}
      </span>
      {page < pageCount ? <Link to={pageUrl(page + 1)}>Next →</Link> : <span />}
    </nav>
  );
}

function Artwork({
  src,
  label,
  shape,
}: {
  src: string;
  label: string;
  shape: "square" | "round";
}) {
  const className = `catalog-art catalog-art-${shape}`;
  return src ? (
    <img className={className} src={src} alt="" loading="lazy" />
  ) : (
    <span className={className} aria-hidden="true">
      {label.slice(0, 1).toUpperCase()}
    </span>
  );
}

function primaryArtists(track: Track) {
  const primary = track.artists.filter(({ role }) => role === "primary");
  return (primary.length ? primary : track.artists)
    .map(({ artist }) => artist.name)
    .join(", ");
}

function formatDuration(milliseconds: number) {
  const seconds = Math.max(0, Math.floor(milliseconds / 1000));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
