# ADR 0005: Authorize shared services with explicit grants

- Status: Accepted
- Date: 2026-07-17

## Context

HomeHub is owner-first, but selected services may later be shared with named
friends. A service must not implement its own account database or infer access
from possession of a route URL. The catalog is deployment configuration, while
runtime grants and their expiry are mutable security state.

## Decision

- HomeHub Control remains the policy decision point for protected HTTP routes.
- A non-administrator may access a service only when the catalog marks it
  `shared`, enables sharing, and an active unexpired database grant exists.
- Administrators retain access to every registered service.
- ForwardAuth resolves the request path against the catalog using a segment
  boundary and denies unregistered routes.
- Service lists are filtered by the same policy used at the gateway.
- Grant creation and revocation require an administrator session, trusted
  Origin, and CSRF validation, and create audit events.
- The catalog remains the source of service identity and shareability; the
  database stores principals, grants, expiry, revocation, and audit history.

## Consequences

Adding a shareable service requires an explicit catalog declaration and a
Traefik ForwardAuth middleware. Possessing a URL or stale grant is insufficient.
Friend invitation and credential enrollment are separate follow-up work and do
not weaken this default-deny boundary.
