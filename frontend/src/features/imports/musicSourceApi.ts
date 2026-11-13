import { apiRequest } from "../../shared/api/client";

export type MusicSourceProvider = "jamendo" | "internet_archive";

export type SourceLicense = {
  name: string;
  url: string;
  attribution: string;
  allows_streaming: boolean;
  allows_redistribution: boolean;
};

export type SourceTrack = {
  provider: MusicSourceProvider;
  external_id: string;
  title: string;
  artist: string;
  page_url: string;
  license: SourceLicense;
  metadata: Record<string, string>;
};

export type ReleaseTrackOption = {
  id: string;
  title: string;
  releaseId: string;
  releaseTitle: string;
};

export type SourceTrackPage = {
  items: SourceTrack[];
  limit: number;
  offset: number;
  total: number;
};

export type ImportResult = {
  media_asset_id: string;
  content_track_id: string;
  status: string;
};

type ProviderPage = { items: Array<{ id: MusicSourceProvider }> };
type ReleaseTrack = { id: string; title: string };
type Release = {
  id: string;
  title: string;
  status: string;
  tracks: ReleaseTrack[];
};
type ReleasePage = {
  items: Release[];
  page: { limit: number; offset: number; total: number };
};

export async function listMusicSourceProviders(
  accessToken: string,
  signal?: AbortSignal,
) {
  const response = await apiRequest<ProviderPage>(
    "/v1/media/sources/providers",
    { accessToken, signal },
  );
  return response.items.map(({ id }) => id);
}

export async function discoverSourceTracks(
  accessToken: string,
  provider: MusicSourceProvider,
  query: string,
  signal?: AbortSignal,
) {
  const parameters = new URLSearchParams({ query, limit: "20", offset: "0" });
  return apiRequest<SourceTrackPage>(
    `/v1/media/sources/${provider}/tracks?${parameters}`,
    { accessToken, signal },
  );
}

export function importSourceTrack(
  accessToken: string,
  contentTrackId: string,
  track: Pick<SourceTrack, "provider" | "external_id">,
) {
  return apiRequest<ImportResult>("/v1/media/imports", {
    method: "POST",
    accessToken,
    body: JSON.stringify({
      content_track_id: contentTrackId,
      provider: track.provider,
      external_id: track.external_id,
    }),
  });
}

export async function listDraftReleaseTracks(
  accessToken: string,
  signal?: AbortSignal,
): Promise<ReleaseTrackOption[]> {
  const parameters = new URLSearchParams({
    status: "draft",
    limit: "50",
    offset: "0",
  });
  const page = await apiRequest<ReleasePage>(`/v1/releases?${parameters}`, {
    accessToken,
    signal,
  });
  const releases = await Promise.all(
    page.items.map(({ id }) =>
      apiRequest<Release>(`/v1/releases/${id}`, { accessToken, signal }),
    ),
  );
  return releases.flatMap((release) =>
    release.tracks.map((track) => ({
      id: track.id,
      title: track.title,
      releaseId: release.id,
      releaseTitle: release.title,
    })),
  );
}
