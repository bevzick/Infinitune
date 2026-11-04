ALTER TABLE "user".profiles
    DROP CONSTRAINT profiles_username_key;

CREATE UNIQUE INDEX profiles_username_key
    ON "user".profiles (lower(username));
