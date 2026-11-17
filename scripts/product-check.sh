#!/usr/bin/env bash

# Complete local acceptance runner. Unit/browser checks do not mutate the
# development stack; the real-service journey owns a uniquely named disposable
# Compose project and removes it when finished.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

for command in docker go make node npm curl jq python3; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "$command is required for the product acceptance suite" >&2
        exit 1
    fi
done
if [[ ! -f .env ]]; then
    echo "Create .env from .env.example before running the product acceptance suite" >&2
    exit 1
fi

# Prefer an already installed system browser. When none is available,
# Playwright uses its managed browser and reports the install command if needed.
if [[ -z "${PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH:-}" ]] && command -v chromium >/dev/null 2>&1; then
    export PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH
    PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=$(command -v chromium)
fi

run_step() {
    local label=$1
    shift
    echo
    echo "==> $label"
    "$@"
}

run_step "Validating Docker Compose configuration" make config

backend_targets=(
    auth-test
    user-test
    catalog-test
    media-test
    content-test
    playlist-test
    interaction-test
    comment-test
    history-test
    recommendation-test
)
for target in "${backend_targets[@]}"; do
    run_step "Running ${target}" make "$target"
done

run_step "Linting the frontend" make frontend-lint
run_step "Type-checking the frontend" make frontend-typecheck
run_step "Running frontend unit tests" make frontend-test
run_step "Building the frontend" make frontend-build
run_step "Running frontend browser tests" make frontend-test-e2e

run_step \
    "Running the isolated creator-to-listener real-service journey" \
    make creator-listener-smoke

echo
echo "Infinitune local product acceptance passed"
