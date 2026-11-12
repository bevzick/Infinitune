# Kafka event flow

Kafka is used for asynchronous domain notifications. Synchronous operations that need an
immediate result continue to use REST or gRPC.

```text
Browser/player
      |
      | POST listening event
      v
History service ---- gRPC validation ----> Catalog service
      |
      | one PostgreSQL transaction
      +--> history.listening_events
      +--> history.event_outbox
                  |
                  | retrying outbox dispatcher
                  v
        history.listening.recorded.v1  ┐
                                       ├─ Kafka, key = user_id
        history.listening.cleared.v1   ┘
                  |
         +--------+------------------+
         |                           |
         v                           v
Recommendation consumer       Future analytics consumer
(invalidates stale runs)       (aggregates, trends)

Content approval ---- gRPC publish ----> Catalog service
                                         |
                                         | catalog.event_outbox
                                         v
                       catalog.release.published.v1 ┐
                                                    ├─ Kafka, key = source_release_id
                       catalog.release.archived.v1  ┘
                                         |
                         +---------------+----------------+
                         v               v                v
                    Recommendation   Playlist       Future search index

Like/unlike/follow -----> Interaction service
                                  |
                                  | same PostgreSQL transaction
                                  +--> preference table
                                  +--> interaction.event_outbox
                                                |
                                                v
                              interaction.preference.changed.v1
                              Kafka, key = user_id
                                                |
                                                v
                                   Recommendation consumer
                                   (invalidates that user's stale runs)

Audio processing -----> Media service
                              |
                              | same PostgreSQL transaction
                              +--> media asset status = ready
                              +--> media.event_outbox
                                           |
                                           v
                              media.asset.ready.v1
                              Kafka, key = media_asset_id
                                           |
                                           v
                              Content consumer
                              (updates local readiness projection)

Moderator approval -----> Content service
                                |
                                | same PostgreSQL transaction
                                +--> release status = approved
                                +--> content.event_outbox
                                             |
                                             v
                              content.release.approved.v1
                              Kafka, key = release_id
                                             |
                                             v
                              Future workflow consumers
```

## Delivery contract

- Implemented events are `history.listening.recorded.v1` and
  `history.listening.cleared.v1`, `catalog.release.published.v1`,
  `catalog.release.archived.v1`, `interaction.preference.changed.v1`, and
  `media.asset.ready.v1`, and `content.release.approved.v1`.
- Recording/deleting history and creating the corresponding outbox row happen atomically in
  History PostgreSQL.
- Delivery is **at least once**. A crash after Kafka accepts an event but before the outbox row
  is marked published can produce a duplicate.
- `id` is the stable idempotency key. Every consumer must deduplicate it or make handling idempotent.
- History and Interaction records are partitioned by `user_id`; Media readiness is partitioned by
  `media_asset_id`, so changes for the same aggregate reach the same partition.
  Consumers must still tolerate duplicates and late events because retries can change arrival order.
- Event schemas are versioned in the event type. Breaking changes require a new version/topic.
- Kafka being unavailable does not reject a valid listening event; the outbox dispatcher retries.
- A clear event is emitted only when at least one row was deleted. `before` is absent for a full
  clear and contains the exclusive deletion cutoff for a partial clear.
- Consumers that persist listening signals must apply clear events as deletion watermarks and
  ignore a late recorded event when it falls inside an already cleared range. Recommendation does
  not copy raw history and only invalidates cached runs; its next calculation reads current History.
- Catalog lifecycle payloads contain canonical artist, optional album, and track identifiers.
  Publication/archive changes and their outbox rows share one Catalog transaction. Repeated gRPC
  commands reuse the same `aggregate_id + event_type` outbox identity and do not emit duplicates.
- Catalog topics are a change feed, not a complete snapshot. A new search/index consumer must first
  bootstrap current published metadata through a dedicated Catalog contract and then apply events;
  `occurred_at` resolves late publish/archive delivery for the same release.
- Recommendation consumes Catalog lifecycle events in a separate consumer group. Publication
  expires every run generated before the release became visible; archive expires only older runs
  containing one of the archived track IDs. Runs generated after the event already read the new
  Catalog state and remain valid.
- Track and album likes and artist follows emit a preference event only when the stored state
  actually changes. Comment likes are excluded. The preference mutation and its outbox row are
  atomic, and Recommendation expires only that user's runs older than `occurred_at`.
- Media publishes readiness only after variant metadata and the asset's `ready` status have been
  committed with the outbox row. The payload identifies the source upload, Content/Catalog track,
  duration, and generated audio profiles without exposing storage object keys.
- Content consumes retained readiness events from the beginning for a new consumer group and keeps
  an idempotent projection ordered by `occurred_at`. Submission and publication read the local
  projection first; Media gRPC remains a compatibility fallback for assets created before the
  event flow was deployed or older than Kafka retention.
- Content emits release approval only for a newly recorded `approved` moderation decision. The
  review, release status transition, and outbox message are atomic; idempotent review retries reuse
  the existing review and do not enqueue duplicate approval events.

## Suitable next integrations

| Producer | Event | Consumers | Purpose |
|---|---|---|---|
| Approval workflow | `content.release.approved.v1` | Catalog | Trigger controlled catalog publication |

Commands and permission checks remain synchronous. Kafka should not replace Catalog validation,
authentication, playback authorization, or user-facing writes that require an immediate response.

The Compose broker is a single-node development setup with plaintext traffic. Production requires
multiple brokers, replication, TLS/SASL, monitoring, quotas, and an explicit retention/privacy policy.

Start the full local stack with `make up`. The Makefile includes `compose.kafka.yml`; the
`kafka-init` service creates the History, Catalog, Interaction, and Media topics idempotently
before producers start.
