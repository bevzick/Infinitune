CREATE TYPE media.source_kind AS ENUM (
    'local_device',
    'server_storage',
    'external_provider'
);

CREATE TABLE media.asset_sources (
    media_asset_id uuid PRIMARY KEY,
    source_kind media.source_kind NOT NULL,
    provider varchar(50),
    external_id varchar(512),
    source_url text,
    license_name text,
    license_url text,
    attribution text,
    allows_streaming boolean,
    allows_redistribution boolean,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_sources_media_asset_id_fkey
        FOREIGN KEY (media_asset_id)
        REFERENCES media.media_assets (id)
        ON UPDATE NO ACTION
        ON DELETE CASCADE,
    CONSTRAINT asset_sources_metadata_object
        CHECK (
            jsonb_typeof(metadata) = 'object'
            AND octet_length(metadata::text) <= 32768
        ),
    CONSTRAINT asset_sources_external_fields
        CHECK (
            (
                source_kind = 'external_provider'
                AND provider IS NOT NULL
                AND provider IN ('jamendo', 'internet_archive')
                AND external_id IS NOT NULL
                AND btrim(external_id) <> ''
                AND source_url IS NOT NULL
                AND source_url ~ '^https://'
                AND license_name IS NOT NULL
                AND btrim(license_name) <> ''
                AND license_url IS NOT NULL
                AND license_url ~ '^https://'
                AND allows_streaming IS NOT NULL
                AND allows_redistribution IS NOT NULL
            )
            OR
            (
                source_kind IN ('local_device', 'server_storage')
                AND provider IS NULL
                AND external_id IS NULL
                AND source_url IS NULL
                AND license_name IS NULL
                AND license_url IS NULL
                AND attribution IS NULL
                AND allows_streaming IS NULL
                AND allows_redistribution IS NULL
            )
        )
);

CREATE INDEX asset_sources_external_identity_idx
    ON media.asset_sources (provider, external_id)
    WHERE source_kind = 'external_provider';

COMMENT ON TABLE media.asset_sources IS
    'Immutable provenance and license metadata for an ingested media asset.';
COMMENT ON COLUMN media.asset_sources.source_url IS
    'Stable public provider page, never a temporary or signed audio URL.';
