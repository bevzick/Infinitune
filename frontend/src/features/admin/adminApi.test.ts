import { afterEach, describe, expect, it, vi } from "vitest";

import { assignUserRole, listUsers, updateUserStatus } from "./adminApi";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("adminApi", () => {
  it("loads a filtered user page with admin authentication", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        items: [],
        page: { limit: 50, offset: 0, total: 0 },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await listUsers("admin-token", {
      status: "blocked",
      limit: 50,
      offset: 0,
    });

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe(
      "http://localhost:8000/v1/auth/users?limit=50&offset=0&status=blocked",
    );
    expect(new Headers(options.headers).get("Authorization")).toBe(
      "Bearer admin-token",
    );
  });

  it("sends role and status mutations to their dedicated endpoints", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(() =>
        Promise.resolve(jsonResponse({ user_id: "user-id", roles: [] })),
      );
    vi.stubGlobal("fetch", fetchMock);

    await assignUserRole("admin-token", "user-id", "moderator");
    await updateUserStatus("admin-token", "user-id", "blocked");

    expect(fetchMock.mock.calls[0][0]).toBe(
      "http://localhost:8000/v1/auth/users/user-id/roles/moderator",
    );
    expect(fetchMock.mock.calls[0][1].method).toBe("PUT");
    expect(fetchMock.mock.calls[1][0]).toBe(
      "http://localhost:8000/v1/auth/users/user-id/status",
    );
    expect(fetchMock.mock.calls[1][1].method).toBe("PATCH");
    expect(fetchMock.mock.calls[1][1].body).toBe(
      JSON.stringify({ status: "blocked" }),
    );
  });
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
