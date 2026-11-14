#!/usr/bin/env bash
# One-command disposable system journey. Never reuses the developer's project.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
for command in docker make curl jq python3; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "$command is required for the isolated creator-to-listener smoke test" >&2
        exit 1
    fi
done
if [[ ! -f .env ]]; then
    echo "Create .env from .env.example and configure development-only credentials first" >&2
    exit 1
fi

# Override inherited variables so neither developer data nor a CI project is touched.
export COMPOSE_PROJECT_NAME="infinitune-creator-listener-$(date +%s)-$$"
export COMPOSE_OVERRIDE='-f compose.system-test.yml'
export SYSTEM_TEST_GATEWAY_PORT="${SYSTEM_TEST_GATEWAY_PORT:-18115}"
export SYSTEM_TEST_FRONTEND_PORT="${SYSTEM_TEST_FRONTEND_PORT:-18116}"
export GATEWAY_URL="http://127.0.0.1:${SYSTEM_TEST_GATEWAY_PORT}"

cleanup() {
    status=$?
    trap - EXIT INT TERM
    if (( status != 0 )); then
        echo "System journey failed; removing only the disposable project." >&2
    fi
    # These commands operate solely on the generated project name.
    if ! make down-volumes; then
        echo "Could not remove isolated Compose project: $COMPOSE_PROJECT_NAME" >&2
        if (( status == 0 )); then status=1; fi
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

echo "Starting disposable Compose project: $COMPOSE_PROJECT_NAME"
make _creator-listener-smoke-stack
VERIFY_CREATOR_LISTENER_JOURNEY=1 GATEWAY_URL="$GATEWAY_URL" \
    bash scripts/content-media-smoke.sh
