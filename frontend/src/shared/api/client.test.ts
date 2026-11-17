import { afterEach, describe, expect, it, vi } from "vitest";

import { type ApiError, apiRequest, publicAssetUrl } from "./client";

afterEach(() => vi.unstubAllGlobals());

describe("apiRequest network recovery", () => {
  it("converts a failed network connection into a useful retryable error", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new TypeError("Failed to fetch")),
    );
    await expect(apiRequest("/v1/catalog/tracks")).rejects.toMatchObject({
      status: 0,
      code: "network_unavailable",
      message: "Cannot reach Infinitune. Check your connection and try again.",
    } satisfies Partial<ApiError>);
  });

  it("preserves AbortError so navigating away does not display a network failure", async () => {
    const abort = new DOMException("The operation was aborted.", "AbortError");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abort));
    await expect(apiRequest("/v1/catalog/tracks")).rejects.toBe(abort);
  });

  it("does not mask HTTP errors returned by the gateway", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            error: {
              code: "service_unavailable",
              message: "Service not ready",
            },
          }),
          { status: 503, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );
    await expect(apiRequest("/v1/catalog/tracks")).rejects.toMatchObject({
      status: 503,
      code: "service_unavailable",
      message: "Service not ready",
    });
  });
});

describe("publicAssetUrl", () => {
  it("routes root-relative media paths through the configured gateway", () => {
    expect(publicAssetUrl("/v1/media/covers/release-id")).toBe(
      "http://localhost:8000/v1/media/covers/release-id",
    );
  });

  it("preserves external asset URLs", () => {
    expect(publicAssetUrl("https://cdn.example.test/cover.jpg")).toBe(
      "https://cdn.example.test/cover.jpg",
    );
  });

  it("preserves an empty asset URL", () => {
    expect(publicAssetUrl("  ")).toBe("");
  });
});
