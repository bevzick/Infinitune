#!/bin/sh

set -eu

image=${1:-infinitune-frontend:smoke}
container="infinitune-frontend-smoke-$$"
port=${FRONTEND_SMOKE_PORT:-18088}
base_url="http://127.0.0.1:${port}"

cleanup() {
  docker container rm --force "$container" >/dev/null 2>&1 || true
}

trap cleanup EXIT INT TERM

docker run \
  --detach \
  --name "$container" \
  --publish "127.0.0.1:${port}:8080" \
  "$image" >/dev/null

attempt=1
while ! health=$(curl --fail --silent --show-error "${base_url}/health" 2>/dev/null); do
  if [ "$attempt" -ge 15 ]; then
    docker logs "$container" >&2
    echo "Frontend health endpoint did not become ready" >&2
    exit 1
  fi
  attempt=$((attempt + 1))
  sleep 1
done

if [ "$health" != "ok" ]; then
  echo "Unexpected frontend health response: $health" >&2
  exit 1
fi

curl --fail --silent --show-error "${base_url}/library" |
  grep --quiet '<title>Infinitune</title>'

headers=$(curl --fail --silent --show-error --head "${base_url}/")
printf '%s\n' "$headers" | grep --ignore-case --quiet '^Content-Security-Policy:'
printf '%s\n' "$headers" | grep --ignore-case --quiet '^X-Content-Type-Options: nosniff'
printf '%s\n' "$headers" | grep --ignore-case --quiet '^X-Frame-Options: DENY'

container_user=$(docker inspect --format '{{.Config.User}}' "$container")
if [ "$container_user" != "101" ]; then
  echo "Frontend container must run as UID 101, got: $container_user" >&2
  exit 1
fi

echo "Frontend container smoke test passed"

