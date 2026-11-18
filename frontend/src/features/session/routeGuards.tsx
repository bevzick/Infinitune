import { useEffect, useState } from "react";
import { Navigate, Outlet, useLocation } from "react-router-dom";

import { ApiError } from "../../shared/api/client";
import { getOwnProfile, profileCreatedEvent } from "../profile/profileApi";
import { useSession } from "./SessionContext";

type ProfileStatus = "checking" | "ready" | "missing" | "error";

export function ProfileOnboardingRoute() {
  const session = useSession();
  const location = useLocation();
  const [status, setStatus] = useState<ProfileStatus>("checking");
  const [attempt, setAttempt] = useState(0);

  // biome-ignore lint/correctness/useExhaustiveDependencies: the attempt counter intentionally restarts the request after a retry.
  useEffect(() => {
    if (session.status !== "authenticated" || !session.accessToken) {
      setStatus("checking");
      return;
    }
    const controller = new AbortController();
    setStatus("checking");
    getOwnProfile(session.accessToken, controller.signal)
      .then(() => setStatus("ready"))
      .catch((reason: unknown) => {
        if (controller.signal.aborted) return;
        if (
          reason instanceof ApiError &&
          reason.status === 404 &&
          reason.code === "profile_not_found"
        ) {
          setStatus("missing");
          return;
        }
        setStatus("error");
      });
    return () => controller.abort();
  }, [attempt, session.accessToken, session.status]);

  useEffect(() => {
    const markReady = () => setStatus("ready");
    window.addEventListener(profileCreatedEvent, markReady);
    return () => window.removeEventListener(profileCreatedEvent, markReady);
  }, []);

  if (session.status === "loading") return <SessionLoading />;
  if (session.status !== "authenticated") return <Outlet />;
  if (status === "checking") return <ProfileLoading />;
  if (status === "error") {
    return (
      <ProfileUnavailable onRetry={() => setAttempt((value) => value + 1)} />
    );
  }
  if (status === "missing" && location.pathname !== "/profile") {
    return (
      <Navigate
        to="/profile"
        replace
        state={{ from: location.pathname + location.search }}
      />
    );
  }
  return <Outlet />;
}

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

export function AdminRoute() {
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
  if (!session.identity?.roles.includes("admin")) {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}

export function ModeratorRoute() {
  const session = useSession();
  const location = useLocation();
  if (session.status === "loading") return <SessionLoading />;
  if (session.status === "error") {
    return <SessionUnavailable onRetry={session.retry} />;
  }
  if (session.status === "anonymous") {
    return (
      <Navigate
        to="/login"
        replace
        state={{ from: location.pathname + location.search }}
      />
    );
  }
  const roles = session.identity?.roles ?? [];
  if (!roles.includes("moderator") && !roles.includes("admin")) {
    return <Navigate to="/" replace />;
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

function ProfileLoading() {
  return (
    <main className="session-loading" aria-live="polite">
      <span className="loading-pulse" aria-hidden="true" />
      Loading your listener profile…
    </main>
  );
}

function ProfileUnavailable({ onRetry }: { onRetry(): void }) {
  return (
    <main className="session-loading" role="alert">
      <div className="session-unavailable">
        <strong>Profile service is unavailable</strong>
        <span>We could not check your listener profile.</span>
        <button type="button" onClick={onRetry}>
          Try again
        </button>
      </div>
    </main>
  );
}
