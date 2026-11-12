# Infinitune

## Containerized frontend

The production frontend is served by an unprivileged Nginx container on
<http://localhost:8088>. Requests to `/v1` are proxied to Kong inside the
Compose network, while all other unknown routes fall back to the React SPA.

Copy `.env.example` to `.env`, provide the required credentials and signing
keys, then use the existing stack commands:

```bash
make config
make up
```

Set `FRONTEND_PORT` to change the host port. Keep `VITE_GATEWAY_URL=/` for the
containerized same-origin setup.
