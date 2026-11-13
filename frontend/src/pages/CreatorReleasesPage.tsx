import { type FormEvent, useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";

import {
  type CreatorAccount,
  type CreatorRelease,
  type CreatorReleaseTrack,
  createRelease,
  listCreatorAccounts,
  listDraftReleases,
  type ReleaseType,
  updateRelease,
} from "../features/creator/creatorReleaseApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";

type TrackDraft = Omit<CreatorReleaseTrack, "catalog_track_id"> & {
  key: string;
};

type ReleaseDraft = {
  title: string;
  releaseType: ReleaseType;
  releaseDate: string;
  coverObjectKey: string;
  tracks: TrackDraft[];
};

export function CreatorReleasesPage() {
  const { accessToken } = useSession();
  const [accounts, setAccounts] = useState<CreatorAccount[]>([]);
  const [releases, setReleases] = useState<CreatorRelease[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [error, setError] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [editingId, setEditingId] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!accessToken) return;
      setStatus("loading");
      setError("");
      try {
        const [creatorAccounts, drafts] = await Promise.all([
          listCreatorAccounts(accessToken, signal),
          listDraftReleases(accessToken, signal),
        ]);
        setAccounts(creatorAccounts);
        setReleases(drafts);
        setStatus("ready");
      } catch (reason) {
        if (signal?.aborted) return;
        setError(errorMessage(reason, "Could not load creator releases"));
        setStatus("error");
      }
    },
    [accessToken],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  if (status === "loading") {
    return <CreatorReleaseState>Loading creator releases…</CreatorReleaseState>;
  }
  if (status === "error") {
    return (
      <CreatorReleaseState error>
        {error}
        <button type="button" onClick={() => void load()}>
          Try again
        </button>
      </CreatorReleaseState>
    );
  }

  return (
    <section className="creator-releases-page">
      <header className="creator-releases-heading">
        <div>
          <span className="hero-kicker">Creator studio</span>
          <h1>Release drafts</h1>
          <p>
            Prepare release metadata and track positions before uploading or
            importing audio.
          </p>
        </div>
        <button
          type="button"
          disabled={!accounts.length}
          onClick={() => {
            setShowCreate(true);
            setEditingId("");
            setNotice("");
          }}
        >
          + New release
        </button>
      </header>

      {!accounts.length ? (
        <CreatorReleaseNotice title="No creator account available">
          Create or join a creator account before preparing a release.{" "}
          <Link to="/creator/accounts">Open creator accounts</Link>
        </CreatorReleaseNotice>
      ) : null}

      {notice ? (
        <p className="creator-release-success" role="status">
          {notice}
        </p>
      ) : null}

      {showCreate ? (
        <ReleaseEditor
          accounts={accounts}
          onCancel={() => setShowCreate(false)}
          onCreate={async (creatorId, draft) => {
            if (!accessToken) return;
            const created = await createRelease(accessToken, {
              creator_id: creatorId,
              title: draft.title,
              release_type: draft.releaseType,
              release_date: draft.releaseDate,
              tracks: draft.tracks.map(trackInput),
            });
            setReleases((current) => [created, ...current]);
            setShowCreate(false);
            setNotice(`Draft “${created.title}” was created.`);
          }}
        />
      ) : null}

      <section className="creator-release-list" aria-label="Draft releases">
        {releases.length ? (
          releases.map((release) => (
            <article className="creator-release-card" key={release.id}>
              <div className="creator-release-card-heading">
                <span className="creator-release-cover" aria-hidden="true">
                  ◎
                </span>
                <div>
                  <span className="eyebrow">{release.release_type}</span>
                  <h2>{release.title}</h2>
                  <p>
                    {release.release_date} · {release.tracks.length}{" "}
                    {trackWord(release.tracks.length)}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => {
                    setEditingId((current) =>
                      current === release.id ? "" : release.id,
                    );
                    setShowCreate(false);
                    setNotice("");
                  }}
                >
                  {editingId === release.id ? "Close" : "Edit draft"}
                </button>
              </div>
              <ol className="creator-release-track-summary">
                {release.tracks.map((track) => (
                  <li key={track.id}>
                    <span>
                      {track.disc_number}.{track.track_number}
                    </span>
                    <strong>{track.title}</strong>
                    {track.explicit ? <small>Explicit</small> : null}
                  </li>
                ))}
              </ol>
              {editingId === release.id ? (
                <ReleaseEditor
                  release={release}
                  accounts={accounts}
                  onCancel={() => setEditingId("")}
                  onUpdate={async (draft) => {
                    if (!accessToken) return;
                    const updated = await updateRelease(
                      accessToken,
                      release.id,
                      {
                        title: draft.title,
                        release_type: draft.releaseType,
                        release_date: draft.releaseDate,
                        cover_object_key: draft.coverObjectKey.trim() || null,
                        tracks: draft.tracks.map(
                          ({ key: _key, ...track }) => track,
                        ),
                      },
                    );
                    setReleases((current) =>
                      current.map((item) =>
                        item.id === updated.id ? updated : item,
                      ),
                    );
                    setEditingId("");
                    setNotice(`Draft “${updated.title}” was updated.`);
                  }}
                />
              ) : null}
            </article>
          ))
        ) : (
          <CreatorReleaseNotice title="No draft releases yet">
            Create the first release and define its complete initial track list.
          </CreatorReleaseNotice>
        )}
      </section>
    </section>
  );
}

function ReleaseEditor({
  accounts,
  release,
  onCancel,
  onCreate,
  onUpdate,
}: {
  accounts: CreatorAccount[];
  release?: CreatorRelease;
  onCancel(): void;
  onCreate?(creatorId: string, draft: ReleaseDraft): Promise<void>;
  onUpdate?(draft: ReleaseDraft): Promise<void>;
}) {
  const [creatorId, setCreatorId] = useState(
    release?.creator_id || accounts[0]?.id || "",
  );
  const [draft, setDraft] = useState<ReleaseDraft>(() => draftFrom(release));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const editing = Boolean(release);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (editing) await onUpdate?.(draft);
      else await onCreate?.(creatorId, draft);
    } catch (reason) {
      setError(errorMessage(reason, "Could not save this release"));
    } finally {
      setBusy(false);
    }
  }

  function changeType(releaseType: ReleaseType) {
    setDraft((current) => ({
      ...current,
      releaseType,
      tracks:
        releaseType === "single" && !editing
          ? [{ ...current.tracks[0], disc_number: 1, track_number: 1 }]
          : current.tracks,
    }));
  }

  return (
    <form className="creator-release-editor" onSubmit={submit}>
      <header>
        <div>
          <span className="eyebrow">
            {editing ? "Edit metadata" : "New draft"}
          </span>
          <h2>{editing ? release?.title : "Create a release"}</h2>
        </div>
        <button
          className="creator-release-secondary-action"
          type="button"
          onClick={onCancel}
        >
          Cancel
        </button>
      </header>
      <div className="creator-release-fields">
        {!editing ? (
          <label>
            Creator account
            <select
              value={creatorId}
              onChange={(event) => setCreatorId(event.currentTarget.value)}
              required
            >
              {accounts.map((account) => (
                <option value={account.id} key={account.id}>
                  {account.display_name} · {account.type}
                </option>
              ))}
            </select>
          </label>
        ) : null}
        <label>
          Release title
          <input
            value={draft.title}
            maxLength={200}
            onChange={(event) => {
              const title = event.currentTarget.value;
              setDraft((current) => ({
                ...current,
                title,
              }));
            }}
            required
          />
        </label>
        <label>
          Release type
          <select
            value={draft.releaseType}
            onChange={(event) =>
              changeType(event.currentTarget.value as ReleaseType)
            }
          >
            <option
              value="single"
              disabled={editing && draft.tracks.length !== 1}
            >
              Single
            </option>
            <option value="ep">EP</option>
            <option value="album">Album</option>
            <option value="compilation">Compilation</option>
          </select>
        </label>
        <label>
          Release date
          <input
            type="date"
            value={draft.releaseDate}
            onChange={(event) => {
              const releaseDate = event.currentTarget.value;
              setDraft((current) => ({
                ...current,
                releaseDate,
              }));
            }}
            required
          />
        </label>
        {editing ? (
          <label className="creator-release-cover-field">
            Cover object key
            <input
              value={draft.coverObjectKey}
              onChange={(event) => {
                const coverObjectKey = event.currentTarget.value;
                setDraft((current) => ({
                  ...current,
                  coverObjectKey,
                }));
              }}
              placeholder="creator-content/covers/release.jpg"
            />
          </label>
        ) : null}
      </div>

      <div className="creator-track-editor-heading">
        <div>
          <h3>Tracks</h3>
          <p>
            {editing
              ? "Track identities are preserved after draft creation."
              : "Define the complete initial track list."}
          </p>
        </div>
        {!editing && draft.releaseType !== "single" ? (
          <button
            className="creator-release-secondary-action"
            type="button"
            onClick={() =>
              setDraft((current) => ({
                ...current,
                tracks: [...current.tracks, newTrack(current.tracks.length)],
              }))
            }
          >
            + Add track
          </button>
        ) : null}
      </div>

      <div className="creator-track-editor-list">
        {draft.tracks.map((track, index) => (
          <div className="creator-track-editor-row" key={track.key}>
            <span>{index + 1}</span>
            <label>
              Title
              <input
                value={track.title}
                maxLength={200}
                onChange={(event) => {
                  const title = event.currentTarget.value;
                  setDraft((current) => updateTrack(current, index, { title }));
                }}
                required
              />
            </label>
            <label>
              Disc
              <input
                type="number"
                min={1}
                max={32767}
                value={track.disc_number}
                onChange={(event) => {
                  const discNumber = event.currentTarget.valueAsNumber;
                  setDraft((current) =>
                    updateTrack(current, index, {
                      disc_number: discNumber,
                    }),
                  );
                }}
                required
              />
            </label>
            <label>
              Number
              <input
                type="number"
                min={1}
                max={32767}
                value={track.track_number}
                onChange={(event) => {
                  const trackNumber = event.currentTarget.valueAsNumber;
                  setDraft((current) =>
                    updateTrack(current, index, {
                      track_number: trackNumber,
                    }),
                  );
                }}
                required
              />
            </label>
            <label className="creator-track-explicit">
              <input
                type="checkbox"
                checked={track.explicit}
                onChange={(event) => {
                  const explicit = event.currentTarget.checked;
                  setDraft((current) =>
                    updateTrack(current, index, { explicit }),
                  );
                }}
              />
              Explicit
            </label>
            {!editing && draft.tracks.length > 1 ? (
              <button
                className="creator-track-remove"
                type="button"
                aria-label={`Remove ${track.title || `track ${index + 1}`}`}
                onClick={() =>
                  setDraft((current) => ({
                    ...current,
                    tracks: current.tracks.filter((_, item) => item !== index),
                  }))
                }
              >
                Remove
              </button>
            ) : null}
          </div>
        ))}
      </div>

      {error ? (
        <p className="creator-release-error" role="alert">
          {error}
        </p>
      ) : null}
      <footer>
        <button
          className="creator-release-primary-action"
          type="submit"
          disabled={busy}
        >
          {busy ? "Saving…" : editing ? "Save changes" : "Create draft"}
        </button>
      </footer>
    </form>
  );
}

function draftFrom(release?: CreatorRelease): ReleaseDraft {
  return {
    title: release?.title || "",
    releaseType: release?.release_type || "single",
    releaseDate: release?.release_date || new Date().toISOString().slice(0, 10),
    coverObjectKey: release?.cover_object_key || "",
    tracks: release?.tracks.map((track) => ({ ...track, key: track.id })) || [
      newTrack(0),
    ],
  };
}

function newTrack(index: number): TrackDraft {
  return {
    id: "",
    key: `${Date.now()}-${index}`,
    title: "",
    disc_number: 1,
    track_number: index + 1,
    explicit: false,
  };
}

function trackInput({ key: _key, id: _id, ...track }: TrackDraft) {
  return track;
}

function updateTrack(
  draft: ReleaseDraft,
  index: number,
  update: Partial<TrackDraft>,
) {
  return {
    ...draft,
    tracks: draft.tracks.map((track, item) =>
      item === index ? { ...track, ...update } : track,
    ),
  };
}

function CreatorReleaseState({
  children,
  error = false,
}: {
  children: React.ReactNode;
  error?: boolean;
}) {
  return (
    <section
      className="creator-release-state"
      role={error ? "alert" : "status"}
    >
      {children}
    </section>
  );
}

function CreatorReleaseNotice({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="creator-release-notice">
      <strong>{title}</strong>
      <p>{children}</p>
    </section>
  );
}

function trackWord(count: number) {
  return count === 1 ? "track" : "tracks";
}

function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof ApiError ? reason.message : fallback;
}
