import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AdminPage } from "./AdminPage";

const adminApi = vi.hoisted(() => ({
  assignUserRole: vi.fn(),
  listCommentReports: vi.fn(),
  listPendingCreators: vi.fn(),
  listPendingReleases: vi.fn(),
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
    adminApi.listPendingCreators.mockResolvedValue(emptyPage);
    adminApi.listPendingReleases.mockResolvedValue(emptyPage);
    adminApi.listCommentReports.mockResolvedValue(emptyPage);
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
      "all",
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
    expect(
      screen.getByText("No creators are awaiting verification."),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Comment reports/ }));
    expect(
      screen.getByText("There are no pending comment reports."),
    ).toBeInTheDocument();
  });
});
