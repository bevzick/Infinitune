CREATE SCHEMA "user";

CREATE TYPE "user".interface_theme AS ENUM (
    'light',
    'dark',
    'system'
);

CREATE TYPE "user".audio_quality AS ENUM (
    'low',
    'medium',
    'high',
    'lossless'
);

CREATE TABLE "user".profiles (
    user_id uuid PRIMARY KEY,
    username varchar(50) NOT NULL,
    display_name varchar(100) NOT NULL,
    bio varchar(500) NOT NULL DEFAULT '',
    avatar_url text NOT NULL DEFAULT '',
    country_code char(2) NOT NULL DEFAULT 'ZZ',
    language varchar(10) NOT NULL DEFAULT 'und',
    birth_date date,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT profiles_username_key UNIQUE (username)
);

CREATE TABLE "user".user_settings (
    user_id uuid PRIMARY KEY,
    theme "user".interface_theme NOT NULL DEFAULT 'system',
    autoplay boolean NOT NULL DEFAULT true,
    explicit_content boolean NOT NULL DEFAULT true,
    audio_quality "user".audio_quality NOT NULL DEFAULT 'high',
    normalize_volume boolean NOT NULL DEFAULT true,
    show_listening_activity boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_settings_user_id_fkey
        FOREIGN KEY (user_id)
        REFERENCES "user".profiles (user_id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE TABLE "user".user_preferences (
    user_id uuid PRIMARY KEY,
    recommendations_enabled boolean NOT NULL DEFAULT true,
    preferred_mood_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_preferences_user_id_fkey
        FOREIGN KEY (user_id)
        REFERENCES "user".profiles (user_id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);
