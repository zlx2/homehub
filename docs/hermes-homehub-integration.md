# Hermes and HomeHub V2

## Current status

Nous Research Hermes Agent under `~/.hermes` is a separate host system. It is
not started, stopped, upgraded, or required by the active HomeHub V2 Compose
stack.

The repository contains historical V1 wrappers, a Hermes web-terminal service,
and the V2 `agent`/`system.root` identity model. Those pieces must not be read as
proof that the running Hermes installation is currently connected to V2. As of
2026-07-19:

- no Hermes container exists in `compose.v2.yaml`;
- the Hermes web-terminal route is not active in V2;
- the current HomeHub catalog may still list the terminal as unavailable;
- no repository documentation should expose or assume a reusable raw token.

## Intended V2 integration

Hermes should authenticate as the stable principal `agent:hermes` using a
revocable machine credential stored outside Git. The agent exchanges that
credential with IAM for short-lived tokens whose audience matches the target
service.

```text
Hermes machine credential
  -> IAM token exchange
  -> token(sub=agent:hermes, aud=target-service, permissions=[...])
  -> target service
```

For an explicit human delegation, IAM may instead issue a token whose subject is
the human and whose `act` claim records `agent:hermes`. This preserves the
difference between “Hermes acted directly” and “Hermes acted for Luna”.

`system.root` is the reserved V2 permission for the trusted housekeeper. Even a
root token remains short-lived and audience-bound; the IAM signing key and human
session cookie are never given to Hermes.

## Work required before enabling

1. Provision or verify the `agent:hermes` principal in V2 IAM.
2. Store its machine credential in Bitwarden and materialize a host-readable,
   least-exposed credential file.
3. Add/update an API wrapper or Hermes Skill that performs IAM token exchange.
4. Verify `system.root` handling in each target service.
5. Add integration tests for direct agent action and delegated action.
6. Decide separately whether the native Hermes web terminal returns to the V2
   Compose and Traefik route.

Do not reuse historical `/home/ubuntu/homehub` symlinks or the V1
`hermes_root_token` flow without a deliberate migration review.
