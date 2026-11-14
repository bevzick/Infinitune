import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../../shared/api/client";
import { TrackComments } from "./TrackComments";

const session = vi.hoisted(() => ({
  accessToken: "token" as string | null,
  identity: { userId: "current-user" } as { userId: string } | null,
}));
const api = vi.hoisted(() => ({
  listTrackComments: vi.fn(),
  createTrackComment: vi.fn(),
  updateTrackComment: vi.fn(),
  deleteTrackComment: vi.fn(),
  reportTrackComment: vi.fn(),
  listLikedCommentIds: vi.fn(),
  setCommentLike: vi.fn(),
}));

vi.mock("../session/SessionContext", () => ({ useSession: () => session }));
vi.mock("./commentApi", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./commentApi")>()),
  ...api,
}));

const own = {
  id: "own-comment",
  user_id: "current-user",
  track_id: "track-1",
  parent_id: null,
  content: "My first comment",
  created_at: "2026-11-10T12:00:00Z",
  updated_at: "2026-11-10T12:00:00Z",
};
const other = {
  ...own,
  id: "other-comment",
  user_id: "other-user",
  content: "Another listener comment",
};

function renderComments() {
  render(
    <MemoryRouter>
      <TrackComments trackId="track-1" />
    </MemoryRouter>,
  );
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

describe("TrackComments", () => {
  beforeEach(() => {
    session.accessToken = "token";
    session.identity = { userId: "current-user" };
    api.listTrackComments.mockResolvedValue({
      items: [own, other],
      page: { limit: 20, offset: 0, total: 2 },
    });
    api.listLikedCommentIds.mockResolvedValue(new Set<string>());
    api.createTrackComment.mockResolvedValue({ ...own, id: "new-comment" });
    api.updateTrackComment.mockImplementation(async (_token, _id, content) => ({
      ...own,
      content,
    }));
    api.deleteTrackComment.mockResolvedValue(undefined);
    api.setCommentLike.mockResolvedValue(undefined);
    api.reportTrackComment.mockResolvedValue({
      id: "report-1",
      status: "pending",
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it("shows comments to anonymous users without editing controls", async () => {
    session.accessToken = null;
    session.identity = null;
    renderComments();
    expect(await screen.findByText("My first comment")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Sign in" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Reply" }),
    ).not.toBeInTheDocument();
    expect(api.listLikedCommentIds).not.toHaveBeenCalled();
  });

  it("optimistically adds a comment and rolls back on server error", async () => {
    const user = userEvent.setup();
    const pending = deferred<typeof own>();
    api.createTrackComment.mockReturnValueOnce(pending.promise);
    renderComments();
    await screen.findByText("My first comment");
    await user.type(screen.getByLabelText("Write a comment"), "A new thought");
    await user.click(screen.getByRole("button", { name: "Post comment" }));
    expect(screen.getAllByText("A new thought")).toHaveLength(2);
    expect(screen.getByText("Sending…")).toBeInTheDocument();
    pending.reject(
      new ApiError(503, "unavailable", "Comment service is unavailable"),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Comment service is unavailable",
    );
    expect(screen.getAllByText("A new thought")).toHaveLength(1);
    expect(screen.getByLabelText("Write a comment")).toHaveValue(
      "A new thought",
    );
  });

  it("creates replies with parent_id and shows the new reply", async () => {
    const user = userEvent.setup();
    api.createTrackComment.mockResolvedValueOnce({
      ...own,
      id: "reply",
      parent_id: "other-comment",
      content: "I agree",
    });
    renderComments();
    const comment = await screen.findByRole("article", {
      name: "Comment by other-us",
    });
    await user.click(within(comment).getByRole("button", { name: "Reply" }));
    await user.type(screen.getByLabelText("Reply to other-co"), "I agree");
    await user.click(screen.getByRole("button", { name: "Post reply" }));
    await waitFor(() =>
      expect(api.createTrackComment).toHaveBeenCalledWith(
        "token",
        "track-1",
        "I agree",
        "other-comment",
      ),
    );
    expect(await screen.findByText("I agree")).toBeInTheDocument();
  });

  it("rolls back an optimistic like when the interaction API fails", async () => {
    const user = userEvent.setup();
    const pending = deferred<void>();
    api.setCommentLike.mockReturnValueOnce(pending.promise);
    renderComments();
    const comment = await screen.findByRole("article", {
      name: "Comment by other-us",
    });
    const button = within(comment).getByRole("button", {
      name: "Like comment",
    });
    await waitFor(() => expect(button).toBeEnabled());
    await user.click(button);
    expect(
      within(comment).getByRole("button", { name: "Unlike comment" }),
    ).toHaveAttribute("aria-pressed", "true");
    pending.reject(new ApiError(503, "unavailable", "Like failed"));
    expect(await screen.findByRole("alert")).toHaveTextContent("Like failed");
    expect(
      within(comment).getByRole("button", { name: "Like comment" }),
    ).toHaveAttribute("aria-pressed", "false");
  });

  it("updates own comments and rolls back a failed deletion", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    renderComments();
    const ownCard = await screen.findByRole("article", {
      name: "Comment by you",
    });
    await user.click(within(ownCard).getByRole("button", { name: "Edit" }));
    await user.clear(within(ownCard).getByLabelText("Edit your comment"));
    await user.type(
      within(ownCard).getByLabelText("Edit your comment"),
      "My edited comment",
    );
    await user.click(
      within(ownCard).getByRole("button", { name: "Save edit" }),
    );
    expect(await screen.findByText("My edited comment")).toBeInTheDocument();
    expect(api.updateTrackComment).toHaveBeenCalledWith(
      "token",
      "own-comment",
      "My edited comment",
    );
    api.deleteTrackComment.mockRejectedValueOnce(
      new ApiError(500, "failed", "Delete failed"),
    );
    await user.click(within(ownCard).getByRole("button", { name: "Delete" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Delete failed");
    expect(screen.getByText("My edited comment")).toBeInTheDocument();
  });

  it("reports another user's comment but never its own", async () => {
    const user = userEvent.setup();
    renderComments();
    const otherCard = await screen.findByRole("article", {
      name: "Comment by other-us",
    });
    const ownCard = screen.getByRole("article", { name: "Comment by you" });
    expect(
      within(ownCard).queryByRole("button", { name: "Report" }),
    ).not.toBeInTheDocument();
    await user.click(within(otherCard).getByRole("button", { name: "Report" }));
    await user.selectOptions(
      within(otherCard).getByLabelText("Report reason"),
      "harassment",
    );
    await user.type(
      within(otherCard).getByLabelText("Report details"),
      "Inappropriate message",
    );
    await user.click(
      within(otherCard).getByRole("button", { name: "Send report" }),
    );
    await waitFor(() =>
      expect(api.reportTrackComment).toHaveBeenCalledWith(
        "token",
        "other-comment",
        "harassment",
        "Inappropriate message",
      ),
    );
    expect(
      within(otherCard).getByRole("button", { name: "Reported" }),
    ).toBeDisabled();
  });

  it("loads the next comment page without duplicating entries", async () => {
    const user = userEvent.setup();
    api.listTrackComments
      .mockResolvedValueOnce({
        items: [own],
        page: { limit: 20, offset: 0, total: 2 },
      })
      .mockResolvedValueOnce({
        items: [other],
        page: { limit: 20, offset: 1, total: 2 },
      });
    renderComments();
    await screen.findByText("My first comment");
    await user.click(
      screen.getByRole("button", { name: "Load more comments (1 next)" }),
    );
    expect(
      await screen.findByText("Another listener comment"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Load more comments/ }),
    ).not.toBeInTheDocument();
    expect(api.listTrackComments).toHaveBeenLastCalledWith("track-1", 1);
  });
});
