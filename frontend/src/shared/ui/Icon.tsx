export type IconName =
  | "album"
  | "artist"
  | "back"
  | "forward"
  | "home"
  | "library"
  | "lyrics"
  | "moon"
  | "next"
  | "pause"
  | "play"
  | "previous"
  | "profile"
  | "search"
  | "sun"
  | "volume";

export function Icon({ name, size = 20 }: { name: IconName; size?: number }) {
  return (
    <svg
      className="ui-icon"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name]}
    </svg>
  );
}

const paths: Record<IconName, React.ReactNode> = {
  home: (
    <>
      <path d="m3 11 9-8 9 8" />
      <path d="M5 10v10h14V10M9 20v-6h6v6" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-4-4" />
    </>
  ),
  library: (
    <>
      <path d="M4 4v16M9 4v16M14 6h6M14 12h6M14 18h6" />
    </>
  ),
  artist: (
    <>
      <circle cx="12" cy="8" r="4" />
      <path d="M4.5 21a7.5 7.5 0 0 1 15 0" />
    </>
  ),
  album: (
    <>
      <circle cx="12" cy="12" r="9" />
      <circle cx="12" cy="12" r="2" />
      <path d="M12 3a9 9 0 0 0 0 18" />
    </>
  ),
  profile: (
    <>
      <circle cx="12" cy="8" r="3" />
      <path d="M6 20a6 6 0 0 1 12 0" />
    </>
  ),
  sun: (
    <>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.42-1.41M17.66 6.34l1.41-1.41" />
    </>
  ),
  moon: <path d="M20.5 15.5A8.5 8.5 0 0 1 8.5 3.5a8.5 8.5 0 1 0 12 12Z" />,
  back: <path d="m15 18-6-6 6-6" />,
  forward: <path d="m9 18 6-6-6-6" />,
  play: <path d="m8 5 11 7-11 7Z" fill="currentColor" stroke="none" />,
  pause: (
    <>
      <path d="M9 5v14M15 5v14" strokeWidth="2.5" />
    </>
  ),
  previous: (
    <>
      <path d="M6 5v14M18 6l-9 6 9 6Z" />
    </>
  ),
  next: (
    <>
      <path d="M18 5v14M6 6l9 6-9 6Z" />
    </>
  ),
  volume: (
    <>
      <path d="M5 9H2v6h3l5 4V5Z" />
      <path d="M14 9a4 4 0 0 1 0 6M17 6a8 8 0 0 1 0 12" />
    </>
  ),
  lyrics: (
    <>
      <path d="M5 4h14v16H5zM8 8h8M8 12h8M8 16h5" />
    </>
  ),
};
