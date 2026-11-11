CREATE TABLE catalog.track_lyrics (
    id uuid PRIMARY KEY,
    track_id uuid NOT NULL,
    lyrics_type text NOT NULL,
    language text NOT NULL,
    content text NOT NULL,
    source_type text NOT NULL,
    source_name text NOT NULL,
    copyright_notice text NOT NULL DEFAULT '',
    official boolean NOT NULL DEFAULT false,
    rights_authorized boolean NOT NULL DEFAULT false,
    published boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT track_lyrics_track_id_fkey
        FOREIGN KEY (track_id)
        REFERENCES catalog.tracks (id)
        ON DELETE CASCADE,
    CONSTRAINT track_lyrics_track_language_key UNIQUE (track_id, language),
    CONSTRAINT track_lyrics_type_check
        CHECK (lyrics_type IN ('plain', 'synchronized')),
    CONSTRAINT track_lyrics_language_check
        CHECK (language = lower(language) AND language ~ '^[a-z0-9]{1,8}(-[a-z0-9]{1,8})*$'),
    CONSTRAINT track_lyrics_content_check
        CHECK (btrim(content) <> '' AND char_length(content) <= 100000),
    CONSTRAINT track_lyrics_source_type_check
        CHECK (source_type IN ('artist', 'publisher', 'licensed_provider', 'public_domain')),
    CONSTRAINT track_lyrics_source_name_check
        CHECK (btrim(source_name) <> '' AND char_length(source_name) <= 200),
    CONSTRAINT track_lyrics_copyright_notice_check
        CHECK (char_length(copyright_notice) <= 1000),
    CONSTRAINT track_lyrics_publication_rights_check
        CHECK (NOT published OR rights_authorized)
);

CREATE INDEX track_lyrics_public_track_idx
    ON catalog.track_lyrics (track_id, language, id)
    WHERE published AND rights_authorized;
