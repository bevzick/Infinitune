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
        history.listening.recorded.v1 (Kafka, key = user_id)
                  |
         +--------+------------------+
         |                           |
         v                           v
Recommendation consumer       Future analytics consumer
(invalidates stale runs)       (aggregates, trends)
```

## Delivery contract

- The first implemented event is `history.listening.recorded.v1`.
- The event and its outbox row are committed atomically in History PostgreSQL.
- Delivery is **at least once**. A crash after Kafka accepts an event but before the outbox row
  is marked published can produce a duplicate.
- `id` is the stable idempotency key. Every consumer must deduplicate it or make handling idempotent.
- Kafka records are partitioned by `user_id`, so one user's events reach the same partition.
  Consumers must still tolerate duplicates and late events because retries can change arrival order.
- Event schemas are versioned in the event type. Breaking changes require a new version/topic.
- Kafka being unavailable does not reject a valid listening event; the outbox dispatcher retries.

## Suitable next integrations

| Producer | Event | Consumers | Purpose |
|---|---|---|---|
| Catalog | `catalog.release.published.v1` | Search, Recommendation | Update indexes and candidate sets |
| Catalog | `catalog.release.archived.v1` | Search, Playlist, Recommendation | Remove unavailable tracks |
| Media | `media.asset.ready.v1` | Content, Catalog | Complete publishing without polling |
| Content | `content.release.approved.v1` | Catalog, Media | Start controlled publication workflow |
| Interaction | `interaction.preference.changed.v1` | Recommendation | Invalidate personalized recommendation runs |

Commands and permission checks remain synchronous. Kafka should not replace Catalog validation,
authentication, playback authorization, or user-facing writes that require an immediate response.

The Compose broker is a single-node development setup with plaintext traffic. Production requires
multiple brokers, replication, TLS/SASL, monitoring, quotas, and an explicit retention/privacy policy.

Start the full local stack with `make up`. The Makefile includes `compose.kafka.yml`; the
`kafka-init` service creates the first topic idempotently before History starts.
