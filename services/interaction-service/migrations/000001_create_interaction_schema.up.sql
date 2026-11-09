CREATE SCHEMA interaction;

CREATE TABLE interaction.track_likes (
    user_id uuid NOT NULL,
    track_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, track_id)
);

CREATE TABLE interaction.album_likes (
    user_id uuid NOT NULL,
    album_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, album_id)
);

CREATE TABLE interaction.artist_follows (
    user_id uuid NOT NULL,
    artist_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, artist_id)
);

CREATE TABLE interaction.comment_likes (
    user_id uuid NOT NULL,
    comment_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, comment_id)
);

COMMENT ON SCHEMA interaction IS
    'User-owned likes and follows; referenced Auth, Catalog, and Comment UUIDs have no cross-database foreign keys.';

CREATE INDEX track_likes_user_created_at_idx
    ON interaction.track_likes (user_id, created_at DESC, track_id);
CREATE INDEX track_likes_track_created_at_idx
    ON interaction.track_likes (track_id, created_at DESC, user_id);

CREATE INDEX album_likes_user_created_at_idx
    ON interaction.album_likes (user_id, created_at DESC, album_id);
CREATE INDEX album_likes_album_created_at_idx
    ON interaction.album_likes (album_id, created_at DESC, user_id);

CREATE INDEX artist_follows_user_created_at_idx
    ON interaction.artist_follows (user_id, created_at DESC, artist_id);
CREATE INDEX artist_follows_artist_created_at_idx
    ON interaction.artist_follows (artist_id, created_at DESC, user_id);

CREATE INDEX comment_likes_user_created_at_idx
    ON interaction.comment_likes (user_id, created_at DESC, comment_id);
CREATE INDEX comment_likes_comment_created_at_idx
    ON interaction.comment_likes (comment_id, created_at DESC, user_id);
