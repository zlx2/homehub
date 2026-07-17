# HomeHub

Personal service platform for a single public server. The platform provides a
shared edge gateway, authentication, authorization, service catalog, sharing
grants, and an AI gateway for independently deployable services.

## Stack

- Traefik for TLS termination and request routing
- Go for the control plane and infrastructure-oriented services
- Svelte and TypeScript for the portal
- Rust or Go for business services
- PostgreSQL as the default durable database
- Redis for cache, rate limits, and transient queues
- Docker Compose for deployment and service discovery
- Bitwarden Secrets Manager for production secrets

The current stack includes PostgreSQL, HomeHub Control, the Svelte portal, and
Traefik. The owner portal is available at `https://111.229.205.99` with a trusted
short-lived IP certificate. Owner authentication uses an Argon2id password,
TOTP, an opaque server-side session, strict cookies, Origin validation, and CSRF
protection. Anonymous requests cannot read the service directory APIs.

## Development verification

```sh
make test-control
make compose-config
make dev-up
make dev-check
make public-check
```

The development portal is bound to `127.0.0.1:18080`. Traefik's development
dashboard is bound to `127.0.0.1:18081`.

The public edge binds only the server's private `eth0` address on ports 80 and
443. Port 80 serves ACME HTTP-01 challenges and redirects all other requests.
The certificate renewal timer checks twice daily and deploys renewed material
to Traefik without storing certificates in Git.

## Repository layout

- `apps/control`: HomeHub Control API
- `apps/portal`: owner portal
- `services`: independently deployable business services
- `packages`: shared contracts and small language-specific SDKs
- `deploy`: Compose, Traefik, PostgreSQL, and deployment scripts
- `docs`: architecture decisions and operational documentation
- `tests`: cross-service integration and end-to-end tests

## Safety

This repository must never contain production secrets, database contents,
private keys, or runtime data. Persistent data lives under
`/srv/homehub` on the server.
