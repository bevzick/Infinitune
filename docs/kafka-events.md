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
```

## Delivery contract

- Implemented events are `history.listening.recorded.v1` and
  `history.listening.cleared.v1`.
- Recording/deleting history and creating the corresponding outbox row happen atomically in
  History PostgreSQL.
- Delivery is **at least once**. A crash after Kafka accepts an event but before the outbox row
  is marked published can produce a duplicate.
- `id` is the stable idempotency key. Every consumer must deduplicate it or make handling idempotent.
- Kafka records are partitioned by `user_id`, so one user's events reach the same partition.
  Consumers must still tolerate duplicates and late events because retries can change arrival order.
- Event schemas are versioned in the event type. Breaking changes require a new version/topic.
- Kafka being unavailable does not reject a valid listening event; the outbox dispatcher retries.
- A clear event is emitted only when at least one row was deleted. `before` is absent for a full
  clear and contains the exclusive deletion cutoff for a partial clear.
- Consumers that persist listening signals must apply clear events as deletion watermarks and
  ignore a late recorded event when it falls inside an already cleared range. Recommendation does
  not copy raw history and only invalidates cached runs; its next calculation reads current History.

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
`kafka-init` service creates both History topics idempotently before History starts.
