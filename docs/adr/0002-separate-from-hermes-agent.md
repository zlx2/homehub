# ADR 0002: Keep HomeHub independent from Hermes Agent

- Status: Accepted
- Date: 2026-07-17

## Context

The server already runs Nous Research Hermes Agent from `~/.hermes`. Hermes
Agent is a high-privilege AI assistant with messaging integrations, memories,
skills, automations, and broad access to secrets. Its gateway is a messaging
gateway rather than an HTTP edge gateway for application services.

## Decision

- HomeHub is developed and operated as an independent system.
- HomeHub does not use Hermes Agent for routing, authentication, authorization,
  service discovery, configuration, secret delivery, or runtime availability.
- Hermes Agent remains on its existing localhost and Tailscale-facing paths.
- HomeHub must not read or modify `~/.hermes`.
- A future integration, if desired, treats Hermes Agent as an external client or
  separately registered service with a least-privilege identity and explicit scopes.

## Consequences

Failure, upgrade, compromise, or removal of Hermes Agent cannot break HomeHub's
public services. HomeHub duplicates no Hermes Agent internals. Future integration
requires an explicit API boundary instead of shared files, databases, or secrets.
