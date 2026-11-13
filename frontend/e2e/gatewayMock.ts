import type { Page, Request, Route } from "@playwright/test";

const track = {
  id: "00000000-0000-4000-8000-000000000101",
  album_id: "00000000-0000-4000-8000-000000000201",
  playback_url: "/v1/media/tracks/night-drive/playback",
  title: "Night Drive",
  duration_ms: 228000,
  track_number: 1,
  disc_number: 1,
  explicit: false,
  release_date: "2026-11-10",
  artists: [
    {
      artist: {
        id: "00000000-0000-4000-8000-000000000301",
        name: "Luna Veil",
        image_url: "",
      },
      role: "primary",
      position: 1,
    },
  ],
  genres: [],
  moods: [],
  created_at: "2026-11-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

const album = {
  id: "00000000-0000-4000-8000-000000000201",
  title: "Night City",
  album_type: "album",
  release_date: "2026-11-10",
  cover_url: "",
  artists: [{ artist: track.artists[0].artist, position: 1 }],
  created_at: "2026-11-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

const artist = {
  ...track.artists[0].artist,
  description: "Atmospheric electronic music.",
  created_at: "2026-11-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

const releaseId = "00000000-0000-4000-8000-000000000501";
const releaseTrackId = "00000000-0000-4000-8000-000000000502";

type GatewayCreatorAccount = {
  id: string;
  type: "artist" | "label" | "distributor";
  display_name: string;
  verification_status: "unverified" | "pending" | "verified" | "rejected";
  created_at: string;
  updated_at: string;
};

type GatewayTrackRight = {
  id: string;
  release_track_id: string;
  territory_code: string;
  rights_type: "streaming" | "download" | "distribution";
  valid_from: string;
  valid_until: string | null;
  granted_by_user_id: string;
  created_at: string;
  updated_at: string;
};

export async function mockGateway(
  page: Page,
  options: { roles?: string[]; emptyCreatorAccounts?: boolean } = {},
) {
  const requests: Request[] = [];
  let createdDraft: ReturnType<typeof draftRelease> | null = null;
  let creatorAccounts: GatewayCreatorAccount[] = options.emptyCreatorAccounts
    ? []
    : [defaultCreatorAccount()];
  let trackRights: GatewayTrackRight[] = [];
  await page.route("http://localhost:8000/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    requests.push(request);

    if (url.pathname === "/v1/auth/login") {
      return json(route, {
        access_token: "e2e-access-token",
        token_type: "Bearer",
        expires_at: "2099-01-01T00:00:00Z",
      });
    }
    if (url.pathname === "/v1/auth/identity") {
      return json(route, {
        user_id: "00000000-0000-4000-8000-000000000001",
        roles: options.roles || ["listener"],
        issued_at: "2026-11-12T00:00:00Z",
        expires_at: "2099-01-01T00:00:00Z",
      });
    }
    if (url.pathname === "/v1/users/me/settings") {
      return json(route, {
        theme: "dark",
        autoplay: true,
        explicit_content: true,
        audio_quality: "high",
        normalize_volume: true,
        show_listening_activity: false,
        created_at: "2026-11-12T00:00:00Z",
        updated_at: "2026-11-12T00:00:00Z",
      });
    }
    if (url.pathname === "/v1/creators" && request.method() === "POST") {
      const input = request.postDataJSON() as {
        type: "artist" | "label" | "distributor";
        display_name: string;
      };
      const created = {
        id: "00000000-0000-4000-8000-000000000521",
        type: input.type,
        display_name: input.display_name,
        verification_status: "unverified",
        created_at: "2026-11-13T00:00:00Z",
        updated_at: "2026-11-13T00:00:00Z",
      };
      creatorAccounts = [created, ...creatorAccounts];
      return json(route, created, 201);
    }
    if (url.pathname === "/v1/creators") {
      return json(route, creatorAccounts);
    }
    const verification = url.pathname.match(
      /^\/v1\/creators\/([^/]+)\/verification-submissions$/,
    );
    if (verification && request.method() === "POST") {
      const account = creatorAccounts.find(({ id }) => id === verification[1]);
      if (!account) return apiError(route, 404, "creator_not_found");
      const updated = { ...account, verification_status: "pending" };
      creatorAccounts = creatorAccounts.map((item) =>
        item.id === updated.id ? updated : item,
      );
      return json(route, updated);
    }
    if (url.pathname === "/v1/media/sources/providers") {
      return json(route, {
        items: [{ id: "jamendo" }, { id: "internet_archive" }],
      });
    }
    if (url.pathname === "/v1/releases" && request.method() === "POST") {
      const input = request.postDataJSON() as {
        creator_id: string;
        title: string;
        release_type: "single" | "ep" | "album" | "compilation";
        release_date: string;
        tracks: Array<{
          title: string;
          track_number: number;
          disc_number: number;
          explicit: boolean;
        }>;
      };
      createdDraft = {
        ...draftRelease(false),
        id: "00000000-0000-4000-8000-000000000511",
        creator_id: input.creator_id,
        title: input.title,
        release_type: input.release_type,
        release_date: input.release_date,
        tracks: input.tracks.map((item, index) => ({
          id: `00000000-0000-4000-8000-${String(512 + index).padStart(12, "0")}`,
          ...item,
        })),
      };
      return json(route, createdDraft, 201);
    }
    if (url.pathname === "/v1/releases") {
      const items = [draftRelease(false)];
      if (createdDraft) items.unshift({ ...createdDraft, tracks: [] });
      return json(route, {
        items,
        page: { limit: 100, offset: 0, total: items.length },
      });
    }
    if (url.pathname === `/v1/releases/${releaseId}`) {
      return json(route, draftRelease(true));
    }
    const releaseTrackRights = url.pathname.match(
      /^\/v1\/release-tracks\/([^/]+)\/rights$/,
    );
    if (releaseTrackRights && request.method() === "PUT") {
      const input = request.postDataJSON() as Pick<
        GatewayTrackRight,
        "territory_code" | "rights_type" | "valid_from" | "valid_until"
      >;
      const stored: GatewayTrackRight = {
        id: `00000000-0000-4000-8000-0000000005${
          input.rights_type === "streaming" ? "31" : "32"
        }`,
        release_track_id: releaseTrackRights[1],
        ...input,
        granted_by_user_id: "00000000-0000-4000-8000-000000000001",
        created_at: "2026-11-13T00:00:00Z",
        updated_at: "2026-11-13T00:00:00Z",
      };
      trackRights = [
        ...trackRights.filter(
          (right) =>
            right.release_track_id !== stored.release_track_id ||
            right.territory_code !== stored.territory_code ||
            right.rights_type !== stored.rights_type,
        ),
        stored,
      ];
      return json(route, stored);
    }
    if (releaseTrackRights) {
      return json(
        route,
        trackRights.filter(
          (right) => right.release_track_id === releaseTrackRights[1],
        ),
      );
    }
    const deletedRight = url.pathname.match(/^\/v1\/track-rights\/([^/]+)$/);
    if (deletedRight && request.method() === "DELETE") {
      const exists = trackRights.some(({ id }) => id === deletedRight[1]);
      if (!exists) return apiError(route, 404, "track_rights_not_found");
      trackRights = trackRights.filter(({ id }) => id !== deletedRight[1]);
      return route.fulfill({ status: 204 });
    }
    if (url.pathname === "/v1/media/sources/jamendo/tracks") {
      return json(route, {
        items: [
          {
            provider: "jamendo",
            external_id: "42",
            title: "Night Signal",
            artist: "Luna Veil",
            page_url: "https://www.jamendo.com/track/42",
            license: {
              name: "CC BY 4.0",
              url: "https://creativecommons.org/licenses/by/4.0/",
              attribution: "Luna Veil",
              allows_streaming: true,
              allows_redistribution: true,
            },
            metadata: { album_name: "After Dark" },
          },
        ],
        limit: 20,
        offset: 0,
        total: 1,
      });
    }
    if (url.pathname === "/v1/media/imports") {
      return json(
        route,
        {
          media_asset_id: "00000000-0000-4000-8000-000000000601",
          content_track_id: releaseTrackId,
          status: "uploaded",
        },
        201,
      );
    }
    if (url.pathname === "/v1/auth/users") {
      return json(route, {
        items: [
          {
            id: "00000000-0000-4000-8000-000000000002",
            email: "listener@example.com",
            status: "active",
            email_verified: true,
            roles: ["listener"],
            created_at: "2026-11-12T00:00:00Z",
            updated_at: "2026-11-12T00:00:00Z",
            last_login_at: null,
          },
        ],
        page: { limit: 50, offset: 0, total: 1 },
      });
    }
    if (
      url.pathname === "/v1/moderation/creators" ||
      url.pathname === "/v1/moderation/releases" ||
      url.pathname === "/v1/moderation/comment-reports" ||
      url.pathname === "/v1/moderation/release-reviews" ||
      url.pathname === "/v1/moderation/comment-actions"
    ) {
      return json(route, {
        items: [],
        page: { limit: 50, offset: 0, total: 0 },
      });
    }
    if (url.pathname === "/v1/playlists") {
      return json(route, {
        items: [
          {
            id: "00000000-0000-4000-8000-000000000401",
            owner_id: "00000000-0000-4000-8000-000000000001",
            name: "Night Focus",
            description: "Quiet electronic tracks",
            type: "manual",
            visibility: "private",
            created_at: "2026-11-12T00:00:00Z",
            updated_at: "2026-11-12T00:00:00Z",
          },
        ],
        page: { limit: 100, offset: 0, total: 1 },
      });
    }
    if (url.pathname === "/v1/interactions/tracks/likes") {
      return json(route, { items: [], page: interactionPage() });
    }
    if (url.pathname === "/v1/interactions/albums/likes") {
      return json(route, { items: [], page: interactionPage() });
    }
    if (url.pathname === "/v1/interactions/artists/follows") {
      return json(route, { items: [], page: interactionPage() });
    }
    if (url.pathname === "/v1/catalog/tracks") {
      return json(route, {
        items: [track],
        pagination: { limit: 20, offset: 0, total: 1 },
      });
    }
    if (url.pathname === "/v1/catalog/albums") {
      return json(route, {
        items: [album],
        pagination: { limit: 100, offset: 0, total: 1 },
      });
    }
    if (url.pathname === "/v1/catalog/artists") {
      return json(route, {
        items: [artist],
        pagination: { limit: 100, offset: 0, total: 1 },
      });
    }
    if (
      url.pathname === "/v1/catalog/genres" ||
      url.pathname === "/v1/catalog/moods"
    ) {
      return json(route, {
        items: [],
        pagination: { limit: 100, offset: 0, total: 0 },
      });
    }

    return route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({
        error: { code: "not_mocked", message: `No mock for ${url.pathname}` },
      }),
    });
  });
  return requests;
}

function interactionPage() {
  return { limit: 100, offset: 0, total: 0 };
}

function defaultCreatorAccount(): GatewayCreatorAccount {
  return {
    id: "00000000-0000-4000-8000-000000000503",
    type: "artist",
    display_name: "Luna Veil",
    verification_status: "verified",
    created_at: "2026-11-13T00:00:00Z",
    updated_at: "2026-11-13T00:00:00Z",
  };
}

function draftRelease(withTracks: boolean) {
  return {
    id: releaseId,
    creator_id: "00000000-0000-4000-8000-000000000503",
    title: "Signals",
    release_type: "single",
    release_date: "2026-11-13",
    status: "draft",
    cover_object_key: null,
    tracks: withTracks
      ? [
          {
            id: releaseTrackId,
            title: "Night Signal",
            track_number: 1,
            disc_number: 1,
            explicit: false,
          },
        ]
      : [],
    created_at: "2026-11-13T00:00:00Z",
    updated_at: "2026-11-13T00:00:00Z",
  };
}

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

function apiError(route: Route, status: number, code: string) {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify({ error: { code, message: code } }),
  });
}
