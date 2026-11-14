import { type FormEvent, useEffect, useState } from "react";
import { ApiError } from "../../shared/api/client";
import {
  addCreatorMember,
  type CreatorMember,
  type CreatorMemberRole,
  changeCreatorMemberRole,
  listCreatorMembers,
  type ManagedMemberRole,
  removeCreatorMember,
} from "./creatorMembersApi";

type LoadStatus = "loading" | "ready" | "forbidden" | "error";

const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

const roleLabels: Record<CreatorMemberRole, string> = {
  owner: "Owner",
  manager: "Manager",
  editor: "Editor",
  uploader: "Uploader",
};

function allowedRoles(actorRole?: CreatorMemberRole): ManagedMemberRole[] {
  if (actorRole === "owner") return ["manager", "editor", "uploader"];
  if (actorRole === "manager") return ["editor", "uploader"];
  return [];
}

function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof ApiError ? reason.message : fallback;
}

export function CreatorMembersManager({
  accessToken,
  creatorId,
  creatorName,
  currentUserId,
}: {
  accessToken: string;
  creatorId: string;
  creatorName: string;
  currentUserId: string;
}) {
  const [members, setMembers] = useState<CreatorMember[]>([]);
  const [status, setStatus] = useState<LoadStatus>("loading");
  const [refresh, setRefresh] = useState(0);
  const [userId, setUserId] = useState("");
  const [role, setRole] = useState<ManagedMemberRole>("editor");
  const [busy, setBusy] = useState("");
  const [confirmRemove, setConfirmRemove] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setStatus("loading");
    if (refresh > 0) setNotice("");
    setError("");
    listCreatorMembers(accessToken, creatorId, controller.signal)
      .then((result) => {
        if (controller.signal.aborted) return;
        setMembers(result);
        setStatus("ready");
      })
      .catch((reason: unknown) => {
        if (controller.signal.aborted) return;
        if (reason instanceof ApiError && reason.status === 403) {
          setStatus("forbidden");
        } else {
          setError(errorMessage(reason, "Could not load creator members"));
          setStatus("error");
        }
      });
    return () => controller.abort();
  }, [accessToken, creatorId, refresh]);

  const actorRole = members.find(
    (member) => member.user_id === currentUserId,
  )?.role;
  const assignable = allowedRoles(actorRole);
  const canManage = assignable.length > 0;

  function canEdit(member: CreatorMember) {
    if (member.user_id === currentUserId || member.role === "owner") {
      return false;
    }
    return (
      actorRole === "owner" ||
      (actorRole === "manager" &&
        (member.role === "editor" || member.role === "uploader"))
    );
  }

  async function addMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const targetId = userId.trim();
    if (!uuidPattern.test(targetId)) {
      setError("Enter a valid user UUID.");
      return;
    }
    if (targetId.toLowerCase() === currentUserId.toLowerCase()) {
      setError("You cannot add yourself as a member.");
      return;
    }
    if (
      members.some(
        (member) => member.user_id.toLowerCase() === targetId.toLowerCase(),
      )
    ) {
      setError("This user is already a creator member.");
      return;
    }
    if (!assignable.includes(role)) return;
    setBusy("add");
    setError("");
    setNotice("");
    try {
      const added = await addCreatorMember(accessToken, creatorId, {
        user_id: targetId,
        role,
      });
      setMembers((current) => [...current, added]);
      setUserId("");
      setNotice("Member added.");
    } catch (reason) {
      setError(errorMessage(reason, "Could not add member"));
    } finally {
      setBusy("");
    }
  }

  async function changeRole(
    member: CreatorMember,
    nextRole: ManagedMemberRole,
  ) {
    if (!canEdit(member) || !assignable.includes(nextRole)) return;
    setBusy(member.user_id);
    setError("");
    setNotice("");
    try {
      const updated = await changeCreatorMemberRole(
        accessToken,
        creatorId,
        member.user_id,
        nextRole,
      );
      setMembers((current) =>
        current.map((item) =>
          item.user_id === updated.user_id ? updated : item,
        ),
      );
      setNotice("Member role updated.");
    } catch (reason) {
      setError(errorMessage(reason, "Could not change member role"));
    } finally {
      setBusy("");
    }
  }

  async function removeMember(member: CreatorMember) {
    if (!canEdit(member)) return;
    setBusy(member.user_id);
    setError("");
    setNotice("");
    try {
      await removeCreatorMember(accessToken, creatorId, member.user_id);
      setMembers((current) =>
        current.filter((item) => item.user_id !== member.user_id),
      );
      setConfirmRemove("");
      setNotice("Member removed.");
    } catch (reason) {
      setError(errorMessage(reason, "Could not remove member"));
    } finally {
      setBusy("");
    }
  }

  return (
    <section
      className="creator-members-manager"
      aria-label={`Members of ${creatorName}`}
    >
      <header>
        <div>
          <h3>Team members</h3>
          <p>
            Members are identified by their user UUID. Roles are enforced by the
            server.
          </p>
        </div>
        <button
          type="button"
          disabled={status === "loading" || Boolean(busy)}
          onClick={() => setRefresh((count) => count + 1)}
        >
          Refresh members
        </button>
      </header>

      {status === "loading" ? <p role="status">Loading members…</p> : null}
      {status === "forbidden" ? (
        <p role="status">
          Only owners and managers can view this account’s members.
        </p>
      ) : null}
      {status === "error" ? <p role="alert">{error}</p> : null}
      {status === "ready" ? (
        <>
          {error ? (
            <p className="creator-account-error" role="alert">
              {error}
            </p>
          ) : null}
          {notice ? (
            <p className="creator-account-success" role="status">
              {notice}
            </p>
          ) : null}
          <ul className="creator-members-list">
            {members.map((member) => (
              <li key={member.user_id}>
                <div className="creator-member-identity">
                  <strong>
                    {member.user_id === currentUserId ? "You" : member.user_id}
                  </strong>
                  {member.user_id === currentUserId ? (
                    <small>{member.user_id}</small>
                  ) : null}
                </div>
                {canEdit(member) ? (
                  <div className="creator-member-actions">
                    <label>
                      Role for {member.user_id}
                      <select
                        aria-label={`Role for ${member.user_id}`}
                        value={member.role}
                        disabled={Boolean(busy)}
                        onChange={(event) =>
                          void changeRole(
                            member,
                            event.target.value as ManagedMemberRole,
                          )
                        }
                      >
                        {assignable.map((item) => (
                          <option value={item} key={item}>
                            {roleLabels[item]}
                          </option>
                        ))}
                      </select>
                    </label>
                    {confirmRemove === member.user_id ? (
                      <div className="creator-member-confirm">
                        <button
                          type="button"
                          disabled={Boolean(busy)}
                          onClick={() => void removeMember(member)}
                        >
                          Confirm remove {member.user_id}
                        </button>
                        <button
                          type="button"
                          disabled={Boolean(busy)}
                          onClick={() => setConfirmRemove("")}
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <button
                        type="button"
                        disabled={Boolean(busy)}
                        onClick={() => setConfirmRemove(member.user_id)}
                      >
                        Remove {member.user_id}
                      </button>
                    )}
                  </div>
                ) : (
                  <span className="creator-member-role">
                    {roleLabels[member.role]}
                  </span>
                )}
              </li>
            ))}
          </ul>
          {canManage ? (
            <form
              className="creator-members-add"
              onSubmit={(event) => void addMember(event)}
            >
              <h4>Add member</h4>
              <label>
                User UUID
                <input
                  value={userId}
                  onChange={(event) => setUserId(event.target.value)}
                  placeholder="00000000-0000-4000-8000-000000000000"
                  autoComplete="off"
                  maxLength={36}
                  required
                />
              </label>
              <label>
                New member role
                <select
                  value={assignable.includes(role) ? role : assignable[0]}
                  onChange={(event) =>
                    setRole(event.target.value as ManagedMemberRole)
                  }
                >
                  {assignable.map((item) => (
                    <option key={item} value={item}>
                      {roleLabels[item]}
                    </option>
                  ))}
                </select>
              </label>
              <button type="submit" disabled={Boolean(busy)}>
                {busy === "add" ? "Adding…" : "Add member"}
              </button>
            </form>
          ) : (
            <p>Your membership does not permit managing other members.</p>
          )}
        </>
      ) : null}
    </section>
  );
}
