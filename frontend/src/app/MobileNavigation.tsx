import { useEffect, useRef, useState } from "react";
import { NavLink, useLocation } from "react-router-dom";

import { Icon, type IconName } from "../shared/ui/Icon";

type MobileItem = { to: string; label: string; icon: IconName; end?: boolean };

const quickLinks: MobileItem[] = [
  { to: "/", label: "Discover", icon: "home", end: true },
  { to: "/search", label: "Search", icon: "search" },
  { to: "/library", label: "Library", icon: "library" },
  { to: "/history", label: "History", icon: "history" },
];

export function MobileNavigation({
  authenticated,
  isAdmin,
}: {
  authenticated: boolean;
  isAdmin: boolean;
}) {
  const [open, setOpen] = useState(false);
  const location = useLocation();
  const rootRef = useRef<HTMLElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const creatorLinks: MobileItem[] = authenticated
    ? [
        { to: "/profile", label: "Profile", icon: "profile" },
        { to: "/creator/accounts", label: "Creator accounts", icon: "artist" },
        { to: "/creator/releases", label: "Creator releases", icon: "album" },
        { to: "/creator/import", label: "Import music", icon: "album" },
        ...(isAdmin
          ? ([
              { to: "/admin", label: "Administration", icon: "admin" },
            ] satisfies MobileItem[])
          : []),
      ]
    : [
        { to: "/login", label: "Sign in", icon: "profile" },
        { to: "/register", label: "Register", icon: "user-plus" },
      ];
  const moreLinks: MobileItem[] = [
    { to: "/artists", label: "Artists", icon: "artist" },
    { to: "/albums", label: "Albums", icon: "album" },
    ...creatorLinks,
  ];

  // Route changes close the menu, including browser back/forward navigation.
  // biome-ignore lint/correctness/useExhaustiveDependencies: location pathname is the intentional trigger.
  useEffect(() => setOpen(false), [location.pathname]);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    };
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    window.addEventListener("keydown", onKeyDown);
    window.addEventListener("pointerdown", onPointerDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("pointerdown", onPointerDown);
    };
  }, [open]);

  const moreActive = moreLinks.some((item) =>
    item.end
      ? location.pathname === item.to
      : location.pathname.startsWith(item.to),
  );

  return (
    <nav ref={rootRef} className="mobile-nav" aria-label="Mobile navigation">
      {quickLinks.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          onClick={() => setOpen(false)}
          className={({ isActive }) =>
            `nav-link${isActive ? " nav-link-active" : ""}`
          }
        >
          <span className="nav-icon" aria-hidden="true">
            <Icon name={item.icon} />
          </span>
          <span className="nav-label">{item.label}</span>
        </NavLink>
      ))}
      <div className="mobile-more">
        <button
          ref={triggerRef}
          type="button"
          className={`nav-link mobile-more-trigger${moreActive ? " nav-link-active" : ""}`}
          aria-label="More navigation"
          aria-expanded={open}
          aria-controls="mobile-more-panel"
          onClick={() => setOpen((current) => !current)}
        >
          <span className="nav-icon" aria-hidden="true">
            <Icon name="menu" />
          </span>
          <span className="nav-label">More</span>
        </button>
        {open ? (
          <div id="mobile-more-panel" className="mobile-more-panel">
            {moreLinks.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                onClick={() => setOpen(false)}
                className={({ isActive }) =>
                  `nav-link${isActive ? " nav-link-active" : ""}`
                }
              >
                <span className="nav-icon" aria-hidden="true">
                  <Icon name={item.icon} />
                </span>
                <span>{item.label}</span>
              </NavLink>
            ))}
          </div>
        ) : null}
      </div>
    </nav>
  );
}
