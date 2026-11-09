#!/bin/bash

set -euo pipefail

echo "Running smoke checks..."

echo "Checking PostgreSQL..."
docker compose exec --no-tty postgres \
    sh -c 'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"'

echo "Checking Redis..."
docker compose exec --no-tty redis redis-cli ping

echo "Checking Kong..."
docker compose exec --no-tty kong kong health

if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required for gateway smoke checks" >&2
    exit 1
fi

gateway_url="http://127.0.0.1:8000"

echo "Checking protected Interaction routes..."
interaction_id="00000000-0000-4000-8000-000000000001"
interaction_routes=(
    "GET /v1/interactions/tracks/likes"
    "GET /v1/interactions/albums/likes"
    "GET /v1/interactions/artists/follows"
    "PUT /v1/interactions/tracks/$interaction_id/like"
    "DELETE /v1/interactions/tracks/$interaction_id/like"
    "PUT /v1/interactions/albums/$interaction_id/like"
    "DELETE /v1/interactions/albums/$interaction_id/like"
    "PUT /v1/interactions/artists/$interaction_id/follow"
    "DELETE /v1/interactions/artists/$interaction_id/follow"
)
for route in "${interaction_routes[@]}"; do
    read -r method path <<<"$route"
    interaction_status="$(
        curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
            --request "$method" "$gateway_url$path"
    )"
    if [[ "$interaction_status" != "401" ]]; then
        echo "$method $path returned HTTP $interaction_status, expected 401 without a token" >&2
        exit 1
    fi
done

echo "Checking Interaction CORS preflight..."
preflight_status="$(
    curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
        --request OPTIONS \
        --header 'Origin: http://localhost:5173' \
        --header 'Access-Control-Request-Method: GET' \
        "$gateway_url/v1/interactions/artists/follows"
)"
if [[ "$preflight_status" != "204" ]]; then
    echo "Interaction preflight returned HTTP $preflight_status, expected 204" >&2
    exit 1
fi

echo "All smoke checks passed!"
