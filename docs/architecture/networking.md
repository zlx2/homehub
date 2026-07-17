# Network model

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

## Host bindings during development

- `127.0.0.1:18080` forwards to the Traefik development entry point.
- `127.0.0.1:18081` forwards to Traefik's internal admin entry point.
- Existing host and Tailscale bindings remain unchanged.

Port 443 migration is a separate operation requiring an explicit cutover and
rollback plan.
