# Keycloak realm import (`custom-rag-realm`)

`realm-export.json` is consumed by Keycloak when `deploy/docker-compose.dev.yml` runs `start-dev --import-realm` with `./keycloak` mounted at `/opt/keycloak/data/import`.

## First start

From the **repository root**:

```bash
docker compose -f deploy/docker-compose.dev.yml up -d keycloak
```

Keycloak generates **new client secrets** for confidential clients (`rag-query`, `rag-ingest`) because the export omits `secret` fields. Copy them from **Admin Console → Clients → Credentials** (or use the CLI) and set your app env (for example `OIDC_CLIENT_SECRET` only where you exchange tokens with `rag-query`).

## Re-import after editing the JSON

The dev stack persists Keycloak’s H2 data in the `keycloak_data` volume. Edits to `realm-export.json` are **not** reapplied automatically. To force a fresh import:

```bash
docker compose -f deploy/docker-compose.dev.yml down
docker volume rm custom-rag-dev_keycloak_data   # name may differ; use docker volume ls
docker compose -f deploy/docker-compose.dev.yml up -d keycloak
```

Or `docker compose ... down -v` to remove all compose volumes (including Qdrant).

## Hardening in this export

- **`rag-ingest`**: no browser redirects; `redirectUris` / `webOrigins` are empty (client-credentials / service account only). No wildcard `/*`.
- **`rag-query`**: exact callback URLs only (`http://127.0.0.1:5555/oauth-callback`, `http://localhost:5555/oauth-callback` for `cmd/oauth-dev`). `webOrigins` is empty (no CORS wildcard via empty string).
