CREATE SCHEMA content;

CREATE TYPE content.creator_type AS ENUM (
    'artist',
    'label',
    'distributor'
);

CREATE TYPE content.verification_status AS ENUM (
    'unverified',
    'pending',
    'verified',
    'rejected'
);

CREATE TABLE content.creator_accounts (
    id uuid PRIMARY KEY,
    account_type content.creator_type NOT NULL,
    display_name varchar(150) NOT NULL,
    verification_status content.verification_status NOT NULL DEFAULT 'unverified',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT creator_accounts_display_name_not_blank CHECK (btrim(display_name) <> '')
);

CREATE TABLE content.creator_members (
    creator_id uuid NOT NULL,
    user_id uuid NOT NULL,
    member_role varchar(30) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (creator_id, user_id),
    CONSTRAINT creator_members_role_check
        CHECK (member_role IN ('owner', 'manager', 'editor', 'uploader')),
    CONSTRAINT creator_members_creator_id_fkey
        FOREIGN KEY (creator_id)
        REFERENCES content.creator_accounts (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON COLUMN content.creator_members.user_id IS
    'Cross-service Auth user UUID; intentionally has no database foreign key.';

CREATE INDEX creator_members_user_id_idx
    ON content.creator_members (user_id, created_at DESC, creator_id);
CREATE INDEX creator_accounts_status_updated_at_idx
    ON content.creator_accounts (verification_status, updated_at DESC, id);
