import { Link } from "react-router-dom";

import {
  AlbumGrid,
  ArtistGrid,
  CatalogEmpty,
  CatalogError,
  CatalogLoading,
  TrackList,
} from "../features/catalog/CatalogViews";
import {
  type Album,
  type Artist,
  catalogPagePath,
  type Track,
  trackPagePath,
} from "../features/catalog/catalogApi";
import { useCatalogPage } from "../features/catalog/useCatalogPage";

export function DiscoverPage() {
  const tracks = useCatalogPage<Track>(trackPagePath({ limit: 6 }));
  const albums = useCatalogPage<Album>(catalogPagePath("albums", 4));
  const artists = useCatalogPage<Artist>(catalogPagePath("artists", 6));

  return (
    <section className="discover-page">
      <div className="catalog-hero">
        <div className="catalog-hero-copy">
          <span className="hero-kicker">Fresh signals from the catalog</span>
          <h1>
            Find your next
            <br />
            <em>repeat.</em>
          </h1>
          <p>
            Browse new releases, meet independent voices, or search every track
            by the detail that matters.
          </p>
          <search>
            <form className="hero-search" action="/search">
              <label htmlFor="discover-search">Search the catalog</label>
              <div>
                <input
                  id="discover-search"
                  name="q"
                  type="search"
                  placeholder="Track title…"
                />
                <button type="submit">Search</button>
              </div>
            </form>
          </search>
        </div>
        <div className="catalog-signal" aria-hidden="true">
          <span>∞</span>
          <i />
          <i />
          <i />
        </div>
      </div>

      <CatalogSection
        eyebrow="Recently published"
        title="Tracks in motion"
        link="/search"
        linkLabel="Browse all tracks"
      >
        <CatalogResult state={tracks} empty="No tracks have been published yet">
          {(page) => <TrackList tracks={page.items} />}
        </CatalogResult>
      </CatalogSection>

      <CatalogSection
        eyebrow="New releases"
        title="Albums & EPs"
        link="/albums"
        linkLabel="See all releases"
      >
        <CatalogResult state={albums} empty="No releases are available yet">
          {(page) => <AlbumGrid albums={page.items} />}
        </CatalogResult>
      </CatalogSection>

      <CatalogSection
        eyebrow="Voices to know"
        title="Artists"
        link="/artists"
        linkLabel="Meet every artist"
      >
        <CatalogResult state={artists} empty="No artists are available yet">
          {(page) => <ArtistGrid artists={page.items} />}
        </CatalogResult>
      </CatalogSection>
    </section>
  );
}

type ResultState<T> = ReturnType<typeof useCatalogPage<T>>;

function CatalogResult<T>({
  state,
  empty,
  children,
}: {
  state: ResultState<T>;
  empty: string;
  children(page: NonNullable<ResultState<T>["page"]>): React.ReactNode;
}) {
  if (state.status === "loading") return <CatalogLoading label="music" />;
  if (state.status === "error") {
    return <CatalogError message={state.message} onRetry={state.retry} />;
  }
  if (!state.page.items.length) return <CatalogEmpty>{empty}</CatalogEmpty>;
  return children(state.page);
}

function CatalogSection({
  eyebrow,
  title,
  link,
  linkLabel,
  children,
}: {
  eyebrow: string;
  title: string;
  link: string;
  linkLabel: string;
  children: React.ReactNode;
}) {
  return (
    <section className="catalog-section">
      <header className="section-heading">
        <div>
          <span className="eyebrow">{eyebrow}</span>
          <h2>{title}</h2>
        </div>
        <Link to={link}>{linkLabel} →</Link>
      </header>
      {children}
    </section>
  );
}
