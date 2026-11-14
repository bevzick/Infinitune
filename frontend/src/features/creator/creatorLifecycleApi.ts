import { ApiError, apiRequest } from "../../shared/api/client";
import type { ReleaseStatus } from "./creatorReleaseApi";

export type ReadinessIssue = {
  code: string;
  message: string;
  track_id?: string;
};

export type ReleaseReadiness = {
  release_id: string;
  status: ReleaseStatus;
  ready_for_submission: boolean;
  issues: ReadinessIssue[];
};

export type ReleaseTransitionResult = {
  release_id?: string;
  id?: string;
  status: ReleaseStatus;
  submitted_at?: string | null;
  published_at?: string | null;
  updated_at: string;
};

export function getReleaseReadiness(
  accessToken: string,
  releaseId: string,
  signal?: AbortSignal,
) {
  return apiRequest<ReleaseReadiness>(`/v1/releases/${releaseId}/readiness`, {
    accessToken,
    signal,
  });
}

export function submitRelease(accessToken: string, releaseId: string) {
  return apiRequest<ReleaseTransitionResult>(
    `/v1/releases/${releaseId}/submissions`,
    { method: "POST", accessToken },
  );
}

export function withdrawRelease(accessToken: string, releaseId: string) {
  return apiRequest<ReleaseTransitionResult>(
    `/v1/releases/${releaseId}/withdrawals`,
    { method: "POST", accessToken },
  );
}

export function archiveRelease(accessToken: string, releaseId: string) {
  return apiRequest<ReleaseTransitionResult>(`/v1/releases/${releaseId}`, {
    method: "DELETE",
    accessToken,
  });
}

export function publishRelease(accessToken: string, releaseId: string) {
  return apiRequest<ReleaseTransitionResult>(
    `/v1/releases/${releaseId}/publications`,
    { method: "POST", accessToken },
  );
}

const publicationRetryDelays = [750, 1500];

// Catalog observes approval asynchronously. Retry only the documented
// transient 409; other conflicts and failures must be shown immediately.
export async function publishReleaseWithRetry(
  accessToken: string,
  releaseId: string,
  delay: (ms: number) => Promise<void> = (ms) =>
    new Promise((resolve) => setTimeout(resolve, ms)),
) {
  for (let attempt = 0; ; attempt++) {
    try {
      return await publishRelease(accessToken, releaseId);
    } catch (reason) {
      if (
        !(reason instanceof ApiError) ||
        reason.status !== 409 ||
        reason.code !== "release_not_ready" ||
        attempt >= publicationRetryDelays.length
      ) {
        throw reason;
      }
      await delay(publicationRetryDelays[attempt]);
    }
  }
}
