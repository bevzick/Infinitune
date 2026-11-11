import { apiRequest } from "../../shared/api/client";

export type Pagination = {
  limit: number;
  offset: number;
  total: number;
};

export type CatalogPage<T> = {
  items: T[];
  pagination: Pagination;
};

export type ArtistSummary = {
  id: string;
  name: string;
  image_url: string;
};

export type Artist = ArtistSummary & {
  description: string;
  created_at: string;
  updated_at: string;
};

export type Album = {
  id: string;
  title: string;
  album_type: "album" | "single" | "ep" | "compilation";
  release_date: string;
  cover_url: string;
  artists: Array<{ artist: ArtistSummary; position: number }>;
  created_at: string;
  updated_at: string;
};

export type Genre = {
  id: string;
  name: string;
  slug: string;
};

export type Mood = Genre;

export type Track = {
  id: string;
  album_id?: string;
  playback_url: string;
  title: string;
  duration_ms: number;
  track_number: number;
  disc_number: number;
  explicit: boolean;
  release_date: string;
  artists: Array<{
    artist: ArtistSummary;
    role: "primary" | "featured" | "producer" | "composer";
    position: number;
  }>;
  genres: Genre[];
  moods: Array<{ mood: Mood; weight: number }>;
  created_at: string;
  updated_at: string;
};

export type LyricsLine = {
  line_number: number;
  start_ms: number;
  end_ms: number;
  text: string;
};

export type TrackLyrics = {
  id: string;
  track_id: string;
  lyrics_type: "plain" | "synchronized";
  language: string;
  content: string;
  source_type: "artist" | "publisher" | "licensed_provider" | "public_domain";
  source_name: string;
  copyright_notice: string;
  official: boolean;
  updated_at: string;
  lines: LyricsLine[];
};

export type TrackLyricsResponse = {
  items: TrackLyrics[];
};

export type TrackSort = "newest" | "oldest" | "title_asc" | "title_desc";

export type TrackFilters = {
  title?: string;
  artistId?: string;
  albumId?: string;
  genreId?: string;
  moodId?: string;
  releaseYear?: number;
  explicit?: boolean;
  sort?: TrackSort;
  limit?: number;
  offset?: number;
};

export function catalogPagePath(
  resource: "artists" | "albums" | "genres" | "moods",
  limit: number,
  offset = 0,
) {
  return `/v1/catalog/${resource}?${new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  })}`;
}

export function trackPagePath(filters: TrackFilters = {}) {
  const parameters = new URLSearchParams();
  append(parameters, "title", filters.title?.trim());
  append(parameters, "artist_id", filters.artistId);
  append(parameters, "album_id", filters.albumId);
  append(parameters, "genre_id", filters.genreId);
  append(parameters, "mood_id", filters.moodId);
  append(parameters, "release_year", filters.releaseYear);
  append(parameters, "explicit", filters.explicit);
  append(parameters, "sort", filters.sort ?? "newest");
  append(parameters, "limit", filters.limit ?? 20);
  append(parameters, "offset", filters.offset ?? 0);
  return `/v1/catalog/tracks?${parameters}`;
}

export function fetchCatalogPage<T>(path: string, signal?: AbortSignal) {
  return apiRequest<CatalogPage<T>>(path, { signal });
}

export function catalogDetailPath(
  resource: "artists" | "albums" | "tracks",
  id: string,
) {
  return `/v1/catalog/${resource}/${encodeURIComponent(id)}`;
}

export function fetchCatalogItem<T>(
  resource: "artists" | "albums" | "tracks",
  id: string,
  signal?: AbortSignal,
) {
  return apiRequest<T>(catalogDetailPath(resource, id), { signal });
}

export function trackLyricsPath(trackId: string) {
  return `/v1/catalog/tracks/${encodeURIComponent(trackId)}/lyrics`;
}

export function fetchTrackLyrics(trackId: string, signal?: AbortSignal) {
  return apiRequest<TrackLyricsResponse>(trackLyricsPath(trackId), { signal });
}

function append(
  parameters: URLSearchParams,
  name: string,
  value: string | number | boolean | undefined,
) {
  if (value !== undefined && value !== "") {
    parameters.set(name, String(value));
  }
}
