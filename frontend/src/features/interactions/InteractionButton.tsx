import { useState } from "react";

import { Icon } from "../../shared/ui/Icon";
import { useInteractions } from "./InteractionContext";
import type { InteractionKind } from "./interactionApi";

export function InteractionButton({
  kind,
  id,
  label,
  compact = true,
}: {
  kind: InteractionKind;
  id: string;
  label: string;
  compact?: boolean;
}) {
  const interactions = useInteractions();
  const [error, setError] = useState(false);
  if (!interactions?.available) return null;

  const active = interactions.isActive(kind, id);
  const busy = interactions.isBusy(kind, id);
  const action = active ? removeLabel(kind) : addLabel(kind);

  return (
    <button
      className={`interaction-button${active ? " interaction-button-active" : ""}${compact ? " interaction-button-compact" : ""}`}
      type="button"
      disabled={!interactions.loaded || busy}
      aria-pressed={active}
      aria-label={`${action} ${label}`}
      title={error ? "Could not update your library. Try again." : action}
      onClick={() => {
        setError(false);
        void interactions.toggle(kind, id).catch(() => setError(true));
      }}
    >
      <Icon
        name={
          kind === "artist"
            ? "user-plus"
            : kind === "album"
              ? "bookmark"
              : "heart"
        }
        size={18}
      />
      {!compact ? (
        <span>{active ? removeLabel(kind) : addLabel(kind)}</span>
      ) : null}
    </button>
  );
}

function addLabel(kind: InteractionKind) {
  if (kind === "artist") return "Follow";
  if (kind === "album") return "Save";
  return "Like";
}

function removeLabel(kind: InteractionKind) {
  if (kind === "artist") return "Unfollow";
  if (kind === "album") return "Remove saved";
  return "Unlike";
}
