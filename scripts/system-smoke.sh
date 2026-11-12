#!/bin/bash

set -euo pipefail

gateway_url=${GATEWAY_URL:-http://127.0.0.1:8000}
track_id=30000000-0000-4000-8000-000000000001
response_file=$(mktemp)
cookie_jar=$(mktemp)

cleanup() {
    rm -f "$response_file" "$cookie_jar"
}
trap cleanup EXIT

for command in curl jq; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "$command is required for the system smoke test" >&2
        exit 1
    fi
done

request() {
    local expected_status=$1
    local method=$2
    local path=$3
    local body=${4:-}
    local token=${5:-}
    local arguments=(
        --silent
        --show-error
        --connect-timeout 5
        --max-time 30
        --output "$response_file"
        --write-out '%{http_code}'
        --request "$method"
        --cookie "$cookie_jar"
        --cookie-jar "$cookie_jar"
        --header 'Accept: application/json'
    )
    if [[ -n "$body" ]]; then
        arguments+=(--header 'Content-Type: application/json' --data "$body")
    fi
    if [[ -n "$token" ]]; then
        arguments+=(--header "Authorization: Bearer $token")
    fi

    local status
    status=$(curl "${arguments[@]}" "$gateway_url$path")
    if [[ "$status" != "$expected_status" ]]; then
        echo "$method $path returned HTTP $status, expected $expected_status" >&2
        cat "$response_file" >&2
        exit 1
    fi
}

echo "Waiting for the API gateway..."
gateway_ready=false
for _ in {1..90}; do
    if status=$(curl --silent --output /dev/null --write-out '%{http_code}' \
        --connect-timeout 2 --max-time 5 "$gateway_url/v1/auth/jwks") && [[ "$status" == "200" ]]; then
        gateway_ready=true
        break
    fi
    sleep 1
done
if [[ "$gateway_ready" != "true" ]]; then
    echo "API gateway did not become ready at $gateway_url" >&2
    exit 1
fi

echo "Checking seeded public catalog..."
request 200 GET "/v1/catalog/tracks?limit=10"
jq --exit-status --arg track_id "$track_id" \
    '.items | any(.id == $track_id and .title == "First Light Engine")' \
    "$response_file" >/dev/null

suffix="$(date +%s)${RANDOM}"
email="smoke-${suffix}@example.test"
password="Smoke-password-${suffix}"
username="smoke${suffix}"

echo "Registering and authenticating a listener..."
request 201 POST /v1/auth/register \
    "$(jq --null-input --compact-output --arg email "$email" --arg password "$password" \
        '{email: $email, password: $password}')"
user_id=$(jq --exit-status --raw-output '.id' "$response_file")

request 200 POST /v1/auth/login \
    "$(jq --null-input --compact-output --arg email "$email" --arg password "$password" \
        '{email: $email, password: $password}')"
access_token=$(jq --exit-status --raw-output '.access_token' "$response_file")

request 200 GET /v1/auth/identity "" "$access_token"
jq --exit-status --arg user_id "$user_id" \
    '.user_id == $user_id and (.roles | index("listener") != null)' \
    "$response_file" >/dev/null

echo "Creating private and public user data..."
request 201 POST /v1/users/me/profile \
    "$(jq --null-input --compact-output --arg username "$username" \
        '{username: $username, display_name: "System Smoke Listener"}')" \
    "$access_token"
jq --exit-status --arg user_id "$user_id" --arg username "$username" \
    '.user_id == $user_id and .username == $username' "$response_file" >/dev/null

request 200 GET "/v1/users/profiles/$user_id"
jq --exit-status --arg username "$username" '.username == $username' \
    "$response_file" >/dev/null

request 201 POST /v1/playlists \
    '{"name":"System smoke playlist","description":"Cross-service verification","visibility":"public"}' \
    "$access_token"
playlist_id=$(jq --exit-status --raw-output '.id' "$response_file")
jq --exit-status --arg user_id "$user_id" \
    '.owner_id == $user_id and .type == "manual" and .visibility == "public"' \
    "$response_file" >/dev/null

request 201 POST "/v1/playlists/$playlist_id/tracks" \
    "$(jq --null-input --compact-output --arg track_id "$track_id" '{track_id: $track_id}')" \
    "$access_token"
jq --exit-status --arg track_id "$track_id" '.track_id == $track_id' \
    "$response_file" >/dev/null

request 200 GET "/v1/playlists/$playlist_id/tracks"
jq --exit-status --arg track_id "$track_id" \
    '.items | any(.track_id == $track_id)' "$response_file" >/dev/null

echo "Checking interactions and comments..."
request 200 PUT "/v1/interactions/tracks/$track_id/like" "" "$access_token"
request 200 GET /v1/interactions/tracks/likes "" "$access_token"
jq --exit-status --arg track_id "$track_id" '.items | any(.id == $track_id)' \
    "$response_file" >/dev/null

request 201 POST /v1/comments \
    "$(jq --null-input --compact-output --arg track_id "$track_id" \
        '{track_id: $track_id, content: "Full-stack smoke comment"}')" \
    "$access_token"
comment_id=$(jq --exit-status --raw-output '.id' "$response_file")

request 200 GET "/v1/tracks/$track_id/comments"
jq --exit-status --arg comment_id "$comment_id" \
    '.items | any(.id == $comment_id and .content == "Full-stack smoke comment")' \
    "$response_file" >/dev/null

request 200 PUT "/v1/interactions/comments/$comment_id/like" "" "$access_token"
request 200 GET /v1/interactions/comments/likes "" "$access_token"
jq --exit-status --arg comment_id "$comment_id" '.items | any(.id == $comment_id)' \
    "$response_file" >/dev/null

echo "Recording and reading listening history..."
event_id=$(cat /proc/sys/kernel/random/uuid)
finished_at=$(date --utc +%Y-%m-%dT%H:%M:%SZ)
started_at=$(date --utc --date='6 seconds ago' +%Y-%m-%dT%H:%M:%SZ)
request 200 POST /v1/history/events \
    "$(jq --null-input --compact-output \
        --arg event_id "$event_id" \
        --arg track_id "$track_id" \
        --arg started_at "$started_at" \
        --arg finished_at "$finished_at" \
        '{event_id: $event_id, track_id: $track_id, started_at: $started_at,
          finished_at: $finished_at, listened_ms: 6000, context_type: "direct",
          device_type: "web"}')" \
    "$access_token"
jq --exit-status --arg event_id "$event_id" \
    '.event_id == $event_id and .recorded == true and .completed == true' \
    "$response_file" >/dev/null

request 200 GET "/v1/history/events?limit=10" "" "$access_token"
jq --exit-status --arg event_id "$event_id" --arg track_id "$track_id" \
    '.items | any(.event_id == $event_id and .track_id == $track_id)' \
    "$response_file" >/dev/null

echo "Cleaning up mutable smoke resources..."
request 204 DELETE "/v1/interactions/comments/$comment_id/like" "" "$access_token"
request 204 DELETE "/v1/comments/$comment_id" "" "$access_token"
request 204 DELETE "/v1/interactions/tracks/$track_id/like" "" "$access_token"
request 204 DELETE "/v1/playlists/$playlist_id" "" "$access_token"

echo "Full-stack API smoke test passed"

