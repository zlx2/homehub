# Architecture overview

## Public request path

```text
client
  -> Cloudflare Tunnel
  -> Traefik
  -> IAM ForwardAuth for protected routes
  -> audience-bound short-lived token
  -> Control or business service
```

1. A client connects to `zlx2.com` over HTTPS.
2. Cloudflare Tunnel carries the request to the V2 Traefik origin.
3. Traefik clears client-supplied authorization context and tells IAM which
   audience is being requested.
4. IAM validates the HomeHub browser session or direct-share capability and
   returns a short-lived Ed25519 token.
5. Traefik forwards that trusted token to the target service.
6. The service validates signature, issuer, audience, expiry, and permissions
   locally before reading or writing its own resources.

Portal static assets and IAM login endpoints are reachable before login. Control
and Drop routes require the ForwardAuth flow. Workload-to-service calls skip the
browser edge and exchange their own machine credential directly with IAM.

## Configuration ownership

- `deploy/compose/compose.v2.yaml`: runtime services, networks, mounts, loopback
  ports, and health checks.
- `deploy/traefik-v2`: public hosts, paths, TLS, header cleanup, and ForwardAuth.
- Bitwarden Secrets Manager: credentials, signing keys, provider API keys, bot
  tokens, and tunnel token source material.
- IAM database: principals, sessions, credentials, grants, delegation, and
  authorization/audit state.
- OpenFGA database: relationship tuples and authorization models.
- Control catalog: service presentation and health targets.
- Service database/files: service-owned business data only.

## Network intent

- `homehub-v2-edge`: Traefik, Cloudflared, IAM, Control, Portal, and explicitly
  routed services.
- `homehub-v2-backend`: internal service APIs and IAM/OpenFGA communication;
  marked internal.
- `homehub-v2-data`: PostgreSQL and authorized clients; marked internal.
- Telegram Bridge currently uses host networking only to reach the host Mihomo
  proxy and loopback V2 endpoints.
- PostgreSQL and OpenFGA are not public.
- Existing MySQL `42061` and Redis `38291` public endpoints are intentionally
  preserved outside the V2 trust boundary.

## UI boundary

- Portal owns login, account/security screens, direct-share management, and the
  aggregate service dashboard.
- A business service may own an independent UI under its route. Drop currently
  serves its own React application at `/drop/`.
- UI state and disabled buttons are never authorization truth; the receiving
  API always verifies its IAM token and permission.
