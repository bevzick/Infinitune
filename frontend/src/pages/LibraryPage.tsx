import { useSession } from "../features/session/SessionContext";

export function LibraryPage() {
  const { identity } = useSession();
  return (
    <section className="message-page">
      <span className="eyebrow">Private route · Session active</span>
      <h1>Your library is ready for music.</h1>
      <p>
        Authenticated as {identity?.userId}. Playlist and collection screens
        arrive in the next MVP steps.
      </p>
    </section>
  );
}
