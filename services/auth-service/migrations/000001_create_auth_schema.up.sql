CREATE SCHEMA auth;

CREATE TYPE auth.account_status AS ENUM (
    'active',
    'blocked',
    'deleted'
);

CREATE TABLE auth.users_auth (
    id uuid PRIMARY KEY,
    email varchar(320) NOT NULL,
    password_hash varchar(255) NOT NULL,
    status auth.account_status NOT NULL,
    email_verified boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    last_login_at timestamptz,
    CONSTRAINT users_auth_email_key UNIQUE (email)
);

CREATE TABLE auth.roles (
    id smallserial PRIMARY KEY,
    name varchar(30) NOT NULL,
    description varchar(255),
    CONSTRAINT roles_name_key UNIQUE (name)
);

CREATE TABLE auth.user_roles (
    user_id uuid NOT NULL,
    role_id smallint NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT user_roles_pkey PRIMARY KEY (user_id, role_id),
    CONSTRAINT user_roles_user_id_fkey
        FOREIGN KEY (user_id)
        REFERENCES auth.users_auth (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION,
    CONSTRAINT user_roles_role_id_fkey
        FOREIGN KEY (role_id)
        REFERENCES auth.roles (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE INDEX user_roles_role_id_idx ON auth.user_roles (role_id);
