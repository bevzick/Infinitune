# Content publishing and media playback

This guide describes the complete API workflow for publishing creator audio.
All HTTP requests go through Kong. The examples assume:

```bash
export GATEWAY_URL=http://localhost:8000
export CREATOR_TOKEN='<creator access token>'
export MODERATOR_TOKEN='<moderator or admin access token>'
```

Access tokens contain a snapshot of the user's global roles. After an admin
assigns the `moderator` role through
`PUT /v1/auth/users/{user_id}/roles/moderator`, the moderator must log in again
before the new role appears in the token. Creator account permissions are
derived from creator membership rather than the global `creator` role.

## Workflow

The publishing state transition is:

```text
unverified creator -> pending verification -> verified creator
draft release -> pending review -> approved -> published
pending upload -> uploading -> completed
media uploaded -> processing -> ready
```

The release can be submitted only after its creator is verified, its cover key
is set, every track has completed media processing, and every track has current
worldwide streaming and distribution rights.

### 1. Create and verify a creator account

Create an artist, label, or distributor account:

```bash
creator_response=$(curl --fail-with-body --silent \
  --request POST "$GATEWAY_URL/v1/creators" \
  --header "Authorization: Bearer $CREATOR_TOKEN" \
  --header 'Content-Type: application/json' \
  --data '{"type":"artist","display_name":"Example Artist"}')
creator_id=$(jq --raw-output '.id' <<<"$creator_response")
```

Submit it for verification:

```bash
curl --fail-with-body --silent \
  --request POST \
  "$GATEWAY_URL/v1/creators/$creator_id/verification-submissions" \
  --header "Authorization: Bearer $CREATOR_TOKEN"
```

A moderator who is not a member of this creator account reviews it:

```bash
curl --fail-with-body --silent \
  --request POST \
  "$GATEWAY_URL/v1/creators/$creator_id/verification-reviews" \
  --header "Authorization: Bearer $MODERATOR_TOKEN" \
  --header 'Content-Type: application/json' \
  --data '{"decision":"verified"}'
```

The alternative review decision is `rejected`.

### 2. Create a release draft

Create a release and its complete initial track set. A `single` must contain
exactly one track at disc 1, position 1.

```bash
release_date=$(date --utc +%Y-%m-%d)
release_response=$(curl --fail-with-body --silent \
  --request POST "$GATEWAY_URL/v1/releases" \
  --header "Authorization: Bearer $CREATOR_TOKEN" \
  --header 'Content-Type: application/json' \
  --data "{
    \"creator_id\":\"$creator_id\",
    \"title\":\"Example Single\",
    \"release_type\":\"single\",
    \"release_date\":\"$release_date\",
    \"tracks\":[{
      \"title\":\"Example Single\",
      \"track_number\":1,
      \"disc_number\":1,
      \"explicit\":false
    }]
  }")
release_id=$(jq --raw-output '.id' <<<"$release_response")
release_track_id=$(jq --raw-output '.tracks[0].id' <<<"$release_response")
```

Set the cover object key while preserving every existing track in the update:

```bash
curl --fail-with-body --silent \
  --request PUT "$GATEWAY_URL/v1/releases/$release_id" \
  --header "Authorization: Bearer $CREATOR_TOKEN" \
  --header 'Content-Type: application/json' \
  --data "{
    \"title\":\"Example Single\",
    \"release_type\":\"single\",
    \"release_date\":\"$release_date\",
    \"cover_object_key\":\"creator-content/covers/example.jpg\",
    \"tracks\":[{
      \"id\":\"$release_track_id\",
      \"title\":\"Example Single\",
      \"track_number\":1,
      \"disc_number\":1,
      \"explicit\":false
    }]
  }"
```

The current backend stores the cover key as release metadata. A public cover
binary upload endpoint is not yet exposed, so production clients must obtain
the private object key from a separately authorized cover-ingest flow.

### 3. Declare track rights

Territory `001` represents worldwide rights. Both rights must cover the release
date. Omit `valid_until` for an open-ended grant.

```bash
for rights_type in streaming distribution; do
  curl --fail-with-body --silent \
    --request PUT \
    "$GATEWAY_URL/v1/release-tracks/$release_track_id/rights" \
    --header "Authorization: Bearer $CREATOR_TOKEN" \
    --header 'Content-Type: application/json' \
    --data "{
      \"territory_code\":\"001\",
      \"rights_type\":\"$rights_type\",
      \"valid_from\":\"$release_date\"
    }"
done
```

### 4. Create an upload session and send the audio

The declared file name, MIME type, and byte size must exactly match the binary
upload. Supported formats are WAV, FLAC, MP3, and M4A, up to 500 MiB.

```bash
audio_file=example.wav
audio_size=$(wc -c <"$audio_file" | tr -d ' ')
idempotency_key=$(cat /proc/sys/kernel/random/uuid)

upload_response=$(curl --fail-with-body --silent \
  --request POST \
  "$GATEWAY_URL/v1/release-tracks/$release_track_id/uploads" \
  --header "Authorization: Bearer $CREATOR_TOKEN" \
  --header 'Content-Type: application/json' \
  --data "{
    \"idempotency_key\":\"$idempotency_key\",
    \"original_name\":\"example.wav\",
    \"mime_type\":\"audio/wav\",
    \"size_bytes\":$audio_size
  }")
upload_url=$(jq --raw-output '.upload_url' <<<"$upload_response")

media_response=$(curl --fail-with-body --silent \
  --request PUT "$GATEWAY_URL$upload_url" \
  --header "Authorization: Bearer $CREATOR_TOKEN" \
  --header 'Content-Type: audio/wav' \
  --data-binary "@$audio_file")
media_asset_id=$(jq --raw-output '.media_asset_id' <<<"$media_response")
```

The upload returns before asynchronous FFmpeg processing finishes. Reusing an
idempotency key with different metadata returns
`409 upload_idempotency_conflict`.

### Readiness before submission

An authenticated creator-account member may request a read-only readiness snapshot:

```http
GET /v1/releases/{release_id}/readiness
```

HTTP `200` returns `release_id`, `status`, `ready_for_submission` and an `issues`
array. Each issue has a stable `code` and human-readable `message`; track
issues also contain `track_id`. For example:

```json
{
  "release_id": "00000000-0000-4000-8000-000000000101",
  "status": "draft",
  "ready_for_submission": false,
  "issues": [
    {"code": "cover_missing", "message": "cover artwork is required"},
    {"code": "streaming_rights_missing", "message": "worldwide streaming rights are required", "track_id": "00000000-0000-4000-8000-000000000102"}
  ]
}
```

The check evaluates creator verification, member publication permission,
release lifecycle status and metadata, cover presence, track list, worldwide
streaming and distribution rights at the release date, and associated ready
media assets. Issues may include `creator_unverified`,
`publish_permission_required`, `release_status_ineligible`,
`metadata_incomplete`, `cover_missing`, `tracks_missing`,
`streaming_rights_missing`, `distribution_rights_missing`,
`media_missing` and `media_not_ready`.

Only members with `creator.account.read` permission can inspect readiness;
other authenticated users receive `403 forbidden`. HTTP `404` means that the
release is missing. A temporary Media Service failure returns HTTP `503` with `media_unavailable`
instead of claiming that the release is ready or that an
asset is missing. Inconsistent Media association is a server error.

This response is a **snapshot**, not permission to publish: POST submission
rechecks rights, media and lifecycle state, and may return `409` after a
concurrent update. The caller must still have `creator.content.publish` and
a verified creator account.

### 5. Submit and moderate the release

Submit only after the media asset becomes ready. Until then, the endpoint
returns `409 release_not_ready`; clients should retry with bounded backoff.

```bash
curl --fail-with-body --silent \
  --request POST "$GATEWAY_URL/v1/releases/$release_id/submissions" \
  --header "Authorization: Bearer $CREATOR_TOKEN"
```

Approve the pending submission with a different moderator account:

```bash
curl --fail-with-body --silent \
  --request POST \
  "$GATEWAY_URL/v1/releases/$release_id/moderation-reviews" \
  --header "Authorization: Bearer $MODERATOR_TOKEN" \
  --header 'Content-Type: application/json' \
  --data '{"decision":"approved","comment":"Approved for publication"}'
```

Other release decisions are `rejected` and `changes_requested`; both require a
non-empty comment.

### 6. Publish, browse, and play

Approval is delivered to Catalog through Kafka. Publication can briefly return
`409 release_not_ready` until Catalog observes that event. The publication
operation is idempotent and safe to retry with bounded backoff.

```bash
curl --fail-with-body --silent \
  --request POST "$GATEWAY_URL/v1/releases/$release_id/publications" \
  --header "Authorization: Bearer $CREATOR_TOKEN"

published_release=$(curl --fail-with-body --silent \
  "$GATEWAY_URL/v1/releases/$release_id" \
  --header "Authorization: Bearer $CREATOR_TOKEN")
catalog_track_id=$(jq --raw-output '.tracks[0].catalog_track_id' \
  <<<"$published_release")

catalog_track=$(curl --fail-with-body --silent \
  "$GATEWAY_URL/v1/catalog/tracks/$catalog_track_id")
playback_url=$(jq --raw-output '.playback_url' <<<"$catalog_track")

curl --fail-with-body \
  "$GATEWAY_URL$playback_url?quality=high" \
  --header "Authorization: Bearer $CREATOR_TOKEN" \
  --header 'Range: bytes=0-1023' \
  --output playback-part.bin
```

Playback supports `low`, `medium`, `high`, and `lossless` quality selection.
Successful range requests return `206 Partial Content`, `Content-Range`,
`Accept-Ranges: bytes`, and audio quality and codec headers.

## Automated verification

The repository contains an isolated end-to-end version of this workflow:

```bash
COMPOSE_PROJECT_NAME=infinitune-content-media-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make content-media-smoke

COMPOSE_PROJECT_NAME=infinitune-content-media-smoke \
COMPOSE_OVERRIDE='-f compose.system-test.yml' \
make down-volumes
```

The test generates a safe WAV fixture, processes it through Media, publishes it
through Content and Catalog, and verifies authenticated byte-range playback.


### Cover upload sessions (creator)

Authenticated creator members with `content_upload` permission can request a private
cover upload session for an editable release (draft, rejected or changes requested):

```http
POST /v1/releases/{release_id}/cover-uploads
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "idempotency_key": "9c4115f0-4145-4d3e-b184-94d60fa32163",
  "original_name": "front-cover.webp",
  "mime_type": "image/webp",
  "size_bytes": 102400
}
```

The API responds with HTTP `201`, `Location: /v1/media/cover-uploads/{id}`,
`id`, `release_id`, `upload_url`, `original_name`, `mime_type`, `size_bytes`,
`status` (`pending`) and timestamps. Repeating the same idempotency key and
metadata returns the same session; conflicting metadata returns HTTP `409`.
Supported formats: JPEG (`.jpg`/`.jpeg`), PNG (`.png`), WebP (`.webp`) up to **10 MiB**.
The object key is generated by Content and is never accepted from the browser.

### Binary cover ingest

The creator sends the raw image to `PUT /v1/media/cover-uploads/{id}` with
`Authorization: Bearer ...`, exact `Content-Type` and `Content-Length` from the
session. Media verifies the actor with Content over the private gRPC contract,
rejects unsupported or malformed images and disallows images exceeding 10 MiB,
8192 pixels per edge or 16 million pixels. JPEG/PNG are decoded and WebP is
verified with the bundled ffprobe/ffmpeg. Objects are stored privately with an
atomic `If-None-Match: *` write; the client never selects an object key.

On success Media asks Content to atomically mark the session `completed` and
assign `releases.cover_object_key` in Content's own database (only while the
release is editable). An already completed request returns `200` without
writing the object again. A new session and idempotency key are required after
an upload enters `failed`; no retry is permitted to overwrite the prior key.
Uploads with a pending session do not mark the release ready. If the private
completion RPC times out after object storage, Media retains the object to
avoid deleting a potentially committed cover; reconciliation may be required.

The private CoverUploadService contract is documented in both services' copies
of `api/cover/v1/cover.proto`, and exchanges `google.protobuf.Struct` with
explicit field names. No generated protobuf files are edited by hand.

### Creator Studio lifecycle controls

The Creator Studio release list includes the user's releases across lifecycle
statuses, not only drafts. **Manage lifecycle** displays the current status,
submission/readiness blockers from the Content API, and submitted/reviewed/
published/archived timestamps when present. The client never reimplements the
server's rights, media, creator verification, or metadata readiness rules.

The interface offers **Submit for review** for `draft`, `rejected`, and
`changes_requested` releases only after a successful readiness check. During
`pending_review`, the creator may withdraw. Approved releases may be withdrawn
or published; published releases may be archived after explicit confirmation.
Moderation remains an independent moderator-only workflow, not an action in
Creator Studio. All permissions and lifecycle transitions are enforced by the
Content Service regardless of visible buttons.

Publication retries a transient `409 release_not_ready` at most twice after the
initial attempt, allowing a short interval for Catalog event propagation. Other
errors are shown immediately. The UI reloads canonical release details after
each successful action and does not assume that Catalog visibility is immediate.
