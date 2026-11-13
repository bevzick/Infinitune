import { apiRequest } from "../../shared/api/client";

export type PageMetadata = {
  limit: number;
  offset: number;
  total: number;
};

export type AdminUser = {
  id: string;
  email: string;
  status: "active" | "blocked" | "deleted";
  email_verified: boolean;
  roles: string[];
  created_at: string;
  updated_at: string;
  last_login_at: string | null;
};

export type AdminUserPage = {
  items: AdminUser[];
  page: PageMetadata;
};

export type CreatorReviewItem = {
  id: string;
  type: "artist" | "label" | "distributor";
  display_name: string;
  verification_status: "pending" | "verified" | "rejected" | "unverified";
  created_at: string;
  updated_at: string;
};

export type ReleaseReviewItem = {
  id: string;
  creator_id: string;
  title: string;
  release_type: "single" | "ep" | "album" | "compilation";
  release_date: string;
  status: string;
  submitted_at: string | null;
  created_at: string;
  updated_at: string;
};

export type CommentReport = {
  report: {
    id: string;
    comment_id: string;
    reporter_user_id: string;
    reason: string;
    description: string;
    status: "pending" | "reviewing" | "resolved" | "rejected";
    created_at: string;
  };
  comment: {
    id: string;
    user_id: string;
    track_id: string;
    parent_id: string | null;
    content: string;
    status: "published" | "hidden" | "deleted";
  };
};

export type Page<T> = { items: T[]; page: PageMetadata };

export type PageRequest = {
  limit: number;
  offset: number;
};

function pageQuery(page: PageRequest, status?: string) {
  const query = new URLSearchParams({
    limit: String(page.limit),
    offset: String(page.offset),
  });
  if (status && status !== "all") query.set("status", status);
  return query;
}

export function listUsers(
  accessToken: string,
  request: PageRequest & { status?: AdminUser["status"] | "all" },
  signal?: AbortSignal,
) {
  const query = pageQuery(request, request.status);
  return apiRequest<AdminUserPage>(`/v1/auth/users?${query}`, {
    accessToken,
    signal,
  });
}

export function updateUserStatus(
  accessToken: string,
  userId: string,
  status: "active" | "blocked",
) {
  return apiRequest<AdminUser>(`/v1/auth/users/${userId}/status`, {
    method: "PATCH",
    accessToken,
    body: JSON.stringify({ status }),
  });
}

export function assignUserRole(
  accessToken: string,
  userId: string,
  role: "admin" | "moderator" | "creator",
) {
  return apiRequest<{ user_id: string; roles: string[] }>(
    `/v1/auth/users/${userId}/roles/${role}`,
    { method: "PUT", accessToken },
  );
}

export function removeUserRole(
  accessToken: string,
  userId: string,
  role: "admin" | "moderator" | "creator",
) {
  return apiRequest<{ user_id: string; roles: string[] }>(
    `/v1/auth/users/${userId}/roles/${role}`,
    { method: "DELETE", accessToken },
  );
}

export function listCreatorsForModeration(
  accessToken: string,
  request: PageRequest & {
    status: CreatorReviewItem["verification_status"];
  },
  signal?: AbortSignal,
) {
  return apiRequest<Page<CreatorReviewItem>>(
    `/v1/moderation/creators?${pageQuery(request, request.status)}`,
    { accessToken, signal },
  );
}

export function reviewCreator(
  accessToken: string,
  creatorId: string,
  decision: "verified" | "rejected",
) {
  return apiRequest<CreatorReviewItem>(
    `/v1/creators/${creatorId}/verification-reviews`,
    {
      method: "POST",
      accessToken,
      body: JSON.stringify({ decision }),
    },
  );
}

export function listReleasesForModeration(
  accessToken: string,
  request: PageRequest & { status: string },
  signal?: AbortSignal,
) {
  return apiRequest<Page<ReleaseReviewItem>>(
    `/v1/moderation/releases?${pageQuery(request, request.status)}`,
    { accessToken, signal },
  );
}

export function reviewRelease(
  accessToken: string,
  releaseId: string,
  decision: "approved" | "rejected" | "changes_requested",
  comment = "",
) {
  return apiRequest(`/v1/releases/${releaseId}/moderation-reviews`, {
    method: "POST",
    accessToken,
    body: JSON.stringify({ decision, comment }),
  });
}

export function listCommentReports(
  accessToken: string,
  request: PageRequest & { status: CommentReport["report"]["status"] },
  signal?: AbortSignal,
) {
  return apiRequest<Page<CommentReport>>(
    `/v1/moderation/comment-reports?${pageQuery(request, request.status)}`,
    { accessToken, signal },
  );
}

export function moderateCommentReport(
  accessToken: string,
  reportId: string,
  action:
    | "start_review"
    | "hide_comment"
    | "restore_comment"
    | "resolve_report"
    | "reject_report",
  note: string,
) {
  return apiRequest(`/v1/moderation/comment-reports/${reportId}/actions`, {
    method: "POST",
    accessToken,
    body: JSON.stringify({ action, note }),
  });
}
