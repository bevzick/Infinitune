import { ApiError, apiRequest, gatewayUrl } from "../../shared/api/client";

export type CoverUploadSession = {
  id: string;
  release_id: string;
  upload_url: string;
  original_name: string;
  mime_type: string;
  size_bytes: number;
  status: "pending" | "uploading" | "completed" | "failed";
};

export type CoverUploadResult = {
  release_id: string;
  status: "completed";
};

export type CreateCoverUploadInput = {
  idempotency_key: string;
  original_name: string;
  mime_type: string;
  size_bytes: number;
};

export function createCoverUploadSession(
  accessToken: string,
  releaseId: string,
  input: CreateCoverUploadInput,
) {
  return apiRequest<CoverUploadSession>(
    `/v1/releases/${releaseId}/cover-uploads`,
    { method: "POST", accessToken, body: JSON.stringify(input) },
  );
}

/** Sends the original file to the Media Service, not a multipart form. */
export function uploadCoverFile(
  accessToken: string,
  session: CoverUploadSession,
  file: File,
  onProgress: (percentage: number) => void,
) {
  if (
    !/^\/v1\/media\/cover-uploads\/[0-9a-f-]{36}$/i.test(session.upload_url) ||
    session.upload_url !== `/v1/media/cover-uploads/${session.id}`
  ) {
    return Promise.reject(new Error("Invalid cover upload URL"));
  }

  return new Promise<CoverUploadResult>((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open("PUT", `${gatewayUrl}${session.upload_url}`);
    request.responseType = "json";
    request.withCredentials = true;
    request.setRequestHeader("Accept", "application/json");
    request.setRequestHeader("Authorization", `Bearer ${accessToken}`);
    request.setRequestHeader("Content-Type", session.mime_type);
    request.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable && event.total > 0) {
        onProgress(Math.round((event.loaded / event.total) * 100));
      }
    });
    request.addEventListener("load", () => {
      if (request.status < 200 || request.status >= 300) {
        const payload = request.response as {
          error?: { code?: string; message?: string };
        } | null;
        reject(
          new ApiError(
            request.status,
            payload?.error?.code || "cover_upload_failed",
            payload?.error?.message ||
              `Cover upload failed (${request.status})`,
          ),
        );
        return;
      }
      const result = request.response as Partial<CoverUploadResult> | null;
      if (
        !result ||
        result.release_id !== session.release_id ||
        result.status !== "completed"
      ) {
        reject(
          new ApiError(
            request.status,
            "invalid_cover_upload_response",
            "Media service returned an invalid cover upload response",
          ),
        );
        return;
      }
      onProgress(100);
      resolve(result as CoverUploadResult);
    });
    request.addEventListener("error", () =>
      reject(new ApiError(0, "cover_upload_failed", "Cover upload failed")),
    );
    request.addEventListener("abort", () =>
      reject(new ApiError(0, "cover_upload_aborted", "Cover upload cancelled")),
    );
    request.send(file);
  });
}
