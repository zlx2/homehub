# HomeHub Agent Instructions

## Architecture

- This is a monorepo.
- Keep the control plane and portal separate from business microservices.
- Traefik is the edge gateway.
- Docker Compose provides deployment and service discovery.
- PostgreSQL is the default persistent database.
- Each service owns its database and must not query another service's database.
- Redis is cache or transient queue infrastructure, never the sole source of durable data.
- Production secrets must come from Bitwarden Secrets Manager.
- Never commit secrets, tokens, certificates, private keys, production data, or `.env` files.

## Languages and protocols

- Use Go 1.26.5 for HomeHub Control and infrastructure-oriented services.
- Use Rust stable 1.97 with Edition 2024 for suitable business microservices.
- Use Svelte and TypeScript for the portal.
- Start with REST and JSON for service APIs.
- Use SSE for streaming AI responses.
- Define public and internal API contracts with OpenAPI 3.1.

## Security

- Deny access by default.
- Public HTTP traffic enters through Traefik.
- Services must not trust identity headers supplied by clients.
- HomeHub Control performs authentication and authorization.
- Internal identity tokens must validate signature, issuer, audience, expiry, and scopes.
- Databases must not be exposed publicly unless explicitly documented.
- Existing public MySQL port 42061 and Redis port 38291 must remain available and will be hardened separately.
- Never mount the unrestricted Docker socket into application containers.

## Development

- Keep changes small and independently testable.
- Add tests for all security-sensitive behavior.
- Each service owns and applies its own schema migrations.
- Use structured logs, request IDs, timeouts, and graceful shutdown.
- Do not introduce Kubernetes, Nacos, a service mesh, or a message broker without an ADR.
- Do not modify, stop, or recreate existing server containers without explicit approval.
- Do not bind new development services to port 443 until the edge migration is approved.
- The existing Nous Research Hermes Agent under `~/.hermes` is a separate system.
- Do not read, modify, stop, or depend on Hermes Agent as part of HomeHub development.
- Any future Hermes Agent integration requires a separate ADR and a least-privilege service identity.
