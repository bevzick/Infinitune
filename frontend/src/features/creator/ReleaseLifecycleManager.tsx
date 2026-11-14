import { useEffect, useState } from "react";
import { ApiError } from "../../shared/api/client";
import {
  archiveRelease,
  getReleaseReadiness,
  publishReleaseWithRetry,
  type ReleaseReadiness,
  submitRelease,
  withdrawRelease,
} from "./creatorLifecycleApi";
import type { CreatorRelease, ReleaseStatus } from "./creatorReleaseApi";

type Action = "submit" | "withdraw" | "publish" | "archive";

const submitStatuses = new Set<ReleaseStatus>([
  "draft",
  "rejected",
  "changes_requested",
]);

function labelForStatus(status: ReleaseStatus) {
  return status.replaceAll("_", " ");
}

function formatTimestamp(value?: string | null) {
  if (!value) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof Error ? reason.message : fallback;
}

export function ReleaseLifecycleManager({
  accessToken,
  release,
  onUpdated,
}: {
  accessToken: string;
  release: CreatorRelease;
  onUpdated: () => Promise<CreatorRelease>;
}) {
  const canSubmit = submitStatuses.has(release.status);
  const [readiness, setReadiness] = useState<ReleaseReadiness | null>(null);
  const [checking, setChecking] = useState(canSubmit);
  const [readinessError, setReadinessError] = useState("");
  const [pending, setPending] = useState<Action | "refresh" | "">("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [confirmArchive, setConfirmArchive] = useState(false);

  useEffect(() => {
    if (!submitStatuses.has(release.status)) {
      setReadiness(null);
      setReadinessError("");
      setChecking(false);
      return;
    }
    const controller = new AbortController();
    setChecking(true);
    setReadiness(null);
    setReadinessError("");
    void getReleaseReadiness(accessToken, release.id, controller.signal)
      .then((report) => {
        if (!controller.signal.aborted) setReadiness(report);
      })
      .catch((reason) => {
        if (!controller.signal.aborted) {
          setReadinessError(errorMessage(reason, "Could not check readiness"));
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setChecking(false);
      });
    return () => controller.abort();
  }, [accessToken, release.id, release.status]);

  async function refresh() {
    setPending("refresh");
    setError("");
    setNotice("");
    setReadiness(null);
    setReadinessError("");
    if (canSubmit) setChecking(true);
    try {
      // Re-read the canonical status before checking submission readiness.
      // A moderator may have transitioned this release since the last fetch.
      const updated = await onUpdated();
      if (submitStatuses.has(updated.status)) {
        const report = await getReleaseReadiness(accessToken, release.id);
        setReadiness(report);
      }
    } catch (reason) {
      setReadinessError(errorMessage(reason, "Could not refresh release"));
    } finally {
      setChecking(false);
      setPending("");
    }
  }

  async function run(action: Action) {
    if (pending) return;
    setPending(action);
    setError("");
    setNotice("");
    setConfirmArchive(false);
    try {
      switch (action) {
        case "submit":
          await submitRelease(accessToken, release.id);
          break;
        case "withdraw":
          await withdrawRelease(accessToken, release.id);
          break;
        case "publish":
          await publishReleaseWithRetry(accessToken, release.id);
          break;
        case "archive":
          await archiveRelease(accessToken, release.id);
          break;
      }
      await onUpdated();
      setNotice(
        action === "submit"
          ? "Submitted for moderation. Approval is completed by a different account."
          : action === "withdraw"
            ? "Submission withdrawn; the release is back in draft."
            : action === "publish"
              ? "Publication accepted. Catalog visibility may take a moment; refresh to check."
              : "Release archived.",
      );
    } catch (reason) {
      if (
        action === "publish" &&
        reason instanceof ApiError &&
        reason.status === 409 &&
        reason.code === "release_not_ready"
      ) {
        setError(
          "Catalog synchronization is pending. Try Publish again shortly.",
        );
      } else {
        setError(errorMessage(reason, `Could not ${action} this release`));
      }
    } finally {
      setPending("");
    }
  }

  const busy = Boolean(pending);
  const availableActions: Action[] = [];
  if (canSubmit) availableActions.push("submit");
  if (release.status === "pending_review" || release.status === "approved") {
    availableActions.push("withdraw");
  }
  if (release.status === "approved") availableActions.push("publish");
  if (release.status !== "archived") availableActions.push("archive");

  return (
    <section
      className="creator-release-lifecycle"
      aria-label="Release lifecycle"
    >
      <header>
        <div>
          <span className="eyebrow">Publishing</span>
          <h3>Release lifecycle</h3>
          <p>
            Current status: <strong>{labelForStatus(release.status)}</strong>
          </p>
        </div>
        <button type="button" disabled={busy} onClick={() => void refresh()}>
          {pending === "refresh" ? "Refreshing…" : "Refresh status"}
        </button>
      </header>

      {canSubmit ? (
        <section
          className="creator-readiness"
          aria-label="Submission readiness"
        >
          <h4>Submission checklist</h4>
          {checking ? (
            <p role="status">Checking submission readiness…</p>
          ) : null}
          {readinessError ? (
            <p role="alert" className="creator-release-error">
              {readinessError}
            </p>
          ) : null}
          {readiness && !checking ? (
            readiness.ready_for_submission ? (
              <p role="status">All submission requirements are satisfied.</p>
            ) : (
              <ul>
                {readiness.issues.map((issue, index) => (
                  <li key={`${issue.code}:${issue.track_id || index}`}>
                    <strong>{issue.code.replaceAll("_", " ")}</strong>:{" "}
                    {issue.message}
                    {issue.track_id ? (
                      <span>
                        {" "}
                        —{" "}
                        {release.tracks.find(
                          (track) => track.id === issue.track_id,
                        )?.title || "Track"}
                      </span>
                    ) : null}
                  </li>
                ))}
              </ul>
            )
          ) : null}
        </section>
      ) : (
        <p className="creator-release-lifecycle-description">
          {release.status === "pending_review"
            ? "Awaiting an independent moderator review."
            : release.status === "approved"
              ? "Approved. Publish when the Catalog service is ready."
              : release.status === "published"
                ? "Publication was accepted; availability in Catalog can be asynchronous."
                : release.status === "archived"
                  ? "This release is archived."
                  : "Refresh to check its latest processing status."}
        </p>
      )}

      <dl className="creator-release-timestamps">
        {(
          [
            ["Submitted", release.submitted_at],
            ["Reviewed", release.reviewed_at],
            ["Published", release.published_at],
            ["Archived", release.archived_at],
          ] as const
        ).map(([label, value]) =>
          value ? (
            <div key={label}>
              <dt>{label}</dt>
              <dd>{formatTimestamp(value)}</dd>
            </div>
          ) : null,
        )}
      </dl>

      <div className="creator-release-lifecycle-actions">
        {availableActions.map((action) =>
          action === "archive" ? (
            <div className="creator-release-archive-actions" key={action}>
              {confirmArchive ? (
                <>
                  <span className="creator-archive-warning">
                    Archiving removes this release from active publishing.
                  </span>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => void run("archive")}
                  >
                    Confirm archive
                  </button>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => setConfirmArchive(false)}
                  >
                    Cancel archive
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => setConfirmArchive(true)}
                >
                  Archive
                </button>
              )}
            </div>
          ) : (
            <button
              type="button"
              key={action}
              disabled={
                busy ||
                (action === "submit" &&
                  (!readiness?.ready_for_submission || checking))
              }
              onClick={() => void run(action)}
            >
              {pending === action
                ? `${action[0].toUpperCase()}${action.slice(1)}ing…`
                : action === "submit"
                  ? "Submit for review"
                  : action === "withdraw"
                    ? "Withdraw submission"
                    : "Publish release"}
            </button>
          ),
        )}
      </div>
      {error ? (
        <p role="alert" className="creator-release-error">
          {error}
        </p>
      ) : null}
      {notice ? (
        <p role="status" className="creator-release-success">
          {notice}
        </p>
      ) : null}
    </section>
  );
}
