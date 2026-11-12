# Infinitune

## Containerized frontend

The production frontend is served by an unprivileged Nginx container on
<http://localhost:8088>. Requests to `/v1` are proxied to Kong inside the
Compose network, while all other unknown routes fall back to the React SPA.

Copy `.env.example` to `.env`, provide the required credentials and signing
keys, then use the existing stack commands:

```bash
make config
make up
```

Set `FRONTEND_PORT` to change the host port. Keep `VITE_GATEWAY_URL=/` for the
containerized same-origin setup.

To verify the production image without starting the backend stack:

```bash
docker build --tag infinitune-frontend:smoke frontend
frontend/scripts/container-smoke.sh infinitune-frontend:smoke
```

The smoke test checks the health endpoint, SPA route fallback, security headers,
and the unprivileged runtime user. The same check runs in GitHub Actions.

## Continuous integration

Frontend changes run linting, type checking, unit and browser tests, and the
production-container smoke test. Backend changes use a matrix job for every Go
service and the Kafka replay tool; each module verifies dependencies, runs
`go vet`, executes unit tests with the race detector, and builds all packages.
Integration tests run in an isolated Docker Compose project per service. They
create fresh databases, apply migrations, exercise PostgreSQL, Redis, Kafka and
MinIO integrations, print container logs on failure, and remove resources after
every job. The CI Compose override keeps internal service ports off the host,
so the suites remain isolated from an already running development stack.

## Full-stack API smoke test

The system smoke test exercises a real listener journey through Kong and seven
services: registration, login, profile creation, catalog browsing, playlists,
likes, comments, and listening history. Run it in an isolated Compose project:

```bash
COMPOSE_PROJECT_NAME=infinitune-system-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make system-smoke

COMPOSE_PROJECT_NAME=infinitune-system-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make down-volumes
```

The same isolated journey runs in GitHub Actions when one of the participating
services, Kong routes, or Compose definitions changes. Failed runs print only
the relevant service logs before removing all containers and volumes.

## Creator content publishing

The creator workflow covers account verification, release drafts, track
rights, idempotent media uploads, asynchronous processing, moderation, Catalog
publication, and authenticated byte-range playback. See the
[content publishing guide](docs/content-publishing.md) for API examples,
state transitions, retry behavior, and the isolated end-to-end test command.

## Local operations

See the [local operations runbook](docs/local-operations.md) for first-time
configuration, stack health checks, migration commands, failure diagnosis, and
data-preserving recovery steps.
