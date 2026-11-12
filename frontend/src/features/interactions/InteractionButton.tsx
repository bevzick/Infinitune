import { useState } from "react";

import { Icon } from "../../shared/ui/Icon";
import { useFeedback } from "../feedback/FeedbackContext";
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
  const feedback = useFeedback();
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
        void interactions
          .toggle(kind, id)
          .then(() =>
            feedback?.notify(successMessage(kind, label, !active), "success"),
          )
          .catch(() => {
            setError(true);
            feedback?.notify(
              `Could not update ${label}. Please try again.`,
              "error",
            );
          });
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

function successMessage(kind: InteractionKind, label: string, active: boolean) {
  if (kind === "artist")
    return `${active ? "Following" : "Unfollowed"} ${label}.`;
  if (kind === "album") return `${active ? "Saved" : "Removed"} ${label}.`;
  return `${active ? "Liked" : "Unliked"} ${label}.`;
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
