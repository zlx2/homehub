# Network model

## Public domain edge

`zlx2.com` and `www.zlx2.com` are proxied through a remotely managed
Cloudflare Tunnel. The `cloudflared` connector joins `homehub-v2-edge`, opens
no host port, and forwards both hostnames to `https://traefik:443`.

The Tunnel route must keep TLS verification enabled and set **Origin Server
Name** to `zlx2.com`. Traefik presents the public domain certificate, routes
the request by its original Host header, and redirects `www.zlx2.com` to the
canonical apex domain.

The connector token is stored in Bitwarden Secrets Manager as
`cloudflare_tunnel_token` and materialized read-only at runtime. It must never
be stored in Compose, an environment file, or the repository.

## `homehub-edge`

Traefik and HTTP services explicitly published through Traefik join this network.
A container is not routed unless `traefik.enable=true` is present.

## `homehub-backend`

Internal service APIs use this network. It is marked internal and has no direct
route outside Docker. Services address each other by Compose service name.

## `homehub-data`

PostgreSQL and authorized database clients use this internal network. PostgreSQL
does not publish a host port.

## `homehub-docker-api`

Only Traefik and the Docker socket proxy join this internal network. The proxy
permits the read-only API groups needed for container discovery and Docker event
watching. It does not publish a host port and denies POST requests.

## Host bindings

- `127.0.0.1:18080` forwards to the Traefik development entry point.
- `127.0.0.1:18081` forwards to Traefik's internal admin entry point.
- Existing host and Tailscale bindings remain unchanged.
- `10.0.0.15:80` serves ACME HTTP-01 and redirects other traffic to HTTPS.
- `10.0.0.15:443` is the public HomeHub edge and maps through Tencent Cloud to
  `111.229.205.99:443`.
- Tailscale continues to own only `100.102.192.32:443`; the bindings coexist.

Public traffic terminates at Traefik. The portal shell and authentication/setup
endpoints are reachable anonymously, while Control enforces authentication on
the system and service-directory APIs. Future business services attach the
Control forward-auth endpoint and remain deny-by-default.
