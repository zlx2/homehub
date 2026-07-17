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

The current stack includes PostgreSQL, HomeHub Control, the Svelte portal,
Traefik, a Beszel server-monitoring module, and Drop as the first shareable
business service. The owner portal is available at `https://111.229.205.99` with a trusted
short-lived IP certificate. Owner authentication uses an Argon2id password,
TOTP, an opaque server-side session, strict cookies, Origin validation, and CSRF
protection. Anonymous requests cannot read the service directory APIs.
The monitoring panel is mounted at `/server/`, reuses the HomeHub session through
ForwardAuth, maps an authenticated HomeHub administrator to its internal owner
account, and is restricted to the `admin` scope. Its local agent has no TCP
listener and reaches Docker only through a loopback-bound read-only socket proxy.

Service access is deny-by-default. Administrators can access every registered
service; other principals only see and reach services explicitly marked as
shareable and covered by an active, unexpired grant. Runtime grants live in the
Control database and grant changes are CSRF-protected and audited.

Friend access uses expiring capability links. An administrator selects one or
more shareable services and sends the generated URL; opening it creates a
restricted guest session without registration, a password, or TOTP. Revoking a
link immediately revokes its active guest sessions and grants. Plaintext link
tokens are never stored.

Internal service identity uses short-lived, audience-bound Ed25519 tokens.
Only HomeHub Control receives the signing seed; business containers receive a
read-only public key and verify tokens again with the shared Go or Rust SDK.
Catalog entries explicitly opt in with `identity_enabled`, so adding a service
does not require another Control code branch.

## Creating a service

Generate a compile-ready Go or Rust service with its health endpoints,
OpenAPI contract, hardened image, HomeHub identity middleware, Traefik labels,
Compose fragment, and catalog registration:

```sh
make new-service NAME=quick-notes LANG=go VISIBILITY=owner
make new-service NAME=shared-tool LANG=rust VISIBILITY=shared
```

Compose automatically discovers `services/*/compose.homehub.yaml`. Generated
changes remain normal source files and must pass review and tests before they
are deployed. The provider-neutral AI Gateway skeleton exists under
`services/ai-gateway`, but is intentionally not registered in production until
the delegated service-to-AI identity flow from ADR 0005 is implemented.

## Development verification

```sh
make test-control
make test-sdk-go
make test-sdk-rust
make test-drop
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

Static host firewall policy is intentionally minimal and lives in
`deploy/host`. Docker, Tailscale, and Fail2Ban own their dynamic rules; their
runtime chains must not be captured with `iptables-save` and restored at boot.
The same directory contains bounded journald and Docker logging defaults to
prevent routine logs from consuming the small system disk.
Install or refresh these host settings with `make host-baseline`. The installer
does not restart Docker; its logging defaults take effect after the next planned
daemon restart.

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
