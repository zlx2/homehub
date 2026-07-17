# Architecture overview

## Request path

1. A client connects to Traefik over HTTPS.
2. Traefik asks HomeHub Control to authorize protected requests.
3. HomeHub Control validates the owner or guest session and requested scope.
4. Traefik removes untrusted identity headers and forwards trusted short-lived identity context.
5. The destination service validates issuer, audience, expiry, and scopes.
6. The service reads and writes only its own database and file volumes.

## Configuration ownership

- Compose and labels: deployment, networking, route declarations, catalog defaults.
- Bitwarden Secrets Manager: credentials, signing keys, provider API keys.
- HomeHub Control database: principals, sessions, service grants, expiry,
  revocation, scopes, and audit events.
- Service database: service-owned business data.

## Network intent

- Edge network: Traefik and explicitly routed HTTP services.
- Backend network: internal service-to-service APIs.
- Data network: PostgreSQL, Redis, and their authorized clients.
- Only approved host ports are published.
- Existing MySQL 42061 and Redis 38291 public endpoints remain available.
- New PostgreSQL is internal-only.
