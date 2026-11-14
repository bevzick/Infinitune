# Backup and recovery for local Infinitune

This runbook is for the **local Docker Compose/course-project installation**.
It is not a high-availability or production disaster-recovery procedure.
Practice on a separate disposable environment before relying on any backup.

## What must be preserved

- **PostgreSQL:** ten service-owned databases: `auth_db`, `user_db`,
  `catalog_db`, `media_db`, `playlist_db`, `content_db`, `interaction_db`,
  `comment_db`, `history_db`, `recommendation_db` by default. Their actual names
  come from `POSTGRES_*_DB` in `.env`.
- **MinIO:** private bucket specified by `MINIO_MEDIA_BUCKET` (default
  `infinitune-media`), including original covers/audio and processed variants.
- **Environment and cryptographic material:** keep a separate secured copy of
  `.env`, Ed25519 JWT keys and any external credentials; never put these in a
  public Git repository or inside an unencrypted shared backup.

Redis contains refresh sessions, while Kafka contains event delivery state and
consumer progress. The PostgreSQL+MinIO procedure below **does not** back up
Redis sessions, Kafka offsets/topics or Grafana/Loki history. After restoring,
users may need to log in again; event replay and downstream state may need
manual reconciliation. Outbox records in PostgreSQL do not by themselves
prove that Kafka and every consumer were restored to a consistent point.

## Before taking a consistent snapshot

1. Choose a maintenance window; stop browser writes/uploads and pause the
   application services/consumers that modify PostgreSQL and MinIO. Leave the
   PostgreSQL and MinIO storage services running to perform the exports.
2. Check that in-flight uploads, media processing and Kafka publication have
   settled or record the outstanding operations for reconciliation.
3. Ensure enough disk space for all PostgreSQL dumps **and** media objects.
4. Keep backups **outside the repository**, with restricted filesystem
   permissions and encrypted/off-machine storage where appropriate.

The commands below are illustrative **logical backups**, not an atomic
cross-service snapshot. If writes continue during backup, PostgreSQL and MinIO
can represent different moments in time. A consistent production-grade
solution requires coordinated quiescence and tested storage snapshots.

## PostgreSQL database exports

From the repository root, after confirming the database names in `.env`:

```bash
umask 077
backup_dir="$HOME/infinitune-backups/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$backup_dir/postgres" "$backup_dir/minio"

# Default database names; adjust if any POSTGRES_*_DB override differs.
for db in auth_db user_db catalog_db media_db playlist_db content_db \
          interaction_db comment_db history_db recommendation_db; do
  docker compose -f compose.yml exec -T postgres \
    sh -c 'exec pg_dump -U "$POSTGRES_USER" -Fc "$1"' sh "$db" \
    > "$backup_dir/postgres/$db.dump"
  test -s "$backup_dir/postgres/$db.dump" || exit 1
done
```

`pg_dump -Fc` captures schema/data of each database, not global PostgreSQL
roles and tablespaces. Keep the original `.env` (including `POSTGRES_USER`) and
init/migration definitions needed to recreate the cluster. For every dump,
check it can be listed by `pg_restore`:

```bash
for file in "$backup_dir"/postgres/*.dump; do
  docker compose -f compose.yml exec -T postgres \
    pg_restore --list < "$file" > /dev/null || exit 1
done
```

A readable dump is **not** a successful restore test. Verify an actual restore
in an isolated disposable PostgreSQL environment before marking recovery
validated.

## Private MinIO media export

With MinIO running and its `minio-init` client image available, mirror the
configured media bucket into the same backup directory:

```bash
docker compose -f compose.media.yml --profile tools run --rm \
  --entrypoint /bin/sh \
  --volume "$backup_dir/minio:/backup" \
  minio-init -ec \
  'mc alias set source http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null && \
   mc mirror "source/$MINIO_MEDIA_BUCKET" /backup'
```

This copies object contents for the current bucket view. It is **not** a
version-history, retention-policy or bucket-metadata backup. Record the bucket
name, source revision and file count; compare counts/sizes on a restore
rehearsal. The media bucket must stay private (`mc anonymous set none` is
performed by `make media-storage-init`).

## Restore rehearsal (disposable environment only)

1. Provision a **new, isolated Compose project** with empty PostgreSQL and
   MinIO volumes and matching service database names. Do not point a rehearsal
   at the primary Infinitune project or its volumes.
2. Start storage/init services so the databases and private media bucket
   exist. Keep application writers stopped until database and media restore
   finish.
3. Restore each dump into its matching **empty** database. The following is a
   reference for an isolated target, not a command to run on a live system:

```bash
# ONLY against a disposable PostgreSQL project with matching empty databases.
# Run from the repository root and configure COMPOSE_PROJECT_NAME for that target.
# `--clean` removes objects already present in the selected database.
db=auth_db
docker compose -f compose.yml exec -T postgres \
  sh -c 'exec pg_restore -U "$POSTGRES_USER" -d "$1" \
    --clean --if-exists --no-owner --no-acl --exit-on-error' sh "$db" \
  < "$backup_dir/postgres/$db.dump"
```

Repeat for each database. If schemas were already migrated, verify that the
restore's schema/migration table and current code versions agree before
starting services; do not automatically rerun all migrations against an
unknown backup.

4. In the **isolated** MinIO project, mirror saved objects into its private
   bucket. Set `COMPOSE_PROJECT_NAME` to the unique restore project first;
   never use the name of your main development stack:

```bash
# ONLY after selecting a disposable restore Compose project.
# The target private bucket must already exist (make media-storage-init).
docker compose -f compose.media.yml --profile tools run --rm \
  --entrypoint /bin/sh \
  --volume "$backup_dir/minio:/backup:ro" \
  minio-init -ec \
  'mc alias set target http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null && \
   mc mirror /backup "target/$MINIO_MEDIA_BUCKET"'
```

   Verify object keys and contents with `mc stat`/downloads; do not grant
   anonymous read access. Mirroring into a nonempty bucket does not guarantee
   deletion of stale objects, so use a clean restore bucket for the rehearsal.
5. Start services, inspect migration versions, replay/reconcile any pending
   Kafka events **under operator supervision**, and run functional health and
   media playback checks. Expect Redis refresh sessions to be missing unless
   separately preserved.
6. Record test date, backup location, observed errors and actual recovery time.

**Never** use `make down-volumes` to prepare the primary development project's
restore. It deletes its databases and media objects. Use an explicitly unique
Compose project for rehearsal and remove **only that project** after checking
its name.

## Operational recovery by severity

- **Transient service error:** `make ps`, `make logs-ci`, then `make restart`
  when no migration/configuration change is involved.
- **Changed schema/configuration:** inspect `*-migrate-version`, use `make up`,
  check logs; preserve volumes and existing data.
- **Damaged or lost data:** stop writes, determine affected stores, restore a
  verified backup in an isolated environment first, and plan Kafka/MinIO
  reconciliation. Do not assert recovery from a successful container restart.
- **Disposable smoke-test data:** the `make creator-listener-smoke` wrapper
  creates and deletes its own volumes. Its temporary state is not a backup.

See [local operations](local-operations.md) for additional health checks,
[release acceptance](release-acceptance.md) for the acceptance gates and
[Kafka events](kafka-events.md) for DLQ/replay constraints.
