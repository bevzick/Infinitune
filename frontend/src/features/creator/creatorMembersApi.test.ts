import { afterEach, describe, expect, it, vi } from "vitest";
import {
  addCreatorMember,
  changeCreatorMemberRole,
  listCreatorMembers,
  removeCreatorMember,
} from "./creatorMembersApi";

const baseUrl = "http://localhost:8000/v1/creators/creator-1/members";
const member = {
  user_id: "00000000-0000-4000-8000-000000000002",
  role: "editor",
  created_at: "2026-11-13T00:00:00Z",
};

function mockResponse(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => vi.unstubAllGlobals());

describe("creator member API", () => {
  it("loads the current member list with authorization and abort signal", async () => {
    const fetchMock = vi.fn().mockResolvedValue(mockResponse([member]));
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();
    await expect(
      listCreatorMembers("token", "creator-1", controller.signal),
    ).resolves.toEqual([member]);
    expect(fetchMock.mock.calls[0][0]).toBe(baseUrl);
    const options = fetchMock.mock.calls[0][1];
    expect(options.credentials).toBe("include");
    expect(options.signal).toBe(controller.signal);
    expect(options.headers.get("Authorization")).toBe("Bearer token");
  });

  it("creates members via POST and serializes the role", async () => {
    const fetchMock = vi.fn().mockResolvedValue(mockResponse(member, 201));
    vi.stubGlobal("fetch", fetchMock);
    await addCreatorMember("token", "creator-1", {
      user_id: member.user_id,
      role: "editor",
    });
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe(baseUrl);
    expect(options.method).toBe("POST");
    expect(options.headers.get("Authorization")).toBe("Bearer token");
    expect(JSON.parse(options.body)).toEqual({
      user_id: member.user_id,
      role: "editor",
    });
  });

  it("changes member roles via PATCH", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(mockResponse({ ...member, role: "uploader" }));
    vi.stubGlobal("fetch", fetchMock);
    await changeCreatorMemberRole(
      "token",
      "creator-1",
      member.user_id,
      "uploader",
    );
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe(`${baseUrl}/${member.user_id}`);
    expect(options.method).toBe("PATCH");
    expect(JSON.parse(options.body)).toEqual({ role: "uploader" });
  });

  it("removes members via DELETE without expecting JSON body", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(
      removeCreatorMember("token", "creator-1", member.user_id),
    ).resolves.toBeUndefined();
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe(`${baseUrl}/${member.user_id}`);
    expect(options.method).toBe("DELETE");
  });
});
