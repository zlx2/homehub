# ADR 0003: Keep service catalog metadata outside the Docker API

- Status: Accepted
- Date: 2026-07-17

## Context

Traefik needs read-only Docker API access to discover routes. HomeHub Control also
needs service names, descriptions, visibility, sharing defaults, and internal
health endpoints. Giving Control access to container inspection would expose
container environment variables and other operational metadata to a public-facing
application process.

## Decision

- Traefik alone uses the restricted Docker socket proxy.
- HomeHub Control does not join the Docker API network.
- Route declarations remain Docker labels consumed by Traefik.
- Dashboard metadata and health targets live in `deploy/catalog/services.json`.
- Control validates the catalog at startup and refuses unknown fields, duplicate
  IDs, invalid visibility values, and non-HTTP health URLs.
- Public API responses never include internal health URLs or raw transport errors.

## Consequences

Adding a service currently requires a route label and a catalog entry. This small
amount of duplication is preferred to expanding Docker API access. A future
deployment manifest compiler may generate both outputs from one reviewed source.
