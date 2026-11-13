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

export async function mockGateway(
  page: Page,
  options: { roles?: string[] } = {},
) {
  const requests: Request[] = [];
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
    if (url.pathname === "/v1/media/sources/providers") {
      return json(route, {
        items: [{ id: "jamendo" }, { id: "internet_archive" }],
      });
    }
    if (url.pathname === "/v1/releases") {
      return json(route, {
        items: [draftRelease(false)],
        page: { limit: 50, offset: 0, total: 1 },
      });
    }
    if (url.pathname === `/v1/releases/${releaseId}`) {
      return json(route, draftRelease(true));
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
