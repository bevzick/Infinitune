import { isRouteErrorResponse, Link, useRouteError } from "react-router-dom";

export function RouteErrorPage() {
  const error = useRouteError();
  const message = isRouteErrorResponse(error)
    ? `${error.status} ${error.statusText}`
    : "The page could not be loaded.";

  return (
    <main className="message-page route-error" role="alert">
      <span className="eyebrow">Something slipped out of rhythm</span>
      <h1>{message}</h1>
      <p>Try returning to the start of the experience.</p>
      <Link className="primary-action" to="/">
        Return home
      </Link>
    </main>
  );
}
