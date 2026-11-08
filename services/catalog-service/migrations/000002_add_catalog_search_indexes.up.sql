CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX tracks_title_trgm_idx
    ON catalog.tracks
    USING gin (title gin_trgm_ops);

CREATE INDEX tracks_title_order_idx
    ON catalog.tracks (lower(title), id);

CREATE INDEX tracks_release_date_title_idx
    ON catalog.tracks (release_date DESC, lower(title), id);

CREATE INDEX tracks_explicit_release_date_idx
    ON catalog.tracks (explicit, release_date DESC, id);
