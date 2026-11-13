import { type FormEvent, useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";

import {
  type CreatorAccount,
  createCreatorAccount,
  listCreatorAccounts,
  submitCreatorVerification,
} from "../features/creator/creatorReleaseApi";
import { useSession } from "../features/session/SessionContext";
import { ApiError } from "../shared/api/client";

const typeLabels: Record<CreatorAccount["type"], string> = {
  artist: "Artist",
  label: "Label",
  distributor: "Distributor",
};

const statusLabels: Record<CreatorAccount["verification_status"], string> = {
  unverified: "Not submitted",
  pending: "Pending review",
  verified: "Verified",
  rejected: "Changes required",
};

export function CreatorAccountsPage() {
  const { accessToken } = useSession();
  const [accounts, setAccounts] = useState<CreatorAccount[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [submittingId, setSubmittingId] = useState("");

  const load = useCallback(
    async (signal?: AbortSignal) => {
      if (!accessToken) return;
      setStatus("loading");
      setError("");
      try {
        setAccounts(await listCreatorAccounts(accessToken, signal));
        setStatus("ready");
      } catch (reason) {
        if (signal?.aborted) return;
        setError(errorMessage(reason, "Could not load creator accounts"));
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

  async function submitVerification(account: CreatorAccount) {
    if (!accessToken) return;
    setSubmittingId(account.id);
    setError("");
    setNotice("");
    try {
      const updated = await submitCreatorVerification(accessToken, account.id);
      replaceAccount(updated);
      setNotice(`“${updated.display_name}” was submitted for verification.`);
    } catch (reason) {
      setError(errorMessage(reason, "Could not submit creator verification"));
    } finally {
      setSubmittingId("");
    }
  }

  function replaceAccount(account: CreatorAccount) {
    setAccounts((current) =>
      current.map((item) => (item.id === account.id ? account : item)),
    );
  }

  if (status === "loading") {
    return <AccountState>Loading creator accounts…</AccountState>;
  }
  if (status === "error") {
    return (
      <AccountState error>
        {error}
        <button type="button" onClick={() => void load()}>
          Try again
        </button>
      </AccountState>
    );
  }

  return (
    <section className="creator-accounts-page">
      <header className="creator-accounts-heading">
        <div>
          <span className="hero-kicker">Creator studio</span>
          <h1>Creator accounts</h1>
          <p>
            Set up the identity that owns releases, then submit it for platform
            verification.
          </p>
        </div>
        <button
          type="button"
          onClick={() => {
            setShowCreate(true);
            setNotice("");
          }}
        >
          + New creator account
        </button>
      </header>

      {showCreate ? (
        <CreatorAccountForm
          onCancel={() => setShowCreate(false)}
          onSubmit={async (input) => {
            if (!accessToken) return;
            const created = await createCreatorAccount(accessToken, input);
            setAccounts((current) => [created, ...current]);
            setShowCreate(false);
            setNotice(`Creator account “${created.display_name}” was created.`);
          }}
        />
      ) : null}

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

      <section className="creator-account-grid" aria-label="Creator accounts">
        {accounts.length ? (
          accounts.map((account) => (
            <article className="creator-account-card" key={account.id}>
              <span className="creator-account-mark" aria-hidden="true">
                {account.type === "artist" ? "♪" : "∞"}
              </span>
              <div className="creator-account-copy">
                <span className="eyebrow">{typeLabels[account.type]}</span>
                <h2>{account.display_name}</h2>
                <span
                  className={`creator-verification-status creator-verification-${account.verification_status}`}
                >
                  {statusLabels[account.verification_status]}
                </span>
              </div>
              <AccountAction
                account={account}
                busy={submittingId === account.id}
                onSubmit={() => void submitVerification(account)}
              />
            </article>
          ))
        ) : (
          <section className="creator-account-empty">
            <strong>No creator accounts yet</strong>
            <p>Create an artist, label, or distributor identity to begin.</p>
          </section>
        )}
      </section>

      {accounts.some(
        ({ verification_status }) => verification_status === "verified",
      ) ? (
        <aside className="creator-account-next-step">
          <div>
            <strong>Ready to prepare music?</strong>
            <p>Create a release draft and define its initial track list.</p>
          </div>
          <Link to="/creator/releases">Open release drafts</Link>
        </aside>
      ) : null}
    </section>
  );
}

function CreatorAccountForm({
  onCancel,
  onSubmit,
}: {
  onCancel(): void;
  onSubmit(input: {
    type: CreatorAccount["type"];
    display_name: string;
  }): Promise<void>;
}) {
  const [type, setType] = useState<CreatorAccount["type"]>("artist");
  const [displayName, setDisplayName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await onSubmit({ type, display_name: displayName });
    } catch (reason) {
      setError(errorMessage(reason, "Could not create creator account"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="creator-account-form" onSubmit={submit}>
      <header>
        <div>
          <span className="eyebrow">New identity</span>
          <h2>Create creator account</h2>
        </div>
        <button
          className="creator-account-cancel"
          type="button"
          onClick={onCancel}
        >
          Cancel
        </button>
      </header>
      <div>
        <label>
          Account type
          <select
            value={type}
            onChange={(event) =>
              setType(event.currentTarget.value as CreatorAccount["type"])
            }
          >
            <option value="artist">Artist</option>
            <option value="label">Label</option>
            <option value="distributor">Distributor</option>
          </select>
        </label>
        <label>
          Display name
          <input
            value={displayName}
            maxLength={200}
            onChange={(event) => setDisplayName(event.currentTarget.value)}
            required
          />
        </label>
      </div>
      {error ? (
        <p className="creator-account-error" role="alert">
          {error}
        </p>
      ) : null}
      <footer>
        <button
          className="creator-account-create"
          type="submit"
          disabled={busy}
        >
          {busy ? "Creating…" : "Create account"}
        </button>
      </footer>
    </form>
  );
}

function AccountAction({
  account,
  busy,
  onSubmit,
}: {
  account: CreatorAccount;
  busy: boolean;
  onSubmit(): void;
}) {
  if (
    account.verification_status === "unverified" ||
    account.verification_status === "rejected"
  ) {
    return (
      <button
        className="creator-account-submit"
        type="button"
        disabled={busy}
        onClick={onSubmit}
      >
        {busy ? "Submitting…" : "Submit for verification"}
      </button>
    );
  }
  if (account.verification_status === "pending") {
    return <small>Awaiting moderator review</small>;
  }
  return <small>Publishing enabled</small>;
}

function AccountState({
  children,
  error = false,
}: {
  children: React.ReactNode;
  error?: boolean;
}) {
  return (
    <section
      className="creator-account-state"
      role={error ? "alert" : "status"}
    >
      {children}
    </section>
  );
}

function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof ApiError ? reason.message : fallback;
}
