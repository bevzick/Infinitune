import { type FormEvent, useCallback, useEffect, useState } from "react";

import {
  createProfile,
  getOwnProfile,
  getSettings,
  type ProfileUpdateInput,
  type SettingsUpdateInput,
  type UserProfile,
  type UserSettings,
  updateProfile,
  updateSettings,
} from "../features/profile/profileApi";
import { useSession } from "../features/session/SessionContext";
import { useTheme } from "../features/theme/ThemeContext";
import { ApiError } from "../shared/api/client";
import { LibraryError, LibraryLoading } from "./LibraryPage";

export function ProfilePage() {
  const { accessToken } = useSession();
  const { setTheme } = useTheme();
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [status, setStatus] = useState<
    "loading" | "missing" | "ready" | "error"
  >("loading");
  const [error, setError] = useState("");

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!accessToken) return;
      setStatus("loading");
      setError("");
      try {
        const currentProfile = await getOwnProfile(accessToken, signal);
        const currentSettings = await getSettings(accessToken, signal);
        setProfile(currentProfile);
        setSettings(currentSettings);
        setTheme(resolveTheme(currentSettings.theme));
        setStatus("ready");
      } catch (reason) {
        if (signal?.aborted) return;
        if (reason instanceof ApiError && reason.status === 404) {
          setProfile(null);
          setSettings(null);
          setStatus("missing");
          return;
        }
        setError(apiError(reason, "Could not load your profile"));
        setStatus("error");
      }
    },
    [accessToken, setTheme],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  if (status === "loading") return <LibraryLoading />;
  if (status === "error") {
    return <LibraryError message={error} onRetry={() => void load()} />;
  }
  if (status === "missing") {
    return (
      <ProfileOnboarding
        onCreated={async () => {
          await load();
        }}
      />
    );
  }
  if (!profile || !settings) return null;

  return (
    <section className="profile-page">
      <ProfileHero profile={profile} />
      <div className="profile-sections">
        <ProfileEditor profile={profile} onChange={setProfile} />
        <SettingsEditor settings={settings} onChange={setSettings} />
      </div>
    </section>
  );
}

function ProfileOnboarding({ onCreated }: { onCreated(): Promise<void> }) {
  const { accessToken } = useSession();
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) return;
    setBusy(true);
    setError("");
    try {
      await createProfile(accessToken, {
        username,
        display_name: displayName,
      });
      await onCreated();
    } catch (reason) {
      setError(apiError(reason, "Could not create your profile"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="profile-onboarding">
      <span className="profile-orbit" aria-hidden="true">
        ◎
      </span>
      <div>
        <span className="hero-kicker">One last step</span>
        <h1>Create your listener profile.</h1>
        <p>
          Choose how you appear across Infinitune. You can add the rest of your
          details and listening settings next.
        </p>
        <form className="profile-create-form" onSubmit={submit}>
          <label>
            Username
            <input
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              minLength={3}
              maxLength={50}
              pattern="[a-z0-9_]+"
              autoComplete="username"
              placeholder="night_listener"
              required
            />
            <small>Lowercase letters, numbers, and underscores.</small>
          </label>
          <label>
            Display name
            <input
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              maxLength={100}
              autoComplete="name"
              required
            />
          </label>
          {error ? (
            <p className="form-error" role="alert">
              {error}
            </p>
          ) : null}
          <button type="submit" disabled={busy}>
            {busy ? "Creating…" : "Create profile"}
          </button>
        </form>
      </div>
    </section>
  );
}

function ProfileHero({ profile }: { profile: UserProfile }) {
  return (
    <header className="profile-hero">
      <span className="profile-avatar" aria-hidden="true">
        {initials(profile.display_name || profile.username)}
        {profile.avatar_url ? <img src={profile.avatar_url} alt="" /> : null}
      </span>
      <div>
        <span className="hero-kicker">Listener profile</span>
        <h1>{profile.display_name}</h1>
        <p>@{profile.username}</p>
        <small>
          Member since {formatMonth(profile.created_at)}
          {profile.country_code && profile.country_code !== "ZZ"
            ? ` · ${profile.country_code}`
            : ""}
        </small>
      </div>
    </header>
  );
}

function ProfileEditor({
  profile,
  onChange,
}: {
  profile: UserProfile;
  onChange(profile: UserProfile): void;
}) {
  const { accessToken } = useSession();
  const [draft, setDraft] = useState<ProfileUpdateInput>(() =>
    profileDraft(profile),
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  function update<K extends keyof ProfileUpdateInput>(
    field: K,
    value: ProfileUpdateInput[K],
  ) {
    setSaved(false);
    setDraft((current) => ({ ...current, [field]: value }));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) return;
    setBusy(true);
    setError("");
    setSaved(false);
    try {
      const updated = await updateProfile(accessToken, draft);
      onChange(updated);
      setDraft(profileDraft(updated));
      setSaved(true);
    } catch (reason) {
      setError(apiError(reason, "Could not save your profile"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="profile-panel" aria-labelledby="profile-details-title">
      <div className="profile-panel-heading">
        <div>
          <span className="eyebrow">Public identity</span>
          <h2 className="profile-panel-title" id="profile-details-title">
            Profile details
          </h2>
        </div>
        <span className="profile-panel-note">
          Visible fields appear on your public profile.
        </span>
      </div>
      <form className="profile-form" onSubmit={submit}>
        <label>
          Username
          <input
            value={draft.username}
            onChange={(event) => update("username", event.target.value)}
            minLength={3}
            maxLength={50}
            pattern="[a-z0-9_]+"
            autoComplete="username"
            required
          />
        </label>
        <label>
          Display name
          <input
            value={draft.display_name}
            onChange={(event) => update("display_name", event.target.value)}
            maxLength={100}
            autoComplete="name"
            required
          />
        </label>
        <label className="profile-wide-field">
          Bio
          <textarea
            value={draft.bio}
            onChange={(event) => update("bio", event.target.value)}
            maxLength={500}
            rows={5}
            placeholder="A few words about your listening world."
          />
          <small>{draft.bio.length}/500 characters</small>
        </label>
        <label className="profile-wide-field">
          Avatar URL
          <input
            type="url"
            value={draft.avatar_url}
            onChange={(event) => update("avatar_url", event.target.value)}
            maxLength={2048}
            placeholder="https://example.com/avatar.jpg"
          />
        </label>
        <label>
          Country code
          <input
            value={draft.country_code === "ZZ" ? "" : draft.country_code}
            onChange={(event) =>
              update("country_code", event.target.value.toUpperCase())
            }
            minLength={2}
            maxLength={2}
            pattern="[A-Za-z]{2}"
            autoComplete="country"
            placeholder="UA"
          />
        </label>
        <label>
          Language
          <input
            value={draft.language === "und" ? "" : draft.language}
            onChange={(event) => update("language", event.target.value)}
            maxLength={10}
            placeholder="uk"
          />
        </label>
        <label>
          Birth date
          <input
            type="date"
            value={draft.birth_date ?? ""}
            onChange={(event) =>
              update("birth_date", event.target.value || null)
            }
            max={new Date().toISOString().slice(0, 10)}
          />
          <small>Private and never shown publicly.</small>
        </label>
        <div className="profile-form-actions">
          <button type="submit" disabled={busy}>
            {busy ? "Saving…" : "Save profile"}
          </button>
          {saved ? <span role="status">Profile saved.</span> : null}
          {error ? (
            <span className="form-error" role="alert">
              {error}
            </span>
          ) : null}
        </div>
      </form>
    </section>
  );
}

function SettingsEditor({
  settings,
  onChange,
}: {
  settings: UserSettings;
  onChange(settings: UserSettings): void;
}) {
  const { accessToken } = useSession();
  const { setTheme } = useTheme();
  const [draft, setDraft] = useState<SettingsUpdateInput>(() =>
    settingsDraft(settings),
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  function update<K extends keyof SettingsUpdateInput>(
    field: K,
    value: SettingsUpdateInput[K],
  ) {
    setSaved(false);
    setDraft((current) => ({ ...current, [field]: value }));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!accessToken) return;
    setBusy(true);
    setError("");
    setSaved(false);
    try {
      const updated = await updateSettings(accessToken, draft);
      onChange(updated);
      setDraft(settingsDraft(updated));
      setTheme(resolveTheme(updated.theme));
      setSaved(true);
    } catch (reason) {
      setError(apiError(reason, "Could not save your settings"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section
      className="profile-panel"
      aria-labelledby="listening-settings-title"
    >
      <div className="profile-panel-heading">
        <div>
          <span className="eyebrow">Playback & privacy</span>
          <h2 className="profile-panel-title" id="listening-settings-title">
            Listening settings
          </h2>
        </div>
        <span className="profile-panel-note">
          Saved to your account across devices.
        </span>
      </div>
      <form className="settings-form" onSubmit={submit}>
        <div className="settings-selects">
          <label>
            Interface theme
            <select
              value={draft.theme}
              onChange={(event) => {
                const preference = event.target
                  .value as SettingsUpdateInput["theme"];
                update("theme", preference);
                setTheme(resolveTheme(preference));
              }}
            >
              <option value="system">Use system setting</option>
              <option value="dark">Dark</option>
              <option value="light">Light</option>
            </select>
          </label>
          <label>
            Audio quality
            <select
              value={draft.audio_quality}
              onChange={(event) =>
                update(
                  "audio_quality",
                  event.target.value as SettingsUpdateInput["audio_quality"],
                )
              }
            >
              <option value="low">Low — saves data</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="lossless">Lossless</option>
            </select>
          </label>
        </div>
        <div className="setting-toggle-list">
          <SettingToggle
            label="Autoplay"
            description="Continue with the next track in the current queue."
            checked={draft.autoplay}
            onChange={(value) => update("autoplay", value)}
          />
          <SettingToggle
            label="Allow explicit content"
            description="Keep explicit tracks available while browsing and listening."
            checked={draft.explicit_content}
            onChange={(value) => update("explicit_content", value)}
          />
          <SettingToggle
            label="Normalize volume"
            description="Prefer a consistent loudness between tracks."
            checked={draft.normalize_volume}
            onChange={(value) => update("normalize_volume", value)}
          />
          <SettingToggle
            label="Show listening activity"
            description="Allow your recent listening activity to be visible where supported."
            checked={draft.show_listening_activity}
            onChange={(value) => update("show_listening_activity", value)}
          />
        </div>
        <div className="profile-form-actions">
          <button type="submit" disabled={busy}>
            {busy ? "Saving…" : "Save settings"}
          </button>
          {saved ? <span role="status">Settings saved.</span> : null}
          {error ? (
            <span className="form-error" role="alert">
              {error}
            </span>
          ) : null}
        </div>
      </form>
    </section>
  );
}

function SettingToggle({
  label,
  description,
  checked,
  onChange,
}: {
  label: string;
  description: string;
  checked: boolean;
  onChange(value: boolean): void;
}) {
  return (
    <label className="setting-toggle">
      <span className="setting-toggle-copy">
        <strong>{label}</strong>
        <small className="setting-toggle-description">{description}</small>
      </span>
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
      />
      <i aria-hidden="true" />
    </label>
  );
}

function profileDraft(profile: UserProfile): ProfileUpdateInput {
  return {
    username: profile.username,
    display_name: profile.display_name,
    bio: profile.bio,
    avatar_url: profile.avatar_url,
    country_code: profile.country_code,
    language: profile.language,
    birth_date: profile.birth_date ?? null,
  };
}

function settingsDraft(settings: UserSettings): SettingsUpdateInput {
  return {
    theme: settings.theme,
    autoplay: settings.autoplay,
    explicit_content: settings.explicit_content,
    audio_quality: settings.audio_quality,
    normalize_volume: settings.normalize_volume,
    show_listening_activity: settings.show_listening_activity,
  };
}

function resolveTheme(preference: SettingsUpdateInput["theme"]) {
  if (preference === "light" || preference === "dark") return preference;
  return window.matchMedia?.("(prefers-color-scheme: light)").matches
    ? "light"
    : "dark";
}

function apiError(reason: unknown, fallback: string) {
  return reason instanceof ApiError ? reason.message : fallback;
}

function initials(value: string) {
  const words = value.trim().split(/\s+/).filter(Boolean);
  return words
    .slice(0, 2)
    .map((word) => word[0]?.toUpperCase())
    .join("");
}

function formatMonth(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? "recently"
    : new Intl.DateTimeFormat(undefined, {
        month: "long",
        year: "numeric",
      }).format(date);
}
