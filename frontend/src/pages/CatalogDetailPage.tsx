import { Link, useParams } from "react-router-dom";

import {
  AlbumGrid,
  CatalogArtwork,
  CatalogEmpty,
  CatalogError,
  CatalogLoading,
  TrackList,
} from "../features/catalog/CatalogViews";
import {
  type Album,
  type Artist,
  artistDiscographyPath,
  type DiscographyItem,
  type Track,
  trackPagePath,
} from "../features/catalog/catalogApi";
import { useCatalogItem } from "../features/catalog/useCatalogItem";
import { useCatalogPage } from "../features/catalog/useCatalogPage";
import { usePlayer } from "../features/player/PlayerContext";
import { Icon } from "../shared/ui/Icon";

export function AlbumPage() {
  const { albumId } = useParams();
  const result = useCatalogItem<Album>("albums", albumId);

  return (
    <DetailRequest<Album>
      label="album"
      result={result}
      render={(album) => <AlbumDetail album={album} />}
    />
  );
}

function AlbumDetail({ album }: { album: Album }) {
  const player = usePlayer();
  const tracks = useCatalogPage<Track>(
    trackPagePath({ albumId: album.id, limit: 100, sort: "oldest" }),
  );
  const items = tracks.page?.items ?? [];
  const duration = items.reduce((sum, track) => sum + track.duration_ms, 0);

  return (
    <article className="detail-page">
      <DetailHero
        artwork={album.cover_url}
        title={album.title}
        eyebrow={`${album.album_type} · ${year(album.release_date)}`}
        subtitle={album.artists.map(({ artist }) => artist.name).join(", ")}
        action={
          <button
            className="detail-primary"
            type="button"
            disabled={!items.length}
            onClick={() =>
              void player.playQueue(items, 0, { type: "album", id: album.id })
            }
          >
            <Icon name="play" size={18} /> Play release
          </button>
        }
      />

      <div className="detail-layout">
        <section className="detail-main" aria-labelledby="album-tracks-title">
          <header className="detail-section-heading">
            <div>
              <span className="eyebrow">Full release</span>
              <h2 id="album-tracks-title">Tracks</h2>
            </div>
            {tracks.page ? (
              <span>{tracks.page.pagination.total} tracks</span>
            ) : null}
          </header>
          {tracks.status === "loading" ? (
            <CatalogLoading label="tracks" />
          ) : null}
          {tracks.status === "error" ? (
            <CatalogError message={tracks.message} onRetry={tracks.retry} />
          ) : null}
          {tracks.status === "ready" && !items.length ? (
            <CatalogEmpty>This release has no published tracks</CatalogEmpty>
          ) : null}
          {items.length ? (
            <TrackList
              tracks={items}
              playbackContext={{ type: "album", id: album.id }}
            />
          ) : null}
        </section>

        <aside className="detail-facts">
          <span className="eyebrow">About the release</span>
          <Fact label="Type" value={album.album_type} />
          <Fact label="Released" value={formatDate(album.release_date)} />
          <Fact
            label="Duration"
            value={duration ? formatLongDuration(duration) : "—"}
          />
          <div className="detail-credits">
            <span>Artists</span>
            {album.artists.map(({ artist }) => (
              <Link to={`/artists/${artist.id}`} key={artist.id}>
                {artist.name}
              </Link>
            ))}
          </div>
        </aside>
      </div>
    </article>
  );
}

export function ArtistPage() {
  const { artistId } = useParams();
  const result = useCatalogItem<Artist>("artists", artistId);

  return (
    <DetailRequest<Artist>
      label="artist"
      result={result}
      render={(artist) => <ArtistDetail artist={artist} />}
    />
  );
}

function ArtistDetail({ artist }: { artist: Artist }) {
  const player = usePlayer();
  const discography = useCatalogPage<DiscographyItem>(
    artistDiscographyPath(artist.id),
  );
  const artistTracks = useCatalogPage<Track>(
    trackPagePath({ artistId: artist.id, limit: 100, sort: "newest" }),
  );
  const albums =
    discography.page?.items.flatMap((item) =>
      item.album ? [item.album] : [],
    ) ?? [];
  const tracks = artistTracks.page?.items ?? [];

  return (
    <article className="detail-page artist-detail-page">
      <DetailHero
        artwork={artist.image_url}
        title={artist.name}
        eyebrow="Artist"
        subtitle={artist.description || "Artist on Infinitune"}
        round
        action={
          <button
            className="detail-primary"
            type="button"
            disabled={!tracks.length}
            onClick={() =>
              void player.playQueue(tracks, 0, {
                type: "artist",
                id: artist.id,
              })
            }
          >
            <Icon name="play" size={18} /> Play tracks
          </button>
        }
      />

      {discography.status === "loading" ? (
        <CatalogLoading label="discography" />
      ) : null}
      {discography.status === "error" ? (
        <CatalogError
          message={discography.message}
          onRetry={discography.retry}
        />
      ) : null}
      {artistTracks.status === "loading" ? (
        <CatalogLoading label="artist tracks" />
      ) : null}
      {artistTracks.status === "error" ? (
        <CatalogError
          message={artistTracks.message}
          onRetry={artistTracks.retry}
        />
      ) : null}
      {discography.status === "ready" &&
      artistTracks.status === "ready" &&
      !albums.length &&
      !tracks.length ? (
        <CatalogEmpty>No releases have been published yet</CatalogEmpty>
      ) : null}
      {albums.length ? (
        <section
          className="detail-section"
          aria-labelledby="artist-albums-title"
        >
          <header className="detail-section-heading">
            <div>
              <span className="eyebrow">Discography</span>
              <h2 id="artist-albums-title">Albums & EPs</h2>
            </div>
          </header>
          <AlbumGrid albums={albums} />
        </section>
      ) : null}
      {tracks.length ? (
        <section
          className="detail-section"
          aria-labelledby="artist-tracks-title"
        >
          <header className="detail-section-heading">
            <div>
              <span className="eyebrow">Listen now</span>
              <h2 id="artist-tracks-title">Artist tracks</h2>
            </div>
          </header>
          <TrackList
            tracks={tracks}
            playbackContext={{ type: "artist", id: artist.id }}
          />
        </section>
      ) : null}
    </article>
  );
}

export function TrackPage() {
  const { trackId } = useParams();
  const result = useCatalogItem<Track>("tracks", trackId);

  return (
    <DetailRequest<Track>
      label="track"
      result={result}
      render={(track) => <TrackDetail track={track} />}
    />
  );
}

function TrackDetail({ track }: { track: Track }) {
  const player = usePlayer();
  const album = useCatalogItem<Album>("albums", track.album_id);
  const cover = album.item?.cover_url ?? "";
  const artists = track.artists.filter(({ role }) => role === "primary");

  return (
    <article className="detail-page track-detail-page">
      <DetailHero
        artwork={cover}
        title={track.title}
        eyebrow={`Track · ${year(track.release_date)}`}
        subtitle={(artists.length ? artists : track.artists)
          .map(({ artist }) => artist.name)
          .join(", ")}
        action={
          <button
            className="detail-primary"
            type="button"
            onClick={() => void player.playQueue([track], 0)}
          >
            <Icon name="play" size={18} /> Play track
          </button>
        }
      />

      <div className="track-detail-grid">
        <section className="detail-copy">
          <span className="eyebrow">Track details</span>
          <h2>A closer look</h2>
          <p>
            {track.moods.length
              ? `A ${track.moods.map(({ mood }) => mood.name.toLowerCase()).join(", ")} track`
              : "A release from the Infinitune catalog"}
            {track.genres.length
              ? ` shaped by ${track.genres.map((genre) => genre.name).join(", ")}.`
              : "."}
          </p>
          <div className="detail-tags">
            {track.genres.map((genre) => (
              <span key={genre.id}>{genre.name}</span>
            ))}
            {track.moods.map(({ mood }) => (
              <span key={mood.id}>{mood.name}</span>
            ))}
          </div>
        </section>
        <aside className="detail-facts">
          <Fact
            label="Duration"
            value={formatTrackDuration(track.duration_ms)}
          />
          <Fact label="Released" value={formatDate(track.release_date)} />
          <Fact label="Track number" value={String(track.track_number)} />
          {album.item ? (
            <div className="detail-credits">
              <span>From the album</span>
              <Link to={`/albums/${album.item.id}`}>{album.item.title}</Link>
            </div>
          ) : null}
          <div className="detail-credits">
            <span>Artists</span>
            {track.artists.map(({ artist, role }) => (
              <Link to={`/artists/${artist.id}`} key={`${artist.id}:${role}`}>
                {artist.name} <small>{role}</small>
              </Link>
            ))}
          </div>
        </aside>
      </div>
    </article>
  );
}

type RequestResult<T> =
  | { status: "idle" | "loading"; item: null; message: ""; retry(): void }
  | { status: "error"; item: null; message: string; retry(): void }
  | { status: "ready"; item: T; message: ""; retry(): void };

function DetailRequest<T>({
  label,
  result,
  render,
}: {
  label: string;
  result: RequestResult<T>;
  render(item: T): React.ReactNode;
}) {
  if (result.status === "idle" || result.status === "loading") {
    return <CatalogLoading label={label} />;
  }
  if (result.status === "error") {
    return <CatalogError message={result.message} onRetry={result.retry} />;
  }
  return render(result.item as T);
}

function DetailHero({
  artwork,
  title,
  eyebrow,
  subtitle,
  action,
  round = false,
}: {
  artwork: string;
  title: string;
  eyebrow: string;
  subtitle: string;
  action: React.ReactNode;
  round?: boolean;
}) {
  return (
    <header className="detail-hero">
      <CatalogArtwork
        src={artwork}
        label={title}
        shape={round ? "round" : "square"}
      />
      <div className="detail-hero-copy">
        <span className="hero-kicker">{eyebrow}</span>
        <h1>{title}</h1>
        <p>{subtitle}</p>
        <div className="detail-actions">{action}</div>
      </div>
    </header>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="detail-fact">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function year(value: string) {
  return value.slice(0, 4);
}

function formatDate(value: string) {
  const date = new Date(`${value}T00:00:00`);
  return Number.isNaN(date.valueOf())
    ? value
    : new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(date);
}

function formatLongDuration(milliseconds: number) {
  const minutes = Math.round(milliseconds / 60000);
  const hours = Math.floor(minutes / 60);
  return hours ? `${hours} hr ${minutes % 60} min` : `${minutes} min`;
}

function formatTrackDuration(milliseconds: number) {
  const seconds = Math.floor(milliseconds / 1000);
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
