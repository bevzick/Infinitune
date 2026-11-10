import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ProtectedRoute } from "./routeGuards";

const session = vi.hoisted(() => ({ status: "anonymous" }));

vi.mock("./SessionContext", () => ({
  useSession: () => session,
}));

describe("ProtectedRoute", () => {
  beforeEach(() => {
    session.status = "anonymous";
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
});

function LocationProbe() {
  const location = useLocation();
  return <span>{`${location.pathname}|${location.state.from}`}</span>;
}
