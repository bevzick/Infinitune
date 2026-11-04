CREATE TABLE "user".user_genre_preferences (
    user_id uuid NOT NULL,
    genre_id uuid NOT NULL,
    weight numeric(4,3) NOT NULL,
    CONSTRAINT user_genre_preferences_pkey PRIMARY KEY (user_id, genre_id),
    CONSTRAINT user_genre_preferences_weight_check CHECK (weight >= 0 AND weight <= 1),
    CONSTRAINT user_genre_preferences_user_id_fkey
        FOREIGN KEY (user_id)
        REFERENCES "user".profiles (user_id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

CREATE TABLE "user".user_artist_preferences (
    user_id uuid NOT NULL,
    artist_id uuid NOT NULL,
    weight numeric(4,3) NOT NULL,
    CONSTRAINT user_artist_preferences_pkey PRIMARY KEY (user_id, artist_id),
    CONSTRAINT user_artist_preferences_weight_check CHECK (weight >= 0 AND weight <= 1),
    CONSTRAINT user_artist_preferences_user_id_fkey
        FOREIGN KEY (user_id)
        REFERENCES "user".profiles (user_id)
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);
