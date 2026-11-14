import { describe, expect, it } from "vitest";

import { playbackPathWithQuality } from "./audioQuality";

describe("playbackPathWithQuality", () => {
  it("adds the account preference without discarding existing parameters", () => {
    expect(
      playbackPathWithQuality(
        "/v1/media/assets/asset-id/playback?token=opaque",
        "lossless",
      ),
    ).toBe("/v1/media/assets/asset-id/playback?token=opaque&quality=lossless");
  });

  it("replaces an existing quality preference", () => {
    expect(
      playbackPathWithQuality(
        "/v1/media/assets/asset-id/playback?quality=low",
        "high",
      ),
    ).toBe("/v1/media/assets/asset-id/playback?quality=high");
  });
});
