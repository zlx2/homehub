# V2 component boundaries

## Runtime components

| Component | Owns | Must not own |
| --- | --- | --- |
| Traefik | public TLS, routes, coarse limits | users, roles, business data |
| IAM | principals, credentials, sessions, grants, delegation, tokens, audit | service catalog, business resources |
| OpenFGA | relationship tuples and authorization models | credentials, sessions, JWT signing keys |
| Control | service catalog, health aggregation, node metadata | passwords, sessions, authorization grants |
| Portal | user experience and client-side routing | authorization truth |
| Business service | its API, data, migrations, local resource ownership | another service's database |

## Principal identifiers

Principal identifiers are immutable and globally unambiguous:

```text
human:<id>
guest:<id>
device:<id>
node:<id>
workload:<id>
agent:<id>
```

Display names, Telegram IDs, email addresses, and device names are mutable
attributes or external account mappings, never principal IDs.

## Request paths

Human browser request:

```text
browser -> Traefik -> IAM session check/token mint -> target service
```

East-west workload request:

```text
workload -> IAM token exchange (cached until near expiry)
         -> target service over Docker DNS
         -> local signature/audience/permission validation
```

Delegated agent request:

```text
Hermes credential -> IAM -> token(sub=human:luna, act=agent:hermes)
                         -> audience-bound target service
```

## Service manifest

Every service publishes a versioned manifest containing its stable audience,
permission definitions, default role templates, OpenAPI document location,
token TTL ceiling, and supported event contracts. IAM rejects unknown audiences
and permissions.
