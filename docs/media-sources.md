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

## Provider boundary

A provider adapter has two responsibilities:

1. resolve a provider track identifier into normalized title, artist, public
   page, license, attribution, and metadata;
2. open a bounded audio stream for the same identifier.

The registry rejects unregistered providers, malformed provider output,
unsupported media types, and streams outside the 500 MiB ingest limit.
Provider-specific credentials, rate limits, retries, and HTTP behavior stay
inside each adapter.

The next implementation stages add the Jamendo and Internet Archive HTTP
adapters, followed by an authorized import job that copies a validated stream
into private object storage and records the provenance transactionally.
