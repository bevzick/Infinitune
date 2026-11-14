import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../shared/api/client";
import {
  archiveRelease,
  getReleaseReadiness,
  publishReleaseWithRetry,
  submitRelease,
  withdrawRelease,
} from "./creatorLifecycleApi";

function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("creator release lifecycle API", () => {
  it("requests readiness and uses the existing transition methods", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(async () =>
        response({ status: "draft", issues: [] }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await getReleaseReadiness("token", "release-1");
    await submitRelease("token", "release-1");
    await withdrawRelease("token", "release-1");
    await archiveRelease("token", "release-1");

    expect(fetchMock.mock.calls.map(([path]) => path)).toEqual([
      "http://localhost:8000/v1/releases/release-1/readiness",
      "http://localhost:8000/v1/releases/release-1/submissions",
      "http://localhost:8000/v1/releases/release-1/withdrawals",
      "http://localhost:8000/v1/releases/release-1",
    ]);
    expect(fetchMock.mock.calls.map(([, options]) => options.method)).toEqual([
      undefined,
      "POST",
      "POST",
      "DELETE",
    ]);
    for (const [, options] of fetchMock.mock.calls) {
      expect(options.headers.get("Authorization")).toBe("Bearer token");
    }
  });

  it("retries temporary publication readiness conflict at most twice", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        response(
          { error: { code: "release_not_ready", message: "Not yet" } },
          409,
        ),
      )
      .mockResolvedValueOnce(
        response(
          { error: { code: "release_not_ready", message: "Not yet" } },
          409,
        ),
      )
      .mockResolvedValueOnce(response({ status: "published" }));
    vi.stubGlobal("fetch", fetchMock);
    const delay = vi.fn(async (_ms: number) => {});

    const result = await publishReleaseWithRetry("token", "release-1", delay);
    expect(result.status).toBe("published");
    expect(delay.mock.calls).toEqual([[750], [1500]]);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock.mock.calls[0][1].method).toBe("POST");
    expect(fetchMock.mock.calls[0][0]).toContain("/publications");
  });

  it("does not retry other conflicts or unboundedly retry a pending Catalog", async () => {
    const conflict = vi
      .fn()
      .mockResolvedValue(
        response({ error: { code: "release_publication_conflict" } }, 409),
      );
    vi.stubGlobal("fetch", conflict);
    const delay = vi.fn(async (_ms: number) => {});
    await expect(
      publishReleaseWithRetry("token", "release-1", delay),
    ).rejects.toMatchObject({ code: "release_publication_conflict" });
    expect(conflict).toHaveBeenCalledTimes(1);
    expect(delay).not.toHaveBeenCalled();

    const pending = vi
      .fn()
      .mockImplementation(async () =>
        response({ error: { code: "release_not_ready" } }, 409),
      );
    vi.stubGlobal("fetch", pending);
    await expect(
      publishReleaseWithRetry("token", "release-1", delay),
    ).rejects.toBeInstanceOf(ApiError);
    expect(pending).toHaveBeenCalledTimes(3);
    expect(delay).toHaveBeenCalledTimes(2);
  });
});
