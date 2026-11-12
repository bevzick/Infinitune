import { Navigate, Outlet, useLocation } from "react-router-dom";

import { useSession } from "./SessionContext";

export function ProtectedRoute() {
  const session = useSession();
  const location = useLocation();
  if (session.status === "loading") return <SessionLoading />;
  if (session.status === "error")
    return <SessionUnavailable onRetry={session.retry} />;
  if (session.status === "anonymous") {
    return (
      <Navigate
        to="/login"
        replace
        state={{ from: location.pathname + location.search }}
      />
    );
  }
  return <Outlet />;
}

export function GuestRoute() {
  const session = useSession();
  const location = useLocation();
  if (session.status === "loading") return <SessionLoading />;
  if (session.status === "error")
    return <SessionUnavailable onRetry={session.retry} />;
  if (session.status === "authenticated") {
    return <Navigate to={guestDestination(location.state)} replace />;
  }
  return <Outlet />;
}

function guestDestination(state: unknown) {
  if (!state || typeof state !== "object" || !("from" in state)) return "/";
  const from = state.from;
  return typeof from === "string" &&
    from.startsWith("/") &&
    !from.startsWith("//")
    ? from
    : "/";
}

function SessionUnavailable({ onRetry }: { onRetry(): void }) {
  return (
    <main className="session-loading" role="alert">
      <div className="session-unavailable">
        <strong>Session service is unavailable</strong>
        <span>Check your connection and try again.</span>
        <button type="button" onClick={onRetry}>
          Try again
        </button>
      </div>
    </main>
  );
}

function SessionLoading() {
  return (
    <main className="session-loading" aria-live="polite">
      <span className="loading-pulse" aria-hidden="true" />
      Restoring your session…
    </main>
  );
}
