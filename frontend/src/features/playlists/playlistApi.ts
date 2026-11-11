import { apiRequest } from "../../shared/api/client";
import type { Track } from "../catalog/catalogApi";

export type PlaylistVisibility = "private" | "unlisted" | "public";

export type Playlist = {
  id: string;
  owner_id: string;
  name: string;
  description: string;
  type: "manual" | "dynamic";
  visibility: PlaylistVisibility;
  rule_set_id?: string;
  created_at: string;
  updated_at: string;
};

export type PlaylistTrack = {
  track_id: string;
  position: number;
  added_by: string;
  added_at: string;
};

export type Page = {
  limit: number;
  offset: number;
  total: number;
};

export type PlaylistPage = { items: Playlist[]; page: Page };
export type PlaylistTrackPage = { items: PlaylistTrack[]; page: Page };

export type RuleLogic = "AND" | "OR";
export type RuleField =
  | "title"
  | "artist_id"
  | "album_id"
  | "genre_id"
  | "mood_id"
  | "release_year"
  | "duration_ms"
  | "explicit";
export type RuleOperator =
  | "equals"
  | "contains"
  | "in"
  | "greater_than"
  | "greater_than_or_equal"
  | "less_than"
  | "less_than_or_equal"
  | "between";

export type RuleValue =
  | { kind: "text"; text: string }
  | { kind: "uuid"; uuid: string }
  | { kind: "uuid_list"; uuid_list: string[] }
  | { kind: "integer"; integer: number }
  | {
      kind: "integer_range";
      integer_range: { minimum: number; maximum: number };
    }
  | { kind: "boolean"; boolean: boolean };

export type RuleConditionInput = {
  field: RuleField;
  operator: RuleOperator;
  value: RuleValue;
};

export type RuleSetInput = {
  name: string;
  logic: RuleLogic;
  conditions: RuleConditionInput[];
};

export type RuleCondition = RuleConditionInput & {
  id: string;
  position: number;
};

export type RuleSet = {
  id: string;
  owner_id: string;
  name: string;
  logic: RuleLogic;
  conditions: RuleCondition[];
  created_at: string;
  updated_at: string;
};

export type DynamicPlaylistInput = PlaylistInput & {
  criteria: RuleSetInput;
};

export type DynamicPlaylist = {
  playlist: Playlist;
  criteria: RuleSet;
};

export type DynamicTrackPage = {
  items: Array<{ track_id: string; position: number }>;
  page: Page;
  resolution: { status: "ready" | "empty"; reason?: "no_matches" };
  personalized: boolean;
};

export type PlaylistInput = {
  name: string;
  description: string;
  visibility: PlaylistVisibility;
};

export function listPlaylists(
  accessToken: string,
  limit: number,
  offset: number,
  signal?: AbortSignal,
) {
  const query = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  return apiRequest<PlaylistPage>(`/v1/playlists?${query}`, {
    accessToken,
    signal,
  });
}

export function createPlaylist(accessToken: string, input: PlaylistInput) {
  return apiRequest<Playlist>("/v1/playlists", {
    method: "POST",
    accessToken,
    body: JSON.stringify(input),
  });
}

export function createDynamicPlaylist(
  accessToken: string,
  input: DynamicPlaylistInput,
) {
  return apiRequest<DynamicPlaylist>("/v1/playlists/dynamic", {
    method: "POST",
    accessToken,
    body: JSON.stringify(input),
  });
}

export function getPlaylist(
  accessToken: string,
  playlistId: string,
  signal?: AbortSignal,
) {
  return apiRequest<Playlist>(playlistPath(playlistId), {
    accessToken,
    signal,
  });
}

export function updatePlaylist(
  accessToken: string,
  playlistId: string,
  input: PlaylistInput,
) {
  return apiRequest<Playlist>(playlistPath(playlistId), {
    method: "PATCH",
    accessToken,
    body: JSON.stringify(input),
  });
}

export function deletePlaylist(accessToken: string, playlistId: string) {
  return apiRequest<void>(playlistPath(playlistId), {
    method: "DELETE",
    accessToken,
  });
}

export function listPlaylistTracks(
  accessToken: string,
  playlistId: string,
  signal?: AbortSignal,
) {
  return apiRequest<PlaylistTrackPage>(
    `${playlistPath(playlistId)}/tracks?limit=100&offset=0`,
    { accessToken, signal },
  );
}

export function addPlaylistTrack(
  accessToken: string,
  playlistId: string,
  trackId: string,
) {
  return apiRequest<PlaylistTrack>(`${playlistPath(playlistId)}/tracks`, {
    method: "POST",
    accessToken,
    body: JSON.stringify({ track_id: trackId }),
  });
}

export function removePlaylistTrack(
  accessToken: string,
  playlistId: string,
  trackId: string,
) {
  return apiRequest<void>(
    `${playlistPath(playlistId)}/tracks/${encodeURIComponent(trackId)}`,
    { method: "DELETE", accessToken },
  );
}

export function reorderPlaylistTracks(
  accessToken: string,
  playlistId: string,
  trackIds: string[],
) {
  return apiRequest<PlaylistTrack[]>(
    `${playlistPath(playlistId)}/tracks/order`,
    {
      method: "PUT",
      accessToken,
      body: JSON.stringify({ track_ids: trackIds }),
    },
  );
}

export function getDynamicCriteria(
  accessToken: string,
  playlistId: string,
  signal?: AbortSignal,
) {
  return apiRequest<RuleSet>(`${playlistPath(playlistId)}/criteria`, {
    accessToken,
    signal,
  });
}

export function replaceDynamicCriteria(
  accessToken: string,
  playlistId: string,
  criteria: RuleSetInput,
) {
  return apiRequest<RuleSet>(`${playlistPath(playlistId)}/criteria`, {
    method: "PUT",
    accessToken,
    body: JSON.stringify(criteria),
  });
}

export function resolveDynamicTracks(
  accessToken: string,
  playlistId: string,
  signal?: AbortSignal,
) {
  return apiRequest<DynamicTrackPage>(
    `${playlistPath(playlistId)}/resolved-tracks?limit=100&offset=0`,
    { accessToken, signal },
  );
}

export function getCatalogTrack(trackId: string, signal?: AbortSignal) {
  return apiRequest<Track>(
    `/v1/catalog/tracks/${encodeURIComponent(trackId)}`,
    { signal },
  );
}

function playlistPath(playlistId: string) {
  return `/v1/playlists/${encodeURIComponent(playlistId)}`;
}
