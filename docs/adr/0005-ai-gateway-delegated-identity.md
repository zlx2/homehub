# ADR 0005: Delegated identity for the AI Gateway

## Status

Accepted for implementation; AI provider routing remains disabled.

## Decision

Browser-to-service identities stay bound to exactly one audience. Business
services must not forward their own token to the AI Gateway and the AI Gateway
must not relax audience validation.

HomeHub Control will issue a separate, short-lived AI delegation only for
services whose catalog policy explicitly grants AI access. The delegated token
will preserve the user subject and scopes, use `ai-gateway` as its audience,
and identify the calling service as the authorized party. The source service
may forward only that delegated token to the internal AI endpoint.

Only Control holds the Ed25519 signing key. The AI Gateway and all business
services receive the public verification key. Provider API credentials remain
in Bitwarden and are mounted only into the AI Gateway.

## Consequences

- A token stolen from one business service cannot be replayed at another
  ordinary service.
- AI access can be revoked per source service without changing user sessions.
- Provider keys never enter business-service containers.
- The AI Gateway skeleton is not deployed until delegated issuance and its
  tests are complete.
