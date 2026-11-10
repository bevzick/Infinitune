import { type FormEvent, useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";

import {
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
  readCatalogSearch,
  searchPageSize,
  searchParameters,
  searchTrackFilters,
} from "../features/catalog/searchState";
import { useCatalogPage } from "../features/catalog/useCatalogPage";

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
      <header className="catalog-page-heading">
        <span className="hero-kicker">Catalog search</span>
        <h1>Every sound, one query away.</h1>
        <p>
          Combine metadata filters and keep the result shareable in the URL.
        </p>
      </header>

      <form className="search-panel" onSubmit={submit} key={formKey}>
        <label className="search-query" htmlFor="catalog-query">
          <span>Track title</span>
          <input
            id="catalog-query"
            name="query"
            type="search"
            defaultValue={search.query}
            placeholder="Try First Light…"
          />
        </label>
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
          <Link to="/search">Clear</Link>
        </div>
      </form>

      <div className="result-heading">
        <div>
          <span className="eyebrow">Results</span>
          <h2>{resultTitle(search.query)}</h2>
        </div>
        {results.page ? (
          <span>{results.page.pagination.total} tracks</span>
        ) : null}
      </div>

      {results.status === "loading" ? <CatalogLoading label="tracks" /> : null}
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
    </section>
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

function resultTitle(query: string) {
  return query ? `Tracks matching “${query}”` : "Browse all tracks";
}
