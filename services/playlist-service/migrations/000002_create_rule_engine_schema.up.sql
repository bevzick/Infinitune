CREATE SCHEMA rule_engine;

CREATE TYPE rule_engine.logical_operator AS ENUM (
    'AND',
    'OR'
);

CREATE TABLE rule_engine.rule_sets (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    name varchar(150) NOT NULL,
    logic_operator rule_engine.logical_operator NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT rule_sets_name_not_blank CHECK (btrim(name) <> '')
);

COMMENT ON COLUMN rule_engine.rule_sets.owner_id IS
    'Cross-service Auth user UUID; intentionally has no database foreign key.';

CREATE TABLE rule_engine.rule_conditions (
    id uuid PRIMARY KEY,
    rule_set_id uuid NOT NULL,
    field_name varchar(50) NOT NULL,
    operator varchar(30) NOT NULL,
    value jsonb NOT NULL,
    position integer NOT NULL,
    CONSTRAINT rule_conditions_field_name_not_blank CHECK (btrim(field_name) <> ''),
    CONSTRAINT rule_conditions_operator_not_blank CHECK (btrim(operator) <> ''),
    CONSTRAINT rule_conditions_position_positive CHECK (position > 0),
    CONSTRAINT rule_conditions_rule_set_position_key UNIQUE (rule_set_id, position),
    CONSTRAINT rule_conditions_rule_set_id_fkey
        FOREIGN KEY (rule_set_id)
        REFERENCES rule_engine.rule_sets (id)
        ON UPDATE NO ACTION
        ON DELETE CASCADE
);

CREATE INDEX rule_sets_owner_id_updated_at_idx
    ON rule_engine.rule_sets (owner_id, updated_at DESC, id);

CREATE INDEX playlists_rule_set_id_idx
    ON playlist.playlists (rule_set_id)
    WHERE rule_set_id IS NOT NULL;
