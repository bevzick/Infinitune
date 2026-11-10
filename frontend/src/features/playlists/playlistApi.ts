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

export function getCatalogTrack(trackId: string, signal?: AbortSignal) {
  return apiRequest<Track>(
    `/v1/catalog/tracks/${encodeURIComponent(trackId)}`,
    { signal },
  );
}

function playlistPath(playlistId: string) {
  return `/v1/playlists/${encodeURIComponent(playlistId)}`;
}
