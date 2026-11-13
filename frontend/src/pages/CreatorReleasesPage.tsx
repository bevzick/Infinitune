import { type FormEvent, useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";

import {
  type CreatorAccount,
  type CreatorRelease,
  type CreatorReleaseTrack,
  createRelease,
  createTrackUploadSession,
  deleteTrackRight,
  listCreatorAccounts,
  listDraftReleases,
  listTrackRights,
  type ReleaseType,
  setTrackRight,
  type TrackRight,
  type TrackRightType,
  updateRelease,
  uploadAudioFile,
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
  const [rightsReleaseId, setRightsReleaseId] = useState("");
  const [uploadReleaseId, setUploadReleaseId] = useState("");
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
            setRightsReleaseId("");
            setUploadReleaseId("");
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
                    setRightsReleaseId("");
                    setUploadReleaseId("");
                    setNotice("");
                  }}
                >
                  {editingId === release.id ? "Close" : "Edit draft"}
                </button>
                <button
                  className="creator-release-secondary-action"
                  type="button"
                  onClick={() => {
                    setRightsReleaseId((current) =>
                      current === release.id ? "" : release.id,
                    );
                    setEditingId("");
                    setUploadReleaseId("");
                    setShowCreate(false);
                    setNotice("");
                  }}
                >
                  {rightsReleaseId === release.id
                    ? "Close rights"
                    : "Manage rights"}
                </button>
                <button
                  className="creator-release-secondary-action"
                  type="button"
                  onClick={() => {
                    setUploadReleaseId((current) =>
                      current === release.id ? "" : release.id,
                    );
                    setEditingId("");
                    setRightsReleaseId("");
                    setShowCreate(false);
                    setNotice("");
                  }}
                >
                  {uploadReleaseId === release.id
                    ? "Close uploads"
                    : "Upload audio"}
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
              {rightsReleaseId === release.id && accessToken ? (
                <TrackRightsManager
                  accessToken={accessToken}
                  release={release}
                />
              ) : null}
              {uploadReleaseId === release.id && accessToken ? (
                <TrackAudioManager
                  accessToken={accessToken}
                  release={release}
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

const maximumAudioBytes = 500 * 1024 * 1024;
const audioTypes: Record<string, string> = {
  wav: "audio/wav",
  flac: "audio/flac",
  mp3: "audio/mpeg",
  m4a: "audio/mp4",
};

type TrackUploadState = {
  stage: "idle" | "creating" | "uploading" | "uploaded" | "error";
  progress: number;
  message: string;
};

function TrackAudioManager({
  accessToken,
  release,
}: {
  accessToken: string;
  release: CreatorRelease;
}) {
  const [files, setFiles] = useState<Record<string, File | undefined>>({});
  const [uploads, setUploads] = useState<Record<string, TrackUploadState>>({});

  function selectFile(trackId: string, file?: File) {
    setFiles((current) => ({ ...current, [trackId]: file }));
    const error = file ? validateAudioFile(file) : "";
    setUploads((current) => ({
      ...current,
      [trackId]: error
        ? { stage: "error", progress: 0, message: error }
        : { stage: "idle", progress: 0, message: "" },
    }));
  }

  async function upload(track: CreatorReleaseTrack) {
    const file = files[track.id];
    if (!file) return;
    const validationError = validateAudioFile(file);
    if (validationError) {
      selectFile(track.id, file);
      return;
    }
    const mimeType = audioMimeType(file);
    setUploads((current) => ({
      ...current,
      [track.id]: {
        stage: "creating",
        progress: 0,
        message: "Creating upload session…",
      },
    }));
    try {
      const session = await createTrackUploadSession(accessToken, track.id, {
        idempotency_key: crypto.randomUUID(),
        original_name: file.name,
        mime_type: mimeType,
        size_bytes: file.size,
      });
      setUploads((current) => ({
        ...current,
        [track.id]: {
          stage: "uploading",
          progress: 0,
          message: "Uploading audio…",
        },
      }));
      const result = await uploadAudioFile(
        accessToken,
        session.upload_url,
        file,
        mimeType,
        (progress) =>
          setUploads((current) => ({
            ...current,
            [track.id]: {
              stage: "uploading",
              progress,
              message: `Uploading audio… ${progress}%`,
            },
          })),
      );
      setUploads((current) => ({
        ...current,
        [track.id]: {
          stage: "uploaded",
          progress: 100,
          message:
            result.status === "ready"
              ? "Audio is ready."
              : "Uploaded. Audio processing has been queued.",
        },
      }));
    } catch (reason) {
      setUploads((current) => ({
        ...current,
        [track.id]: {
          stage: "error",
          progress: 0,
          message: errorMessage(reason, "Could not upload audio"),
        },
      }));
    }
  }

  return (
    <section className="creator-track-uploads" aria-label="Track audio uploads">
      <header>
        <div>
          <span className="eyebrow">Local source audio</span>
          <h3>Upload masters</h3>
        </div>
        <p>WAV, FLAC, MP3 or M4A · maximum 500 MiB per track.</p>
      </header>
      <div className="creator-track-upload-list">
        {release.tracks.map((track) => {
          const state = uploads[track.id];
          const busy =
            state?.stage === "creating" || state?.stage === "uploading";
          return (
            <div className="creator-track-upload-row" key={track.id}>
              <div className="creator-track-upload-title">
                <strong>{track.title}</strong>
                <span>
                  Disc {track.disc_number} · Track {track.track_number}
                </span>
              </div>
              <label>
                Audio file for {track.title}
                <input
                  type="file"
                  accept=".wav,.flac,.mp3,.m4a,audio/wav,audio/flac,audio/mpeg,audio/mp4"
                  disabled={busy}
                  onChange={(event) =>
                    selectFile(track.id, event.currentTarget.files?.[0])
                  }
                />
              </label>
              <button
                type="button"
                disabled={!files[track.id] || busy || state?.stage === "error"}
                onClick={() => void upload(track)}
              >
                {busy ? "Uploading…" : `Upload ${track.title}`}
              </button>
              {state?.message ? (
                <div
                  className={`creator-track-upload-status ${state.stage}`}
                  role={state.stage === "error" ? "alert" : "status"}
                >
                  <span>{state.message}</span>
                  {state.stage === "uploading" ? (
                    <progress value={state.progress} max={100}>
                      {state.progress}%
                    </progress>
                  ) : null}
                </div>
              ) : null}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function audioMimeType(file: File) {
  const extension = file.name.split(".").pop()?.toLowerCase() || "";
  return audioTypes[extension] || file.type;
}

function validateAudioFile(file: File) {
  const extension = file.name.split(".").pop()?.toLowerCase() || "";
  if (!audioTypes[extension]) return "Choose a WAV, FLAC, MP3 or M4A file.";
  if (file.size < 1) return "The audio file is empty.";
  if (file.size > maximumAudioBytes) return "The audio file exceeds 500 MiB.";
  return "";
}

const managedRights: Array<{ type: TrackRightType; label: string }> = [
  { type: "streaming", label: "Streaming" },
  { type: "distribution", label: "Distribution" },
];

function TrackRightsManager({
  accessToken,
  release,
}: {
  accessToken: string;
  release: CreatorRelease;
}) {
  const [rights, setRights] = useState<Record<string, TrackRight[]>>({});
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [pending, setPending] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setStatus("loading");
    Promise.all(
      release.tracks.map(
        async (track) =>
          [
            track.id,
            await listTrackRights(accessToken, track.id, controller.signal),
          ] as const,
      ),
    )
      .then((entries) => {
        setRights(Object.fromEntries(entries));
        setStatus("ready");
      })
      .catch((reason) => {
        if (controller.signal.aborted) return;
        setError(errorMessage(reason, "Could not load track rights"));
        setStatus("error");
      });
    return () => controller.abort();
  }, [accessToken, release.tracks]);

  async function toggle(
    track: CreatorReleaseTrack,
    type: TrackRightType,
    enabled: boolean,
  ) {
    const operation = `${track.id}:${type}`;
    setPending(operation);
    setError("");
    try {
      if (enabled) {
        const stored = await setTrackRight(accessToken, track.id, {
          territory_code: "001",
          rights_type: type,
          valid_from: release.release_date,
          valid_until: null,
        });
        setRights((current) => ({
          ...current,
          [track.id]: [
            ...(current[track.id] || []).filter(
              (right) =>
                right.territory_code !== "001" || right.rights_type !== type,
            ),
            stored,
          ],
        }));
      } else {
        const existing = worldwideRight(
          rights[track.id],
          type,
          release.release_date,
        );
        if (!existing) return;
        await deleteTrackRight(accessToken, existing.id);
        setRights((current) => ({
          ...current,
          [track.id]: (current[track.id] || []).filter(
            (right) => right.id !== existing.id,
          ),
        }));
      }
    } catch (reason) {
      setError(errorMessage(reason, "Could not update track rights"));
    } finally {
      setPending("");
    }
  }

  return (
    <section className="creator-track-rights" aria-label="Track rights">
      <header>
        <div>
          <span className="eyebrow">Worldwide · territory 001</span>
          <h3>Streaming and distribution rights</h3>
        </div>
        <p>Rights begin on {release.release_date} and remain open-ended.</p>
      </header>
      {status === "loading" ? <p role="status">Loading track rights…</p> : null}
      {status === "error" ? (
        <p className="creator-release-error" role="alert">
          {error}
        </p>
      ) : null}
      {status === "ready" ? (
        <div className="creator-track-rights-list">
          {release.tracks.map((track) => (
            <div className="creator-track-rights-row" key={track.id}>
              <div>
                <strong>{track.title}</strong>
                <span>
                  Disc {track.disc_number} · Track {track.track_number}
                </span>
              </div>
              {managedRights.map(({ type, label }) => {
                const checked = Boolean(
                  worldwideRight(rights[track.id], type, release.release_date),
                );
                const operation = `${track.id}:${type}`;
                return (
                  <label key={type}>
                    <input
                      type="checkbox"
                      checked={checked}
                      disabled={Boolean(pending)}
                      onChange={(event) =>
                        void toggle(track, type, event.currentTarget.checked)
                      }
                    />
                    {pending === operation ? "Saving…" : label}
                  </label>
                );
              })}
            </div>
          ))}
        </div>
      ) : null}
      {status === "ready" && error ? (
        <p className="creator-release-error" role="alert">
          {error}
        </p>
      ) : null}
    </section>
  );
}

function worldwideRight(
  rights: TrackRight[] | undefined,
  type: TrackRightType,
  releaseDate: string,
) {
  return rights?.find(
    (right) =>
      right.territory_code === "001" &&
      right.rights_type === type &&
      right.valid_from <= releaseDate &&
      (!right.valid_until || right.valid_until >= releaseDate),
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
