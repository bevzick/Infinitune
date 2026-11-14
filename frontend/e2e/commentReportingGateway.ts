import type { Page, Request, Route } from "@playwright/test";

import { mockGateway } from "./gatewayMock";

type Actor = "author" | "reporter" | "moderator";
type CommentStatus = "published" | "hidden" | "deleted";
type ReportStatus = "pending" | "reviewing" | "resolved" | "rejected";

export const communityIds = {
  track: "00000000-0000-4000-8000-000000000101",
  album: "00000000-0000-4000-8000-000000000201",
  artist: "00000000-0000-4000-8000-000000000301",
  author: "00000000-0000-4000-8000-000000000701",
  reporter: "00000000-0000-4000-8000-000000000702",
  moderator: "00000000-0000-4000-8000-000000000703",
};

const now = "2026-11-14T01:00:00Z";
const corsHeaders = {
  "access-control-allow-origin": "http://127.0.0.1:5173",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
  "access-control-allow-headers":
    "Accept, Authorization, Content-Type, X-Requested-With, X-CSRF-Token",
};

const idsFor = (actor: Actor) => communityIds[actor];
const tokenFor = (actor: Actor) => `community-${actor}-token`;

export type CommunityRequest = {
  actor: Actor;
  request: Request;
};

type Comment = {
  id: string;
  user_id: string;
  track_id: string;
  parent_id: string | null;
  content: string;
  status: CommentStatus;
  created_at: string;
  updated_at: string;
};

type Report = {
  id: string;
  comment_id: string;
  reporter_user_id: string;
  reason: string;
  description: string;
  status: ReportStatus;
  created_at: string;
};

function respond(route: Route, value: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    headers: corsHeaders,
    body: JSON.stringify(value),
  });
}
function fail(route: Route, status: number, code: string) {
  return respond(route, { error: { code, message: code } }, status);
}
function paginated<T>(items: T[], limit: number, offset: number) {
  return {
    items: items.slice(offset, offset + limit),
    page: { limit, offset, total: items.length },
  };
}
function uuid(counter: number) {
  return `00000000-0000-4000-8000-${String(900 + counter).padStart(12, "0")}`;
}

/** Shared in-memory mock; separate browser contexts represent distinct identities.
 * Backend authorization and moderation rules are modeled, not replaced in production.
 */
export function createCommunityJourneyMock() {
  const comments: Comment[] = [];
  const reports: Report[] = [];
  const likes = new Map<string, Set<string>>();
  const requests: CommunityRequest[] = [];
  let idSequence = 0;
  let failNextLike = false;

  async function attach(page: Page, actor: Actor) {
    // All unrelated API requests remain handled by the existing browser mock.
    await mockGateway(page, {
      roles:
        actor === "moderator"
          ? ["listener", "moderator", "admin"]
          : ["listener"],
    });
    await page.route("http://localhost:8000/v1/**", async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const url = new URL(request.url());
      const method = request.method();
      requests.push({ actor, request });

      if (method === "OPTIONS") {
        return route.fulfill({ status: 204, headers: corsHeaders });
      }
      if (path === "/v1/auth/login" && method === "POST") {
        const input = request.postDataJSON() as { email?: string };
        if (input.email !== `${actor}@example.com`) {
          return fail(route, 401, "invalid_credentials");
        }
        return respond(route, {
          access_token: tokenFor(actor),
          token_type: "Bearer",
          expires_at: "2099-01-01T00:00:00Z",
        });
      }
      if (path === "/v1/auth/refresh" && method === "POST") {
        return respond(route, {
          access_token: tokenFor(actor),
          token_type: "Bearer",
          expires_at: "2099-01-01T00:00:00Z",
        });
      }
      if (path === "/v1/auth/identity") {
        if (request.headers().authorization !== `Bearer ${tokenFor(actor)}`) {
          return fail(route, 401, "unauthorized");
        }
        return respond(route, {
          user_id: idsFor(actor),
          roles:
            actor === "moderator"
              ? ["listener", "admin", "moderator"]
              : ["listener"],
          issued_at: now,
          expires_at: "2099-01-01T00:00:00Z",
        });
      }

      // Public catalog detail and comment reads do not require authentication.
      if (
        path === `/v1/catalog/tracks/${communityIds.track}` &&
        method === "GET"
      ) {
        return respond(route, {
          id: communityIds.track,
          album_id: communityIds.album,
          playback_url: `/v1/media/tracks/${communityIds.track}/playback`,
          title: "Night Drive",
          duration_ms: 228000,
          track_number: 1,
          disc_number: 1,
          explicit: false,
          release_date: "2026-11-10",
          artists: [
            {
              artist: {
                id: communityIds.artist,
                name: "Luna Veil",
                image_url: "",
              },
              role: "primary",
              position: 1,
            },
          ],
          genres: [],
          moods: [],
          created_at: now,
          updated_at: now,
        });
      }
      if (
        path === `/v1/catalog/albums/${communityIds.album}` &&
        method === "GET"
      ) {
        return respond(route, {
          id: communityIds.album,
          title: "Night City",
          album_type: "album",
          release_date: "2026-11-10",
          cover_url: "",
          artists: [
            {
              artist: {
                id: communityIds.artist,
                name: "Luna Veil",
                image_url: "",
              },
              position: 1,
            },
          ],
          created_at: now,
          updated_at: now,
        });
      }
      if (
        path === `/v1/tracks/${communityIds.track}/comments` &&
        method === "GET"
      ) {
        const limit = Number(url.searchParams.get("limit") || 20);
        const offset = Number(url.searchParams.get("offset") || 0);
        const visible = comments.filter((item) => item.status === "published");
        return respond(
          route,
          paginated(visible.slice().reverse(), limit, offset),
        );
      }

      const commentPath = path.match(/^\/v1\/comments\/([^/]+)$/);
      const reportPath = path.match(/^\/v1\/comments\/([^/]+)\/reports$/);
      const likePath = path.match(
        /^\/v1\/interactions\/comments\/([^/]+)\/like$/,
      );
      const moderationPath = path.match(
        /^\/v1\/moderation\/comment-reports\/([^/]+)\/actions$/,
      );
      const protectedEndpoint =
        path === "/v1/comments" ||
        Boolean(commentPath || reportPath || likePath || moderationPath) ||
        path === "/v1/interactions/comments/likes" ||
        path === "/v1/moderation/comment-reports" ||
        path === "/v1/moderation/comment-actions";
      if (!protectedEndpoint) return route.fallback();
      if (request.headers().authorization !== `Bearer ${tokenFor(actor)}`) {
        return fail(route, 401, "unauthorized");
      }
      if (actor !== "moderator" && path.startsWith("/v1/moderation/")) {
        return fail(route, 403, "moderator_role_required");
      }

      if (path === "/v1/comments" && method === "POST") {
        const input = request.postDataJSON() as {
          track_id: string;
          parent_id: string | null;
          content: string;
        };
        if (
          input.track_id !== communityIds.track ||
          !input.content?.trim() ||
          (input.parent_id &&
            !comments.some(
              (item) =>
                item.id === input.parent_id && item.status === "published",
            ))
        ) {
          return fail(route, 422, "invalid_comment");
        }
        const comment: Comment = {
          id: uuid(++idSequence),
          user_id: idsFor(actor),
          track_id: input.track_id,
          parent_id: input.parent_id || null,
          content: input.content,
          status: "published",
          created_at: now,
          updated_at: now,
        };
        comments.push(comment);
        return respond(route, comment, 201);
      }
      if (commentPath && (method === "PATCH" || method === "DELETE")) {
        const existing = comments.find((item) => item.id === commentPath[1]);
        if (existing?.status !== "published") {
          return fail(route, 404, "comment_not_found");
        }
        if (existing.user_id !== idsFor(actor))
          return fail(route, 403, "not_comment_owner");
        if (method === "DELETE") {
          existing.status = "deleted";
          return route.fulfill({ status: 204, headers: corsHeaders });
        }
        const input = request.postDataJSON() as { content: string };
        if (!input.content?.trim()) return fail(route, 422, "invalid_comment");
        existing.content = input.content;
        existing.updated_at = now;
        return respond(route, existing);
      }
      if (path === "/v1/interactions/comments/likes" && method === "GET") {
        const liked = Array.from(likes.get(idsFor(actor)) || []);
        return respond(
          route,
          paginated(
            liked.map((id) => ({ id, liked_at: now })),
            100,
            0,
          ),
        );
      }
      if (likePath && (method === "PUT" || method === "DELETE")) {
        const target = comments.find(
          (item) => item.id === likePath[1] && item.status === "published",
        );
        if (!target) return fail(route, 404, "comment_not_found");
        if (failNextLike) {
          failNextLike = false;
          return fail(route, 503, "like_unavailable");
        }
        const liked = likes.get(idsFor(actor)) || new Set<string>();
        if (method === "PUT") liked.add(target.id);
        else liked.delete(target.id);
        likes.set(idsFor(actor), liked);
        return route.fulfill({ status: 204, headers: corsHeaders });
      }
      if (reportPath && method === "POST") {
        const target = comments.find(
          (item) => item.id === reportPath[1] && item.status === "published",
        );
        if (!target) return fail(route, 404, "comment_not_found");
        if (target.user_id === idsFor(actor))
          return fail(route, 403, "self_report_forbidden");
        const input = request.postDataJSON() as {
          reason: string;
          description: string;
        };
        if (!input.reason || !input.description?.trim())
          return fail(route, 422, "invalid_report");
        if (
          reports.some(
            (item) =>
              item.comment_id === target.id &&
              item.reporter_user_id === idsFor(actor),
          )
        ) {
          return fail(route, 409, "already_reported");
        }
        const report: Report = {
          id: uuid(++idSequence),
          comment_id: target.id,
          reporter_user_id: idsFor(actor),
          reason: input.reason,
          description: input.description,
          status: "pending",
          created_at: now,
        };
        reports.push(report);
        return respond(route, { id: report.id, status: report.status }, 201);
      }
      if (path === "/v1/moderation/comment-reports" && method === "GET") {
        const status = url.searchParams.get("status");
        const limit = Number(url.searchParams.get("limit") || 10);
        const offset = Number(url.searchParams.get("offset") || 0);
        const matching = reports.filter(
          (report) => !status || report.status === status,
        );
        return respond(
          route,
          paginated(
            matching.map((report) => ({
              report,
              comment: comments.find((item) => item.id === report.comment_id),
            })),
            limit,
            offset,
          ),
        );
      }
      if (path === "/v1/moderation/comment-actions" && method === "GET") {
        return respond(route, paginated([], 10, 0));
      }
      if (moderationPath && method === "POST") {
        const report = reports.find((item) => item.id === moderationPath[1]);
        const comment = comments.find((item) => item.id === report?.comment_id);
        if (!report || !comment) return fail(route, 404, "report_not_found");
        if (report.status === "resolved" || report.status === "rejected")
          return fail(route, 409, "report_closed");
        const input = request.postDataJSON() as {
          action: string;
          note: string;
        };
        if (!input.note?.trim()) return fail(route, 422, "note_required");
        if (input.action === "hide_comment" && comment.status === "published") {
          comment.status = "hidden";
          report.status = "reviewing";
        } else if (
          input.action === "resolve_report" &&
          report.status === "reviewing"
        ) {
          report.status = "resolved";
        } else if (
          input.action === "start_review" &&
          report.status === "pending"
        ) {
          report.status = "reviewing";
        } else {
          return fail(route, 409, "invalid_transition");
        }
        return respond(route, { report_id: report.id, action: input.action });
      }
      return fail(route, 404, "not_mocked");
    });
  }

  return {
    attach,
    requests,
    failNextCommentLike() {
      failNextLike = true;
    },
    visibleComments() {
      return comments.filter((item) => item.status === "published");
    },
    reports,
  };
}
