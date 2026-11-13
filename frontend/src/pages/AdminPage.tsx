import { useCallback, useEffect, useMemo, useState } from "react";

import {
  type AdminUser,
  assignUserRole,
  type CommentReport,
  type CreatorReviewItem,
  listCommentReports,
  listPendingCreators,
  listPendingReleases,
  listUsers,
  moderateCommentReport,
  type ReleaseReviewItem,
  removeUserRole,
  reviewCreator,
  reviewRelease,
  updateUserStatus,
} from "../features/admin/adminApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";

type Section = "users" | "content" | "reports";
type LoadState = "loading" | "ready" | "error";

export function AdminPage() {
  const { accessToken, identity } = useSession();
  const [section, setSection] = useState<Section>("users");
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [creators, setCreators] = useState<CreatorReviewItem[]>([]);
  const [releases, setReleases] = useState<ReleaseReviewItem[]>([]);
  const [reports, setReports] = useState<CommentReport[]>([]);
  const [state, setState] = useState<LoadState>("loading");
  const [error, setError] = useState("");
  const [busyId, setBusyId] = useState("");

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!accessToken) return;
      setState("loading");
      setError("");
      try {
        const [userPage, creatorPage, releasePage, reportPage] =
          await Promise.all([
            listUsers(accessToken, "all", signal),
            listPendingCreators(accessToken, signal),
            listPendingReleases(accessToken, signal),
            listCommentReports(accessToken, "pending", signal),
          ]);
        setUsers(userPage.items);
        setCreators(creatorPage.items);
        setReleases(releasePage.items);
        setReports(reportPage.items);
        setState("ready");
      } catch (reason) {
        if (signal?.aborted) return;
        setError(errorMessage(reason, "Could not load administration data"));
        setState("error");
      }
    },
    [accessToken],
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
      users: users.length,
      content: creators.length + releases.length,
      reports: reports.length,
    }),
    [users, creators, releases, reports],
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
        <SummaryCard label="Users loaded" value={counts.users} />
        <SummaryCard label="Content awaiting review" value={counts.content} />
        <SummaryCard label="Open reports" value={counts.reports} />
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
              users={users}
              currentUserId={identity?.userId || ""}
              busyId={busyId}
              mutate={mutate}
              accessToken={accessToken || ""}
            />
          ) : null}
          {section === "content" ? (
            <ContentSection
              creators={creators}
              releases={releases}
              busyId={busyId}
              mutate={mutate}
              accessToken={accessToken || ""}
            />
          ) : null}
          {section === "reports" ? (
            <ReportsSection
              reports={reports}
              busyId={busyId}
              mutate={mutate}
              accessToken={accessToken || ""}
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
  users,
  currentUserId,
  busyId,
  mutate,
  accessToken,
}: {
  users: AdminUser[];
  currentUserId: string;
  busyId: string;
  mutate: Mutation;
  accessToken: string;
}) {
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<AdminUser["status"] | "all">("all");
  const normalized = query.trim().toLowerCase();
  const visible = users.filter(
    (user) =>
      (status === "all" || user.status === status) &&
      (!normalized ||
        user.email.toLowerCase().includes(normalized) ||
        user.id.toLowerCase().includes(normalized)),
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
                setStatus(event.target.value as AdminUser["status"] | "all")
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
    </section>
  );
}

function ContentSection({
  creators,
  releases,
  busyId,
  mutate,
  accessToken,
}: {
  creators: CreatorReviewItem[];
  releases: ReleaseReviewItem[];
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
            <span>{creators.length}</span>
          </header>
          {creators.length ? (
            creators.map((creator) => (
              <article className="admin-review-item" key={creator.id}>
                <div>
                  <strong>{creator.display_name}</strong>
                  <small>
                    {creator.type} · submitted {formatDate(creator.updated_at)}
                  </small>
                  <code>{creator.id}</code>
                </div>
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
              </article>
            ))
          ) : (
            <AdminEmpty message="No creators are awaiting verification." />
          )}
        </div>
        <div className="admin-review-panel">
          <header>
            <h3>Release moderation</h3>
            <span>{releases.length}</span>
          </header>
          {releases.length ? (
            releases.map((release) => (
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
              </article>
            ))
          ) : (
            <AdminEmpty message="No releases are awaiting moderation." />
          )}
        </div>
      </div>
    </section>
  );
}

function ReportsSection({
  reports,
  busyId,
  mutate,
  accessToken,
}: {
  reports: CommentReport[];
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
          <h2 id="admin-reports-title">Pending comment reports</h2>
        </div>
      </div>
      {reports.length ? (
        <div className="admin-report-list">
          {reports.map(({ report, comment }) => (
            <article className="admin-report-card" key={report.id}>
              <div className="admin-report-meta">
                <span className="admin-reason">{humanize(report.reason)}</span>
                <small>{formatDate(report.created_at)}</small>
              </div>
              <blockquote>{comment.content}</blockquote>
              {report.description ? <p>{report.description}</p> : null}
              <small>Comment author: {comment.user_id}</small>
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
                <button
                  type="button"
                  disabled={busyId === report.id || !notes[report.id]?.trim()}
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
                <button
                  className="admin-action-danger"
                  type="button"
                  disabled={busyId === report.id || !notes[report.id]?.trim()}
                  onClick={() =>
                    void mutate(report.id, () =>
                      moderateCommentReport(
                        accessToken,
                        report.id,
                        "hide_comment",
                        notes[report.id].trim(),
                      ),
                    )
                  }
                >
                  Hide comment
                </button>
                <button
                  type="button"
                  disabled={busyId === report.id || !notes[report.id]?.trim()}
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
            </article>
          ))}
        </div>
      ) : (
        <AdminEmpty message="There are no pending comment reports." />
      )}
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
