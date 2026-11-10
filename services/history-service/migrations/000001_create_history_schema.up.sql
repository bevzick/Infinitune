CREATE SCHEMA history;

CREATE TABLE history.listening_events (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    track_id uuid NOT NULL,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    listened_ms integer NOT NULL,
    completed boolean NOT NULL,
    context_type varchar(30) NOT NULL,
    context_id uuid,
    device_type varchar(30) NOT NULL,
    CONSTRAINT listening_events_listened_ms_non_negative CHECK (listened_ms >= 0),
    CONSTRAINT listening_events_time_order CHECK (finished_at IS NULL OR finished_at >= started_at),
    CONSTRAINT listening_events_completed_finished CHECK (NOT completed OR finished_at IS NOT NULL),
    CONSTRAINT listening_events_context_type_valid CHECK (
        context_type IN ('direct', 'album', 'playlist', 'artist', 'recommendation', 'search')
    ),
    CONSTRAINT listening_events_context_id_valid CHECK (
        (context_type IN ('album', 'playlist', 'artist', 'recommendation') AND context_id IS NOT NULL)
        OR (context_type IN ('direct', 'search') AND context_id IS NULL)
    ),
    CONSTRAINT listening_events_device_type_valid CHECK (
        device_type IN ('web', 'mobile', 'desktop', 'tv', 'other')
    )
);

COMMENT ON SCHEMA history IS
    'Private listening history; referenced Auth and Catalog UUIDs have no cross-database foreign keys.';
COMMENT ON COLUMN history.listening_events.id IS
    'Stable playback event identifier used as the idempotency key.';
COMMENT ON TABLE history.listening_events IS
    'Qualified listening events are retained for 24 months; cleanup is performed by a maintenance job.';

CREATE INDEX listening_events_user_started_at_idx
    ON history.listening_events (user_id, started_at DESC, id);
CREATE INDEX listening_events_track_started_at_idx
    ON history.listening_events (track_id, started_at DESC, id);
CREATE INDEX listening_events_retention_idx
    ON history.listening_events (started_at);
