import { apiRequest } from "../../shared/api/client";

export type UserProfile = {
  user_id: string;
  username: string;
  display_name: string;
  bio: string;
  avatar_url: string;
  country_code: string;
  language: string;
  birth_date?: string;
  created_at: string;
  updated_at: string;
};

export type ProfileCreateInput = {
  username: string;
  display_name: string;
};

export type ProfileUpdateInput = {
  username: string;
  display_name: string;
  bio: string;
  avatar_url: string;
  country_code: string;
  language: string;
  birth_date: string | null;
};

export type InterfaceTheme = "light" | "dark" | "system";
export type AudioQuality = "low" | "medium" | "high" | "lossless";

export type UserSettings = {
  theme: InterfaceTheme;
  autoplay: boolean;
  explicit_content: boolean;
  audio_quality: AudioQuality;
  normalize_volume: boolean;
  show_listening_activity: boolean;
  created_at: string;
  updated_at: string;
};

export type SettingsUpdateInput = Omit<
  UserSettings,
  "created_at" | "updated_at"
>;

export const settingsUpdatedEvent = "infinitune:settings-updated";

export function getOwnProfile(accessToken: string, signal?: AbortSignal) {
  return apiRequest<UserProfile>("/v1/users/me/profile", {
    accessToken,
    signal,
  });
}

export function createProfile(accessToken: string, input: ProfileCreateInput) {
  return apiRequest<UserProfile>("/v1/users/me/profile", {
    method: "POST",
    accessToken,
    body: JSON.stringify(input),
  });
}

export function updateProfile(accessToken: string, input: ProfileUpdateInput) {
  return apiRequest<UserProfile>("/v1/users/me/profile", {
    method: "PATCH",
    accessToken,
    body: JSON.stringify(input),
  });
}

export function getSettings(accessToken: string, signal?: AbortSignal) {
  return apiRequest<UserSettings>("/v1/users/me/settings", {
    accessToken,
    signal,
  });
}

export async function updateSettings(
  accessToken: string,
  input: SettingsUpdateInput,
) {
  const settings = await apiRequest<UserSettings>("/v1/users/me/settings", {
    method: "PATCH",
    accessToken,
    body: JSON.stringify(input),
  });
  window.dispatchEvent(
    new CustomEvent<UserSettings>(settingsUpdatedEvent, { detail: settings }),
  );
  return settings;
}
