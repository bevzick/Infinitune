import { useCallback, useEffect, useMemo, useState } from "react";

import {
  type AdminUser,
  assignUserRole,
  type CommentAuditEvent,
  type CommentReport,
  type CreatorReviewItem,
  listCommentAudit,
  listCommentReports,
  listCreatorsForModeration,
  listReleaseAudit,
  listReleasesForModeration,
  listUsers,
  moderateCommentReport,
  type Page,
  type PageMetadata,
  type ReleaseAuditEvent,
  type ReleaseReviewItem,
  removeUserRole,
  reviewCreator,
  reviewRelease,
  updateUserStatus,
} from "../features/admin/adminApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";

type Section = "users" | "content" | "reports" | "activity";
type LoadState = "loading" | "ready" | "error";
type UserStatus = AdminUser["status"] | "all";
type CreatorStatus = CreatorReviewItem["verification_status"];
type ReleaseStatus =
  | "pending_review"
  | "approved"
  | "changes_requested"
  | "rejected"
  | "published";
type ReportStatus = CommentReport["report"]["status"];

const pageSize = 10;

export function AdminPage() {
  const { accessToken, identity } = useSession();
  const [section, setSection] = useState<Section>("users");
  const [users, setUsers] = useState<Page<AdminUser>>(emptyPage);
  const [creators, setCreators] = useState<Page<CreatorReviewItem>>(emptyPage);
  const [releases, setReleases] = useState<Page<ReleaseReviewItem>>(emptyPage);
  const [reports, setReports] = useState<Page<CommentReport>>(emptyPage);
  const [releaseAudit, setReleaseAudit] =
    useState<Page<ReleaseAuditEvent>>(emptyPage);
  const [commentAudit, setCommentAudit] =
    useState<Page<CommentAuditEvent>>(emptyPage);
  const [userStatus, setUserStatus] = useState<UserStatus>("all");
  const [userOffset, setUserOffset] = useState(0);
  const [creatorStatus, setCreatorStatus] = useState<CreatorStatus>("pending");
  const [creatorOffset, setCreatorOffset] = useState(0);
  const [releaseStatus, setReleaseStatus] =
    useState<ReleaseStatus>("pending_review");
  const [releaseOffset, setReleaseOffset] = useState(0);
  const [reportStatus, setReportStatus] = useState<ReportStatus>("pending");
  const [reportOffset, setReportOffset] = useState(0);
  const [releaseAuditOffset, setReleaseAuditOffset] = useState(0);
  const [commentAuditOffset, setCommentAuditOffset] = useState(0);
  const [state, setState] = useState<LoadState>("loading");
  const [error, setError] = useState("");
  const [busyId, setBusyId] = useState("");

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!accessToken) return;
      setState("loading");
      setError("");
      try {
        const [
          userPage,
          creatorPage,
          releasePage,
          reportPage,
          releaseAuditPage,
          commentAuditPage,
        ] = await Promise.all([
          listUsers(
            accessToken,
            { status: userStatus, limit: pageSize, offset: userOffset },
            signal,
          ),
          listCreatorsForModeration(
            accessToken,
            {
              status: creatorStatus,
              limit: pageSize,
              offset: creatorOffset,
            },
            signal,
          ),
          listReleasesForModeration(
            accessToken,
            {
              status: releaseStatus,
              limit: pageSize,
              offset: releaseOffset,
            },
            signal,
          ),
          listCommentReports(
            accessToken,
            {
              status: reportStatus,
              limit: pageSize,
              offset: reportOffset,
            },
            signal,
          ),
          listReleaseAudit(
            accessToken,
            { limit: pageSize, offset: releaseAuditOffset },
            signal,
          ),
          listCommentAudit(
            accessToken,
            { limit: pageSize, offset: commentAuditOffset },
            signal,
          ),
        ]);
        setUsers(userPage);
        setCreators(creatorPage);
        setReleases(releasePage);
        setReports(reportPage);
        setReleaseAudit(releaseAuditPage);
        setCommentAudit(commentAuditPage);
        setState("ready");
      } catch (reason) {
        if (signal?.aborted) return;
        setError(errorMessage(reason, "Could not load administration data"));
        setState("error");
      }
    },
    [
      accessToken,
      creatorOffset,
      creatorStatus,
      commentAuditOffset,
      releaseOffset,
      releaseStatus,
      releaseAuditOffset,
      reportOffset,
      reportStatus,
      userOffset,
      userStatus,
    ],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  async function mutate(id: string, action: () => Promise<unknown>) {
    setBusyId(id);
    setError("");
    try {
      await action();
      await load();
    } catch (reason) {
      setError(errorMessage(reason, "The administration action failed"));
    } finally {
      setBusyId("");
    }
  }

  const counts = useMemo(
    () => ({
      users: users.page.total,
      content: creators.page.total + releases.page.total,
      reports: reports.page.total,
      activity: releaseAudit.page.total + commentAudit.page.total,
    }),
    [users, creators, releases, reports, releaseAudit, commentAudit],
  );

  return (
    <section className="admin-page">
      <header className="admin-hero">
        <div>
          <span className="hero-kicker">Platform operations</span>
          <h1>Administration</h1>
          <p>
            Manage access, review creator submissions, and resolve community
            reports from one protected workspace.
          </p>
        </div>
        <div className="admin-hero-mark" aria-hidden="true">
          <span>∞</span>
          <small>ADMIN</small>
        </div>
      </header>

      <section className="admin-summary" aria-label="Administration summary">
        <SummaryCard label="Users in selected status" value={counts.users} />
        <SummaryCard label="Content results" value={counts.content} />
        <SummaryCard label="Report results" value={counts.reports} />
      </section>

      <nav className="admin-tabs" aria-label="Administration sections">
        <AdminTab
          active={section === "users"}
          label="Users & roles"
          count={counts.users}
          onClick={() => setSection("users")}
        />
        <AdminTab
          active={section === "content"}
          label="Content review"
          count={counts.content}
          onClick={() => setSection("content")}
        />
        <AdminTab
          active={section === "reports"}
          label="Comment reports"
          count={counts.reports}
          onClick={() => setSection("reports")}
        />
        <AdminTab
          active={section === "activity"}
          label="Activity log"
          count={counts.activity}
          onClick={() => setSection("activity")}
        />
      </nav>

      {error ? (
        <div className="admin-alert" role="alert">
          <span>{error}</span>
          {state === "error" ? (
            <button type="button" onClick={() => void load()}>
              Try again
            </button>
          ) : null}
        </div>
      ) : null}

      {state === "loading" ? (
        <AdminLoading />
      ) : state === "ready" ? (
        <div className="admin-workspace">
          {section === "users" ? (
            <UsersSection
              page={users}
              status={userStatus}
              onStatusChange={(next) => {
                setUserStatus(next);
                setUserOffset(0);
              }}
              onPageChange={setUserOffset}
              currentUserId={identity?.userId || ""}
              busyId={busyId}
              mutate={mutate}
              accessToken={accessToken || ""}
            />
          ) : null}
          {section === "content" ? (
            <ContentSection
              creators={creators}
              creatorStatus={creatorStatus}
              onCreatorStatusChange={(next) => {
                setCreatorStatus(next);
                setCreatorOffset(0);
              }}
              onCreatorPageChange={setCreatorOffset}
              releases={releases}
              releaseStatus={releaseStatus}
              onReleaseStatusChange={(next) => {
                setReleaseStatus(next);
                setReleaseOffset(0);
              }}
              onReleasePageChange={setReleaseOffset}
              busyId={busyId}
              mutate={mutate}
              accessToken={accessToken || ""}
            />
          ) : null}
          {section === "reports" ? (
            <ReportsSection
              page={reports}
              status={reportStatus}
              onStatusChange={(next) => {
                setReportStatus(next);
                setReportOffset(0);
              }}
              onPageChange={setReportOffset}
              busyId={busyId}
              mutate={mutate}
              accessToken={accessToken || ""}
            />
          ) : null}
          {section === "activity" ? (
            <ActivitySection
              releaseEvents={releaseAudit}
              commentEvents={commentAudit}
              onReleasePageChange={setReleaseAuditOffset}
              onCommentPageChange={setCommentAuditOffset}
            />
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

function SummaryCard({ label, value }: { label: string; value: number }) {
  return (
    <article className="admin-summary-card">
      <strong>{value}</strong>
      <span>{label}</span>
    </article>
  );
}

function AdminTab({
  active,
  label,
  count,
  onClick,
}: {
  active: boolean;
  label: string;
  count: number;
  onClick(): void;
}) {
  return (
    <button
      className={active ? "admin-tab admin-tab-active" : "admin-tab"}
      type="button"
      aria-pressed={active}
      onClick={onClick}
    >
      {label}
      <span>{count}</span>
    </button>
  );
}

type Mutation = (id: string, action: () => Promise<unknown>) => Promise<void>;

function UsersSection({
  page,
  status,
  onStatusChange,
  onPageChange,
  currentUserId,
  busyId,
  mutate,
  accessToken,
}: {
  page: Page<AdminUser>;
  status: UserStatus;
  onStatusChange(status: UserStatus): void;
  onPageChange(offset: number): void;
  currentUserId: string;
  busyId: string;
  mutate: Mutation;
  accessToken: string;
}) {
  const [query, setQuery] = useState("");
  const normalized = query.trim().toLowerCase();
  const visible = page.items.filter(
    (user) =>
      !normalized ||
      user.email.toLowerCase().includes(normalized) ||
      user.id.toLowerCase().includes(normalized),
  );

  return (
    <section aria-labelledby="admin-users-title">
      <div className="admin-section-heading">
        <div>
          <span className="eyebrow">Identity and access</span>
          <h2 id="admin-users-title">Users & roles</h2>
        </div>
        <div className="admin-filters">
          <label>
            <span className="visually-hidden">Search users</span>
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search email or user ID"
            />
          </label>
          <label>
            <span className="visually-hidden">Filter by status</span>
            <select
              value={status}
              onChange={(event) =>
                onStatusChange(event.target.value as UserStatus)
              }
            >
              <option value="all">All statuses</option>
              <option value="active">Active</option>
              <option value="blocked">Blocked</option>
              <option value="deleted">Deleted</option>
            </select>
          </label>
        </div>
      </div>
      {visible.length ? (
        <div className="admin-user-list">
          {visible.map((user) => (
            <article className="admin-user-card" key={user.id}>
              <div className="admin-user-identity">
                <span className="admin-avatar" aria-hidden="true">
                  {user.email.slice(0, 1).toUpperCase()}
                </span>
                <div>
                  <strong>{user.email}</strong>
                  <small>{user.id}</small>
                  <span className={`admin-status admin-status-${user.status}`}>
                    {user.status}
                  </span>
                </div>
              </div>
              <fieldset
                className="admin-role-list"
                aria-label={`Roles for ${user.email}`}
              >
                {(["admin", "moderator", "creator"] as const).map((role) => {
                  const assigned = user.roles.includes(role);
                  const selfAdmin =
                    user.id === currentUserId && role === "admin";
                  return (
                    <button
                      key={role}
                      className={
                        assigned ? "admin-role admin-role-active" : "admin-role"
                      }
                      type="button"
                      aria-pressed={assigned}
                      disabled={
                        busyId === user.id ||
                        selfAdmin ||
                        user.status === "deleted"
                      }
                      title={
                        selfAdmin
                          ? "You cannot remove your own admin role"
                          : undefined
                      }
                      onClick={() =>
                        void mutate(user.id, () =>
                          assigned
                            ? removeUserRole(accessToken, user.id, role)
                            : assignUserRole(accessToken, user.id, role),
                        )
                      }
                    >
                      {role}
                    </button>
                  );
                })}
                {user.roles.includes("listener") ? (
                  <span className="admin-role admin-role-system">listener</span>
                ) : null}
              </fieldset>
              <button
                className={
                  user.status === "blocked"
                    ? "admin-action"
                    : "admin-action admin-action-danger"
                }
                type="button"
                disabled={
                  busyId === user.id ||
                  user.id === currentUserId ||
                  user.status === "deleted"
                }
                onClick={() =>
                  void mutate(user.id, () =>
                    updateUserStatus(
                      accessToken,
                      user.id,
                      user.status === "blocked" ? "active" : "blocked",
                    ),
                  )
                }
              >
                {busyId === user.id
                  ? "Saving…"
                  : user.status === "blocked"
                    ? "Activate"
                    : "Block"}
              </button>
            </article>
          ))}
        </div>
      ) : (
        <AdminEmpty message="No users match the selected filters." />
      )}
      <Pagination page={page.page} label="users" onPageChange={onPageChange} />
    </section>
  );
}

function ContentSection({
  creators,
  creatorStatus,
  onCreatorStatusChange,
  onCreatorPageChange,
  releases,
  releaseStatus,
  onReleaseStatusChange,
  onReleasePageChange,
  busyId,
  mutate,
  accessToken,
}: {
  creators: Page<CreatorReviewItem>;
  creatorStatus: CreatorStatus;
  onCreatorStatusChange(status: CreatorStatus): void;
  onCreatorPageChange(offset: number): void;
  releases: Page<ReleaseReviewItem>;
  releaseStatus: ReleaseStatus;
  onReleaseStatusChange(status: ReleaseStatus): void;
  onReleasePageChange(offset: number): void;
  busyId: string;
  mutate: Mutation;
  accessToken: string;
}) {
  const [notes, setNotes] = useState<Record<string, string>>({});
  return (
    <section aria-labelledby="admin-content-title">
      <div className="admin-section-heading">
        <div>
          <span className="eyebrow">Publishing workflow</span>
          <h2 id="admin-content-title">Content review</h2>
        </div>
      </div>
      <div className="admin-review-columns">
        <div className="admin-review-panel">
          <header>
            <h3>Creator verification</h3>
            <label className="admin-history-filter">
              <span className="visually-hidden">
                Creator verification status
              </span>
              <select
                value={creatorStatus}
                onChange={(event) =>
                  onCreatorStatusChange(event.target.value as CreatorStatus)
                }
              >
                <option value="pending">Pending</option>
                <option value="verified">Verified</option>
                <option value="rejected">Rejected</option>
              </select>
            </label>
          </header>
          {creators.items.length ? (
            creators.items.map((creator) => (
              <article className="admin-review-item" key={creator.id}>
                <div>
                  <strong>{creator.display_name}</strong>
                  <small>
                    {creator.type} · submitted {formatDate(creator.updated_at)}
                  </small>
                  <code>{creator.id}</code>
                </div>
                {creator.verification_status === "pending" ? (
                  <div className="admin-inline-actions">
                    <button
                      type="button"
                      disabled={busyId === creator.id}
                      onClick={() =>
                        void mutate(creator.id, () =>
                          reviewCreator(accessToken, creator.id, "verified"),
                        )
                      }
                    >
                      Verify
                    </button>
                    <button
                      className="admin-action-danger"
                      type="button"
                      disabled={busyId === creator.id}
                      onClick={() =>
                        void mutate(creator.id, () =>
                          reviewCreator(accessToken, creator.id, "rejected"),
                        )
                      }
                    >
                      Reject
                    </button>
                  </div>
                ) : (
                  <StatusBadge value={creator.verification_status} />
                )}
              </article>
            ))
          ) : (
            <AdminEmpty
              message={`No ${humanize(creatorStatus)} creators found.`}
            />
          )}
          <Pagination
            page={creators.page}
            label="creators"
            onPageChange={onCreatorPageChange}
          />
        </div>
        <div className="admin-review-panel">
          <header>
            <h3>Release moderation</h3>
            <label className="admin-history-filter">
              <span className="visually-hidden">Release moderation status</span>
              <select
                value={releaseStatus}
                onChange={(event) =>
                  onReleaseStatusChange(event.target.value as ReleaseStatus)
                }
              >
                <option value="pending_review">Pending review</option>
                <option value="approved">Approved</option>
                <option value="changes_requested">Changes requested</option>
                <option value="rejected">Rejected</option>
                <option value="published">Published</option>
              </select>
            </label>
          </header>
          {releases.items.length ? (
            releases.items.map((release) => (
              <article
                className="admin-review-item admin-release-review"
                key={release.id}
              >
                <div>
                  <strong>{release.title}</strong>
                  <small>
                    {release.release_type} · release date{" "}
                    {formatDate(release.release_date)}
                  </small>
                  <code>{release.id}</code>
                </div>
                {release.status === "pending_review" ? (
                  <>
                    <label>
                      <span className="visually-hidden">
                        Review note for {release.title}
                      </span>
                      <textarea
                        value={notes[release.id] || ""}
                        onChange={(event) =>
                          setNotes((current) => ({
                            ...current,
                            [release.id]: event.target.value,
                          }))
                        }
                        maxLength={1000}
                        placeholder="Review note (required for rejection or changes)"
                      />
                    </label>
                    <div className="admin-inline-actions">
                      <button
                        type="button"
                        disabled={busyId === release.id}
                        onClick={() =>
                          void mutate(release.id, () =>
                            reviewRelease(accessToken, release.id, "approved"),
                          )
                        }
                      >
                        Approve
                      </button>
                      <button
                        type="button"
                        disabled={
                          busyId === release.id || !notes[release.id]?.trim()
                        }
                        onClick={() =>
                          void mutate(release.id, () =>
                            reviewRelease(
                              accessToken,
                              release.id,
                              "changes_requested",
                              notes[release.id].trim(),
                            ),
                          )
                        }
                      >
                        Request changes
                      </button>
                      <button
                        className="admin-action-danger"
                        type="button"
                        disabled={
                          busyId === release.id || !notes[release.id]?.trim()
                        }
                        onClick={() =>
                          void mutate(release.id, () =>
                            reviewRelease(
                              accessToken,
                              release.id,
                              "rejected",
                              notes[release.id].trim(),
                            ),
                          )
                        }
                      >
                        Reject
                      </button>
                    </div>
                  </>
                ) : (
                  <StatusBadge value={release.status} />
                )}
              </article>
            ))
          ) : (
            <AdminEmpty
              message={`No ${humanize(releaseStatus)} releases found.`}
            />
          )}
          <Pagination
            page={releases.page}
            label="releases"
            onPageChange={onReleasePageChange}
          />
        </div>
      </div>
    </section>
  );
}

function ReportsSection({
  page,
  status,
  onStatusChange,
  onPageChange,
  busyId,
  mutate,
  accessToken,
}: {
  page: Page<CommentReport>;
  status: ReportStatus;
  onStatusChange(status: ReportStatus): void;
  onPageChange(offset: number): void;
  busyId: string;
  mutate: Mutation;
  accessToken: string;
}) {
  const [notes, setNotes] = useState<Record<string, string>>({});
  return (
    <section aria-labelledby="admin-reports-title">
      <div className="admin-section-heading">
        <div>
          <span className="eyebrow">Community safety</span>
          <h2 id="admin-reports-title">Comment report history</h2>
        </div>
        <label className="admin-history-filter">
          <span className="visually-hidden">Comment report status</span>
          <select
            value={status}
            onChange={(event) =>
              onStatusChange(event.target.value as ReportStatus)
            }
          >
            <option value="pending">Pending</option>
            <option value="reviewing">Reviewing</option>
            <option value="resolved">Resolved</option>
            <option value="rejected">Rejected</option>
          </select>
        </label>
      </div>
      {page.items.length ? (
        <div className="admin-report-list">
          {page.items.map(({ report, comment }) => (
            <article className="admin-report-card" key={report.id}>
              <div className="admin-report-meta">
                <span className="admin-reason">{humanize(report.reason)}</span>
                <span>
                  <StatusBadge value={report.status} />
                  <small>{formatDate(report.created_at)}</small>
                </span>
              </div>
              <blockquote>{comment.content}</blockquote>
              {report.description ? <p>{report.description}</p> : null}
              <small>Comment author: {comment.user_id}</small>
              {report.status === "pending" || report.status === "reviewing" ? (
                <>
                  <label>
                    <span className="visually-hidden">
                      Moderation note for report {report.id}
                    </span>
                    <textarea
                      value={notes[report.id] || ""}
                      onChange={(event) =>
                        setNotes((current) => ({
                          ...current,
                          [report.id]: event.target.value,
                        }))
                      }
                      maxLength={2000}
                      placeholder="Required moderation note"
                    />
                  </label>
                  <div className="admin-inline-actions">
                    {report.status === "pending" ? (
                      <button
                        type="button"
                        disabled={
                          busyId === report.id || !notes[report.id]?.trim()
                        }
                        onClick={() =>
                          void mutate(report.id, () =>
                            moderateCommentReport(
                              accessToken,
                              report.id,
                              "start_review",
                              notes[report.id].trim(),
                            ),
                          )
                        }
                      >
                        Start review
                      </button>
                    ) : null}
                    <button
                      className="admin-action-danger"
                      type="button"
                      disabled={
                        busyId === report.id || !notes[report.id]?.trim()
                      }
                      onClick={() =>
                        void mutate(report.id, () =>
                          moderateCommentReport(
                            accessToken,
                            report.id,
                            comment.status === "hidden"
                              ? "restore_comment"
                              : "hide_comment",
                            notes[report.id].trim(),
                          ),
                        )
                      }
                    >
                      {comment.status === "hidden"
                        ? "Restore comment"
                        : "Hide comment"}
                    </button>
                    {report.status === "reviewing" ? (
                      <button
                        type="button"
                        disabled={
                          busyId === report.id || !notes[report.id]?.trim()
                        }
                        onClick={() =>
                          void mutate(report.id, () =>
                            moderateCommentReport(
                              accessToken,
                              report.id,
                              "resolve_report",
                              notes[report.id].trim(),
                            ),
                          )
                        }
                      >
                        Resolve
                      </button>
                    ) : null}
                    <button
                      type="button"
                      disabled={
                        busyId === report.id || !notes[report.id]?.trim()
                      }
                      onClick={() =>
                        void mutate(report.id, () =>
                          moderateCommentReport(
                            accessToken,
                            report.id,
                            "reject_report",
                            notes[report.id].trim(),
                          ),
                        )
                      }
                    >
                      Reject report
                    </button>
                  </div>
                </>
              ) : null}
            </article>
          ))}
        </div>
      ) : (
        <AdminEmpty
          message={`There are no ${humanize(status)} comment reports.`}
        />
      )}
      <Pagination
        page={page.page}
        label="comment reports"
        onPageChange={onPageChange}
      />
    </section>
  );
}

function ActivitySection({
  releaseEvents,
  commentEvents,
  onReleasePageChange,
  onCommentPageChange,
}: {
  releaseEvents: Page<ReleaseAuditEvent>;
  commentEvents: Page<CommentAuditEvent>;
  onReleasePageChange(offset: number): void;
  onCommentPageChange(offset: number): void;
}) {
  return (
    <section aria-labelledby="admin-activity-title">
      <div className="admin-section-heading">
        <div>
          <span className="eyebrow">Accountability</span>
          <h2 id="admin-activity-title">Moderation activity log</h2>
        </div>
      </div>
      <div className="admin-audit-columns">
        <section
          className="admin-review-panel"
          aria-labelledby="release-audit-title"
        >
          <header>
            <h3 id="release-audit-title">Release decisions</h3>
            <small>{releaseEvents.page.total} events</small>
          </header>
          {releaseEvents.items.length ? (
            <div className="admin-audit-list">
              {releaseEvents.items.map((event) => (
                <article className="admin-audit-item" key={event.id}>
                  <div className="admin-audit-heading">
                    <StatusBadge value={event.decision} />
                    <time dateTime={event.created_at}>
                      {formatDateTime(event.created_at)}
                    </time>
                  </div>
                  <strong>Release {shortID(event.release_id)}</strong>
                  <small>Moderator: {event.reviewer_user_id}</small>
                  {event.comment ? (
                    <p>{event.comment}</p>
                  ) : (
                    <p>No review note.</p>
                  )}
                </article>
              ))}
            </div>
          ) : (
            <AdminEmpty message="No release decisions have been recorded." />
          )}
          <Pagination
            page={releaseEvents.page}
            label="release activity"
            onPageChange={onReleasePageChange}
          />
        </section>

        <section
          className="admin-review-panel"
          aria-labelledby="comment-audit-title"
        >
          <header>
            <h3 id="comment-audit-title">Comment moderation</h3>
            <small>{commentEvents.page.total} events</small>
          </header>
          {commentEvents.items.length ? (
            <div className="admin-audit-list">
              {commentEvents.items.map((event) => (
                <article className="admin-audit-item" key={event.id}>
                  <div className="admin-audit-heading">
                    <StatusBadge value={event.action} />
                    <time dateTime={event.created_at}>
                      {formatDateTime(event.created_at)}
                    </time>
                  </div>
                  <strong>Report {shortID(event.report_id)}</strong>
                  <small>Moderator: {event.moderator_user_id}</small>
                  <p>{event.note}</p>
                  <small>
                    Report: {humanize(event.previous_report_status)} →{" "}
                    {humanize(event.resulting_report_status)}
                    {event.previous_comment_status !==
                    event.resulting_comment_status
                      ? ` · Comment: ${humanize(event.previous_comment_status)} → ${humanize(event.resulting_comment_status)}`
                      : ""}
                  </small>
                </article>
              ))}
            </div>
          ) : (
            <AdminEmpty message="No comment moderation actions have been recorded." />
          )}
          <Pagination
            page={commentEvents.page}
            label="comment activity"
            onPageChange={onCommentPageChange}
          />
        </section>
      </div>
    </section>
  );
}

function AdminLoading() {
  return (
    <div className="admin-loading" aria-live="polite">
      <span className="loading-pulse" aria-hidden="true" />
      Loading administration workspace…
    </div>
  );
}

function StatusBadge({ value }: { value: string }) {
  return (
    <span className={`admin-status admin-status-${value}`}>
      {humanize(value)}
    </span>
  );
}

function Pagination({
  page,
  label,
  onPageChange,
}: {
  page: PageMetadata;
  label: string;
  onPageChange(offset: number): void;
}) {
  if (page.total <= page.limit || page.limit < 1) return null;
  const start = page.offset + 1;
  const end = Math.min(page.offset + page.limit, page.total);
  return (
    <nav className="admin-pagination" aria-label={`${label} pagination`}>
      <span>
        {start}–{end} of {page.total}
      </span>
      <div>
        <button
          type="button"
          disabled={page.offset === 0}
          onClick={() => onPageChange(Math.max(0, page.offset - page.limit))}
        >
          Previous
        </button>
        <button
          type="button"
          disabled={page.offset + page.limit >= page.total}
          onClick={() => onPageChange(page.offset + page.limit)}
        >
          Next
        </button>
      </div>
    </nav>
  );
}

function AdminEmpty({ message }: { message: string }) {
  return <p className="admin-empty">{message}</p>;
}

function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof ApiError ? reason.message : fallback;
}

function humanize(value: string) {
  return value.replaceAll("_", " ");
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(new Date(value));
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

function shortID(value: string) {
  return value.length > 12 ? `${value.slice(0, 8)}…` : value;
}

function emptyPage<T>(): Page<T> {
  return {
    items: [],
    page: { limit: pageSize, offset: 0, total: 0 },
  };
}
