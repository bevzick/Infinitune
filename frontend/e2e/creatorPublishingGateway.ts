import type { Page, Request, Route } from "@playwright/test";

import { mockGateway } from "./gatewayMock";

/** Stateful browser-only contract mock shared by separate creator and moderator contexts.
 * This does not replace the real-service content/media/system smoke tests.
 */
type Actor = "creator" | "moderator" | "listener";
type ReleaseStatus = "draft" | "pending_review" | "approved" | "published";
type CreatorStatus = "unverified" | "pending" | "verified";
type RightsType = "streaming" | "distribution";

const ids = {
  creatorUser: "00000000-0000-4000-8000-000000000001",
  moderatorUser: "00000000-0000-4000-8000-000000000002",
  listenerUser: "00000000-0000-4000-8000-000000000003",
  account: "00000000-0000-4000-8000-000000000503",
  release: "00000000-0000-4000-8000-000000000501",
  track: "00000000-0000-4000-8000-000000000502",
  coverUpload: "00000000-0000-4000-8000-000000000553",
  audioUpload: "00000000-0000-4000-8000-000000000554",
  catalogTrack: "00000000-0000-4000-8000-000000000555",
  catalogAlbum: "00000000-0000-4000-8000-000000000556",
};

const corsHeaders = {
  "access-control-allow-origin": "http://127.0.0.1:5173",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
  "access-control-allow-headers":
    "Accept, Authorization, Content-Type, X-Requested-With, X-CSRF-Token",
};
const now = "2026-11-14T01:00:00Z";
const tokenFor = (actor: Actor) => `e2e-${actor}-token`;
const userFor = (actor: Actor) =>
  actor === "creator"
    ? ids.creatorUser
    : actor === "moderator"
      ? ids.moderatorUser
      : ids.listenerUser;

function respond(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    headers: corsHeaders,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}
function failure(route: Route, status: number, code: string) {
  return respond(route, { error: { code, message: code } }, status);
}
function pageOf<T>(items: T[]) {
  return { items, page: { limit: 10, offset: 0, total: items.length } };
}

function silentWav(): Buffer {
  const samples = 8000;
  const buffer = Buffer.alloc(44 + samples * 2);
  buffer.write("RIFF", 0);
  buffer.writeUInt32LE(buffer.length - 8, 4);
  buffer.write("WAVEfmt ", 8);
  buffer.writeUInt32LE(16, 16);
  buffer.writeUInt16LE(1, 20);
  buffer.writeUInt16LE(1, 22);
  buffer.writeUInt32LE(8000, 24);
  buffer.writeUInt32LE(16000, 28);
  buffer.writeUInt16LE(2, 32);
  buffer.writeUInt16LE(16, 34);
  buffer.write("data", 36);
  buffer.writeUInt32LE(samples * 2, 40);
  return buffer;
}

type Draft = {
  id: string;
  creator_id: string;
  title: string;
  release_type: "single" | "ep" | "album" | "compilation";
  release_date: string;
  status: ReleaseStatus;
  cover_object_key: string | null;
  tracks: Array<{
    id: string;
    title: string;
    track_number: number;
    disc_number: number;
    explicit: boolean;
  }>;
  submitted_at: string | null;
  reviewed_at: string | null;
  published_at: string | null;
  created_at: string;
  updated_at: string;
};

export function createPublishingJourneyMock() {
  let account: {
    id: string;
    display_name: string;
    type: "artist" | "label" | "distributor";
    verification_status: CreatorStatus;
    created_at: string;
    updated_at: string;
  } | null = null;
  let release: Draft | null = null;
  let coverSession:
    | {
        original_name: string;
        mime_type: string;
        size_bytes: number;
        key: string;
      }
    | undefined;
  let audioSession:
    | {
        original_name: string;
        mime_type: string;
        size_bytes: number;
        key: string;
      }
    | undefined;
  let audioStage: "none" | "uploaded" | "ready" = "none";
  const rights = new Map<
    RightsType,
    {
      id: string;
      release_track_id: string;
      territory_code: string;
      rights_type: RightsType;
      valid_from: string;
      valid_until: null;
      granted_by_user_id: string;
      created_at: string;
      updated_at: string;
    }
  >();
  let catalogSynced = false;
  const requests: Array<{ actor: Actor; request: Request }> = [];

  function issues() {
    if (!release)
      return [{ code: "release_not_found", message: "Release not found" }];
    const blockers: Array<{
      code: string;
      message: string;
      track_id?: string;
    }> = [];
    if (account?.verification_status !== "verified") {
      blockers.push({
        code: "creator_not_verified",
        message: "Verify the creator account",
      });
    }
    if (!release.cover_object_key) {
      blockers.push({ code: "cover_missing", message: "Upload cover artwork" });
    }
    if (audioStage !== "ready") {
      blockers.push({
        code: "media_not_ready",
        message: "Audio processing is not ready",
        track_id: ids.track,
      });
    }
    for (const type of ["streaming", "distribution"] as const) {
      if (!rights.has(type)) {
        blockers.push({
          code: `${type}_rights_missing`,
          message: `Worldwide ${type} rights are required`,
          track_id: ids.track,
        });
      }
    }
    return blockers;
  }

  async function attach(page: Page, actor: Actor) {
    await mockGateway(page, {
      roles: actor === "moderator" ? ["admin", "moderator"] : ["listener"],
      emptyCreatorAccounts: true,
    });
    await page.route("http://localhost:8000/v1/**", async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const path = url.pathname;
      const method = request.method();
      requests.push({ actor, request });
      if (method === "OPTIONS")
        return route.fulfill({ status: 204, headers: corsHeaders });

      if (path === "/v1/auth/refresh" && method === "POST") {
        return respond(route, {
          access_token: tokenFor(actor),
          token_type: "Bearer",
          expires_at: "2099-01-01T00:00:00Z",
        });
      }
      if (path === "/v1/auth/login" && method === "POST") {
        const input = request.postDataJSON() as { email: string };
        if (
          !input.email.startsWith(
            actor === "creator"
              ? "creator@"
              : actor === "moderator"
                ? "moderator@"
                : "listener@",
          )
        ) {
          return failure(route, 401, "invalid_credentials");
        }
        return respond(route, {
          access_token: tokenFor(actor),
          token_type: "Bearer",
          expires_at: "2099-01-01T00:00:00Z",
        });
      }
      if (path === "/v1/auth/identity") {
        return respond(route, {
          user_id: userFor(actor),
          roles: actor === "moderator" ? ["admin", "moderator"] : ["listener"],
          issued_at: now,
          expires_at: "2099-01-01T00:00:00Z",
        });
      }

      const known =
        path.startsWith("/v1/creators") ||
        path.startsWith("/v1/releases") ||
        path.startsWith("/v1/release-tracks") ||
        path.startsWith("/v1/media/cover-uploads") ||
        path.startsWith("/v1/media/uploads") ||
        path.startsWith("/v1/moderation/creators") ||
        path.startsWith("/v1/moderation/releases") ||
        path === "/v1/catalog/tracks" ||
        path.startsWith("/v1/media/tracks/");
      if (!known) return route.fallback();
      // Catalog search is public; only protected endpoints need a Bearer token.
      const publicCatalogRead =
        path === "/v1/catalog/tracks" && method === "GET";
      if (
        !publicCatalogRead &&
        request.headers().authorization !== `Bearer ${tokenFor(actor)}`
      ) {
        return failure(route, 401, "unauthorized");
      }
      if (path === "/v1/creators" && method === "GET") {
        return respond(route, actor === "creator" && account ? [account] : []);
      }
      if (path === "/v1/creators" && method === "POST") {
        if (actor !== "creator") return failure(route, 403, "forbidden");
        const data = request.postDataJSON() as {
          display_name: string;
          type: "artist" | "label" | "distributor";
        };
        account = {
          id: ids.account,
          display_name: data.display_name,
          type: data.type,
          verification_status: "unverified",
          created_at: now,
          updated_at: now,
        };
        return respond(route, account, 201);
      }
      if (
        path === `/v1/creators/${ids.account}/verification-submissions` &&
        method === "POST"
      ) {
        if (actor !== "creator" || !account)
          return failure(route, 403, "forbidden");
        account.verification_status = "pending";
        return respond(route, account);
      }
      if (path === "/v1/moderation/creators" && method === "GET") {
        if (actor !== "moderator") return failure(route, 403, "forbidden");
        return respond(
          route,
          pageOf(
            account &&
              url.searchParams.get("status") === account.verification_status
              ? [account]
              : [],
          ),
        );
      }
      if (
        path === `/v1/creators/${ids.account}/verification-reviews` &&
        method === "POST"
      ) {
        if (actor !== "moderator" || account?.verification_status !== "pending")
          return failure(route, 403, "forbidden");
        const input = request.postDataJSON() as { decision: string };
        if (input.decision !== "verified")
          return failure(route, 422, "invalid_decision");
        account.verification_status = "verified";
        return respond(route, account);
      }

      if (path === "/v1/releases" && method === "GET") {
        return respond(route, {
          items: release && actor === "creator" ? [release] : [],
          page: {
            limit: 100,
            offset: 0,
            total: release && actor === "creator" ? 1 : 0,
          },
        });
      }
      if (path === "/v1/releases" && method === "POST") {
        if (actor !== "creator" || !account)
          return failure(route, 403, "forbidden");
        const data = request.postDataJSON() as {
          creator_id: string;
          title: string;
          release_type: Draft["release_type"];
          release_date: string;
          tracks: Array<{
            title: string;
            track_number: number;
            disc_number: number;
            explicit: boolean;
          }>;
        };
        if (data.creator_id !== account.id || data.tracks.length !== 1)
          return failure(route, 422, "invalid_release");
        release = {
          id: ids.release,
          creator_id: account.id,
          title: data.title,
          release_type: data.release_type,
          release_date: data.release_date,
          status: "draft",
          cover_object_key: null,
          tracks: data.tracks.map((track) => ({ id: ids.track, ...track })),
          submitted_at: null,
          reviewed_at: null,
          published_at: null,
          created_at: now,
          updated_at: now,
        };
        return respond(route, release, 201);
      }
      if (path === `/v1/releases/${ids.release}` && method === "GET") {
        return release
          ? respond(route, release)
          : failure(route, 404, "release_not_found");
      }
      if (
        path === `/v1/releases/${ids.release}/readiness` &&
        method === "GET"
      ) {
        if (!release || actor !== "creator")
          return failure(route, 403, "forbidden");
        const blockers = issues();
        return respond(route, {
          release_id: release.id,
          status: release.status,
          ready_for_submission:
            release.status === "draft" && blockers.length === 0,
          issues: blockers,
        });
      }
      if (
        path === `/v1/releases/${ids.release}/cover-uploads` &&
        method === "POST"
      ) {
        if (release?.status !== "draft" || actor !== "creator")
          return failure(route, 409, "release_not_editable");
        const input = request.postDataJSON() as {
          idempotency_key: string;
          original_name: string;
          mime_type: string;
          size_bytes: number;
        };
        if (
          !/^[0-9a-f-]{36}$/i.test(input.idempotency_key) ||
          input.mime_type !== "image/png" ||
          input.size_bytes < 1 ||
          input.size_bytes > 10 * 1024 * 1024
        ) {
          return failure(route, 422, "invalid_cover");
        }
        coverSession = {
          key: input.idempotency_key,
          original_name: input.original_name,
          mime_type: input.mime_type,
          size_bytes: input.size_bytes,
        };
        return respond(
          route,
          {
            id: ids.coverUpload,
            release_id: release.id,
            upload_url: `/v1/media/cover-uploads/${ids.coverUpload}`,
            ...input,
            status: "pending",
            created_at: now,
            updated_at: now,
          },
          201,
        );
      }
      if (
        path === `/v1/media/cover-uploads/${ids.coverUpload}` &&
        method === "PUT"
      ) {
        const bytes = request.postDataBuffer();
        if (
          !coverSession ||
          actor !== "creator" ||
          !release ||
          !bytes ||
          bytes.length !== coverSession.size_bytes ||
          request.headers()["content-type"] !== coverSession.mime_type ||
          bytes.subarray(0, 8).toString("hex") !== "89504e470d0a1a0a"
        ) {
          return failure(route, 422, "invalid_cover_file");
        }
        release.cover_object_key = `covers/${ids.release}/${ids.coverUpload}.png`;
        return respond(
          route,
          { release_id: release.id, status: "completed" },
          201,
        );
      }
      if (
        path === `/v1/release-tracks/${ids.track}/uploads` &&
        method === "POST"
      ) {
        if (actor !== "creator" || !release || release.status !== "draft")
          return failure(route, 409, "release_not_editable");
        const input = request.postDataJSON() as {
          idempotency_key: string;
          original_name: string;
          mime_type: string;
          size_bytes: number;
        };
        if (
          !input.idempotency_key ||
          input.mime_type !== "audio/wav" ||
          input.size_bytes < 1
        )
          return failure(route, 422, "invalid_audio");
        audioSession = {
          key: input.idempotency_key,
          original_name: input.original_name,
          mime_type: input.mime_type,
          size_bytes: input.size_bytes,
        };
        return respond(
          route,
          {
            id: ids.audioUpload,
            release_track_id: ids.track,
            upload_url: `/v1/media/uploads/${ids.audioUpload}`,
            ...input,
            status: "pending",
            created_at: now,
            updated_at: now,
          },
          201,
        );
      }
      if (path === `/v1/media/uploads/${ids.audioUpload}` && method === "PUT") {
        const bytes = request.postDataBuffer();
        if (
          !audioSession ||
          actor !== "creator" ||
          !bytes ||
          bytes.length !== audioSession.size_bytes ||
          request.headers()["content-type"] !== audioSession.mime_type ||
          bytes.subarray(0, 4).toString("ascii") !== "RIFF"
        ) {
          return failure(route, 422, "invalid_audio_file");
        }
        audioStage = "uploaded";
        return respond(
          route,
          {
            media_asset_id: ids.audioUpload,
            content_track_id: ids.track,
            status: "uploaded",
          },
          201,
        );
      }
      if (
        path === `/v1/release-tracks/${ids.track}/rights` &&
        method === "GET"
      ) {
        return respond(route, [...rights.values()]);
      }
      if (
        path === `/v1/release-tracks/${ids.track}/rights` &&
        method === "PUT"
      ) {
        if (actor !== "creator" || !release || release.status !== "draft")
          return failure(route, 403, "forbidden");
        const input = request.postDataJSON() as {
          territory_code: string;
          rights_type: RightsType;
          valid_from: string;
          valid_until: null;
        };
        if (
          input.territory_code !== "001" ||
          !["streaming", "distribution"].includes(input.rights_type)
        )
          return failure(route, 422, "invalid_rights");
        const right = {
          id:
            input.rights_type === "streaming"
              ? "00000000-0000-4000-8000-000000000561"
              : "00000000-0000-4000-8000-000000000562",
          release_track_id: ids.track,
          ...input,
          granted_by_user_id: ids.creatorUser,
          created_at: now,
          updated_at: now,
        };
        rights.set(input.rights_type, right);
        return respond(route, right);
      }
      if (
        path === `/v1/releases/${ids.release}/submissions` &&
        method === "POST"
      ) {
        if (
          actor !== "creator" ||
          !release ||
          release.status !== "draft" ||
          issues().length > 0
        )
          return failure(route, 409, "release_not_ready");
        release.status = "pending_review";
        release.submitted_at = now;
        return respond(route, release);
      }
      if (path === "/v1/moderation/releases" && method === "GET") {
        if (actor !== "moderator") return failure(route, 403, "forbidden");
        const items =
          release && release.status === url.searchParams.get("status")
            ? [release]
            : [];
        return respond(route, pageOf(items));
      }
      if (
        path === `/v1/releases/${ids.release}/moderation-reviews` &&
        method === "POST"
      ) {
        if (actor !== "moderator")
          return failure(route, 403, "self_approval_forbidden");
        if (release?.status !== "pending_review")
          return failure(route, 409, "release_not_pending");
        const input = request.postDataJSON() as { decision: string };
        if (input.decision !== "approved")
          return failure(route, 422, "invalid_decision");
        release.status = "approved";
        release.reviewed_at = now;
        return respond(route, release);
      }
      if (
        path === `/v1/releases/${ids.release}/publications` &&
        method === "POST"
      ) {
        if (actor !== "creator" || !release || release.status !== "approved")
          return failure(route, 409, "release_not_approved");
        release.status = "published";
        release.published_at = now;
        return respond(route, release);
      }
      if (path === "/v1/catalog/tracks" && method === "GET") {
        if (!catalogSynced || release?.status !== "published")
          return respond(route, {
            items: [],
            pagination: { limit: 20, offset: 0, total: 0 },
          });
        const queried = url.searchParams.get("title")?.toLowerCase();
        const tracks = release.tracks
          .filter(
            (track) => !queried || track.title.toLowerCase().includes(queried),
          )
          .map((track) => ({
            id: ids.catalogTrack,
            album_id: ids.catalogAlbum,
            playback_url: `/v1/media/tracks/${ids.catalogTrack}/playback`,
            title: track.title,
            duration_ms: 1000,
            track_number: track.track_number,
            disc_number: track.disc_number,
            explicit: track.explicit,
            release_date: release.release_date,
            artists: [
              {
                artist: {
                  id: ids.account,
                  name: account?.display_name || "Artist",
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
          }));
        return respond(route, {
          items: tracks,
          pagination: { limit: 20, offset: 0, total: tracks.length },
        });
      }
      if (
        path === `/v1/media/tracks/${ids.catalogTrack}/playback` &&
        method === "GET"
      ) {
        if (!catalogSynced || actor === "moderator")
          return failure(route, 403, "playback_forbidden");
        return route.fulfill({
          status: 200,
          headers: { ...corsHeaders, "accept-ranges": "bytes" },
          contentType: "audio/wav",
          body: silentWav(),
        });
      }
      return route.fallback();
    });
  }

  return {
    ids,
    requests,
    attach,
    completeAudioProcessing() {
      if (audioStage !== "uploaded")
        throw new Error("Audio has not been uploaded");
      audioStage = "ready";
    },
    syncCatalog() {
      if (release?.status !== "published")
        throw new Error("Release has not been published");
      catalogSynced = true;
    },
  };
}
