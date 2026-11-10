import { afterEach, describe, expect, it, vi } from "vitest";

import { refresh } from "./authApi";

const csrfToken = "csrf-token-value";

afterEach(() => {
  // biome-ignore lint/suspicious/noDocumentCookie: jsdom models cookie writes through document.cookie.
  document.cookie = "infinitune_csrf=; Max-Age=0; Path=/";
  vi.unstubAllGlobals();
});

describe("refresh", () => {
  it("stays anonymous without exposing or requesting a refresh token", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    await expect(refresh()).resolves.toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("uses the readable CSRF cookie and keeps the access token in memory", async () => {
    // biome-ignore lint/suspicious/noDocumentCookie: jsdom models cookie writes through document.cookie.
    document.cookie = `infinitune_csrf=${csrfToken}; Path=/`;
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          access_token: "access-token",
          token_type: "Bearer",
          expires_at: "2026-11-10T12:15:00Z",
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          user_id: "11111111-1111-4111-8111-111111111111",
          roles: ["listener"],
          issued_at: "2026-11-10T12:00:00Z",
          expires_at: "2026-11-10T12:15:00Z",
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const session = await refresh();

    expect(session?.accessToken).toBe("access-token");
    expect(session?.identity.roles).toEqual(["listener"]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    const [refreshUrl, refreshOptions] = fetchMock.mock.calls[0];
    expect(refreshUrl).toBe("http://localhost:8000/v1/auth/refresh");
    expect(refreshOptions.credentials).toBe("include");
    expect(new Headers(refreshOptions.headers).get("X-CSRF-Token")).toBe(
      csrfToken,
    );
    const [, identityOptions] = fetchMock.mock.calls[1];
    expect(new Headers(identityOptions.headers).get("Authorization")).toBe(
      "Bearer access-token",
    );
  });
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
