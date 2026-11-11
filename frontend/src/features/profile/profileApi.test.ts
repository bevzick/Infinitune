import { afterEach, describe, expect, it, vi } from "vitest";

import {
  settingsUpdatedEvent,
  updateProfile,
  updateSettings,
} from "./profileApi";

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
    const response = {
      theme: "system",
      autoplay: false,
      explicit_content: false,
      audio_quality: "high",
      normalize_volume: false,
      show_listening_activity: false,
      created_at: "2026-01-10T00:00:00Z",
      updated_at: "2026-11-11T00:00:00Z",
    };
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(response));
    vi.stubGlobal("fetch", fetchMock);
    const settingsListener = vi.fn();
    window.addEventListener(settingsUpdatedEvent, settingsListener);

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
    expect(settingsListener).toHaveBeenCalledOnce();
    expect(
      (settingsListener.mock.calls[0][0] as CustomEvent).detail.audio_quality,
    ).toBe("high");
    window.removeEventListener(settingsUpdatedEvent, settingsListener);
  });
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
