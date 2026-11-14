export function RouteLoading({
  label,
  fullPage = false,
}: {
  label: string;
  fullPage?: boolean;
}) {
  return (
    <div
      className={`route-loading${fullPage ? " route-loading-full" : ""}`}
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <span className="route-loading-mark" aria-hidden="true">
        ∞
      </span>
      <div>
        <strong>Loading {label}…</strong>
        <span>Preparing your Infinitune space.</span>
      </div>
    </div>
  );
}
