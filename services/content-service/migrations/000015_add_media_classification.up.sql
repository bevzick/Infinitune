ALTER TABLE content.media_asset_readiness
    ADD COLUMN genres text[] NOT NULL DEFAULT '{}',
    ADD COLUMN moods text[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN content.media_asset_readiness.genres IS
    'Bounded genre slugs normalized by Media from external provider metadata.';
COMMENT ON COLUMN content.media_asset_readiness.moods IS
    'Bounded mood slugs inferred by Media from external provider metadata.';
