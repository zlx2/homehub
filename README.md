# HomeHub

HomeHub is a personal service platform for one or more private servers. It
provides a shared public edge, identity and authorization, a modular portal,
agent delegation, service discovery, and independently deployable business
services.

V2 is an intentional clean rebuild. V1 test data, sessions, tokens, database
schemas, and API compatibility are not retained.

## V2 stack

- Traefik for TLS termination and public routing
- Go 1.26.5 as the default backend language
- React 19, TypeScript, Vite, and Tailwind CSS for the static portal
- Rust stable 1.97 only for measured performance, memory, parsing, or safety needs
- OpenFGA for relationship authorization
- PostgreSQL for durable data, with a database and user owned by each service
- Redis for cache, rate limits, and transient work only
- Docker Compose for deployment and service discovery
- Bitwarden Secrets Manager for production credentials and private keys

## Control plane

- `apps/iam`: principals, credentials, sessions, authorization orchestration,
  delegation, token exchange, signing keys, and audit
- `apps/control`: service catalog, health aggregation, node metadata, and portal
  control-plane APIs
- `apps/portal`: the single React application shell and its feature modules
- `OpenFGA`: group, role, ownership, and cross-resource relationships

HomeHub uses six stable principal kinds: `human`, `guest`, `device`, `node`,
`workload`, and `agent`. Permissions use
`<service>.<resource>.<action>`. Hermes is the trusted housekeeper agent and may
receive the reserved `system.root` permission, but still uses short-lived,
audience-bound tokens and remains attributable as the actual actor.

## Business services

Business services live under `services`. Each service owns its API, database,
migrations, local resource rules, and versioned service manifest. Services do
not read another service's database and do not trust identity headers supplied
by clients.

The normal request path validates a short-lived Ed25519 token locally. IAM and
OpenFGA are not network dependencies for every business request.

## Development

The authoritative development worktree is `/home/ubuntu/homehub-v2` on the
HomeHub server. The previous `/home/ubuntu/homehub` tree is retained only as a
reference during reconstruction.

```sh
make test-iam
make test-control
make test-portal
make test-sdk-go
make compose-config
```

Production secrets, private keys, certificates, `.env` files, and runtime data
must never be committed. Persistent runtime data lives outside this repository.

See [ADR 0011](docs/adr/0011-v2-identity-and-service-architecture.md) and the
[V2 component boundaries](docs/architecture/v2-boundaries.md) for the current
architecture contract.
