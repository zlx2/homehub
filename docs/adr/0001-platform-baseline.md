# ADR 0001: Platform baseline

- Status: Accepted
- Date: 2026-07-17

## Context

Hermes is a personal platform hosted on one public Ubuntu server. The owner is
the primary user, with occasional scoped access granted to guests. Services
must be easy to add without reimplementing authentication, authorization,
routing, AI provider integration, or operational conventions.

## Decision

- Use Traefik as the edge data plane.
- Build Hermes Control in Go as the authentication, authorization, sharing,
  catalog, and policy control plane.
- Use Docker Compose DNS and labels instead of a separate registry such as Nacos.
- Use one physical PostgreSQL instance with a separate database and role owned
  by each service.
- Permit service-local SQLite only for single-instance, low-write local state.
- Use Redis only for cache, rate limits, and transient work.
- Keep large binary objects in service-owned volumes rather than relational tables.
- Use Bitwarden Secrets Manager for production secrets.
- Begin with synchronous REST APIs. Introduce a transactional outbox before a
  message broker is justified.

## Consequences

The system remains small enough for one host while preserving logical service
boundaries. The physical PostgreSQL instance remains a shared failure domain,
which is acceptable because the host is already the system-wide failure domain.
Services cannot perform cross-database joins or depend on another service's
schema.
