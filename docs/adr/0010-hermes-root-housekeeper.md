# ADR 0010: Hermes as the HomeHub root housekeeper

## Status

Accepted and implemented — 2026-07-18

## Context

Hermes Agent is the owner's primary automation environment and HomeHub
housekeeper. It must be able to develop against, transform content for, and
operate every HomeHub microservice without human login, TOTP, invitation,
browser cookie, CSRF, or Origin workflows.

Issuing a separate credential per service would make Hermes configuration
fragile and would require manual changes whenever a new service is registered.
Sending one reusable internal identity directly to every service would remove
audience isolation and allow replay between services.

## Decision

- `agent.root` is the reserved scope for the Hermes housekeeper.
- Control issues one revocable long-lived `hht_` API token with `service_id="homehub"`
  and the single `agent.root` scope. Its raw value is displayed once and must be
  stored in Bitwarden Secrets Manager, never Git or logs.
- The token is non-interactive authentication. Hermes sends it as
  `Authorization: Bearer ...`; no human authentication checks apply.
- Control accepts this token for every route matched to a registered catalog
  service and for Control's authenticated/admin APIs.
- Before forwarding to an identity-aware service, Control exchanges the root
  token for a signed identity that expires after 60 seconds and whose audience
  is exactly the matched service ID.
- Every identity-aware service must accept `agent.root` and map it to its
  highest permission level. Drop maps it to the explicit `hermes` role.
- Newly generated Go and Rust services include `agent.root` in their accepted
  identity scopes by default.
- HomeHub remains operationally independent from Hermes. Hermes being stopped
  must not affect routing, authentication for humans, or service availability.

## Consequences

- One Hermes credential works across current and future registered services.
- Hermes can perform reads, writes, administration, and destructive operations
  without interactive verification.
- Services still reject client-supplied identity headers and still validate
  signature, issuer, audience, expiry, and scope on Control-issued identities.
- Compromise of the Hermes root token is equivalent to compromise of the HomeHub
  owner. Revocation in Control immediately blocks new requests; already issued
  internal identities expire within 60 seconds.
