import { gatewayUrl } from "../shared/api/client";

const steps = [
  {
    number: "01",
    title: "Discover",
    copy: "Browse releases and shape your sound.",
  },
  {
    number: "02",
    title: "Collect",
    copy: "Build playlists that stay in sync.",
  },
  {
    number: "03",
    title: "Listen",
    copy: "Keep the player moving between pages.",
  },
] as const;

export function DiscoverPage() {
  return (
    <section className="discover-page">
      <div className="hero-grid">
        <div className="hero-copy">
          <span className="hero-kicker">A listening space without limits</span>
          <h1>
            Music that keeps
            <br />
            <em>finding you.</em>
          </h1>
          <p>
            Infinitune brings discovery, playlists, and uninterrupted playback
            into one focused place.
          </p>
          <div className="hero-actions">
            <a className="primary-action" href="#foundation">
              Explore the foundation
            </a>
            <span className="api-pill" title={gatewayUrl}>
              API connected
            </span>
          </div>
        </div>
        <div className="orbit-card" aria-hidden="true">
          <div className="orbit orbit-one" />
          <div className="orbit orbit-two" />
          <div className="orbit-core">∞</div>
          <span className="orbit-label orbit-label-top">PLAY</span>
          <span className="orbit-label orbit-label-bottom">REPEAT</span>
        </div>
      </div>

      <div id="foundation" className="foundation-grid">
        {steps.map((step) => (
          <article className="foundation-card" key={step.number}>
            <span>{step.number}</span>
            <h2>{step.title}</h2>
            <p>{step.copy}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
