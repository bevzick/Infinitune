import { afterEach, describe, expect, it, vi } from "vitest";

import { apiBlobRequest } from "../../shared/api/client";

afterEach(() => vi.unstubAllGlobals());

describe("playback request", () => {
  it("downloads the opaque playback URL with the in-memory access token", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response("audio bytes", {
        status: 200,
        headers: { "Content-Type": "audio/wav" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const blob = await apiBlobRequest("/v1/media/assets/asset-id/playback", {
      accessToken: "access-token",
    });

    expect(await blob.text()).toBe("audio bytes");
    expect(fetchMock).toHaveBeenCalledOnce();
    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe("http://localhost:8000/v1/media/assets/asset-id/playback");
    expect(options.credentials).toBe("include");
    expect(new Headers(options.headers).get("Authorization")).toBe(
      "Bearer access-token",
    );
  });
});
