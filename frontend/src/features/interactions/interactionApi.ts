import { apiRequest } from "../../shared/api/client";

export type InteractionKind = "track" | "album" | "artist";

type Like = { id: string; liked_at: string };
type Follow = { artist_id: string; followed_at: string };
type InteractionPage<T> = {
  items: T[];
  page: { limit: number; offset: number; total: number };
};

const pageSize = 100;

export async function listInteractionIDs(
  accessToken: string,
  kind: InteractionKind,
  signal?: AbortSignal,
) {
  const ids: string[] = [];
  let offset = 0;
  let total = 0;

  do {
    const path = listPath(kind, offset);
    const page = await apiRequest<InteractionPage<Like | Follow>>(path, {
      accessToken,
      signal,
    });
    ids.push(
      ...page.items.map((item) =>
        "artist_id" in item ? item.artist_id : item.id,
      ),
    );
    total = page.page.total;
    if (!page.items.length) break;
    offset += page.items.length;
  } while (offset < total);

  return ids;
}

export function setInteraction(
  accessToken: string,
  kind: InteractionKind,
  id: string,
  active: boolean,
) {
  return apiRequest<void>(interactionPath(kind, id), {
    method: active ? "PUT" : "DELETE",
    accessToken,
  });
}

function listPath(kind: InteractionKind, offset: number) {
  const resource = kind === "artist" ? "artists/follows" : `${kind}s/likes`;
  const query = new URLSearchParams({
    limit: String(pageSize),
    offset: String(offset),
  });
  return `/v1/interactions/${resource}?${query}`;
}

function interactionPath(kind: InteractionKind, id: string) {
  const action = kind === "artist" ? "follow" : "like";
  return `/v1/interactions/${kind}s/${encodeURIComponent(id)}/${action}`;
}
