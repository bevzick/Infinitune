import { useCallback, useEffect, useState } from "react";
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
import {
  getRecommendedTracks,
  type RecommendedTrack,
} from "../features/recommendations/recommendationApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";

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

      <PersonalizedSection />

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

function PersonalizedSection() {
  const { accessToken } = useSession();
  const [items, setItems] = useState<RecommendedTrack[]>([]);
  const [status, setStatus] = useState<
    "idle" | "loading" | "ready" | "disabled" | "error"
  >("idle");
  const [message, setMessage] = useState("");
  const [attempt, setAttempt] = useState(0);

  const retry = useCallback(() => setAttempt((value) => value + 1), []);

  useEffect(() => {
    if (!accessToken) {
      setStatus("idle");
      setItems([]);
      return;
    }
    const controller = new AbortController();
    setStatus("loading");
    setMessage("");
    if (attempt > 0) setItems([]);
    getRecommendedTracks(accessToken, 6, controller.signal)
      .then((recommended) => {
        setItems(recommended);
        setStatus("ready");
      })
      .catch((reason: unknown) => {
        if (controller.signal.aborted) return;
        if (
          reason instanceof ApiError &&
          reason.code === "personalization_disabled"
        ) {
          setStatus("disabled");
          return;
        }
        setMessage(
          reason instanceof ApiError
            ? reason.message
            : "Recommendations are temporarily unavailable",
        );
        setStatus("error");
      });
    return () => controller.abort();
  }, [accessToken, attempt]);

  if (!accessToken || status === "idle" || status === "disabled") return null;

  return (
    <CatalogSection
      eyebrow="Selected from your signals"
      title="Made for you"
      link="/library?view=tracks"
      linkLabel="Open your library"
    >
      {status === "loading" ? <CatalogLoading label="recommendations" /> : null}
      {status === "error" ? (
        <CatalogError message={message} onRetry={retry} />
      ) : null}
      {status === "ready" && !items.length ? (
        <CatalogEmpty>
          Listen to and like a few tracks to shape this space.
        </CatalogEmpty>
      ) : null}
      {status === "ready" && items.length ? (
        <TrackList
          tracks={items.map(({ track }) => track)}
          playbackContext={{
            type: "recommendation",
            id: items[0].recommendationId,
          }}
          actions={(_, index) => (
            <span className="recommendation-reason">
              {items[index].reason || "Picked for you"}
            </span>
          )}
        />
      ) : null}
    </CatalogSection>
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
