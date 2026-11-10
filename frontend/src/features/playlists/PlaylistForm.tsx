import { type FormEvent, useState } from "react";

import type { PlaylistInput, PlaylistVisibility } from "./playlistApi";

export function PlaylistForm({
  initial,
  submitLabel,
  busy,
  onSubmit,
  onCancel,
}: {
  initial?: PlaylistInput;
  submitLabel: string;
  busy: boolean;
  onSubmit(input: PlaylistInput): Promise<void>;
  onCancel?(): void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [visibility, setVisibility] = useState<PlaylistVisibility>(
    initial?.visibility ?? "private",
  );

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void onSubmit({ name, description, visibility });
  }

  return (
    <form className="playlist-form" onSubmit={submit}>
      <label className="playlist-name-field">
        <span>Name</span>
        <input
          name="name"
          required
          maxLength={150}
          value={name}
          onChange={(event) => setName(event.currentTarget.value)}
          placeholder="Late-night signals"
        />
      </label>
      <label>
        <span>Visibility</span>
        <select
          name="visibility"
          value={visibility}
          onChange={(event) =>
            setVisibility(event.currentTarget.value as PlaylistVisibility)
          }
        >
          <option value="private">Private</option>
          <option value="unlisted">Unlisted</option>
          <option value="public">Public</option>
        </select>
      </label>
      <label className="playlist-description-field">
        <span>Description</span>
        <textarea
          name="description"
          rows={3}
          value={description}
          onChange={(event) => setDescription(event.currentTarget.value)}
          placeholder="What belongs in this playlist?"
        />
      </label>
      <div className="playlist-form-actions">
        <button type="submit" disabled={busy}>
          {busy ? "Saving…" : submitLabel}
        </button>
        {onCancel ? (
          <button type="button" className="secondary-button" onClick={onCancel}>
            Cancel
          </button>
        ) : null}
      </div>
    </form>
  );
}
