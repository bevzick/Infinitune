import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../shared/api/client";
import { CreatorMembersManager } from "./CreatorMembersManager";

const api = vi.hoisted(() => ({
  listCreatorMembers: vi.fn(),
  addCreatorMember: vi.fn(),
  changeCreatorMemberRole: vi.fn(),
  removeCreatorMember: vi.fn(),
}));
vi.mock("./creatorMembersApi", () => api);

const ownerId = "00000000-0000-4000-8000-000000000001";
const managerId = "00000000-0000-4000-8000-000000000002";
const editorId = "00000000-0000-4000-8000-000000000003";
const uploaderId = "00000000-0000-4000-8000-000000000004";
const newMemberId = "00000000-0000-4000-8000-000000000005";
const stamp = "2026-11-13T00:00:00Z";
const owner = { user_id: ownerId, role: "owner", created_at: stamp };
const manager = { user_id: managerId, role: "manager", created_at: stamp };
const editor = { user_id: editorId, role: "editor", created_at: stamp };
const uploader = { user_id: uploaderId, role: "uploader", created_at: stamp };

function view(currentUserId = ownerId) {
  return render(
    <CreatorMembersManager
      accessToken="token"
      creatorId="creator-1"
      creatorName="Signals"
      currentUserId={currentUserId}
    />,
  );
}

beforeEach(() => {
  api.listCreatorMembers.mockResolvedValue([owner, manager, editor, uploader]);
  api.addCreatorMember.mockImplementation(
    async (_token, _creatorId, input) => ({ ...input, created_at: stamp }),
  );
  api.changeCreatorMemberRole.mockImplementation(
    async (_token, _creatorId, userId, role) => ({
      user_id: userId,
      role,
      created_at: stamp,
    }),
  );
  api.removeCreatorMember.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("CreatorMembersManager", () => {
  it("allows the owner to add a manager using a user UUID", async () => {
    const user = userEvent.setup();
    view();
    await screen.findByRole("button", { name: "Add member" });
    await user.type(screen.getByLabelText("User UUID"), newMemberId);
    await user.selectOptions(
      screen.getByLabelText("New member role"),
      "manager",
    );
    await user.click(screen.getByRole("button", { name: "Add member" }));
    await waitFor(() =>
      expect(api.addCreatorMember).toHaveBeenCalledWith("token", "creator-1", {
        user_id: newMemberId,
        role: "manager",
      }),
    );
    expect(await screen.findByText("Member added.")).toBeInTheDocument();
    expect(screen.getByText(newMemberId)).toBeInTheDocument();
    expect(screen.getByLabelText("User UUID")).toHaveValue("");
  });

  it("prevents the owner from editing or removing their own membership", async () => {
    view();
    await screen.findByRole("button", { name: "Add member" });
    expect(
      screen.queryByRole("button", { name: `Remove ${ownerId}` }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText(`Role for ${ownerId}`),
    ).not.toBeInTheDocument();
  });

  it("allows a manager to edit editors/uploaders but not other managers", async () => {
    const user = userEvent.setup();
    view(managerId);
    await screen.findByRole("button", { name: "Add member" });
    expect(screen.getByLabelText("New member role")).not.toHaveTextContent(
      "Manager",
    );
    expect(
      screen.queryByLabelText(`Role for ${managerId}`),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText(`Role for ${ownerId}`),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText(`Role for ${editorId}`)).toHaveValue("editor");
    await user.selectOptions(
      screen.getByLabelText(`Role for ${editorId}`),
      "uploader",
    );
    await waitFor(() =>
      expect(api.changeCreatorMemberRole).toHaveBeenCalledWith(
        "token",
        "creator-1",
        editorId,
        "uploader",
      ),
    );
    expect(await screen.findByText("Member role updated.")).toBeInTheDocument();
  });

  it("requires confirmation before removing a member", async () => {
    const user = userEvent.setup();
    view();
    await screen.findByRole("button", { name: "Add member" });
    await user.click(
      screen.getByRole("button", { name: `Remove ${uploaderId}` }),
    );
    expect(api.removeCreatorMember).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(api.removeCreatorMember).not.toHaveBeenCalled();
    await user.click(
      screen.getByRole("button", { name: `Remove ${uploaderId}` }),
    );
    await user.click(
      screen.getByRole("button", { name: `Confirm remove ${uploaderId}` }),
    );
    await waitFor(() =>
      expect(api.removeCreatorMember).toHaveBeenCalledWith(
        "token",
        "creator-1",
        uploaderId,
      ),
    );
    expect(await screen.findByText("Member removed.")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: `Remove ${uploaderId}` }),
    ).not.toBeInTheDocument();
  });

  it("does not send invalid or self user IDs", async () => {
    const user = userEvent.setup();
    view();
    await screen.findByRole("button", { name: "Add member" });
    await user.type(screen.getByLabelText("User UUID"), "not-a-uuid");
    await user.click(screen.getByRole("button", { name: "Add member" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "valid user UUID",
    );
    await user.clear(screen.getByLabelText("User UUID"));
    await user.type(screen.getByLabelText("User UUID"), ownerId);
    await user.click(screen.getByRole("button", { name: "Add member" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "cannot add yourself",
    );
    expect(api.addCreatorMember).not.toHaveBeenCalled();
  });

  it("handles forbidden membership lists without allowing changes", async () => {
    api.listCreatorMembers.mockRejectedValueOnce(
      new ApiError(403, "creator_forbidden", "Forbidden"),
    );
    view(editorId);
    expect(
      await screen.findByText(/Only owners and managers/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Add member" }),
    ).not.toBeInTheDocument();
  });

  it("preserves roles when role update fails", async () => {
    const user = userEvent.setup();
    api.changeCreatorMemberRole.mockRejectedValueOnce(
      new ApiError(409, "creator_conflict", "Update conflict"),
    );
    view();
    const select = await screen.findByLabelText(`Role for ${editorId}`);
    await user.selectOptions(select, "uploader");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Update conflict",
    );
    expect(select).toHaveValue("editor");
  });
});
