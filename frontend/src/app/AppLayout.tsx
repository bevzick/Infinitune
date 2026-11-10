import { useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";

import { useSession } from "../features/session/SessionContext";

const navigation = [
  { to: "/", label: "Discover", icon: "✦" },
  { to: "/search", label: "Search", icon: "⌕" },
  { to: "/library", label: "Library", icon: "◫" },
] as const;

export function AppLayout() {
  const session = useSession();
  const navigate = useNavigate();
  const [logoutError, setLogoutError] = useState("");

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

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>
      <aside className="sidebar" aria-label="Primary navigation">
        <NavLink className="brand" to="/" aria-label="Infinitune home">
          <span className="brand-mark" aria-hidden="true">
            ∞
          </span>
          <span>Infinitune</span>
        </NavLink>
        <nav className="nav-list">
          {navigation.map((item) => (
            <NavLink
              className={({ isActive }) =>
                `nav-link${isActive ? " nav-link-active" : ""}`
              }
              key={item.to}
              to={item.to}
              end={item.to === "/"}
            >
              <span aria-hidden="true">{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-note">
          <span className="status-dot" aria-hidden="true" />
          <span>Gateway ready</span>
        </div>
      </aside>

      <div className="content-column">
        <header className="topbar">
          <div>
            <span className="eyebrow">Your sound, uninterrupted</span>
          </div>
          {session.status === "authenticated" ? (
            <button
              className="sign-in-link session-button"
              type="button"
              onClick={signOut}
            >
              Sign out
            </button>
          ) : (
            <NavLink className="sign-in-link" to="/login">
              Sign in
            </NavLink>
          )}
        </header>
        {logoutError ? (
          <p className="session-banner" role="alert">
            {logoutError}
          </p>
        ) : null}
        <main id="main-content" className="page-content">
          <Outlet />
        </main>
      </div>

      <section className="player-shell" aria-label="Audio player">
        <div className="player-art" aria-hidden="true">
          ∞
        </div>
        <div className="player-copy">
          <strong>Nothing playing yet</strong>
          <span>Choose a track to begin</span>
        </div>
        <button
          className="play-button"
          type="button"
          disabled
          aria-label="Play"
        >
          ▶
        </button>
        <div className="player-progress" aria-hidden="true">
          <span />
        </div>
        <span className="player-time">0:00</span>
      </section>
    </div>
  );
}
