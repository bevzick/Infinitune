import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  Link,
  MemoryRouter,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../../shared/api/client";
import {
  AdminRoute,
  GuestRoute,
  ModeratorRoute,
  ProfileOnboardingRoute,
  ProtectedRoute,
} from "./routeGuards";

const session = vi.hoisted(() => ({
  status: "anonymous",
  identity: null as { roles: string[] } | null,
  accessToken: null as string | null,
  retry: vi.fn(),
}));

const profileApi = vi.hoisted(() => ({
  getOwnProfile: vi.fn(),
  profileCreatedEvent: "infinitune:profile-created",
}));

vi.mock("./SessionContext", () => ({
  useSession: () => session,
}));

vi.mock("../profile/profileApi", () => profileApi);

afterEach(() => cleanup());

describe("ProtectedRoute", () => {
  beforeEach(() => {
    session.status = "anonymous";
    session.identity = null;
    session.accessToken = null;
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

describe("ProfileOnboardingRoute", () => {
  beforeEach(() => {
    session.status = "authenticated";
    session.identity = { roles: ["listener"] };
    session.accessToken = "access-token";
    profileApi.getOwnProfile.mockReset();
  });

  it("redirects a listener without a profile and preserves the destination", async () => {
    profileApi.getOwnProfile.mockRejectedValue(
      new ApiError(404, "profile_not_found", "Profile was not found"),
    );

    renderOnboardingRoute("/library?sort=recent");

    expect(
      await screen.findByText("/profile|/library?sort=recent"),
    ).toBeInTheDocument();
    expect(profileApi.getOwnProfile).toHaveBeenCalledWith(
      "access-token",
      expect.any(AbortSignal),
    );
  });

  it("renders the requested page when the listener profile exists", async () => {
    profileApi.getOwnProfile.mockResolvedValue({});

    renderOnboardingRoute("/library");

    expect(await screen.findByText("Private library")).toBeInTheDocument();
  });

  it("allows the profile page to create a missing profile", async () => {
    profileApi.getOwnProfile.mockRejectedValue(
      new ApiError(404, "profile_not_found", "Profile was not found"),
    );

    renderOnboardingRoute("/profile");

    expect(await screen.findByText("Profile editor")).toBeInTheDocument();
  });

  it("unlocks navigation after profile creation", async () => {
    const user = userEvent.setup();
    profileApi.getOwnProfile.mockRejectedValue(
      new ApiError(404, "profile_not_found", "Profile was not found"),
    );

    renderOnboardingRoute("/profile");
    expect(await screen.findByText("Profile editor")).toBeInTheDocument();

    window.dispatchEvent(new Event(profileApi.profileCreatedEvent));

    await user.click(screen.getByRole("link", { name: "Continue" }));

    await waitFor(() =>
      expect(screen.getByText("Private library")).toBeInTheDocument(),
    );
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

describe("ModeratorRoute", () => {
  beforeEach(() => {
    session.status = "authenticated";
    session.identity = { roles: ["listener"] };
  });

  it("redirects a listener without a moderation role", () => {
    render(
      <MemoryRouter initialEntries={["/moderation"]}>
        <Routes>
          <Route path="/" element={<LocationProbe />} />
          <Route element={<ModeratorRoute />}>
            <Route path="/moderation" element={<div>Moderation</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByText("/|")).toBeInTheDocument();
  });

  it.each([["moderator"], ["admin"]])(
    "allows a %s to access moderation",
    (role) => {
      session.identity = { roles: ["listener", role] };
      render(
        <MemoryRouter initialEntries={["/moderation"]}>
          <Routes>
            <Route element={<ModeratorRoute />}>
              <Route path="/moderation" element={<div>Moderation</div>} />
            </Route>
          </Routes>
        </MemoryRouter>,
      );

      expect(screen.getByText("Moderation")).toBeInTheDocument();
    },
  );
});

function LocationProbe() {
  const location = useLocation();
  const from = (location.state as { from?: string } | null)?.from ?? "";
  return <span>{`${location.pathname}|${from}`}</span>;
}

function renderOnboardingRoute(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route element={<ProfileOnboardingRoute />}>
          <Route path="/library" element={<div>Private library</div>} />
          <Route path="/profile" element={<ProfileProbe />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

function ProfileProbe() {
  const location = useLocation();
  const from = (location.state as { from?: string } | null)?.from ?? "";
  return (
    <div>
      <span>Profile editor</span>
      <span>{`${location.pathname}|${from}`}</span>
      <Link to="/library">Continue</Link>
    </div>
  );
}
