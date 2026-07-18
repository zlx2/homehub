# ADR 0008: Delegated identity for the AI Gateway

## Status

Accepted; implementation and tests exist, but AI Gateway is not deployed in the
active V2 Compose stack as of 2026-07-19.

## Decision

Browser-to-service identities stay bound to exactly one audience. Business
services must not forward their own token to the AI Gateway and the AI Gateway
must not relax audience validation.

For a catalog service with `ai_enabled=true`, HomeHub Control returns a second
`X-HomeHub-AI-Identity` header from ForwardAuth. This token has `ai-gateway` as
its audience, identifies the source service with `azp`, carries the user
subject, includes the `ai.use` scope, and embeds the exact model aliases allowed
by the service catalog. It expires after 60 seconds.

Traefik removes all client-supplied HomeHub identity headers before ForwardAuth
injects trusted values. The source service forwards only the delegated token as
`X-HomeHub-Identity` on the internal AI request.

Only Control holds the Ed25519 signing key. Provider API credentials remain in
Bitwarden and are mounted only into the AI Gateway. The gateway is connected to
the internal backend network and a dedicated outbound network, has no host port,
and has no Traefik route.

## Consequences

- Browser clients cannot call the AI Gateway directly.
- A normal service identity cannot be replayed at the AI Gateway.
- A delegated token cannot use a model absent from its signed policy.
- Provider keys never enter business-service containers.
- Provider routing can change behind stable aliases without changing services.
