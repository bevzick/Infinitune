# Music source architecture

Infinitune normalizes playable audio into private Media service storage before
processing and playback. The source describes where the original audio came
from; it does not change the playback path. FFmpeg still creates controlled
variants, and clients stream those variants through the Media API.

## Source kinds

| Kind | Meaning | Initial workflow |
| --- | --- | --- |
| `local_device` | A creator selected a file on their device. | Existing Content upload session and Media ingest endpoint. |
| `server_storage` | Audio was already available in an authorized server-side collection. | Internal import flow; no arbitrary client-provided object keys. |
| `external_provider` | Audio is fetched from a supported provider under an identified license. | Provider adapter resolves metadata and opens the source stream. |

Every ingested asset can have one immutable provenance record. External
provenance contains the provider identifier, stable public page, external
track ID, license, attribution, permission flags, and bounded metadata. Signed
or temporary download URLs must never be persisted.

## Initial external providers

- **Jamendo** exposes a catalog of independent music with explicit licensing
  metadata and stable track identifiers.
- **Internet Archive** hosts downloadable audio collections and exposes item
  metadata. An adapter must select only audio files whose item metadata
  provides an acceptable license.

Spotify, Apple Music, YouTube, and YouTube Music are not import providers.
Their public APIs do not grant Infinitune permission to download, copy, and
redistribute full audio. Linking accounts or controlling playback in those
platforms would be a separate integration.

## Configuration summary

| Provider | Enabled when | Required value | Default endpoints |
| --- | --- | --- | --- |
| Internet Archive | Always | No API key | `archive.org/metadata` and `archive.org/download` |
| Jamendo | `MEDIA_JAMENDO_CLIENT_ID` is non-empty | Jamendo application client ID | `api.jamendo.com/v3.0` |

Copy `.env.example` to `.env` before starting the stack. Internet Archive is
available with the checked-in defaults. To add Jamendo, create an application
at <https://developer.jamendo.com>, set its client ID, and recreate Media:

```dotenv
MEDIA_JAMENDO_CLIENT_ID=<client-id>
```

```bash
make up
```

The API endpoint override variables are intended for controlled development,
testing, or an approved compatible endpoint. Provider URLs must use HTTPS
without embedded credentials, query strings, or fragments. Provider request
timeouts must be positive durations no greater than one minute.

### Jamendo configuration

Jamendo import is disabled unless `MEDIA_JAMENDO_CLIENT_ID` is set. Register an
application in the Jamendo developer portal and configure:

- `MEDIA_JAMENDO_CLIENT_ID` with the public API client identifier;
- `MEDIA_JAMENDO_API_URL` with the HTTPS API base URL (the default is
  `https://api.jamendo.com/v3.0`);
- `MEDIA_JAMENDO_TIMEOUT` with a positive request timeout up to one minute.

The adapter resolves tracks through the Jamendo tracks endpoint, requests MP3
quality, and accepts only recognized Creative Commons license URLs. It opens a
source stream only when Jamendo returns `audiodownload_allowed=true` and a
download URL on an approved Jamendo storage host. Redirects, media type, and
declared size are validated before the stream reaches the ingest workflow.
Temporary download URLs and the client identifier are never stored in asset
provenance.

### Internet Archive configuration

Public Internet Archive reads do not require an API key. The adapter uses the
official item metadata response to verify the item, file, declared size, and
license. Optional endpoint configuration is available through:

- `MEDIA_INTERNET_ARCHIVE_METADATA_URL` (default
  `https://archive.org/metadata`);
- `MEDIA_INTERNET_ARCHIVE_DOWNLOAD_URL` (default
  `https://archive.org/download`);
- `MEDIA_INTERNET_ARCHIVE_TIMEOUT` (default `20s`, maximum `1m`).

The `external_id` must identify both an item and one exact file, for example
`concert-identifier/audio/track-01.flac`. Only original MP3, FLAC, WAV, and M4A
files from `audio` or `etree` items are accepted. The item must contain a
recognized Creative Commons license URL, CC0 declaration, or Public Domain
Mark. Arbitrary rights text is not treated as permission to copy or stream.

An authenticated creator imports a configured source with:

```http
POST /v1/media/imports
Content-Type: application/json

{
  "content_track_id": "<release-track-uuid>",
  "provider": "jamendo",
  "external_id": "<jamendo-track-id>"
}
```

Before contacting the provider, Media asks Content whether the user may upload
content for that draft release track. A successful import copies the original
audio into private object storage, then creates the media asset, immutable
provenance, and processing job in one PostgreSQL transaction. Retrying the same
source for the same release track returns the existing asset; selecting a
different source for that track returns a conflict.

When processing finishes, Content projects the ready external asset by
`content_track_id`. Submission and publication use the latest completed upload
for a local file when one exists; otherwise they use this external projection.
The same worldwide streaming and distribution rights checks apply to both
paths.

## Provider boundary

A provider adapter has two responsibilities:

1. resolve a provider track identifier into normalized title, artist, public
   page, license, attribution, and metadata;
2. open a bounded audio stream for the same identifier.

The registry rejects unregistered providers, malformed provider output,
unsupported media types, and streams outside the 500 MiB ingest limit.
Provider-specific credentials, rate limits, retries, and HTTP behavior stay
inside each adapter.

## Discovery API

Authenticated clients can inspect only the providers that are configured in
the running Media service:

```http
GET /v1/media/sources/providers
```

Search normalized, importable tracks with bounded pagination:

```http
GET /v1/media/sources/jamendo/tracks?query=night&limit=20&offset=0
GET /v1/media/sources/internet_archive/tracks?query=item-identifier&limit=20&offset=0
```

For Jamendo, `query` is a track-name search. Results without a supported
license or download permission are omitted. For Internet Archive, `query` is
an exact item identifier; the result contains eligible original audio files in
that item. This avoids ambiguous full-catalog matches and lets a creator select
the exact file used as `external_id`.

Resolve one exact provider identifier before import with:

```http
GET /v1/media/sources/internet_archive/track?external_id=item-identifier%2Faudio%2Ftrack.flac
```

Discovery responses contain the provider, external identifier, title, artist,
stable public page, normalized license permissions, attribution, and bounded
metadata. They never expose provider credentials or temporary download URLs.

## Creator frontend workflow

The React frontend exposes external import at `/creator/import` and as
**Creator studio → Import music** in authenticated navigation. A user must be
signed in and must already have permission to upload media for at least one
track in a draft release. The page deliberately does not accept an arbitrary
track UUID.

The browser performs this sequence:

1. load the configured provider list from Media;
2. load the user's draft releases from Content and resolve their track lists;
3. let the creator select one destination release track;
4. search Jamendo by track name or inspect an Internet Archive item by its
   exact identifier;
5. show the provider page, normalized license, and attribution before import;
6. submit the selected provider and `external_id` to Media.

The import response means that Media accepted or reused the source asset. It
does not mean the track is immediately playable. Media stores the original in
private object storage and processes controlled playback variants
asynchronously. Content receives the ready projection after processing.

The import operation is idempotent for the same destination track and source.
Repeating it returns the existing media asset. Attempting to replace that
track with a different external source returns a conflict instead of silently
changing provenance.

### Prerequisites for later publication

Importing audio does not bypass the publishing workflow. Before submitting a
release, the creator still needs:

- a verified creator account and access to the draft release;
- completed media processing for every track;
- current worldwide streaming and distribution rights;
- the remaining required release metadata described in the
  [content publishing guide](content-publishing.md).

Provider metadata is evidence associated with the source, not a replacement
for the creator's responsibility to verify that the selected license and
their use of the recording are compatible.

## Failure diagnosis

| Symptom | Likely cause | Action |
| --- | --- | --- |
| Jamendo is absent from the provider selector | `MEDIA_JAMENDO_CLIENT_ID` is empty, or Media was not recreated after it changed | Set the client ID and recreate `media-service` |
| Internet Archive returns no tracks | The query is not an exact item identifier, the item is not public audio, or it has no eligible original files | Open the item on Archive.org and use the identifier from its URL |
| A provider result is omitted | Download permission, supported license metadata, format, or declared size did not pass validation | Select a different provider track or original file |
| Discovery returns `503 discovery_unavailable` | Provider request timed out or the external service is unavailable | Retry later; do not repeatedly submit imports while discovery is failing |
| Import returns `403 forbidden` | The user cannot upload media for the selected draft track | Check creator membership and release ownership |
| Import returns `409 import_conflict` | The destination track already has different immutable provenance | Use the existing source or create a new draft track |
| Import succeeds but playback is unavailable | FFmpeg processing has not produced ready variants yet | Wait for media processing and inspect Media worker logs if it does not complete |

Media and provider failures are written to the centralized container log
pipeline. In the local stack, inspect them in Grafana/Loki or use the commands
from the [local operations runbook](local-operations.md). Credentials and
temporary provider download URLs are not included in persisted provenance.

## Verification

Media provider, registry, and HTTP tests run with the Media module:

```bash
cd services/media-service
go test ./...
```

Frontend unit tests cover API serialization and the import component. The
Playwright smoke suite covers authentication, draft-track selection, provider
discovery, and the final import request without contacting real providers:

```bash
cd frontend
npm test
npm run test:e2e
```

If Playwright browsers are not installed but Chromium is available locally:

```bash
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/chromium npm run test:e2e
```
