import { type FormEvent, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";

import { useSession } from "../features/session/SessionContext";
import { ThemeToggle } from "../features/theme/ThemeToggle";
import { ApiError } from "../shared/api/client";

type LoginLocationState = {
  from?: string;
  registeredEmail?: string;
};

export function LoginPage() {
  const session = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const state = (location.state ?? {}) as LoginLocationState;
  const [email, setEmail] = useState(state.registeredEmail ?? "");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      await session.login(email, password);
      navigate(safeDestination(state.from), { replace: true });
    } catch (reason) {
      setError(
        reason instanceof ApiError
          ? reason.message
          : "Sign in is temporarily unavailable",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AuthPage
      title="Welcome back"
      subtitle="Continue listening where you left off."
    >
      {state.registeredEmail ? (
        <div className="form-notice" role="status">
          Account created. Sign in to start your session.
        </div>
      ) : null}
      <form className="auth-form" onSubmit={submit}>
        <FormField label="Email" htmlFor="login-email">
          <input
            id="login-email"
            name="email"
            type="email"
            autoComplete="email"
            inputMode="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </FormField>
        <FormField label="Password" htmlFor="login-password">
          <input
            id="login-password"
            name="password"
            type="password"
            autoComplete="current-password"
            required
            minLength={12}
            maxLength={128}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </FormField>
        {error ? (
          <p className="form-error" role="alert">
            {error}
          </p>
        ) : null}
        <button className="auth-submit" type="submit" disabled={submitting}>
          {submitting ? "Signing in…" : "Sign in"}
        </button>
      </form>
      <p className="auth-switch">
        New to Infinitune? <Link to="/register">Create an account</Link>
      </p>
    </AuthPage>
  );
}

function safeDestination(value?: string) {
  return value?.startsWith("/") && !value.startsWith("//") ? value : "/";
}

type AuthPageProps = {
  title: string;
  subtitle: string;
  children: React.ReactNode;
};

export function AuthPage({ title, subtitle, children }: AuthPageProps) {
  return (
    <main className="auth-page">
      <Link className="auth-brand" to="/" aria-label="Infinitune home">
        <span aria-hidden="true">∞</span> Infinitune
      </Link>
      <div className="auth-theme-toggle">
        <ThemeToggle />
      </div>
      <section className="auth-card">
        <span className="eyebrow">A private listening space</span>
        <h1>{title}</h1>
        <p className="auth-subtitle">{subtitle}</p>
        {children}
      </section>
      <aside className="auth-showcase" aria-hidden="true">
        <div className="auth-showcase-copy">
          <span>YOUR MUSIC, YOUR SPACE</span>
          <strong>Keep every discovery within reach.</strong>
          <p>Build playlists, follow artists, and continue on any device.</p>
        </div>
        <div className="auth-atmosphere">
          <span>∞</span>
        </div>
      </aside>
    </main>
  );
}

type FormFieldProps = {
  label: string;
  htmlFor: string;
  hint?: string;
  children: React.ReactNode;
};

export function FormField({ label, htmlFor, hint, children }: FormFieldProps) {
  return (
    <label className="form-field" htmlFor={htmlFor}>
      <span>{label}</span>
      {children}
      {hint ? <small>{hint}</small> : null}
    </label>
  );
}
