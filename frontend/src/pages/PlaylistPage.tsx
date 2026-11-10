import { type FormEvent, useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

import {
  CatalogEmpty,
  CatalogLoading,
  TrackList,
} from "../features/catalog/CatalogViews";
import {
  fetchCatalogPage,
  type Track,
  trackPagePath,
} from "../features/catalog/catalogApi";
import { RuleEditor } from "../features/playlists/DynamicRuleForm";
import { PlaylistForm } from "../features/playlists/PlaylistForm";
import {
  addPlaylistTrack,
  deletePlaylist,
  getCatalogTrack,
  getDynamicCriteria,
  getPlaylist,
  listPlaylistTracks,
  type Playlist,
  type PlaylistTrack,
  type RuleCondition,
  type RuleSet,
  removePlaylistTrack,
  reorderPlaylistTracks,
  replaceDynamicCriteria,
  resolveDynamicTracks,
  updatePlaylist,
} from "../features/playlists/playlistApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";
import { errorMessage, LibraryError, LibraryLoading } from "./LibraryPage";

export function PlaylistPage() {
  const { playlistId = "" } = useParams();
  const navigate = useNavigate();
  const { accessToken, identity } = useSession();
  const [playlist, setPlaylist] = useState<Playlist | null>(null);
  const [entries, setEntries] = useState<PlaylistTrack[]>([]);
  const [tracks, setTracks] = useState<Track[]>([]);
  const [totalTracks, setTotalTracks] = useState(0);
  const [resolvedCount, setResolvedCount] = useState(0);
  const [resolution, setResolution] = useState<"ready" | "empty">("ready");
  const [criteria, setCriteria] = useState<RuleSet | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [error, setError] = useState("");
  const [mutationError, setMutationError] = useState("");
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editingRules, setEditingRules] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [searchResults, setSearchResults] = useState<Track[]>([]);
  const [searching, setSearching] = useState(false);
  const isOwner = playlist?.owner_id === identity?.userId;

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!accessToken || !playlistId) return;
      setStatus("loading");
      setError("");
      try {
        const playlistResult = await getPlaylist(
          accessToken,
          playlistId,
          signal,
        );
        if (playlistResult.type === "dynamic") {
          const [criteriaResult, trackPage] = await Promise.all([
            playlistResult.owner_id === identity?.userId
              ? getDynamicCriteria(accessToken, playlistId, signal)
              : Promise.resolve(null),
            resolveDynamicTracks(accessToken, playlistId, signal),
          ]);
          const resolved = await resolveCatalogTracks(
            trackPage.items.map((entry) => entry.track_id),
            signal,
          );
          setPlaylist(playlistResult);
          setCriteria(criteriaResult);
          setEntries([]);
          setTracks(resolved);
          setResolvedCount(trackPage.items.length);
          setTotalTracks(trackPage.page.total);
          setResolution(trackPage.resolution.status);
          setStatus("ready");
          return;
        }
        const trackPage = await listPlaylistTracks(
          accessToken,
          playlistId,
          signal,
        );
        const resolved = await Promise.all(
          trackPage.items.map(async (entry) => {
            try {
              return await getCatalogTrack(entry.track_id, signal);
            } catch (reason) {
              if (reason instanceof ApiError && reason.status === 404)
                return null;
              throw reason;
            }
          }),
        );
        setPlaylist(playlistResult);
        setEntries(trackPage.items);
        setTracks(resolved.filter((track): track is Track => track !== null));
        setResolvedCount(0);
        setTotalTracks(trackPage.page.total);
        setStatus("ready");
      } catch (reason) {
        if (signal?.aborted) return;
        setError(errorMessage(reason, "Could not load this playlist"));
        setStatus("error");
      }
    },
    [accessToken, identity?.userId, playlistId],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  async function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const query = String(
      new FormData(event.currentTarget).get("query") ?? "",
    ).trim();
    if (!query) {
      setSearchResults([]);
      return;
    }
    setSearching(true);
    setMutationError("");
    try {
      const result = await fetchCatalogPage<Track>(
        trackPagePath({ title: query, sort: "title_asc", limit: 8 }),
      );
      setSearchResults(result.items);
    } catch (reason) {
      setMutationError(errorMessage(reason, "Could not search the catalog"));
    } finally {
      setSearching(false);
    }
  }

  async function addTrack(track: Track) {
    if (!accessToken || !playlist) return;
    await mutate(async () => {
      const added = await addPlaylistTrack(accessToken, playlist.id, track.id);
      setEntries((current) => [...current, added]);
      setTracks((current) => [...current, track]);
      setTotalTracks((current) => current + 1);
    });
  }

  async function removeTrack(trackId: string) {
    if (!accessToken || !playlist) return;
    await mutate(async () => {
      await removePlaylistTrack(accessToken, playlist.id, trackId);
      setEntries((current) =>
        current.filter((item) => item.track_id !== trackId),
      );
      setTracks((current) => current.filter((item) => item.id !== trackId));
      setTotalTracks((current) => Math.max(0, current - 1));
    });
  }

  async function moveTrack(trackId: string, direction: -1 | 1) {
    if (!accessToken || !playlist || totalTracks > entries.length) return;
    const index = entries.findIndex((entry) => entry.track_id === trackId);
    const destination = index + direction;
    if (index < 0 || destination < 0 || destination >= entries.length) return;
    const order = entries.map((entry) => entry.track_id);
    [order[index], order[destination]] = [order[destination], order[index]];
    await mutate(async () => {
      const reordered = await reorderPlaylistTracks(
        accessToken,
        playlist.id,
        order,
      );
      setEntries(reordered);
      const byID = new Map(tracks.map((track) => [track.id, track]));
      setTracks(
        reordered.flatMap((entry) => {
          const track = byID.get(entry.track_id);
          return track ? [track] : [];
        }),
      );
    });
  }

  async function mutate(action: () => Promise<void>) {
    setBusy(true);
    setMutationError("");
    try {
      await action();
    } catch (reason) {
      setMutationError(
        errorMessage(reason, "The playlist could not be changed"),
      );
    } finally {
      setBusy(false);
    }
  }

  if (status === "loading") return <LibraryLoading />;
  if (status === "error") {
    return <LibraryError message={error} onRetry={() => void load()} />;
  }
  if (!playlist) return null;

  const missingTracks = entries.length - tracks.length;
  const canReorder = totalTracks === entries.length;

  return (
    <section className="playlist-page">
      <Link className="back-link" to="/library">
        ← Back to library
      </Link>
      <header className="playlist-detail-heading">
        <span className="playlist-detail-cover" aria-hidden="true">
          {playlist.type === "dynamic" ? "◇" : "≡"}
        </span>
        <div>
          <span className="hero-kicker">
            {playlist.type} · {playlist.visibility}
          </span>
          <h1>{playlist.name}</h1>
          <p>{playlist.description || "No description yet."}</p>
          <small>{totalTracks} tracks</small>
        </div>
        {isOwner ? (
          <button
            type="button"
            className="secondary-button"
            onClick={() => setEditing(true)}
          >
            Edit details
          </button>
        ) : null}
      </header>

      {mutationError ? (
        <p className="form-error" role="alert">
          {mutationError}
        </p>
      ) : null}

      {editing ? (
        <section
          className="playlist-editor"
          aria-labelledby="edit-playlist-title"
        >
          <h2 id="edit-playlist-title">Edit playlist</h2>
          <PlaylistForm
            initial={playlist}
            submitLabel="Save changes"
            busy={busy}
            onCancel={() => setEditing(false)}
            onSubmit={async (input) => {
              if (!accessToken) return;
              await mutate(async () => {
                setPlaylist(
                  await updatePlaylist(accessToken, playlist.id, input),
                );
                setEditing(false);
              });
            }}
          />
          <div className="danger-zone">
            {confirmDelete ? (
              <>
                <span>Delete this playlist permanently?</span>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() =>
                    void mutate(async () => {
                      if (!accessToken) return;
                      await deletePlaylist(accessToken, playlist.id);
                      navigate("/library", { replace: true });
                    })
                  }
                >
                  Confirm delete
                </button>
                <button
                  type="button"
                  className="secondary-button"
                  onClick={() => setConfirmDelete(false)}
                >
                  Keep playlist
                </button>
              </>
            ) : (
              <button type="button" onClick={() => setConfirmDelete(true)}>
                Delete playlist
              </button>
            )}
          </div>
        </section>
      ) : null}

      {playlist.type === "dynamic" ? (
        <>
          {criteria ? (
            <section
              className="dynamic-rules"
              aria-labelledby="dynamic-rules-title"
            >
              <div className="section-heading">
                <div>
                  <span className="eyebrow">Automatic selection</span>
                  <h2 id="dynamic-rules-title">{criteria.name}</h2>
                </div>
                {isOwner && !editingRules ? (
                  <button
                    type="button"
                    className="secondary-button"
                    onClick={() => setEditingRules(true)}
                  >
                    Edit rules
                  </button>
                ) : null}
              </div>
              {editingRules ? (
                <RuleEditor
                  initial={criteria}
                  busy={busy}
                  onCancel={() => setEditingRules(false)}
                  onSubmit={async (input) => {
                    if (!accessToken) return;
                    await mutate(async () => {
                      setCriteria(
                        await replaceDynamicCriteria(
                          accessToken,
                          playlist.id,
                          input,
                        ),
                      );
                      setEditingRules(false);
                      await load();
                    });
                  }}
                />
              ) : (
                <RuleSummary criteria={criteria} />
              )}
            </section>
          ) : null}
          <section
            className="playlist-tracks dynamic-results"
            aria-labelledby="dynamic-tracks-title"
          >
            <div className="section-heading">
              <div>
                <span className="eyebrow">Live catalog result</span>
                <h2 id="dynamic-tracks-title">Matching tracks</h2>
              </div>
              <span className="result-count">{totalTracks} matches</span>
            </div>
            {resolvedCount > tracks.length ? (
              <p className="playlist-note">
                {resolvedCount - tracks.length} unavailable catalog tracks were
                omitted from this view.
              </p>
            ) : null}
            {totalTracks > resolvedCount ? (
              <p className="playlist-note">
                Showing the first {resolvedCount} of {totalTracks} matches.
              </p>
            ) : null}
            {tracks.length ? (
              <TrackList tracks={tracks} />
            ) : (
              <CatalogEmpty>
                {resolution === "empty"
                  ? "No tracks match these rules"
                  : "No matching tracks are available"}
              </CatalogEmpty>
            )}
          </section>
        </>
      ) : (
        <>
          <section
            className="playlist-tracks"
            aria-labelledby="playlist-tracks-title"
          >
            <div className="section-heading">
              <div>
                <span className="eyebrow">Manual order</span>
                <h2 id="playlist-tracks-title">Tracks</h2>
              </div>
            </div>
            {missingTracks > 0 ? (
              <p className="playlist-note">
                {missingTracks} unavailable catalog tracks are kept in the
                playlist order.
              </p>
            ) : null}
            {!canReorder ? (
              <p className="playlist-note">
                Reordering is disabled because this playlist contains more than
                100 tracks.
              </p>
            ) : null}
            {tracks.length ? (
              <TrackList
                tracks={tracks}
                actions={(track) => {
                  const entryIndex = entries.findIndex(
                    (entry) => entry.track_id === track.id,
                  );
                  return isOwner ? (
                    <>
                      <button
                        type="button"
                        disabled={busy || !canReorder || entryIndex <= 0}
                        onClick={() => void moveTrack(track.id, -1)}
                        aria-label={`Move ${track.title} up`}
                      >
                        ↑
                      </button>
                      <button
                        type="button"
                        disabled={
                          busy ||
                          !canReorder ||
                          entryIndex >= entries.length - 1
                        }
                        onClick={() => void moveTrack(track.id, 1)}
                        aria-label={`Move ${track.title} down`}
                      >
                        ↓
                      </button>
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => void removeTrack(track.id)}
                        aria-label={`Remove ${track.title}`}
                      >
                        ×
                      </button>
                    </>
                  ) : null;
                }}
              />
            ) : (
              <CatalogEmpty>This playlist has no available tracks</CatalogEmpty>
            )}
          </section>

          {isOwner ? (
            <section
              className="playlist-add"
              aria-labelledby="add-tracks-title"
            >
              <div>
                <span className="eyebrow">Catalog</span>
                <h2 id="add-tracks-title">Add tracks</h2>
              </div>
              <search>
                <form className="playlist-search" onSubmit={search}>
                  <label htmlFor="playlist-track-query">
                    Search by track title
                  </label>
                  <input
                    id="playlist-track-query"
                    name="query"
                    type="search"
                    required
                  />
                  <button type="submit" disabled={searching}>
                    {searching ? "Searching…" : "Search"}
                  </button>
                </form>
              </search>
              {searching ? <CatalogLoading label="tracks" /> : null}
              {!searching && searchResults.length ? (
                <TrackList
                  tracks={searchResults}
                  actions={(track) => {
                    const added = entries.some(
                      (entry) => entry.track_id === track.id,
                    );
                    return (
                      <button
                        type="button"
                        disabled={busy || added}
                        onClick={() => void addTrack(track)}
                      >
                        {added ? "Added" : "Add"}
                      </button>
                    );
                  }}
                />
              ) : null}
            </section>
          ) : null}
        </>
      )}
    </section>
  );
}

function RuleSummary({ criteria }: { criteria: RuleSet }) {
  return (
    <div className="rule-summary">
      <p>
        Match <strong>{criteria.logic === "AND" ? "every" : "any"}</strong>{" "}
        condition:
      </p>
      <ol>
        {[...criteria.conditions]
          .sort((left, right) => left.position - right.position)
          .map((condition) => (
            <li key={condition.id}>{conditionLabel(condition)}</li>
          ))}
      </ol>
    </div>
  );
}

function conditionLabel(condition: RuleCondition) {
  const field = condition.field.replaceAll("_", " ");
  const operator = condition.operator.replaceAll("_", " ");
  const value = condition.value;
  switch (value.kind) {
    case "text":
      return `${field} ${operator} “${value.text}”`;
    case "uuid":
      return `${field} ${operator} ${value.uuid}`;
    case "uuid_list":
      return `${field} ${operator} ${value.uuid_list.join(", ")}`;
    case "integer":
      return `${field} ${operator} ${formatRuleInteger(condition.field, value.integer)}`;
    case "integer_range":
      return `${field} ${operator} ${formatRuleInteger(condition.field, value.integer_range.minimum)} and ${formatRuleInteger(condition.field, value.integer_range.maximum)}`;
    case "boolean":
      return `${field} ${operator} ${value.boolean ? "yes" : "no"}`;
  }
}

function formatRuleInteger(field: RuleCondition["field"], value: number) {
  if (field !== "duration_ms") return String(value);
  const minutes = Math.floor(value / 60_000);
  const seconds = Math.floor((value % 60_000) / 1000);
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

async function resolveCatalogTracks(trackIDs: string[], signal?: AbortSignal) {
  const resolved = await Promise.all(
    trackIDs.map(async (trackID) => {
      try {
        return await getCatalogTrack(trackID, signal);
      } catch (reason) {
        if (reason instanceof ApiError && reason.status === 404) return null;
        throw reason;
      }
    }),
  );
  return resolved.filter((track): track is Track => track !== null);
}
