import { afterEach, describe, expect, it, vi } from "vitest";

import { updateProfile, updateSettings } from "./profileApi";

afterEach(() => vi.unstubAllGlobals());

describe("profile API", () => {
  it("can explicitly clear the private birth date", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({}));
    vi.stubGlobal("fetch", fetchMock);

    await updateProfile("access-token", {
      username: "night_listener",
      display_name: "Night Listener",
      bio: "",
      avatar_url: "",
      country_code: "UA",
      language: "uk",
      birth_date: null,
    });

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe("http://localhost:8000/v1/users/me/profile");
    expect(options.method).toBe("PATCH");
    expect(JSON.parse(options.body).birth_date).toBeNull();
  });

  it("preserves explicit false setting values", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({}));
    vi.stubGlobal("fetch", fetchMock);

    await updateSettings("access-token", {
      theme: "system",
      autoplay: false,
      explicit_content: false,
      audio_quality: "high",
      normalize_volume: false,
      show_listening_activity: false,
    });

    const [, options] = fetchMock.mock.calls[0];
    expect(new Headers(options.headers).get("Authorization")).toBe(
      "Bearer access-token",
    );
    expect(JSON.parse(options.body)).toMatchObject({
      autoplay: false,
      explicit_content: false,
      normalize_volume: false,
      show_listening_activity: false,
    });
  });
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
