import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AdminRoute, GuestRoute, ProtectedRoute } from "./routeGuards";

const session = vi.hoisted(() => ({
  status: "anonymous",
  identity: null as { roles: string[] } | null,
}));

vi.mock("./SessionContext", () => ({
  useSession: () => session,
}));

describe("ProtectedRoute", () => {
  beforeEach(() => {
    session.status = "anonymous";
    session.identity = null;
  });

  it("redirects an anonymous visitor and preserves the requested route", () => {
    render(
      <MemoryRouter initialEntries={["/library?sort=recent"]}>
        <Routes>
          <Route path="/login" element={<LocationProbe />} />
          <Route element={<ProtectedRoute />}>
            <Route path="/library" element={<div>Private library</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText("/login|/library?sort=recent")).toBeInTheDocument();
    expect(screen.queryByText("Private library")).not.toBeInTheDocument();
  });

  it("returns an authenticated listener to the preserved route", () => {
    session.status = "authenticated";
    render(
      <MemoryRouter
        initialEntries={[
          { pathname: "/login", state: { from: "/library?sort=recent" } },
        ]}
      >
        <Routes>
          <Route path="/library" element={<LocationProbe />} />
          <Route element={<GuestRoute />}>
            <Route path="/login" element={<div>Login form</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText("/library|")).toBeInTheDocument();
    expect(screen.queryByText("Login form")).not.toBeInTheDocument();
  });
});

describe("AdminRoute", () => {
  beforeEach(() => {
    session.status = "authenticated";
    session.identity = { roles: ["listener"] };
  });

  it("redirects an authenticated non-admin user to the application", () => {
    render(
      <MemoryRouter initialEntries={["/admin"]}>
        <Routes>
          <Route path="/" element={<LocationProbe />} />
          <Route element={<AdminRoute />}>
            <Route path="/admin" element={<div>Administration</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText("/|")).toBeInTheDocument();
    expect(screen.queryByText("Administration")).not.toBeInTheDocument();
  });

  it("renders the protected workspace for an admin", () => {
    session.identity = { roles: ["listener", "admin"] };
    render(
      <MemoryRouter initialEntries={["/admin"]}>
        <Routes>
          <Route element={<AdminRoute />}>
            <Route path="/admin" element={<div>Administration</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText("Administration")).toBeInTheDocument();
  });
});

function LocationProbe() {
  const location = useLocation();
  const from = (location.state as { from?: string } | null)?.from ?? "";
  return <span>{`${location.pathname}|${from}`}</span>;
}
