import { type FormEvent, useEffect, useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";

import { PlayerControls } from "../features/player/PlayerControls";
import { useSession } from "../features/session/SessionContext";
import { ThemeToggle } from "../features/theme/ThemeToggle";
import { useDocumentTitle } from "../shared/hooks/useDocumentTitle";
import { Icon, type IconName } from "../shared/ui/Icon";
import { MobileNavigation } from "./MobileNavigation";

type NavigationItem = {
  to: string;
  label: string;
  icon: IconName;
  end?: boolean;
};

const primaryNavigation: NavigationItem[] = [
  { to: "/", label: "Discover", icon: "home", end: true },
  { to: "/search", label: "Search", icon: "search" },
  { to: "/library", label: "Library", icon: "library" },
  { to: "/history", label: "History", icon: "history" },
];

const catalogNavigation: NavigationItem[] = [
  { to: "/artists", label: "Artists", icon: "artist" },
  { to: "/albums", label: "Albums", icon: "album" },
];

const creatorNavigation: NavigationItem[] = [
  { to: "/creator/accounts", label: "Accounts", icon: "artist" },
  { to: "/creator/releases", label: "Releases", icon: "album" },
  { to: "/creator/import", label: "Import music", icon: "album" },
];

export function AppLayout() {
  const session = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const [logoutError, setLogoutError] = useState("");
  const [isOnline, setIsOnline] = useState(() => navigator.onLine);

  useEffect(() => {
    const onOnline = () => setIsOnline(true);
    const onOffline = () => setIsOnline(false);
    window.addEventListener("online", onOnline);
    window.addEventListener("offline", onOffline);
    return () => {
      window.removeEventListener("online", onOnline);
      window.removeEventListener("offline", onOffline);
    };
  }, []);
  const isAdmin = session.identity?.roles.includes("admin") ?? false;
  useDocumentTitle(pageTitle(location.pathname));

  async function signOut() {
    setLogoutError("");
    try {
      await session.logout();
      navigate("/", { replace: true });
    } catch {
      setLogoutError(
        "The server could not confirm sign out. Local session was cleared.",
      );
    }
  }

  function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const query = new FormData(event.currentTarget).get("q");
    const value = typeof query === "string" ? query.trim() : "";
    navigate(value ? `/search?q=${encodeURIComponent(value)}` : "/search");
  }

  return (
    <div className="app-shell">
      <button
        className="skip-link"
        type="button"
        onClick={() => document.getElementById("main-content")?.focus()}
      >
        Skip to content
      </button>

      <aside className="sidebar" aria-label="Primary navigation">
        <Brand />
        <Navigation items={primaryNavigation} />
        <div className="sidebar-section">
          <span className="sidebar-label">Browse music</span>
          <Navigation items={catalogNavigation} />
        </div>
        {session.status === "authenticated" ? (
          <div className="sidebar-section">
            <span className="sidebar-label">Creator studio</span>
            <Navigation items={creatorNavigation} />
          </div>
        ) : null}
        {isAdmin ? (
          <div className="sidebar-section">
            <span className="sidebar-label">Operations</span>
            <Navigation
              items={[{ to: "/admin", label: "Administration", icon: "admin" }]}
            />
          </div>
        ) : null}
        <div className="sidebar-note">
          <span className="status-dot" aria-hidden="true" />
          <span>Music services online</span>
        </div>
      </aside>

      <div className="content-column">
        <header className="topbar">
          <NavLink className="mobile-brand" to="/" aria-label="Infinitune home">
            <span className="brand-mark" aria-hidden="true">
              ∞
            </span>
            <span>Infinitune</span>
          </NavLink>
          <div className="topbar-history">
            <button
              className="icon-button"
              type="button"
              aria-label="Go back"
              onClick={() => navigate(-1)}
            >
              <Icon name="back" />
            </button>
            <button
              className="icon-button"
              type="button"
              aria-label="Go forward"
              onClick={() => navigate(1)}
            >
              <Icon name="forward" />
            </button>
          </div>
          <form className="topbar-search" onSubmit={search}>
            <Icon name="search" size={18} />
            <label className="visually-hidden" htmlFor="global-search">
              Search tracks, artists, and albums
            </label>
            <input
              id="global-search"
              name="q"
              type="search"
              placeholder="Search tracks, artists, albums…"
            />
          </form>
          <div className="topbar-actions">
            <ThemeToggle />
            {session.status === "authenticated" ? (
              <>
                <NavLink className="profile-link" to="/profile">
                  <span className="profile-label">Profile</span>
                  <span className="profile-avatar-small" aria-hidden="true">
                    {session.identity?.userId.slice(0, 1) || "I"}
                  </span>
                </NavLink>
                <button
                  className="sign-in-link session-button"
                  type="button"
                  onClick={signOut}
                >
                  Sign out
                </button>
              </>
            ) : (
              <NavLink className="sign-in-link" to="/login">
                Sign in
              </NavLink>
            )}
          </div>
        </header>
        {!isOnline ? (
          <p className="network-banner" role="status">
            You are offline. Some music and account actions may be unavailable.
            Reconnect to continue.
          </p>
        ) : null}
        {logoutError ? (
          <p className="session-banner" role="alert">
            {logoutError}
          </p>
        ) : null}
        <main id="main-content" className="page-content" tabIndex={-1}>
          <Outlet />
        </main>
      </div>

      <PlayerControls />
      <MobileNavigation
        authenticated={session.status === "authenticated"}
        isAdmin={isAdmin}
      />
    </div>
  );
}

export function pageTitle(pathname: string) {
  if (pathname === "/") return "Discover";
  if (pathname === "/search") return "Search";
  if (pathname === "/artists") return "Artists";
  if (pathname.startsWith("/artists/")) return "Artist";
  if (pathname === "/albums") return "Albums";
  if (pathname.startsWith("/albums/")) return "Album";
  if (pathname.startsWith("/tracks/")) return "Track";
  if (pathname === "/library") return "Library";
  if (pathname === "/history") return "Listening history";
  if (pathname.startsWith("/library/")) return "Playlist";
  if (pathname === "/profile") return "Profile";
  if (pathname === "/creator/import") return "Import music";
  if (pathname === "/creator/releases") return "Creator releases";
  if (pathname === "/creator/accounts") return "Creator accounts";
  if (pathname === "/admin") return "Administration";
  return "Infinitune";
}

function Brand() {
  return (
    <NavLink className="brand" to="/" aria-label="Infinitune home">
      <span className="brand-mark" aria-hidden="true">
        ∞
      </span>
      <span>Infinitune</span>
    </NavLink>
  );
}

function Navigation({ items }: { items: NavigationItem[] }) {
  return (
    <nav className="nav-list">
      {items.map((item) => (
        <NavigationLink item={item} key={item.to} />
      ))}
    </nav>
  );
}

function NavigationLink({ item }: { item: NavigationItem }) {
  return (
    <NavLink
      className={({ isActive }) =>
        `nav-link${isActive ? " nav-link-active" : ""}`
      }
      to={item.to}
      end={item.end}
    >
      <span className="nav-icon" aria-hidden="true">
        <Icon name={item.icon} />
      </span>
      <span className="nav-label">{item.label}</span>
    </NavLink>
  );
}
