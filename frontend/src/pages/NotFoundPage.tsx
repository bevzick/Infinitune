import { Link } from "react-router-dom";

export function NotFoundPage() {
  return (
    <section className="message-page">
      <span className="eyebrow">404 · Off beat</span>
      <h1>This page is not in the queue.</h1>
      <p>The route may be part of a later MVP screen.</p>
      <Link className="primary-action" to="/">
        Return to Discover
      </Link>
    </section>
  );
}
