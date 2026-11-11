import type { AudioQuality } from "../profile/profileApi";

export function playbackPathWithQuality(
  playbackPath: string,
  quality: AudioQuality,
) {
  const url = new URL(playbackPath, "https://infinitune.invalid");
  url.searchParams.set("quality", quality);
  return `${url.pathname}${url.search}`;
}
