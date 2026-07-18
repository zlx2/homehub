# HomeHub V2 network model

## Public domain edge

`zlx2.com` and `www.zlx2.com` are carried through a remotely managed Cloudflare
Tunnel. The `cloudflared` container joins `homehub-v2-edge`, opens no host port,
and forwards both hostnames to the Traefik HTTPS origin.

The Tunnel route keeps TLS verification enabled and uses `zlx2.com` as the
Origin Server Name. Traefik presents the domain certificate and redirects
`www.zlx2.com` to the apex domain.

The connector token is stored in Bitwarden Secrets Manager and materialized as
a read-only runtime file. It is not stored in Compose, `.env.v2`, or Git.

## Docker networks

### `homehub-v2-edge`

Members: Traefik, Cloudflared, ACME Challenge, IAM, Control, Drop, and Portal.
Only Traefik publishes public ports. Services on this network are reachable
publicly only when `deploy/traefik-v2/dynamic/routes.yaml` defines a router.

### `homehub-v2-backend`

Internal network for IAM, Control, OpenFGA, Drop, and other future east-west
APIs. It is marked `internal: true`.

### `homehub-v2-data`

Internal PostgreSQL network. PostgreSQL has no public or loopback host port.

### Telegram Bridge host network

Telegram Bridge currently uses host networking to reach the host Mihomo proxy
at `127.0.0.1:1081`. It reaches IAM and Drop through their loopback ports and
binds its own health endpoint only to `127.0.0.1:8730`.

## Host bindings

- `0.0.0.0:80` and `0.0.0.0:443`: V2 Traefik.
- `127.0.0.1:18080`: Portal direct check.
- `127.0.0.1:18100`: IAM direct check/integration tests.
- `127.0.0.1:18101`: OpenFGA HTTP.
- `127.0.0.1:18110`: Control direct check/integration tests.
- `127.0.0.1:18120`: Drop direct check/integration tests.
- `127.0.0.1:18181`: Traefik admin/ping.
- `127.0.0.1:8730`: Telegram Bridge health.

MySQL `42061` and Redis `38291` remain deliberately public outside HomeHub V2.
They are not protected by IAM ForwardAuth.

## Route policy

- `/`: Portal; login UI and static assets are public, application APIs decide
  session state.
- `/api/iam/`: IAM login, setup, session, Passkey, share, and token endpoints.
- `/api/control/`: IAM ForwardAuth with `homehub-control` audience.
- `/drop` and `/drop/`: IAM ForwardAuth with `homehub-drop` audience, then the
  `/drop` prefix is removed before forwarding.
- `/.well-known/acme-challenge/`: ACME challenge service.

Client-supplied `Authorization` is cleared before protected browser routes.
IAM is the only component allowed to provide the trusted target token to
Traefik for those requests.
