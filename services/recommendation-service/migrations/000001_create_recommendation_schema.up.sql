CREATE SCHEMA recommendation;

CREATE TABLE recommendation.recommendation_runs (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    algorithm_version varchar(50) NOT NULL,
    generated_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    CONSTRAINT recommendation_runs_algorithm_version_not_blank
        CHECK (btrim(algorithm_version) <> ''),
    CONSTRAINT recommendation_runs_expiration_order
        CHECK (expires_at > generated_at)
);

CREATE TABLE recommendation.recommendation_items (
    run_id uuid NOT NULL,
    track_id uuid NOT NULL,
    score double precision NOT NULL,
    rank integer NOT NULL,
    reason varchar(100) NOT NULL,
    CONSTRAINT recommendation_items_pkey PRIMARY KEY (run_id, track_id),
    CONSTRAINT recommendation_items_run_rank_key UNIQUE (run_id, rank),
    CONSTRAINT recommendation_items_score_finite
        CHECK (score > '-Infinity'::double precision AND score < 'Infinity'::double precision),
    CONSTRAINT recommendation_items_rank_positive CHECK (rank > 0),
    CONSTRAINT recommendation_items_reason_not_blank CHECK (btrim(reason) <> ''),
    CONSTRAINT recommendation_items_run_id_fkey
        FOREIGN KEY (run_id)
        REFERENCES recommendation.recommendation_runs (id)
        ON DELETE NO ACTION
        ON UPDATE NO ACTION
);

COMMENT ON SCHEMA recommendation IS
    'Versioned, expiring recommendation results owned by Recommendation Service.';
COMMENT ON COLUMN recommendation.recommendation_runs.user_id IS
    'Auth user UUID; intentionally not a cross-database foreign key.';
COMMENT ON COLUMN recommendation.recommendation_items.track_id IS
    'Catalog track UUID; intentionally not a cross-database foreign key.';

CREATE INDEX recommendation_runs_user_version_generated_idx
    ON recommendation.recommendation_runs (
        user_id,
        algorithm_version,
        generated_at DESC,
        id DESC
    );
CREATE INDEX recommendation_runs_expires_at_idx
    ON recommendation.recommendation_runs (expires_at);
CREATE INDEX recommendation_items_track_id_idx
    ON recommendation.recommendation_items (track_id);
