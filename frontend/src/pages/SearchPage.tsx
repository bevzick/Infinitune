import { type FormEvent, useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";

import {
  CatalogArtwork,
  CatalogEmpty,
  CatalogError,
  CatalogLoading,
  CatalogPagination,
  TrackList,
} from "../features/catalog/CatalogViews";
import {
  type Album,
  type Artist,
  catalogPagePath,
  type Genre,
  type Mood,
  type Track,
  trackPagePath,
} from "../features/catalog/catalogApi";
import {
  type CatalogSearchState,
  readCatalogSearch,
  searchPageSize,
  searchParameters,
  searchTrackFilters,
} from "../features/catalog/searchState";
import { useCatalogPage } from "../features/catalog/useCatalogPage";
import { Icon } from "../shared/ui/Icon";

export function SearchPage() {
  const [parameters, setParameters] = useSearchParams();
  const search = readCatalogSearch(parameters);
  const requestPath = trackPagePath(searchTrackFilters(search));
  const results = useCatalogPage<Track>(requestPath);
  const artists = useCatalogPage<Artist>(catalogPagePath("artists", 100));
  const albums = useCatalogPage<Album>(catalogPagePath("albums", 100));
  const genres = useCatalogPage<Genre>(catalogPagePath("genres", 100));
  const moods = useCatalogPage<Mood>(catalogPagePath("moods", 100));
  const formKey = parameters.toString();
  const bestMatch = results.page?.items[0];

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setParameters(searchParameters(new FormData(event.currentTarget)));
  }

  const pageUrl = useMemo(() => {
    return (page: number) => {
      const next = new URLSearchParams(parameters);
      if (page > 1) next.set("page", String(page));
      else next.delete("page");
      return `/search${next.size ? `?${next}` : ""}`;
    };
  }, [parameters]);

  return (
    <section className="search-page">
      <header className="search-page-heading">
        <span className="hero-kicker">Explore the catalog</span>
        <h1>Search music</h1>
        <p>Find tracks by title, artist, album, genre, or mood.</p>
      </header>

      <form className="search-panel" onSubmit={submit} key={formKey}>
        <div className="search-primary-row">
          <label className="search-query" htmlFor="catalog-query">
            <span>Track title</span>
            <Icon name="search" size={20} />
            <input
              id="catalog-query"
              name="query"
              type="search"
              defaultValue={search.query}
              placeholder="Search music…"
            />
          </label>
          <button className="search-submit" type="submit">
            Search
          </button>
        </div>

        <details
          className="search-filter-drawer"
          open={hasAdvancedFilters(search)}
        >
          <summary>
            Advanced filters
            {activeFilterCount(search) ? (
              <span>{activeFilterCount(search)} active</span>
            ) : null}
          </summary>
          <div className="search-filter-grid">
            <SearchSelect
              label="Artist"
              name="artist"
              value={search.artistId}
              options={artists.page?.items}
            />
            <SearchSelect
              label="Album"
              name="album"
              value={search.albumId}
              options={albums.page?.items}
              getLabel={(album) => album.title}
            />
            <SearchSelect
              label="Genre"
              name="genre"
              value={search.genreId}
              options={genres.page?.items}
            />
            <SearchSelect
              label="Mood"
              name="mood"
              value={search.moodId}
              options={moods.page?.items}
            />
            <label>
              <span>Release year</span>
              <input
                name="year"
                type="number"
                min="1"
                max="9999"
                defaultValue={search.releaseYear}
                placeholder="2026"
              />
            </label>
            <label>
              <span>Explicit</span>
              <select name="explicit" defaultValue={search.explicit}>
                <option value="">Any</option>
                <option value="false">Clean only</option>
                <option value="true">Explicit only</option>
              </select>
            </label>
            <label>
              <span>Sort</span>
              <select name="sort" defaultValue={search.sort}>
                <option value="newest">Newest first</option>
                <option value="oldest">Oldest first</option>
                <option value="title_asc">Title A–Z</option>
                <option value="title_desc">Title Z–A</option>
              </select>
            </label>
            <div className="search-actions">
              <button type="submit">Apply filters</button>
              <Link to="/search">Clear all</Link>
            </div>
          </div>
        </details>
      </form>

      <div className="search-results-layout">
        <div className="search-results-main">
          <div className="result-heading">
            <div>
              <span className="eyebrow">Results</span>
              <h2>{resultTitle(search.query)}</h2>
            </div>
            {results.page ? (
              <span>{results.page.pagination.total} tracks</span>
            ) : null}
          </div>

          {results.status === "loading" ? (
            <CatalogLoading label="tracks" />
          ) : null}
          {results.status === "error" ? (
            <CatalogError message={results.message} onRetry={results.retry} />
          ) : null}
          {results.status === "ready" && !results.page.items.length ? (
            <>
              <CatalogEmpty>No tracks match these filters</CatalogEmpty>
              <CatalogPagination
                page={search.page}
                pageSize={searchPageSize}
                total={results.page.pagination.total}
                pageUrl={pageUrl}
              />
            </>
          ) : null}
          {results.status === "ready" && results.page.items.length ? (
            <>
              <TrackList
                tracks={results.page.items}
                playbackContext={{ type: "search" }}
              />
              <CatalogPagination
                page={search.page}
                pageSize={searchPageSize}
                total={results.page.pagination.total}
                pageUrl={pageUrl}
              />
            </>
          ) : null}
        </div>
        {bestMatch ? <BestMatch track={bestMatch} /> : null}
      </div>
    </section>
  );
}

function BestMatch({ track }: { track: Track }) {
  const artistNames = track.artists.map(({ artist }) => artist.name).join(", ");
  return (
    <aside className="best-match">
      <span className="eyebrow">Best match</span>
      <CatalogArtwork src="" label={track.title} shape="square" />
      <h2>{track.title}</h2>
      <p>{artistNames || "Infinitune artist"}</p>
      <span className="best-match-meta">
        Track · {track.release_date.slice(0, 4)}
      </span>
      <Link to={`/tracks/${track.id}`}>
        <Icon name="play" size={17} /> Open track
      </Link>
    </aside>
  );
}

function SearchSelect<T extends { id: string }>({
  label,
  name,
  value,
  options,
  getLabel,
}: {
  label: string;
  name: string;
  value: string;
  options?: T[];
  getLabel?: (option: T) => string;
}) {
  return (
    <label>
      <span>{label}</span>
      <select
        name={name}
        defaultValue={value}
        key={`${value}:${options?.length ?? 0}`}
      >
        <option value="">Any</option>
        {options?.map((option) => (
          <option value={option.id} key={option.id}>
            {getLabel
              ? getLabel(option)
              : "name" in option
                ? String(option.name)
                : option.id}
          </option>
        ))}
      </select>
    </label>
  );
}

function hasAdvancedFilters(search: CatalogSearchState) {
  return activeFilterCount(search) > 0;
}

function activeFilterCount(search: CatalogSearchState) {
  return [
    search.artistId,
    search.albumId,
    search.genreId,
    search.moodId,
    search.releaseYear,
    search.explicit,
    search.sort === "newest" ? "" : search.sort,
  ].filter(Boolean).length;
}

function resultTitle(query: string) {
  return query ? `Tracks matching “${query}”` : "Browse all tracks";
}
