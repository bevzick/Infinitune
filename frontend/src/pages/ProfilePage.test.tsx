import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../shared/api/client";
import { ProfilePage } from "./ProfilePage";

const profileApi = vi.hoisted(() => ({
  getOwnProfile: vi.fn(),
  getSettings: vi.fn(),
  createProfile: vi.fn(),
  updateProfile: vi.fn(),
  updateSettings: vi.fn(),
}));
const theme = vi.hoisted(() => ({ setTheme: vi.fn() }));

vi.mock("../features/profile/profileApi", async (importOriginal) => ({
  ...(await importOriginal()),
  ...profileApi,
}));

vi.mock("../features/session/SessionContext", () => ({
  useSession: () => ({ accessToken: "access-token" }),
}));

vi.mock("../features/theme/ThemeContext", () => ({
  useTheme: () => theme,
}));

const profile = {
  user_id: "user-id",
  username: "night_listener",
  display_name: "Night Listener",
  bio: "Quiet electronic music.",
  avatar_url: "",
  country_code: "UA",
  language: "uk",
  birth_date: "2000-01-02",
  created_at: "2026-01-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

const settings = {
  theme: "system",
  autoplay: true,
  explicit_content: true,
  audio_quality: "high",
  normalize_volume: true,
  show_listening_activity: false,
  created_at: "2026-01-10T00:00:00Z",
  updated_at: "2026-11-10T00:00:00Z",
};

describe("ProfilePage", () => {
  beforeEach(() => {
    profileApi.getOwnProfile.mockResolvedValue(profile);
    profileApi.getSettings.mockResolvedValue(settings);
    profileApi.updateSettings.mockResolvedValue({
      ...settings,
      explicit_content: false,
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("loads account data and saves explicit false settings", async () => {
    const user = userEvent.setup();
    render(<ProfilePage />);

    expect(
      await screen.findByRole("heading", { name: "Night Listener" }),
    ).toBeInTheDocument();
    const explicit = screen.getByRole("checkbox", {
      name: /Allow explicit content/,
    });
    expect(explicit).toBeChecked();
    await user.click(explicit);
    await user.click(screen.getByRole("button", { name: "Save settings" }));

    expect(profileApi.updateSettings).toHaveBeenCalledWith(
      "access-token",
      expect.objectContaining({ explicit_content: false }),
    );
    expect(await screen.findByText("Settings saved.")).toBeInTheDocument();
  });

  it("creates a missing listener profile", async () => {
    profileApi.getOwnProfile
      .mockRejectedValueOnce(
        new ApiError(404, "profile_not_found", "Not found"),
      )
      .mockResolvedValue(profile);
    profileApi.createProfile.mockResolvedValue(profile);
    const user = userEvent.setup();
    render(<ProfilePage />);

    expect(
      await screen.findByRole("heading", {
        name: "Create your listener profile.",
      }),
    ).toBeInTheDocument();
    await user.type(
      screen.getByRole("textbox", { name: /^Username/ }),
      "night_listener",
    );
    await user.type(screen.getByLabelText("Display name"), "Night Listener");
    await user.click(screen.getByRole("button", { name: "Create profile" }));

    expect(profileApi.createProfile).toHaveBeenCalledWith("access-token", {
      username: "night_listener",
      display_name: "Night Listener",
    });
    expect(
      await screen.findByRole("heading", { name: "Night Listener" }),
    ).toBeInTheDocument();
  });

  it("previews the selected interface theme immediately", async () => {
    const user = userEvent.setup();
    render(<ProfilePage />);

    await user.selectOptions(
      await screen.findByRole("combobox", { name: "Interface theme" }),
      "light",
    );

    expect(theme.setTheme).toHaveBeenCalledWith("light");
  });
});
