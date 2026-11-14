# Infinitune release and acceptance guide

This guide is the acceptance checklist for the **containerized course-project build**.
It documents the repository's implemented development deployment, not a
production-grade public Internet deployment. The checked result must come from
an actual local or CI run; this document does not claim that any test passed.

## 1. Scope and ownership

- **Clients:** React SPA built by Vite and served by an unprivileged Nginx
  container. Browser `/v1` requests are proxied through Kong on the same origin.
- **Backend:** Go/Gin services for Auth, User, Catalog, Media, Playlist,
  Content, Interaction, Comment, History and Recommendation. Each owns its
  domain data in a separate PostgreSQL database; no cross-service SQL joins.
- **Infrastructure:** PostgreSQL, Redis, private MinIO, Kafka, Kong,
  Prometheus, Loki, Grafana Alloy and Grafana.
- **Internal contracts:** gRPC for synchronous service-to-service checks,
  Kafka for asynchronous events. Catalog publication is eventually consistent.
- **Access control:** Ed25519 JWT plus creator account membership. Creator
  approval requires an independent moderator; a creator cannot self-approve.

This workflow covers development/acceptance use only. See
[local operations](local-operations.md),
[content publishing](content-publishing.md),
[media source policy](media-sources.md) and
[Kafka events](kafka-events.md) for details.

## 2. Prerequisites and configuration

From the repository root, provide Docker Engine + Compose v2, GNU Make and
`curl`; the isolated creator-to-listener smoke test additionally requires
`jq` and `python3`. Direct backend testing requires Go **1.25**; direct
frontend testing requires the Node/npm versions supported by `frontend/`.

```bash
cp .env.example .env          # only on first setup; do not overwrite an existing .env
# Edit .env with local-only credentials and a matching Ed25519 key pair.
make config
```

`make config` validates Compose configuration but does not test connectivity.
Set all mandatory fields before running it. The source of truth for variable
names and defaults is [`.env.example`](../.env.example). In particular:

| Group | Variables or behavior |
| --- | --- |
| Database | `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `POSTGRES_*_DB` |
| Signing | `AUTH_JWT_PRIVATE_KEY_BASE64`, `AUTH_JWT_PUBLIC_KEY_BASE64`, `AUTH_JWT_ISSUER`, `AUTH_JWT_AUDIENCE` |
| Media | `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`, `MINIO_MEDIA_BUCKET`, `MINIO_ENDPOINT` |
| Frontend | `FRONTEND_PORT` (default `8088`), `VITE_GATEWAY_URL=/` for same-origin proxy |
| Observability | `GRAFANA_ADMIN_USER`, `GRAFANA_ADMIN_PASSWORD`, `PROMETHEUS_PORT`, `GRAFANA_PORT`, `LOKI_PORT`, `ALLOY_PORT` |
| Optional imports | `MEDIA_JAMENDO_CLIENT_ID` (Internet Archive does not require a read API key) |

The repository's `.env.example` contains **development placeholders**, not
safe production credentials. The setup instructions for generating a matching
Ed25519 pair are in [local operations](local-operations.md#first-startup).
Keep `.env`, private keys, OAuth/provider credentials, backup archives and
sensitive user data outside Git and CI logs. The local stack binds exposed
host ports to loopback; it does not configure public TLS or a public reverse
proxy. For a remote deployment, first provision TLS, proper cookie security,
secrets management, access control, and network isolation outside this guide.

## 3. Start and migrate the complete stack

Run these from the repository root:

```bash
make config
make up
make ps
make smoke
```

`make up` runs the service migrations and initializes the private MinIO media
bucket before starting the stack. It is the preferred command after pulling
changes that include a migration; `make restart` does **not** run migrations.

The migration targets follow the convention `make <service>-migrate-up` and
`make <service>-migrate-version`, where `<service>` is one of:
`auth`, `user`, `catalog`, `media`, `playlist`, `content`, `interaction`,
`comment`, `history`, `recommendation`.

For example:

```bash
make content-migrate-version
make content-migrate-up
make content-migrate-version
make media-storage-init
```

Migrations are append-only after merge. Do not forcibly change the migration
version when a migration is dirty; inspect the error and preserve/backup the
corresponding database before manual recovery.

Default local endpoints (overridable where indicated in `.env.example`):

| Service | Address |
| --- | --- |
| User-facing frontend | <http://127.0.0.1:8088> |
| Kong API gateway | <http://127.0.0.1:8000> |
| Grafana | <http://127.0.0.1:3000> |
| Prometheus | <http://127.0.0.1:9091> |

The same-origin production frontend serves API requests at `/v1`. The Vite
local dev server and direct Kong requests use a different origin; do not copy
local CORS or insecure cookie settings into a public deployment.

### Administrator bootstrap

Register an ordinary account first through the application, then elevate that
existing, active account using the operator-only tool:

```bash
make auth-bootstrap-admin ADMIN_EMAIL=admin@example.com
```

Replace `admin@example.com` with the registered account. Sign out and log in
again to refresh JWT role claims. This operation must not be exposed through
public APIs. Independently assigned moderator identities are required for
creator and release review.

## 4. Verification before acceptance

Run frontend checks from `frontend/`:

```bash
cd frontend
npm ci
npm run lint
npm run typecheck
npm test
npm run build
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/chromium npm run test:e2e
cd ..
```

The Playwright Chromium override is only needed when the browser is installed
at `/usr/bin/chromium`; otherwise use the project's default Playwright setup.

Each Go service is a separate module. For a changed service, for example:

```bash
cd services/content-service
go vet ./...
go test ./...
go build ./...
cd ../..
```

The Makefile also offers `make auth-test`, `make content-test`, etc. Verify
PostgreSQL, MinIO and Kafka integration where relevant using the existing
`*-test-integration` targets, in an isolated Compose project. Review the
GitHub Actions workflows under `.github/workflows/` for the CI checks; a green
unit-test run alone is not a full system acceptance.

### Real-service creator-to-listener system journey

```bash
make creator-listener-smoke
```

This command creates a uniquely named **disposable** Compose project, executes
a real backend/API journey through Kong, and removes only that project's
containers and volumes after completion (including on failure). It requires
`jq`, `python3`, `curl` and a configured `.env`. The default test gateway port
is `18115`; change it with `SYSTEM_TEST_GATEWAY_PORT=18125` if occupied.

It validates creator verification, cover/WAV upload, FFmpeg media processing,
rights, independent moderation, asynchronous Kafka/Catalog publication,
byte-range playback, playlists, interactions, comment reports, listening
history idempotency and personalized recommendations. It uses **real services,
not Playwright gateway mocks**. Follow
[local operations](local-operations.md#isolated-creator-to-listener-acceptance-journey)
for the safety and troubleshooting rules.

### Acceptance checklist (mark only after observing evidence)

- [ ] `make config`, migrations, `make ps` and `make smoke` succeed.
- [ ] Registration, login, refresh, logout and role-based navigation work.
- [ ] An authenticated user can edit profile, appearance and privacy settings.
- [ ] Creator account creation, membership roles and independent verification work.
- [ ] Single/EP/album drafts keep consistent track metadata.
- [ ] Cover and audio upload sessions enforce permissions, formats and limits.
- [ ] Uploaded audio reaches `ready`; private draft media is not publicly playable.
- [ ] Worldwide streaming and distribution rights are checked at the release date.
- [ ] Readiness reports all blockers, submission rechecks them, and a different
      moderator approves or rejects the release.
- [ ] Publication appears in Catalog after asynchronous delivery, without
      assuming immediate visibility or creating duplicate published entries.
- [ ] Published playback supports seeking/range requests and available qualities.
- [ ] Playlists, likes/follows, comments/replies/reports and moderator actions work.
- [ ] Private history is user-isolated, idempotent and can be cleared.
- [ ] Recommendations return valid results, including the cold-start path.
- [ ] Frontend lint, typecheck, unit tests, build and browser E2E all pass.
- [ ] `make creator-listener-smoke` passes with real services in disposable Compose.
- [ ] Logs/metrics are visible in Grafana; `make logging-smoke` passes.
- [ ] Backup **and a restore rehearsal in a disposable environment** have been
      performed following [backup and recovery](backup-and-recovery.md).

Record the tested Git SHA, date, environment, commands, outcomes and any
exceptions in the project's PR/release notes. Do **not** check boxes by
assumption or infer success from a green unit-test run.

## 5. Recovery, operational boundaries and release decision

For diagnosis use `make ps`, `make logs-ci` and the Grafana dashboards.
`make down` preserves volumes; `make down-volumes` **deletes the project's
volumes**. Never use `down-volumes` for the primary development project unless
its data loss has been explicitly approved. Backups, restore rehearsal and
Kafka/Redis limitations are in [backup and recovery](backup-and-recovery.md).

The course-project build intentionally does not include a production Kafka
cluster with replication/TLS/SASL, external secret manager, public TLS,
Kubernetes, CDN, disaster-recovery orchestration or load testing. OpenSearch
is not needed while the PostgreSQL-based search meets project requirements.
These are **deferred production improvements**, not features to claim as
already implemented.

The release is acceptable for the course-project scope only after the
checklist and at least one isolated **real-service** creator-to-listener journey
pass. Any failed or unrun required check must be recorded as an outstanding
blocker, not silently marked complete.
