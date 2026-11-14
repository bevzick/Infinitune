import { beforeEach, describe, expect, it, vi } from "vitest";

import { apiRequest } from "../../shared/api/client";
import {
  createTrackComment,
  deleteTrackComment,
  listLikedCommentIds,
  listTrackComments,
  reportTrackComment,
  setCommentLike,
  updateTrackComment,
} from "./commentApi";

vi.mock("../../shared/api/client", () => ({ apiRequest: vi.fn() }));

const request = vi.mocked(apiRequest);

beforeEach(() => {
  request.mockReset();
  request.mockResolvedValue(undefined);
});

describe("comment API", () => {
  it("loads public comments with offset pagination", async () => {
    await listTrackComments("track id", 20);
    expect(request).toHaveBeenCalledWith(
      "/v1/tracks/track%20id/comments?limit=20&offset=20",
      { signal: undefined },
    );
  });

  it("creates a reply with a parent id and bearer token", async () => {
    await createTrackComment("token", "track", "hello", "parent");
    expect(request).toHaveBeenCalledWith("/v1/comments", {
      method: "POST",
      accessToken: "token",
      body: JSON.stringify({
        track_id: "track",
        parent_id: "parent",
        content: "hello",
      }),
    });
  });

  it("updates and deletes owned comments", async () => {
    await updateTrackComment("token", "comment", "updated");
    await deleteTrackComment("token", "comment");
    expect(request).toHaveBeenNthCalledWith(1, "/v1/comments/comment", {
      method: "PATCH",
      accessToken: "token",
      body: JSON.stringify({ content: "updated" }),
    });
    expect(request).toHaveBeenNthCalledWith(2, "/v1/comments/comment", {
      method: "DELETE",
      accessToken: "token",
    });
  });

  it("reports with the backend reason values", async () => {
    await reportTrackComment(
      "token",
      "comment",
      "harassment",
      "Abusive content",
    );
    expect(request).toHaveBeenCalledWith("/v1/comments/comment/reports", {
      method: "POST",
      accessToken: "token",
      body: JSON.stringify({
        reason: "harassment",
        description: "Abusive content",
      }),
    });
  });

  it("loads the entire liked-comment id set using pagination", async () => {
    request
      .mockResolvedValueOnce({
        items: Array.from({ length: 100 }, (_, n) => ({
          id: `comment-${n}`,
          liked_at: "",
        })),
        page: { limit: 100, offset: 0, total: 101 },
      })
      .mockResolvedValueOnce({
        items: [{ id: "comment-100", liked_at: "" }],
        page: { limit: 100, offset: 100, total: 101 },
      });
    const liked = await listLikedCommentIds("token");
    expect(liked.size).toBe(101);
    expect(request).toHaveBeenLastCalledWith(
      "/v1/interactions/comments/likes?limit=100&offset=100",
      { accessToken: "token", signal: undefined },
    );
  });

  it("toggles comments likes using the interaction service", async () => {
    await setCommentLike("token", "comment", true);
    await setCommentLike("token", "comment", false);
    expect(request).toHaveBeenNthCalledWith(
      1,
      "/v1/interactions/comments/comment/like",
      {
        method: "PUT",
        accessToken: "token",
      },
    );
    expect(request).toHaveBeenNthCalledWith(
      2,
      "/v1/interactions/comments/comment/like",
      {
        method: "DELETE",
        accessToken: "token",
      },
    );
  });
});
