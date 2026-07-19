# ADR 0011: Identity and service architecture

Status: accepted

## Context

The exploratory implementation placed human authentication, sharing grants,
service catalog behavior, and internal token issuance in HomeHub Control. It
also introduced entry-point-specific tokens such as an iPhone upload token and
a Telegram Drop token. That model does not scale cleanly to many workloads,
devices, agents, groups, and object-level sharing relationships.

The platform is intentionally a single IAM-native stack; earlier session,
token, schema, and API formats are outside its compatibility contract.

## Decision

HomeHub V2 uses the following control-plane boundaries:

- **HomeHub IAM** owns principals, credentials, sessions, delegations, signing
  keys, token exchange, and authorization orchestration.
- **OpenFGA** stores and evaluates role, group, ownership, and cross-resource
  relationships. It is not called on every ordinary business request.
- **HomeHub Control** owns service catalog, health aggregation, node metadata,
  and portal-facing control-plane APIs.
- **Business services** own their business data and enforce concrete
  permissions from short-lived, audience-bound tokens.
- **Traefik** remains the public TLS and routing edge.

The stable principal kinds are `human`, `guest`, `device`, `node`, `workload`,
and `agent`. External provider secrets are credentials, not principals.

Permissions use `<service>.<resource>.<action>`. Roles are mutable bundles of
permissions; services never authorize by role name. `system.root` is reserved
for the Hermes housekeeper agent.

Delegation is first class. Tokens retain an effective subject (`sub`), actual
actor (`act`), authorized client (`azp`), audience (`aud`), realm, concrete
permissions, issue/expiry times, and unique token/session identifiers. A
delegated token cannot exceed the intersection of caller permissions, target
audience permissions, and delegation constraints.

The normal service request path performs local Ed25519 signature validation and
local resource checks. IAM and OpenFGA are consulted when issuing tokens,
changing relationships, or evaluating genuinely dynamic cross-resource access.

## Language and deployment

- Go 1.26.5 is the default backend language.
- Rust stable 1.97 is used only after a measured performance, memory, parsing,
  or safety requirement justifies a second SDK implementation.
- The portal is a static React/TypeScript/Vite application.
- PostgreSQL is one server with independently owned databases and users.
- REST/JSON and OpenAPI 3.1 are the default contracts.
- Asynchronous contracts use a versioned event envelope and transactional
  outbox. A broker requires a later ADR.

## Consequences

IAM is a separate availability boundary, while ordinary business traffic remains able
to proceed with already-issued short-lived tokens. New services register a
versioned service manifest instead of requiring IAM conditionals.
