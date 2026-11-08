# Media development seed

Run `make media-seed` to create six private development audio objects and their Media metadata.

The audio is generated locally as short deterministic PCM tones. No third-party recording or large binary file is stored in Git. Stable `track_id` and `media_asset_id` values match `services/catalog-service/seeds/development.sql`.

The seed is idempotent, intended only for local development, and is not executed automatically when the application starts.
