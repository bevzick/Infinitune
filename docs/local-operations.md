# Local operations runbook

This runbook covers the containerized development stack: initial startup,
health verification, migrations, diagnostics, and safe local recovery.

## Prerequisites

- Docker Engine with Docker Compose v2
- GNU Make
- `curl` and OpenSSL
- enough free space for the Go, Node.js, PostgreSQL, Kafka, and MinIO images

Go and Node.js are only required when running services or frontend commands
directly on the host. The normal full-stack workflow builds and runs them in
containers.

## First startup

Create the local configuration:

```bash
cp .env.example .env
```

At minimum, replace the development PostgreSQL and MinIO credentials and add
an Ed25519 signing key pair. Generate the private key value with:

```bash
openssl genpkey -algorithm Ed25519 -outform DER | base64 -w0
```

Store that output as `AUTH_JWT_PRIVATE_KEY_BASE64`, then derive the matching
public key after loading the private value into the current shell:

```bash
printf '%s' "$AUTH_JWT_PRIVATE_KEY_BASE64" \
  | base64 -d \
  | openssl pkey -inform DER -pubout -outform DER \
  | base64 -w0
```

Store the second output as `AUTH_JWT_PUBLIC_KEY_BASE64`. Do not commit `.env`
or reuse these local keys in a deployed environment.

Validate the merged Compose configuration and start the stack:

```bash
make config
make up
make ps
```

`make up` applies every service's pending database migrations, starts MinIO,
creates the private media bucket, and then starts the complete stack. The
migration and initialization containers are short-lived tools; they are either
removed after success or finish with an `Exited (0)` state.

The default host endpoints are:

| Component | URL or address | Purpose |
| --- | --- | --- |
| Frontend | <http://127.0.0.1:8088> | React application and same-origin `/v1` proxy |
| Kong | <http://127.0.0.1:8000> | API gateway |
| PostgreSQL | `127.0.0.1:5432` | local database access |
| Prometheus | <http://127.0.0.1:9091> | metrics and alert rules |
| Grafana | <http://127.0.0.1:3000> | centralized log search and metrics exploration |
| Loki | <http://127.0.0.1:3100> | local log storage API |
| Alloy | <http://127.0.0.1:12345> | log collector status and diagnostics |

Only loopback addresses are published. Redis, Kafka, MinIO, and application
service ports remain inside the Compose network.

### Centralized logs

Grafana Alloy discovers Docker containers that belong to the current Compose
project and forwards their stdout and stderr to Loki. Open Grafana, sign in
with `GRAFANA_ADMIN_USER` and `GRAFANA_ADMIN_PASSWORD`, then use **Explore** and
the provisioned Loki data source. A useful starting query is:

```logql
{compose_project="infinitune"}
```

Narrow the result to one service with, for example:

```logql
{compose_project="infinitune", service="media-service"}
```

Verify all three logging components and a real Loki query from the command
line with:

```bash
make logging-smoke
```

Local logs are retained for seven days. The Loki, Alloy, and Grafana state is
stored in named volumes and is therefore preserved by `make down`, but removed
by `make down-volumes`. Alloy reads the local Docker socket to discover and
follow containers; keep this development-only configuration on a trusted host.
To avoid duplicate ingestion after a collector restart, Docker backlog older
than 55 minutes is discarded before it reaches Loki. Logs already stored in
Loki remain queryable for the full seven-day retention period.

## Verify the running stack

Run the basic infrastructure and gateway checks after the containers become
healthy:

```bash
make ps
make smoke
```

For complete API journeys, use isolated Compose projects so they do not alter
the normal development data:

```bash
COMPOSE_PROJECT_NAME=infinitune-system-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make system-smoke

COMPOSE_PROJECT_NAME=infinitune-content-media-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make content-media-smoke
```

Remove each isolated project, including its disposable volumes, when finished:

```bash
COMPOSE_PROJECT_NAME=infinitune-system-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make down-volumes

COMPOSE_PROJECT_NAME=infinitune-content-media-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make down-volumes
```

## Routine operations

| Command | Effect |
| --- | --- |
| `make ps` | show container state and health |
| `make logs` | follow logs from the whole stack |
| `make logs-ci` | print a finite log snapshot |
| `make stop` | stop containers while preserving them and all data |
| `make restart` | restart existing containers without migrations or reinitialization |
| `make down` | remove containers and networks while preserving named volumes |
| `make up` | apply migrations, initialize storage, and start the stack |
| `make down-volumes` | remove containers, networks, and all local named volumes |

Use `make down && make up` for a clean container recreation that preserves
local data. Use `make up`, rather than only `make restart`, after pulling a
change that includes migrations.

## Database migrations

The stack uses a separate database per service inside the shared PostgreSQL
container. Migration tools connect to the existing PostgreSQL service; they do
not create an additional database server.

Inspect or apply one service's migrations with:

```bash
make auth-migrate-version
make auth-migrate-up
```

Replace `auth` with `user`, `catalog`, `media`, `playlist`, `content`,
`interaction`, `comment`, `history`, or `recommendation`. Migration commands
are idempotent when the schema is already current.

Do not force a dirty migration version without first reading the failed
migration and checking the affected schema. Preserve the PostgreSQL volume or
take a logical backup before manual repair.

## Diagnose an unhealthy stack

Start with a bounded snapshot:

```bash
make ps
make logs-ci
```

Then use the symptom to narrow the check:

| Symptom | Checks |
| --- | --- |
| Compose configuration fails | Compare `.env` with `.env.example`; required JWT and MinIO values must be non-empty. |
| `auth-service` is unhealthy | Validate that both Ed25519 values are DER encoded and form one pair; check PostgreSQL and Redis health; run `make auth-migrate-version`. |
| A service reports JWKS unavailable | Confirm `auth-service` is healthy first. Internal services resolve `http://auth-service:8080/v1/auth/jwks`; Kong uses the configured public key. |
| A service reports a missing table or column | Run its `*-migrate-version`, then `*-migrate-up`; inspect logs before changing migration state manually. |
| Kafka consumer does not progress | Check `kafka` and `kafka-init`; verify the topic and consumer group documented in [Kafka events](kafka-events.md). |
| Media upload or playback fails | Check `minio`, `media-service`, and `content-service`; rerun `make media-storage-init`; confirm the media bucket and credentials match `.env`. |
| Kong returns `502` or `503` | Find the upstream service in Kong logs, then inspect that service and its dependencies. |
| A host port is already allocated | Stop the conflicting process. `FRONTEND_PORT` and `PROMETHEUS_PORT` are configurable; PostgreSQL and Kong currently use fixed local ports. |

The focused smoke-test log commands are useful after a failed isolated run:

```bash
COMPOSE_PROJECT_NAME=infinitune-system-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make system-smoke-logs

COMPOSE_PROJECT_NAME=infinitune-content-media-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make content-media-smoke-logs
```

## Recovery levels

Use the least destructive recovery that addresses the failure:

1. `make restart` for a transient failure when configuration and schema have
   not changed.
2. `make down && make up` to recreate containers and networks while preserving
   application data.
3. Reapply the affected service migration and storage initialization targets
   when logs identify incomplete setup.
4. `make down-volumes && make up` only when disposable local state can be
   deleted and recreated from scratch.

The last option removes all local PostgreSQL databases, Redis state, Kafka
topics, MinIO objects, and Prometheus history. It is not a routine restart and
cannot be undone without an external backup.
