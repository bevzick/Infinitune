CREATE TABLE interaction.event_outbox (
    id uuid PRIMARY KEY,
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL,
    topic text NOT NULL,
    partition_key text NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL,
    available_at timestamptz NOT NULL DEFAULT now(),
    claimed_until timestamptz,
    published_at timestamptz,
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    CONSTRAINT event_outbox_event_type_not_blank CHECK (btrim(event_type) <> ''),
    CONSTRAINT event_outbox_topic_not_blank CHECK (btrim(topic) <> ''),
    CONSTRAINT event_outbox_partition_key_not_blank CHECK (btrim(partition_key) <> ''),
    CONSTRAINT event_outbox_attempts_non_negative CHECK (attempts >= 0)
);

CREATE INDEX event_outbox_pending_idx
    ON interaction.event_outbox (available_at, occurred_at, id)
    WHERE published_at IS NULL;

COMMENT ON TABLE interaction.event_outbox IS
    'Transactional outbox for at-least-once delivery of Interaction domain events.';
