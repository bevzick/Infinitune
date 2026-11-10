import { type FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";

import { register } from "../features/session/authApi";
import { ApiError } from "../shared/api/client";
import { AuthPage, FormField } from "./LoginPage";

export function RegisterPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (password !== confirmation) {
      setError("Passwords do not match");
      return;
    }
    setSubmitting(true);
    try {
      const account = await register(email, password);
      navigate("/login", {
        replace: true,
        state: { registeredEmail: account.email },
      });
    } catch (reason) {
      setError(
        reason instanceof ApiError
          ? reason.message
          : "Registration is temporarily unavailable",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AuthPage
      title="Create your account"
      subtitle="One account for discovery, playlists, and playback."
    >
      <form className="auth-form" onSubmit={submit}>
        <FormField label="Email" htmlFor="register-email">
          <input
            id="register-email"
            name="email"
            type="email"
            autoComplete="email"
            inputMode="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </FormField>
        <FormField
          label="Password"
          htmlFor="register-password"
          hint="Use 12–128 characters."
        >
          <input
            id="register-password"
            name="password"
            type="password"
            autoComplete="new-password"
            required
            minLength={12}
            maxLength={128}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </FormField>
        <FormField label="Confirm password" htmlFor="register-confirmation">
          <input
            id="register-confirmation"
            name="confirmation"
            type="password"
            autoComplete="new-password"
            required
            minLength={12}
            maxLength={128}
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </FormField>
        {error ? (
          <p className="form-error" role="alert">
            {error}
          </p>
        ) : null}
        <button className="auth-submit" type="submit" disabled={submitting}>
          {submitting ? "Creating account…" : "Create account"}
        </button>
      </form>
      <p className="auth-switch">
        Already have an account? <Link to="/login">Sign in</Link>
      </p>
    </AuthPage>
  );
}
