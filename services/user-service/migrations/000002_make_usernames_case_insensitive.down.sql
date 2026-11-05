DROP INDEX "user".profiles_username_key;

ALTER TABLE "user".profiles
    ADD CONSTRAINT profiles_username_key UNIQUE (username);
