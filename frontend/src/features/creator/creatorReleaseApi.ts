import { ApiError, apiRequest, gatewayUrl } from "../../shared/api/client";

export type CreatorAccount = {
  id: string;
  type: "artist" | "label" | "distributor";
  display_name: string;
  verification_status: "unverified" | "pending" | "verified" | "rejected";
};

export type CreateCreatorAccountInput = Pick<
  CreatorAccount,
  "type" | "display_name"
>;

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

export type TrackRightType = "streaming" | "download" | "distribution";

export type TrackRight = {
  id: string;
  release_track_id: string;
  territory_code: string;
  rights_type: TrackRightType;
  valid_from: string;
  valid_until: string | null;
  granted_by_user_id: string;
  created_at: string;
  updated_at: string;
};

export type SetTrackRightInput = Pick<
  TrackRight,
  "territory_code" | "rights_type" | "valid_from" | "valid_until"
>;

export type UploadSession = {
  id: string;
  release_track_id: string;
  upload_url: string;
  original_name: string;
  mime_type: string;
  size_bytes: number;
  status: "pending" | "uploading" | "completed" | "failed";
  created_at: string;
  updated_at: string;
};

export type MediaUploadResult = {
  media_asset_id: string;
  content_track_id: string;
  status: "uploaded" | "processing" | "ready" | "failed";
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

export function createCreatorAccount(
  accessToken: string,
  input: CreateCreatorAccountInput,
) {
  return apiRequest<CreatorAccount>("/v1/creators", {
    method: "POST",
    accessToken,
    body: JSON.stringify(input),
  });
}

export function submitCreatorVerification(
  accessToken: string,
  creatorId: string,
) {
  return apiRequest<CreatorAccount>(
    `/v1/creators/${creatorId}/verification-submissions`,
    { method: "POST", accessToken },
  );
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

export function listTrackRights(
  accessToken: string,
  releaseTrackId: string,
  signal?: AbortSignal,
) {
  return apiRequest<TrackRight[]>(
    `/v1/release-tracks/${releaseTrackId}/rights`,
    { accessToken, signal },
  );
}

export function setTrackRight(
  accessToken: string,
  releaseTrackId: string,
  input: SetTrackRightInput,
) {
  return apiRequest<TrackRight>(`/v1/release-tracks/${releaseTrackId}/rights`, {
    method: "PUT",
    accessToken,
    body: JSON.stringify(input),
  });
}

export function deleteTrackRight(accessToken: string, rightId: string) {
  return apiRequest<void>(`/v1/track-rights/${rightId}`, {
    method: "DELETE",
    accessToken,
  });
}

export function createTrackUploadSession(
  accessToken: string,
  releaseTrackId: string,
  input: {
    idempotency_key: string;
    original_name: string;
    mime_type: string;
    size_bytes: number;
  },
) {
  return apiRequest<UploadSession>(
    `/v1/release-tracks/${releaseTrackId}/uploads`,
    {
      method: "POST",
      accessToken,
      body: JSON.stringify(input),
    },
  );
}

export function uploadAudioFile(
  accessToken: string,
  uploadUrl: string,
  file: File,
  mimeType: string,
  onProgress: (percentage: number) => void,
) {
  if (!uploadUrl.startsWith("/") || uploadUrl.startsWith("//")) {
    return Promise.reject(new Error("Upload URL must be root-relative"));
  }
  return new Promise<MediaUploadResult>((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open("PUT", `${gatewayUrl}${uploadUrl}`);
    request.responseType = "json";
    request.withCredentials = true;
    request.setRequestHeader("Accept", "application/json");
    request.setRequestHeader("Authorization", `Bearer ${accessToken}`);
    request.setRequestHeader("Content-Type", mimeType);
    request.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable && event.total > 0) {
        onProgress(Math.round((event.loaded / event.total) * 100));
      }
    });
    request.addEventListener("load", () => {    
      if (request.status >= 200 && request.status < 300) {
        const result = request.response as Partial<MediaUploadResult> | null;

        if (
          !result ||
          typeof result.media_asset_id !== "string" ||
          typeof result.content_track_id !== "string" ||
          !["uploaded", "processing", "ready", "failed"].includes(
            result.status || "",
          )
        ) {
          reject(
            new ApiError(
              request.status,
              "invalid_upload_response",
              "Media service returned an invalid upload response",
            ),
          );
          return;
        }

        if (result.status === "failed") {
          reject(
            new ApiError(
              request.status,
              "upload_failed",
              "Audio processing failed",
            ),
          );
          return;
        }

        onProgress(100);
        resolve(result as MediaUploadResult);
        return;
      }
    });
    request.addEventListener("error", () =>
      reject(new ApiError(0, "upload_failed", "Audio upload failed")),
    );
    request.send(file);
  });
}
