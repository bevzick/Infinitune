import { ApiError, apiRequest } from "../../shared/api/client";
import {
  type Album,
  type Artist,
  fetchCatalogItem,
  type Track,
} from "../catalog/catalogApi";
import {
  listPlaylists,
  type Playlist,
  type PlaylistPage,
} from "../playlists/playlistApi";

const requestLimit = 100;

type Page = { limit: number; offset: number; total: number };
type Like = { id: string; liked_at: string };
type Follow = { artist_id: string; followed_at: string };
type ItemPage<T> = { items: T[]; page: Page };

export type SavedItem<T> = {
  item: T;
  savedAt: string;
};

export type LibraryData = {
  playlists: Playlist[];
  tracks: SavedItem<Track>[];
  albums: SavedItem<Album>[];
  artists: SavedItem<Artist>[];
};

export async function loadLibrary(
  accessToken: string,
  signal?: AbortSignal,
): Promise<LibraryData> {
  const [playlists, trackLikes, albumLikes, artistFollows] = await Promise.all([
    collectPlaylists(accessToken),
    collectItems<Like>(accessToken, "/v1/interactions/tracks/likes", signal),
    collectItems<Like>(accessToken, "/v1/interactions/albums/likes", signal),
    collectItems<Follow>(
      accessToken,
      "/v1/interactions/artists/follows",
      signal,
    ),
  ]);

  const [tracks, albums, artists] = await Promise.all([
    hydrate<Track, Like>(
      "tracks",
      trackLikes,
      (entry) => entry.id,
      (entry) => entry.liked_at,
      signal,
    ),
    hydrate<Album, Like>(
      "albums",
      albumLikes,
      (entry) => entry.id,
      (entry) => entry.liked_at,
      signal,
    ),
    hydrate<Artist, Follow>(
      "artists",
      artistFollows,
      (entry) => entry.artist_id,
      (entry) => entry.followed_at,
      signal,
    ),
  ]);

  return { playlists, tracks, albums, artists };
}

async function collectPlaylists(accessToken: string) {
  const items: Playlist[] = [];
  let offset = 0;
  let total = 0;
  do {
    const page: PlaylistPage = await listPlaylists(
      accessToken,
      requestLimit,
      offset,
    );
    items.push(...page.items);
    total = page.page.total;
    if (!page.items.length) break;
    offset += page.items.length;
  } while (offset < total);
  return items;
}

async function collectItems<T>(
  accessToken: string,
  path: string,
  signal?: AbortSignal,
) {
  const items: T[] = [];
  let offset = 0;
  let total = 0;
  do {
    const query = new URLSearchParams({
      limit: String(requestLimit),
      offset: String(offset),
    });
    const page = await apiRequest<ItemPage<T>>(`${path}?${query}`, {
      accessToken,
      signal,
    });
    items.push(...page.items);
    total = page.page.total;
    if (!page.items.length) break;
    offset += page.items.length;
  } while (offset < total);
  return items;
}

async function hydrate<T, TReference>(
  resource: "artists" | "albums" | "tracks",
  references: TReference[],
  id: (reference: TReference) => string,
  savedAt: (reference: TReference) => string,
  signal?: AbortSignal,
) {
  const results: Array<SavedItem<T> | null> = await Promise.all(
    references.map(async (reference) => {
      try {
        return {
          item: await fetchCatalogItem<T>(resource, id(reference), signal),
          savedAt: savedAt(reference),
        } as SavedItem<T>;
      } catch (reason) {
        if (reason instanceof ApiError && reason.status === 404) return null;
        throw reason;
      }
    }),
  );
  return results.filter((item): item is SavedItem<T> => item !== null);
}
