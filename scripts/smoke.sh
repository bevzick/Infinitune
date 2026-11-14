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

# Both 200 (Kong CORS plugin) and 204 (upstream middleware) are valid
# preflight statuses. The response headers determine whether browsers can
# actually make the cross-origin request.
cors_header() {
    local field="$1" file="$2"
    awk -F: -v field="$field" '
        tolower($1) == tolower(field) {
            value = substr($0, index($0, ":") + 1)
            sub(/^[[:space:]]+/, "", value)
            sub(/\r$/, "", value)
            print value
            exit
        }
    ' "$file"
}

cors_has_token() {
    local values="${1,,}" token="${2,,}"
    values="${values//[[:space:]]/}"
    [[ ",$values," == *",$token,"* ]]
}

check_cors_preflight() {
    local label="$1" path="$2" requested_method="$3"
    local headers_file status allowed_origin allowed_methods allowed_headers credentials
    headers_file="$(mktemp)"

    if ! status="$(
        curl --silent --show-error --dump-header "$headers_file" \
            --output /dev/null --write-out '%{http_code}' \
            --request OPTIONS \
            --header 'Origin: http://localhost:5173' \
            --header "Access-Control-Request-Method: $requested_method" \
            --header 'Access-Control-Request-Headers: authorization,content-type' \
            "$gateway_url$path"
    )"; then
        rm -f "$headers_file"
        echo "$label preflight request failed" >&2
        return 1
    fi

    allowed_origin="$(cors_header 'Access-Control-Allow-Origin' "$headers_file")"
    allowed_methods="$(cors_header 'Access-Control-Allow-Methods' "$headers_file")"
    allowed_headers="$(cors_header 'Access-Control-Allow-Headers' "$headers_file")"
    credentials="$(cors_header 'Access-Control-Allow-Credentials' "$headers_file")"
    rm -f "$headers_file"

    if [[ "$status" != "200" && "$status" != "204" ]]; then
        echo "$label preflight returned HTTP $status, expected 200 or 204" >&2
        return 1
    fi
    if [[ "$allowed_origin" != "http://localhost:5173" ]]; then
        echo "$label preflight has incorrect Access-Control-Allow-Origin: $allowed_origin" >&2
        return 1
    fi
    if ! cors_has_token "$allowed_methods" "$requested_method"; then
        echo "$label preflight does not allow method $requested_method: $allowed_methods" >&2
        return 1
    fi
    if ! cors_has_token "$allowed_headers" 'authorization' || \
        ! cors_has_token "$allowed_headers" 'content-type'; then
        echo "$label preflight does not allow authorization and content-type: $allowed_headers" >&2
        return 1
    fi
    if [[ "${credentials,,}" != 'true' ]]; then
        echo "$label preflight does not allow credentialed requests" >&2
        return 1
    fi
}

echo "Checking protected Interaction routes..."
interaction_id="00000000-0000-4000-8000-000000000001"
interaction_routes=(
    "GET /v1/interactions/tracks/likes"
    "GET /v1/interactions/albums/likes"
    "GET /v1/interactions/comments/likes"
    "GET /v1/interactions/artists/follows"
    "PUT /v1/interactions/tracks/$interaction_id/like"
    "DELETE /v1/interactions/tracks/$interaction_id/like"
    "PUT /v1/interactions/albums/$interaction_id/like"
    "DELETE /v1/interactions/albums/$interaction_id/like"
    "PUT /v1/interactions/comments/$interaction_id/like"
    "DELETE /v1/interactions/comments/$interaction_id/like"
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
check_cors_preflight "Interaction" "/v1/interactions/artists/follows" "GET"

echo "Checking Comment routes..."
comment_id="00000000-0000-4000-8000-000000000002"
comment_routes=(
    "POST /v1/comments"
    "PATCH /v1/comments/$comment_id"
    "DELETE /v1/comments/$comment_id"
    "POST /v1/comments/$comment_id/reports"
    "GET /v1/moderation/comment-reports"
    "POST /v1/moderation/comment-reports/$comment_id/actions"
)
for route in "${comment_routes[@]}"; do
    read -r method path <<<"$route"
    comment_status="$(
        curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
            --request "$method" "$gateway_url$path"
    )"
    if [[ "$comment_status" != "401" ]]; then
        echo "$method $path returned HTTP $comment_status, expected 401 without a token" >&2
        exit 1
    fi
done

echo "Checking public Comment route..."
public_comment_status="$(
    curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
        "$gateway_url/v1/tracks/$comment_id/comments"
)"
if [[ "$public_comment_status" != "404" ]]; then
    echo "Public Comment route returned HTTP $public_comment_status, expected 404 for an unknown track" >&2
    exit 1
fi

echo "Checking Comment CORS preflight..."
check_cors_preflight "Comment" "/v1/comments" "POST"

echo "All smoke checks passed!"
