CREATE SCHEMA catalog;

CREATE TYPE catalog.album_type AS ENUM (
    'album',
    'single',
    'ep',
    'compilation'
);

CREATE TYPE catalog.artist_role AS ENUM (
    'primary',
    'featured',
    'producer',
    'composer'
);

CREATE TABLE catalog.artists (
    id uuid PRIMARY KEY,
    name varchar(150) NOT NULL,
    description text NOT NULL DEFAULT '',
    image_url text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT artists_name_not_blank CHECK (btrim(name) <> '')
);

CREATE TABLE catalog.albums (
    id uuid PRIMARY KEY,
    title varchar(200) NOT NULL,
    album_type catalog.album_type NOT NULL,
    release_date date NOT NULL,
    cover_url text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT albums_title_not_blank CHECK (btrim(title) <> '')
);

CREATE TABLE catalog.album_artists (
    album_id uuid NOT NULL,
    artist_id uuid NOT NULL,
    position smallint NOT NULL,
    PRIMARY KEY (album_id, artist_id),
    CONSTRAINT album_artists_position_positive CHECK (position > 0),
    CONSTRAINT album_artists_album_position_key UNIQUE (album_id, position),
    CONSTRAINT album_artists_album_id_fkey
        FOREIGN KEY (album_id)
        REFERENCES catalog.albums (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION,
    CONSTRAINT album_artists_artist_id_fkey
        FOREIGN KEY (artist_id)
        REFERENCES catalog.artists (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE TABLE catalog.tracks (
    id uuid PRIMARY KEY,
    album_id uuid,
    title varchar(200) NOT NULL,
    duration_ms integer NOT NULL,
    track_number smallint NOT NULL,
    disc_number smallint NOT NULL DEFAULT 1,
    explicit boolean NOT NULL DEFAULT false,
    release_date date NOT NULL,
    media_asset_id uuid NOT NULL,
    content_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tracks_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT tracks_duration_ms_positive CHECK (duration_ms > 0),
    CONSTRAINT tracks_track_number_positive CHECK (track_number > 0),
    CONSTRAINT tracks_disc_number_positive CHECK (disc_number > 0),
    CONSTRAINT tracks_album_id_fkey
        FOREIGN KEY (album_id)
        REFERENCES catalog.albums (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE TABLE catalog.track_artists (
    track_id uuid NOT NULL,
    artist_id uuid NOT NULL,
    artist_role catalog.artist_role NOT NULL,
    position smallint NOT NULL,
    PRIMARY KEY (track_id, artist_id, artist_role),
    CONSTRAINT track_artists_position_positive CHECK (position > 0),
    CONSTRAINT track_artists_track_role_position_key UNIQUE (track_id, artist_role, position),
    CONSTRAINT track_artists_track_id_fkey
        FOREIGN KEY (track_id)
        REFERENCES catalog.tracks (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION,
    CONSTRAINT track_artists_artist_id_fkey
        FOREIGN KEY (artist_id)
        REFERENCES catalog.artists (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE TABLE catalog.genres (
    id uuid PRIMARY KEY,
    name varchar(80) NOT NULL,
    slug varchar(80) NOT NULL,
    CONSTRAINT genres_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT genres_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE UNIQUE INDEX genres_name_lower_key ON catalog.genres (lower(name));
CREATE UNIQUE INDEX genres_slug_lower_key ON catalog.genres (lower(slug));

CREATE TABLE catalog.track_genres (
    track_id uuid NOT NULL,
    genre_id uuid NOT NULL,
    PRIMARY KEY (track_id, genre_id),
    CONSTRAINT track_genres_track_id_fkey
        FOREIGN KEY (track_id)
        REFERENCES catalog.tracks (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION,
    CONSTRAINT track_genres_genre_id_fkey
        FOREIGN KEY (genre_id)
        REFERENCES catalog.genres (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE TABLE catalog.moods (
    id uuid PRIMARY KEY,
    name varchar(80) NOT NULL,
    slug varchar(80) NOT NULL,
    CONSTRAINT moods_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT moods_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE UNIQUE INDEX moods_name_lower_key ON catalog.moods (lower(name));
CREATE UNIQUE INDEX moods_slug_lower_key ON catalog.moods (lower(slug));

CREATE TABLE catalog.track_moods (
    track_id uuid NOT NULL,
    mood_id uuid NOT NULL,
    weight numeric(4,3) NOT NULL,
    PRIMARY KEY (track_id, mood_id),
    CONSTRAINT track_moods_weight_range CHECK (weight >= 0 AND weight <= 1),
    CONSTRAINT track_moods_track_id_fkey
        FOREIGN KEY (track_id)
        REFERENCES catalog.tracks (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION,
    CONSTRAINT track_moods_mood_id_fkey
        FOREIGN KEY (mood_id)
        REFERENCES catalog.moods (id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE INDEX albums_release_date_idx ON catalog.albums (release_date DESC, id);
CREATE INDEX tracks_album_id_idx ON catalog.tracks (album_id);
CREATE INDEX tracks_release_date_idx ON catalog.tracks (release_date DESC, id);
CREATE INDEX album_artists_artist_id_idx ON catalog.album_artists (artist_id);
CREATE INDEX track_artists_artist_id_idx ON catalog.track_artists (artist_id);
CREATE INDEX track_genres_genre_id_idx ON catalog.track_genres (genre_id);
CREATE INDEX track_moods_mood_id_idx ON catalog.track_moods (mood_id);
