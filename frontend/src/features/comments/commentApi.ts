import { apiRequest } from "../../shared/api/client";

export type TrackComment = {
  id: string;
  user_id: string;
  track_id: string;
  parent_id: string | null;
  content: string;
  created_at: string;
  updated_at: string;
};

export type CommentPage = {
  items: TrackComment[];
  page: { limit: number; offset: number; total: number };
};

export type ReportReason =
  | "spam"
  | "harassment"
  | "hate_speech"
  | "sexual_content"
  | "violence"
  | "misinformation"
  | "other";

export const commentPageLimit = 20;
export const maximumCommentLength = 2000;

export function listTrackComments(
  trackId: string,
  offset = 0,
  signal?: AbortSignal,
) {
  const query = new URLSearchParams({
    limit: String(commentPageLimit),
    offset: String(offset),
  });
  return apiRequest<CommentPage>(
    `/v1/tracks/${encodeURIComponent(trackId)}/comments?${query}`,
    { signal },
  );
}

export function createTrackComment(
  accessToken: string,
  trackId: string,
  content: string,
  parentId: string | null,
) {
  return apiRequest<TrackComment>("/v1/comments", {
    method: "POST",
    accessToken,
    body: JSON.stringify({ track_id: trackId, parent_id: parentId, content }),
  });
}

export function updateTrackComment(
  accessToken: string,
  id: string,
  content: string,
) {
  return apiRequest<TrackComment>(`/v1/comments/${encodeURIComponent(id)}`, {
    method: "PATCH",
    accessToken,
    body: JSON.stringify({ content }),
  });
}

export function deleteTrackComment(accessToken: string, id: string) {
  return apiRequest<void>(`/v1/comments/${encodeURIComponent(id)}`, {
    method: "DELETE",
    accessToken,
  });
}

export function reportTrackComment(
  accessToken: string,
  id: string,
  reason: ReportReason,
  description: string,
) {
  return apiRequest<{ id: string; status: string }>(
    `/v1/comments/${encodeURIComponent(id)}/reports`,
    {
      method: "POST",
      accessToken,
      body: JSON.stringify({ reason, description }),
    },
  );
}

type LikePage = {
  items: Array<{ id: string; liked_at: string }>;
  page: { limit: number; offset: number; total: number };
};

export async function listLikedCommentIds(
  accessToken: string,
  signal?: AbortSignal,
) {
  const result = new Set<string>();
  let offset = 0;
  let total = 0;
  do {
    const query = new URLSearchParams({ limit: "100", offset: String(offset) });
    const page = await apiRequest<LikePage>(
      `/v1/interactions/comments/likes?${query}`,
      { accessToken, signal },
    );
    for (const item of page.items) result.add(item.id);
    offset += page.items.length;
    total = page.page.total;
    if (page.items.length === 0) break;
  } while (offset < total);
  return result;
}

export function setCommentLike(
  accessToken: string,
  id: string,
  liked: boolean,
) {
  return apiRequest<void>(
    `/v1/interactions/comments/${encodeURIComponent(id)}/like`,
    { method: liked ? "PUT" : "DELETE", accessToken },
  );
}
