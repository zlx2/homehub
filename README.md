# HomeHub

Personal service platform for a single public server. The platform provides a
shared edge gateway, authentication, authorization, service catalog, sharing
grants, and an AI gateway for independently deployable services.

## Planned stack

- Traefik for TLS termination and request routing
- Go for the control plane and infrastructure-oriented services
- Svelte and TypeScript for the portal
- Rust or Go for business services
- PostgreSQL as the default durable database
- Redis for cache, rate limits, and transient queues
- Docker Compose for deployment and service discovery
- Bitwarden Secrets Manager for production secrets

No production service is defined in this initial scaffold.

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
