import { ApiError, apiRequest } from "../../shared/api/client";
import { fetchCatalogItem, type Track } from "../catalog/catalogApi";

export type RecommendationItem = {
  track_id: string;
  score: number;
  rank: number;
  reason: string;
};

export type RecommendationPage = {
  run_id: string;
  algorithm_version: string;
  generated_at: string;
  expires_at: string;
  items: RecommendationItem[];
  total: number;
  limit: number;
  offset: number;
};

export type RecommendedTrack = RecommendationItem & {
  recommendationId: string;
  track: Track;
};

export async function getRecommendedTracks(
  accessToken: string,
  limit = 6,
  signal?: AbortSignal,
): Promise<RecommendedTrack[]> {
  const query = new URLSearchParams({ limit: String(limit), offset: "0" });
  const page = await apiRequest<RecommendationPage>(
    `/v1/recommendations?${query}`,
    { accessToken, signal },
  );
  const tracks = await Promise.all(
    page.items.map(async (item) => {
      try {
        return {
          ...item,
          recommendationId: page.run_id,
          track: await fetchCatalogItem<Track>("tracks", item.track_id, signal),
        };
      } catch (reason) {
        if (reason instanceof ApiError && reason.status === 404) return null;
        throw reason;
      }
    }),
  );
  return tracks.filter((item): item is RecommendedTrack => item !== null);
}
