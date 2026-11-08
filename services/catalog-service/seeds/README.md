# Catalog development seed

`development.sql` contains fictional metadata created only for Infinitune development and demos. It does not include audio, lyrics, artwork, or metadata copied from real releases.

Load it explicitly after migrations:

```sh
make catalog-seed
```

The seed is idempotent and uses stable UUIDs so later development services can reference the same demo entities. It is intentionally separate from database migrations and is not loaded by `make up` or by the Catalog Service at startup.
