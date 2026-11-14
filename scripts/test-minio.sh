#!/bin/bash

set -euo pipefail

compose=(docker compose -f compose.media.yml)
probe_path=".checks/persistence-probe"
probe_value="infinitune-minio-persistence-check"

run_client() {
    "${compose[@]}" --profile tools run --rm --no-deps \
        --entrypoint /bin/sh minio-init -ec "$1"
}

configure_alias='mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null'

cleanup() {
    run_client "$configure_alias
mc rm \"local/\$MINIO_MEDIA_BUCKET/$probe_path\" >/dev/null 2>&1 || true" >/dev/null
}
trap cleanup EXIT

run_client "$configure_alias
printf '%s' '$probe_value' | mc pipe \"local/\$MINIO_MEDIA_BUCKET/$probe_path\" >/dev/null"

"${compose[@]}" restart minio >/dev/null
"${compose[@]}" up --detach --wait minio >/dev/null

stored_value="$(run_client "$configure_alias
mc cat \"local/\$MINIO_MEDIA_BUCKET/$probe_path\"")"

if [[ "$stored_value" != "$probe_value" ]]; then
    echo "MinIO persistence check failed: unexpected object content" >&2
    exit 1
fi

echo "MinIO persistence check passed"
