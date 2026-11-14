#!/usr/bin/env bash
# Sourced from content-media-smoke.sh after a real release is published.
# Requires request(), register_and_login(), response_file, gateway_url,
# creator_token, moderator_token, catalog_track_id, and suffix.

set -euo pipefail

echo "Registering an independent listener and checking Catalog visibility..."
listener_email="listener-${suffix}@example.test"
register_and_login "$listener_email" "$password"
listener_id=$registered_user_id
listener_token=$registered_access_token
request 200 GET "/v1/auth/identity" "" "$listener_token"
jq -e --arg id "$listener_id" '.user_id == $id and (.roles | index("listener") != null)' \
    "$response_file" >/dev/null
request 201 POST /v1/users/me/profile \
    "$(jq -nc --arg username "listener${suffix}" \
      '{username:$username,display_name:"System Journey Listener"}')" "$listener_token"
request 200 GET "/v1/catalog/tracks/$catalog_track_id"
jq -e --arg title "$track_title" '.title == $title' "$response_file" >/dev/null

# Verify the published asset is streamable by a listener, not merely by its creator.
listener_playback=$(curl --silent --show-error --connect-timeout 5 --max-time 30 \
    --output "$response_file" --write-out '%{http_code}' \
    --header "Authorization: Bearer $listener_token" \
    --header 'Range: bytes=0-511' "$gateway_url$playback_url?quality=high")
if [[ "$listener_playback" != 206 ]] || [[ ! -s "$response_file" ]]; then
    echo "Listener playback failed: HTTP $listener_playback" >&2
    exit 1
fi

echo "Checking playlist, likes and comments on the newly published track..."
request 201 POST /v1/playlists \
    '{"name":"Creator-to-listener smoke playlist","visibility":"private"}' "$listener_token"
playlist_id=$(jq -er '.id' "$response_file")
request 201 POST "/v1/playlists/$playlist_id/tracks" \
    "$(jq -nc --arg id "$catalog_track_id" '{track_id:$id}')" "$listener_token"
request 200 GET "/v1/playlists/$playlist_id/tracks" "" "$listener_token"
jq -e --arg id "$catalog_track_id" '.items | any(.track_id == $id)' "$response_file" >/dev/null
request 200 PUT "/v1/interactions/tracks/$catalog_track_id/like" "" "$listener_token"
request 200 GET /v1/interactions/tracks/likes "" "$listener_token"
jq -e --arg id "$catalog_track_id" '.items | any(.id == $id)' "$response_file" >/dev/null
request 201 POST /v1/comments \
    "$(jq -nc --arg id "$catalog_track_id" '{track_id:$id,content:"Creator-to-listener smoke comment"}')" \
    "$listener_token"
comment_id=$(jq -er '.id' "$response_file")
request 200 GET "/v1/tracks/$catalog_track_id/comments"
jq -e --arg id "$comment_id" '.items | any(.id == $id)' "$response_file" >/dev/null

# Two independent identities: creator reports the listener's comment and a
# moderator hides it. This is intentionally part of the same real journey.
request 201 POST "/v1/comments/$comment_id/reports" \
    '{"reason":"spam","description":"Smoke verification report"}' "$creator_token"
report_id=$(jq -er '.id' "$response_file")
request 200 GET "/v1/moderation/comment-reports?status=pending" "" "$moderator_token"
jq -e --arg id "$report_id" '.items | any(.report.id == $id)' "$response_file" >/dev/null
request 200 POST "/v1/moderation/comment-reports/$report_id/actions" \
    '{"action":"hide_comment","note":"System journey verification"}' "$moderator_token"
jq -e '.resulting_comment_status == "hidden"' "$response_file" >/dev/null
request 200 GET "/v1/tracks/$catalog_track_id/comments"
jq -e --arg id "$comment_id" '.items | all(.id != $id)' "$response_file" >/dev/null
request 200 POST "/v1/moderation/comment-reports/$report_id/actions" \
    '{"action":"resolve_report","note":"Smoke verification completed"}' "$moderator_token"
jq -e '.resulting_report_status == "resolved"' "$response_file" >/dev/null

echo "Verifying history idempotency and privacy across two identities..."
event_id=$(cat /proc/sys/kernel/random/uuid)
started_at=$(date --utc --date='6 seconds ago' +%Y-%m-%dT%H:%M:%SZ)
finished_at=$(date --utc +%Y-%m-%dT%H:%M:%SZ)
event_body=$(jq -nc --arg id "$event_id" --arg track "$catalog_track_id" \
    --arg start "$started_at" --arg finish "$finished_at" \
    '{event_id:$id,track_id:$track,started_at:$start,finished_at:$finish,
      listened_ms:900,context_type:"direct",device_type:"web"}')
request 200 POST /v1/history/events "$event_body" "$listener_token"
jq -e --arg id "$event_id" '.event_id == $id and .recorded == true' "$response_file" >/dev/null
request 200 POST /v1/history/events "$event_body" "$listener_token"
# The HTTP API reports recorded=true for qualifying idempotent replays;
# verify idempotency by the single persisted row below instead.
jq -e --arg id "$event_id" '.event_id == $id and .recorded == true' "$response_file" >/dev/null
request 200 GET "/v1/history/events?limit=20" "" "$listener_token"
jq -e --arg id "$event_id" '[.items[] | select(.event_id == $id)] | length == 1' \
    "$response_file" >/dev/null
request 200 GET "/v1/history/events?limit=20" "" "$creator_token"
jq -e --arg id "$event_id" '.items | all(.event_id != $id)' "$response_file" >/dev/null

echo "Waiting for personalized recommendations after publication and interaction..."
recommended=false
for _ in {1..45}; do
    status=$(curl --silent --show-error --connect-timeout 5 --max-time 30 \
        --output "$response_file" --write-out '%{http_code}' \
        --header "Authorization: Bearer $listener_token" \
        "$gateway_url/v1/recommendations?limit=20&refresh=true")
    if [[ "$status" == 200 ]]; then
        jq -e '.run_id and (.items | type == "array") and (.algorithm_version | type == "string")' \
            "$response_file" >/dev/null
        if jq -e --arg id "$catalog_track_id" '.items | any(.track_id == $id)' \
            "$response_file" >/dev/null; then
            recommended=true
            break
        fi
    elif [[ "$status" != 503 ]]; then
        echo "Unexpected recommendations HTTP $status" >&2
        cat "$response_file" >&2
        exit 1
    fi
    sleep 2
done
if [[ "$recommended" != true ]]; then
    echo "Published track did not appear in refreshed recommendations" >&2
    exit 1
fi

echo "Creator-to-listener end-to-end API journey passed"
