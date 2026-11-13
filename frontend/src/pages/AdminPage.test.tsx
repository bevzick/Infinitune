import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AdminPage } from "./AdminPage";

const adminApi = vi.hoisted(() => ({
  assignUserRole: vi.fn(),
  listCommentReports: vi.fn(),
  listCommentAudit: vi.fn(),
  listCreatorsForModeration: vi.fn(),
  listReleasesForModeration: vi.fn(),
  listReleaseAudit: vi.fn(),
  listUsers: vi.fn(),
  moderateCommentReport: vi.fn(),
  removeUserRole: vi.fn(),
  reviewCreator: vi.fn(),
  reviewRelease: vi.fn(),
  updateUserStatus: vi.fn(),
}));

vi.mock("../features/admin/adminApi", () => adminApi);
vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({
    accessToken: "admin-token",
    identity: { userId: "admin-id", roles: ["listener", "admin"] },
  }),
}));

const emptyPage = { items: [], page: { limit: 50, offset: 0, total: 0 } };

describe("AdminPage", () => {
  beforeEach(() => {
    adminApi.listUsers.mockResolvedValue({
      items: [
        {
          id: "listener-id",
          email: "listener@example.com",
          status: "active",
          email_verified: true,
          roles: ["listener"],
          created_at: "2026-11-01T00:00:00Z",
          updated_at: "2026-11-01T00:00:00Z",
          last_login_at: null,
        },
      ],
      page: { limit: 50, offset: 0, total: 1 },
    });
    adminApi.listCreatorsForModeration.mockResolvedValue(emptyPage);
    adminApi.listReleasesForModeration.mockResolvedValue(emptyPage);
    adminApi.listCommentReports.mockResolvedValue(emptyPage);
    adminApi.listReleaseAudit.mockResolvedValue(emptyPage);
    adminApi.listCommentAudit.mockResolvedValue(emptyPage);
    adminApi.assignUserRole.mockResolvedValue({
      user_id: "listener-id",
      roles: ["listener", "moderator"],
    });
    adminApi.updateUserStatus.mockResolvedValue({});
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("loads the admin queues and exposes user management controls", async () => {
    render(<AdminPage />);

    expect(await screen.findByText("listener@example.com")).toBeInTheDocument();
    expect(adminApi.listUsers).toHaveBeenCalledWith(
      "admin-token",
      { status: "all", limit: 10, offset: 0 },
      expect.any(AbortSignal),
    );
    expect(screen.getByRole("button", { name: "Block" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "moderator" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });

  it("assigns a role and refreshes administration data", async () => {
    const user = userEvent.setup();
    render(<AdminPage />);

    await user.click(await screen.findByRole("button", { name: "moderator" }));

    await waitFor(() =>
      expect(adminApi.assignUserRole).toHaveBeenCalledWith(
        "admin-token",
        "listener-id",
        "moderator",
      ),
    );
    await waitFor(() => expect(adminApi.listUsers).toHaveBeenCalledTimes(2));
  });

  it("switches to the content and report workspaces", async () => {
    const user = userEvent.setup();
    render(<AdminPage />);
    await screen.findByText("listener@example.com");

    await user.click(screen.getByRole("button", { name: /Content review/ }));
    expect(screen.getByText("No pending creators found.")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Comment reports/ }));
    expect(
      screen.getByText("There are no pending comment reports."),
    ).toBeInTheDocument();
  });

  it("requests the next user page from the server", async () => {
    adminApi.listUsers.mockResolvedValue({
      items: [
        {
          id: "listener-id",
          email: "listener@example.com",
          status: "active",
          email_verified: true,
          roles: ["listener"],
          created_at: "2026-11-01T00:00:00Z",
          updated_at: "2026-11-01T00:00:00Z",
          last_login_at: null,
        },
      ],
      page: { limit: 10, offset: 0, total: 21 },
    });
    const user = userEvent.setup();
    render(<AdminPage />);

    await user.click(await screen.findByRole("button", { name: "Next" }));

    await waitFor(() =>
      expect(adminApi.listUsers).toHaveBeenLastCalledWith(
        "admin-token",
        { status: "all", limit: 10, offset: 10 },
        expect.any(AbortSignal),
      ),
    );
  });

  it("loads completed moderation states from the server", async () => {
    const user = userEvent.setup();
    render(<AdminPage />);
    await screen.findByText("listener@example.com");

    await user.click(screen.getByRole("button", { name: /Content review/ }));
    await user.selectOptions(
      screen.getByLabelText("Creator verification status"),
      "verified",
    );
    await waitFor(() =>
      expect(adminApi.listCreatorsForModeration).toHaveBeenLastCalledWith(
        "admin-token",
        { status: "verified", limit: 10, offset: 0 },
        expect.any(AbortSignal),
      ),
    );

    await user.click(screen.getByRole("button", { name: /Comment reports/ }));
    await user.selectOptions(
      screen.getByLabelText("Comment report status"),
      "resolved",
    );
    await waitFor(() =>
      expect(adminApi.listCommentReports).toHaveBeenLastCalledWith(
        "admin-token",
        { status: "resolved", limit: 10, offset: 0 },
        expect.any(AbortSignal),
      ),
    );
  });

  it("shows persisted moderator identity, note, and action in activity log", async () => {
    adminApi.listReleaseAudit.mockResolvedValue({
      items: [
        {
          id: "review-id",
          release_id: "release-id",
          reviewer_user_id: "release-moderator-id",
          decision: "rejected",
          comment: "Rights evidence is missing",
          reviewed_submission_at: "2026-11-12T10:00:00Z",
          release_status: "rejected",
          created_at: "2026-11-12T10:00:00Z",
        },
      ],
      page: { limit: 10, offset: 0, total: 1 },
    });
    adminApi.listCommentAudit.mockResolvedValue({
      items: [
        {
          id: "action-id",
          report_id: "report-id",
          moderator_user_id: "comment-moderator-id",
          action: "hide_comment",
          note: "Policy violation",
          previous_report_status: "pending",
          resulting_report_status: "reviewing",
          previous_comment_status: "published",
          resulting_comment_status: "hidden",
          created_at: "2026-11-12T11:00:00Z",
        },
      ],
      page: { limit: 10, offset: 0, total: 1 },
    });
    const user = userEvent.setup();
    render(<AdminPage />);

    await user.click(
      await screen.findByRole("button", { name: /Activity log/ }),
    );

    expect(screen.getByText("Rights evidence is missing")).toBeInTheDocument();
    expect(screen.getByText("Policy violation")).toBeInTheDocument();
    expect(
      screen.getByText("Moderator: release-moderator-id"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Moderator: comment-moderator-id"),
    ).toBeInTheDocument();
  });
});
