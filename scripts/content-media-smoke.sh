#!/bin/bash

set -euo pipefail

gateway_url=${GATEWAY_URL:-http://127.0.0.1:18000}
auth_database=${POSTGRES_AUTH_DB:-auth_db}
response_file=$(mktemp)
headers_file=$(mktemp)
audio_file=$(mktemp --suffix=.wav)
cover_file=$(mktemp --suffix=.png)

cleanup() {
    rm -f "$response_file" "$headers_file" "$audio_file" "$cover_file"
}
trap cleanup EXIT

for command in curl docker jq python3; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "$command is required for the content and media smoke test" >&2
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

register_and_login() {
    local email=$1
    local password=$2
    request 201 POST /v1/auth/register \
        "$(jq --null-input --compact-output --arg email "$email" --arg password "$password" \
            '{email: $email, password: $password}')"
    registered_user_id=$(jq --exit-status --raw-output '.id' "$response_file")
    request 200 POST /v1/auth/login \
        "$(jq --null-input --compact-output --arg email "$email" --arg password "$password" \
            '{email: $email, password: $password}')"
    registered_access_token=$(jq --exit-status --raw-output '.access_token' "$response_file")
}

grant_test_role() {
    local user_id=$1
    local role=$2
    if [[ ! "$user_id" =~ ^[0-9a-fA-F-]{36}$ ]] || [[ ! "$role" =~ ^[a-z_]+$ ]]; then
        echo "Refusing to bootstrap an invalid test role assignment" >&2
        exit 1
    fi
    docker compose -f compose.yml exec -T postgres \
        sh -ec "psql --username \"\$POSTGRES_USER\" --dbname '$auth_database' \
            --set=ON_ERROR_STOP=1 --quiet \
            --command=\"INSERT INTO auth.user_roles (user_id, role_id, created_at)
                SELECT '$user_id'::uuid, id, clock_timestamp()
                FROM auth.roles WHERE name = '$role'
                ON CONFLICT (user_id, role_id) DO NOTHING\""
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

suffix="$(date +%s)${RANDOM}"
password="Content-smoke-${suffix}"

echo "Creating a moderator test identity..."
moderator_email="moderator-${suffix}@example.test"
register_and_login "$moderator_email" "$password"
moderator_id=$registered_user_id
grant_test_role "$moderator_id" moderator
request 200 POST /v1/auth/login \
    "$(jq --null-input --compact-output --arg email "$moderator_email" --arg password "$password" \
        '{email: $email, password: $password}')"
moderator_token=$(jq --exit-status --raw-output '.access_token' "$response_file")
request 200 GET /v1/auth/identity "" "$moderator_token"
jq --exit-status '(.roles | index("moderator")) != null' "$response_file" >/dev/null

echo "Creating and verifying a creator account..."
creator_email="creator-${suffix}@example.test"
register_and_login "$creator_email" "$password"
creator_token=$registered_access_token
request 201 POST /v1/creators \
    '{"type":"artist","display_name":"System Smoke Artist"}' "$creator_token"
creator_id=$(jq --exit-status --raw-output '.id' "$response_file")
request 200 POST "/v1/creators/$creator_id/verification-submissions" "" "$creator_token"
jq --exit-status '.verification_status == "pending"' "$response_file" >/dev/null
request 200 POST "/v1/creators/$creator_id/verification-reviews" \
    '{"decision":"verified"}' "$moderator_token"
jq --exit-status '.verification_status == "verified"' "$response_file" >/dev/null

release_date=$(date --utc +%Y-%m-%d)
track_title="System Smoke Signal ${suffix}"
echo "Creating a release draft and worldwide rights..."
request 201 POST /v1/releases \
    "$(jq --null-input --compact-output --arg creator_id "$creator_id" \
        --arg title "$track_title" --arg release_date "$release_date" \
        '{creator_id: $creator_id, title: $title, release_type: "single",
          release_date: $release_date,
          tracks: [{title: $title, track_number: 1, disc_number: 1, explicit: false}]}')" \
    "$creator_token"
release_id=$(jq --exit-status --raw-output '.id' "$response_file")
release_track_id=$(jq --exit-status --raw-output '.tracks[0].id' "$response_file")

# Exercise the real cover ingest path; never bypass private MinIO by writing
# a fictitious cover_object_key into release metadata.
echo "Generating and uploading a private PNG cover..."
python3 - "$cover_file" <<'PYPNG'
import struct
import sys
import zlib


def chunk(kind, payload):
    return (struct.pack(">I", len(payload)) + kind + payload +
            struct.pack(">I", zlib.crc32(kind + payload) & 0xffffffff))


width = height = 16
raw = b"".join(b"\x00" + b"\x22\x66\xaa" * width for _ in range(height))
image = (b"\x89PNG\r\n\x1a\n" +
         chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)) +
         chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b""))
with open(sys.argv[1], "wb") as output:
    output.write(image)
PYPNG
cover_size=$(wc -c < "$cover_file" | tr -d ' ')
cover_key=$(cat /proc/sys/kernel/random/uuid)
request 201 POST "/v1/releases/$release_id/cover-uploads" \
    "$(jq -nc --arg key "$cover_key" --argjson size "$cover_size" \
      '{idempotency_key:$key,original_name:"system-smoke.png",mime_type:"image/png",size_bytes:$size}')" \
    "$creator_token"
cover_upload_url=$(jq -er '.upload_url' "$response_file")
cover_status=$(curl --silent --show-error --connect-timeout 5 --max-time 60 \
    --output "$response_file" --write-out '%{http_code}' --request PUT \
    --header "Authorization: Bearer $creator_token" \
    --header 'Content-Type: image/png' --data-binary "@$cover_file" \
    "$gateway_url$cover_upload_url")
if [[ "$cover_status" != 201 ]]; then
    echo "Cover PUT returned HTTP $cover_status, expected 201" >&2
    cat "$response_file" >&2
    exit 1
fi
jq -e --arg id "$release_id" '.release_id == $id and .status == "completed"' \
    "$response_file" >/dev/null
request 200 GET "/v1/releases/$release_id" "" "$creator_token"
jq -e '.cover_object_key | type == "string" and length > 0' "$response_file" >/dev/null

for rights_type in streaming distribution; do
    request 200 PUT "/v1/release-tracks/$release_track_id/rights" \
        "$(jq --null-input --compact-output --arg rights_type "$rights_type" \
            --arg valid_from "$release_date" \
            '{territory_code: "001", rights_type: $rights_type, valid_from: $valid_from}')" \
        "$creator_token"
done

echo "Generating and uploading a deterministic WAV fixture..."
python3 - "$audio_file" <<'PY'
import math
import struct
import sys
import wave

sample_rate = 44_100
with wave.open(sys.argv[1], "wb") as audio:
    audio.setnchannels(1)
    audio.setsampwidth(2)
    audio.setframerate(sample_rate)
    frames = bytearray()
    for index in range(sample_rate):
        value = int(10_000 * math.sin(2 * math.pi * 440 * index / sample_rate))
        frames.extend(struct.pack("<h", value))
    audio.writeframes(frames)
PY
audio_size=$(wc -c < "$audio_file" | tr -d ' ')
idempotency_key=$(cat /proc/sys/kernel/random/uuid)
request 201 POST "/v1/release-tracks/$release_track_id/uploads" \
    "$(jq --null-input --compact-output --arg idempotency_key "$idempotency_key" \
        --argjson size_bytes "$audio_size" \
        '{idempotency_key: $idempotency_key, original_name: "system-smoke.wav",
          mime_type: "audio/wav", size_bytes: $size_bytes}')" "$creator_token"
upload_url=$(jq --exit-status --raw-output '.upload_url' "$response_file")

upload_status=$(curl --silent --show-error --connect-timeout 5 --max-time 60 \
    --output "$response_file" --write-out '%{http_code}' --request PUT \
    --header "Authorization: Bearer $creator_token" \
    --header 'Content-Type: audio/wav' --data-binary "@$audio_file" \
    "$gateway_url$upload_url")
if [[ "$upload_status" != "201" ]]; then
    echo "PUT $upload_url returned HTTP $upload_status, expected 201" >&2
    cat "$response_file" >&2
    exit 1
fi
media_asset_id=$(jq --exit-status --raw-output '.media_asset_id' "$response_file")

echo "Waiting for media processing and submitting the release..."
submitted=false
for _ in {1..90}; do
    status=$(curl --silent --show-error --connect-timeout 5 --max-time 30 \
        --output "$response_file" --write-out '%{http_code}' --request POST \
        --header "Authorization: Bearer $creator_token" \
        "$gateway_url/v1/releases/$release_id/submissions")
    if [[ "$status" == "200" ]]; then
        submitted=true
        break
    fi
    if [[ "$status" != "409" ]] || \
        ! jq --exit-status '.error.code == "release_not_ready"' "$response_file" >/dev/null; then
        echo "Release submission returned unexpected HTTP $status" >&2
        cat "$response_file" >&2
        exit 1
    fi
    sleep 2
done
if [[ "$submitted" != "true" ]]; then
    echo "Media processing did not become ready before the timeout" >&2
    exit 1
fi
jq --exit-status '.status == "pending_review"' "$response_file" >/dev/null

echo "Approving and publishing the release..."
request 200 POST "/v1/releases/$release_id/moderation-reviews" \
    '{"decision":"approved","comment":"System smoke approval"}' "$moderator_token"
jq --exit-status '.release_status == "approved"' "$response_file" >/dev/null

published=false
for _ in {1..60}; do
    status=$(curl --silent --show-error --connect-timeout 5 --max-time 30 \
        --output "$response_file" --write-out '%{http_code}' --request POST \
        --header "Authorization: Bearer $creator_token" \
        "$gateway_url/v1/releases/$release_id/publications")
    if [[ "$status" == "200" ]]; then
        published=true
        break
    fi
    if [[ "$status" != "409" ]] || \
        ! jq --exit-status '.error.code == "release_not_ready"' "$response_file" >/dev/null; then
        echo "Release publication returned unexpected HTTP $status" >&2
        cat "$response_file" >&2
        exit 1
    fi
    sleep 1
done
if [[ "$published" != "true" ]]; then
    echo "Catalog did not observe release approval before the timeout" >&2
    exit 1
fi
jq --exit-status '.status == "published"' "$response_file" >/dev/null

request 200 GET "/v1/releases/$release_id" "" "$creator_token"
catalog_track_id=$(jq --exit-status --raw-output '.tracks[0].catalog_track_id' "$response_file")
request 200 GET "/v1/catalog/tracks/$catalog_track_id"
jq --exit-status --arg title "$track_title" --arg asset_id "$media_asset_id" \
    '.title == $title and .playback_url == ("/v1/media/assets/" + $asset_id + "/playback")' \
    "$response_file" >/dev/null
playback_url=$(jq --exit-status --raw-output '.playback_url' "$response_file")

echo "Streaming a byte range from the published media asset..."
playback_status=$(curl --silent --show-error --connect-timeout 5 --max-time 30 \
    --dump-header "$headers_file" --output "$response_file" --write-out '%{http_code}' \
    --header "Authorization: Bearer $creator_token" --header 'Range: bytes=0-1023' \
    "$gateway_url$playback_url?quality=high")
if [[ "$playback_status" != "206" ]] || [[ ! -s "$response_file" ]]; then
    echo "Playback returned HTTP $playback_status or an empty body" >&2
    cat "$headers_file" >&2
    exit 1
fi
grep -qi '^Accept-Ranges: bytes' "$headers_file"
grep -qi '^Content-Range: bytes 0-1023/' "$headers_file"
grep -qi '^X-Audio-Quality: high' "$headers_file"

if [[ "${VERIFY_CREATOR_LISTENER_JOURNEY:-0}" == "1" ]]; then
    # This file is sourced deliberately: it reuses the real release/track IDs
    # and the request/login helpers above, instead of creating a second fixture.
    # shellcheck source=scripts/creator-listener-assertions.sh
    source scripts/creator-listener-assertions.sh
fi

echo "Content publication and media playback smoke test passed"
