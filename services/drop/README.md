# Drop

Drop is HomeHub's temporary text and file relay. It keeps its own Vue 3 UI,
Go API, SQLite database, and blob directory while delegating login, sharing,
and authorization to HomeHub Control.

## Runtime contract

- Public route: `/drop/` through Traefik only.
- Internal listener: `0.0.0.0:8080` on the `homehub-edge` network.
- Identity: short-lived `X-HomeHub-Identity` token issued by Control for the
  `drop` audience. Drop validates signature, issuer, audience, expiry, issued
  time, and scopes before trusting it.
- Owner (`admin` scope): upload, list, download, change expiry, delete, and
  view storage status.
- Shared guest (`portal.view` scope): upload, list, and download. Owner-only
  operations return `403`.
- Persistence: `/data/drop.db`, `/data/blobs`, and `/data/tmp`. SQLite is the
  durable metadata source; files are stored on the same service-owned volume.

The former public authorization-code, Tailscale identity, and Hermes bearer
listeners are not registered by the HomeHub runtime.

## Toolchains

- Go `1.26.5`
- Node `24.17.0`
- pnpm `11.7.0`
- Alpine `3.24`

The production Dockerfile builds the Vue bundle first, embeds it in the Go
binary, and produces a non-root image of roughly 10 MB.

## Configuration

| Variable | Production value | Purpose |
| --- | --- | --- |
| `DROP_LISTEN_ADDRESS` | `0.0.0.0:8080` | Internal HTTP listener |
| `DROP_BASE_PATH` | `/drop` | Public URL prefix |
| `DROP_DATA_DIR` | `/data` | Service-owned persistent data |
| `DROP_IDENTITY_KEY_FILE` | `/run/secrets/drop_identity_key` | HomeHub identity verification key |
| `DROP_ALLOWED_ORIGINS` | HomeHub public origins | Mutation origin allowlist |

Size, quota, TTL, and timeout settings remain configurable through the
`DROP_*` values defined in `internal/config`.

Run builds and tests from the monorepo root:

```sh
make test-drop
docker compose --env-file deploy/compose/.env.example \
  -f deploy/compose/compose.yaml up -d --build drop
```

Production secrets are materialized outside Git and should be sourced from
Bitwarden Secrets Manager. Never expose the Drop container port directly.
