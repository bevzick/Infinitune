import { apiRequest } from "../../shared/api/client";

export type CreatorAccount = {
  id: string;
  type: "artist" | "label" | "distributor";
  display_name: string;
  verification_status: "unverified" | "pending" | "verified" | "rejected";
};

export type ReleaseType = "single" | "ep" | "album" | "compilation";
export type ReleaseStatus =
  | "draft"
  | "uploading"
  | "processing"
  | "pending_review"
  | "approved"
  | "rejected"
  | "changes_requested"
  | "published"
  | "archived";

export type CreatorReleaseTrack = {
  id: string;
  title: string;
  track_number: number;
  disc_number: number;
  explicit: boolean;
  catalog_track_id?: string;
};

export type CreatorRelease = {
  id: string;
  creator_id: string;
  title: string;
  release_type: ReleaseType;
  release_date: string;
  status: ReleaseStatus;
  cover_object_key: string | null;
  tracks: CreatorReleaseTrack[];
  created_at: string;
  updated_at: string;
};

export type CreateReleaseInput = {
  creator_id: string;
  title: string;
  release_type: ReleaseType;
  release_date: string;
  tracks: Array<Omit<CreatorReleaseTrack, "id" | "catalog_track_id">>;
};

export type UpdateReleaseInput = {
  title: string;
  release_type: ReleaseType;
  release_date: string;
  cover_object_key: string | null;
  tracks: Array<Omit<CreatorReleaseTrack, "catalog_track_id">>;
};

type ReleasePage = {
  items: CreatorRelease[];
  page: { limit: number; offset: number; total: number };
};

export function listCreatorAccounts(accessToken: string, signal?: AbortSignal) {
  return apiRequest<CreatorAccount[]>("/v1/creators", {
    accessToken,
    signal,
  });
}

export async function listDraftReleases(
  accessToken: string,
  signal?: AbortSignal,
) {
  const query = new URLSearchParams({
    status: "draft",
    limit: "100",
    offset: "0",
  });
  const page = await apiRequest<ReleasePage>(`/v1/releases?${query}`, {
    accessToken,
    signal,
  });
  return Promise.all(
    page.items.map(({ id }) =>
      apiRequest<CreatorRelease>(`/v1/releases/${id}`, {
        accessToken,
        signal,
      }),
    ),
  );
}

export function createRelease(accessToken: string, input: CreateReleaseInput) {
  return apiRequest<CreatorRelease>("/v1/releases", {
    method: "POST",
    accessToken,
    body: JSON.stringify(input),
  });
}

export function updateRelease(
  accessToken: string,
  releaseId: string,
  input: UpdateReleaseInput,
) {
  return apiRequest<CreatorRelease>(`/v1/releases/${releaseId}`, {
    method: "PUT",
    accessToken,
    body: JSON.stringify(input),
  });
}
