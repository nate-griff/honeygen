import { FormEvent, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { loginAdminSession } from "../api/auth";
import { APIClientError } from "../api/client";
import { sanitizeNextPath } from "../app/safeRedirect";

export default function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string>();
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    setError(undefined);

    try {
      await loginAdminSession(password);
      await navigate(sanitizeNextPath(searchParams.get("next")), { replace: true });
    } catch (caughtError) {
      if (caughtError instanceof APIClientError && caughtError.status === 401) {
        setError("Incorrect admin password.");
      } else {
        setError(caughtError instanceof Error ? caughtError.message : "Unable to sign in");
      }
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <main className="login-screen">
      <section className="login-card">
        <span className="app-brand__eyebrow">Honeygen</span>
        <h1>Admin sign in</h1>
        <p className="muted">Use the env-configured admin password to access the protected control plane.</p>
        <form className="stack stack--compact" onSubmit={handleSubmit}>
          <label className="toolbar-field">
            Admin password
            <input
              autoComplete="current-password"
              autoFocus
              name="password"
              onChange={(event) => setPassword(event.target.value)}
              type="password"
              value={password}
            />
          </label>
          {error ? <p className="error-text">{error}</p> : null}
          <button className="button button--primary" disabled={isSubmitting || password.length === 0} type="submit">
            {isSubmitting ? "Signing in…" : "Sign in"}
          </button>
        </form>
      </section>
    </main>
  );
}
