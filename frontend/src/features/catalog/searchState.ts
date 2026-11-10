import type { TrackFilters, TrackSort } from "./catalogApi";

export const searchPageSize = 10;

export type CatalogSearchState = {
  query: string;
  artistId: string;
  albumId: string;
  genreId: string;
  moodId: string;
  releaseYear: string;
  explicit: "" | "true" | "false";
  sort: TrackSort;
  page: number;
};

const trackSorts = new Set<TrackSort>([
  "newest",
  "oldest",
  "title_asc",
  "title_desc",
]);

export function readCatalogSearch(
  parameters: URLSearchParams,
): CatalogSearchState {
  const rawSort = parameters.get("sort") as TrackSort | null;
  const rawExplicit = parameters.get("explicit");
  const rawYear = parameters.get("year") ?? "";
  const rawPage = Number(parameters.get("page") ?? "1");
  return {
    query: parameters.get("q")?.trim() ?? "",
    artistId: parameters.get("artist") ?? "",
    albumId: parameters.get("album") ?? "",
    genreId: parameters.get("genre") ?? "",
    moodId: parameters.get("mood") ?? "",
    releaseYear: validYear(rawYear) ? rawYear : "",
    explicit:
      rawExplicit === "true" || rawExplicit === "false" ? rawExplicit : "",
    sort: rawSort && trackSorts.has(rawSort) ? rawSort : "newest",
    page: Number.isInteger(rawPage) && rawPage > 0 ? rawPage : 1,
  };
}

export function searchTrackFilters(state: CatalogSearchState): TrackFilters {
  return {
    title: state.query,
    artistId: state.artistId,
    albumId: state.albumId,
    genreId: state.genreId,
    moodId: state.moodId,
    releaseYear: state.releaseYear ? Number(state.releaseYear) : undefined,
    explicit: state.explicit ? state.explicit === "true" : undefined,
    sort: state.sort,
    limit: searchPageSize,
    offset: (state.page - 1) * searchPageSize,
  };
}

export function searchParameters(form: FormData) {
  const parameters = new URLSearchParams();
  set(parameters, "q", form.get("query"));
  set(parameters, "artist", form.get("artist"));
  set(parameters, "album", form.get("album"));
  set(parameters, "genre", form.get("genre"));
  set(parameters, "mood", form.get("mood"));
  set(parameters, "year", form.get("year"));
  set(parameters, "explicit", form.get("explicit"));
  set(parameters, "sort", form.get("sort"), "newest");
  return parameters;
}

function validYear(value: string) {
  if (!/^\d{1,4}$/.test(value)) return value === "";
  const year = Number(value);
  return year >= 1 && year <= 9999;
}

function set(
  parameters: URLSearchParams,
  name: string,
  rawValue: FormDataEntryValue | null,
  defaultValue = "",
) {
  const value = typeof rawValue === "string" ? rawValue.trim() : "";
  if (value && value !== defaultValue) parameters.set(name, value);
}
