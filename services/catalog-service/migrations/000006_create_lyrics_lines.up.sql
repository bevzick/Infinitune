CREATE TABLE catalog.lyrics_lines (
    lyrics_id uuid NOT NULL,
    line_number integer NOT NULL,
    start_ms integer NOT NULL,
    end_ms integer NOT NULL,
    text_content text NOT NULL,
    PRIMARY KEY (lyrics_id, line_number),
    CONSTRAINT lyrics_lines_lyrics_id_fkey
        FOREIGN KEY (lyrics_id)
        REFERENCES catalog.track_lyrics (id)
        ON DELETE CASCADE,
    CONSTRAINT lyrics_lines_line_number_positive CHECK (line_number > 0),
    CONSTRAINT lyrics_lines_time_range_check CHECK (start_ms >= 0 AND end_ms > start_ms),
    CONSTRAINT lyrics_lines_text_check
        CHECK (btrim(text_content) <> '' AND char_length(text_content) <= 2000),
    CONSTRAINT lyrics_lines_start_time_key UNIQUE (lyrics_id, start_ms)
);

CREATE INDEX lyrics_lines_playback_position_idx
    ON catalog.lyrics_lines (lyrics_id, start_ms, end_ms, line_number);
