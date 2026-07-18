# ADR 0004: Owner authentication and public IP TLS

## Status

Superseded as the primary public-access plan. HomeHub now uses `zlx2.com`
through Cloudflare Tunnel; direct-IP TLS remains historical/fallback context.

## Decision

- Use a two-step, single-use owner bootstrap flow.
- Store passwords with Argon2id and encrypt TOTP secrets with AES-256-GCM.
- Require password and TOTP for owner login.
- Store only SHA-256 hashes of opaque session and CSRF tokens in PostgreSQL.
- Use Secure, HttpOnly, SameSite=Strict session cookies with a 12-hour idle and
  seven-day absolute lifetime.
- Validate the Origin header on every state-changing authentication request and
  require a session-bound CSRF token for logout and future authenticated writes.
- Keep the portal shell public but deny all service-directory data APIs until
  the owner session is authenticated.
- Terminate public TLS in Traefik with a Let's Encrypt short-lived IP
  certificate. Use Certbot webroot HTTP-01 validation and check renewal twice
  daily. The IP certificate is also Traefik's default certificate because many
  clients omit SNI for IP literals.

## Consequences

The owner can reach HomeHub without Tailscale while the directory and future
owner services remain closed by default. PostgreSQL is internal-only. Short
certificate lifetimes reduce stale-key exposure but make renewal monitoring a
required operational dependency. Friend sharing will use separate scoped
principals or grants; it will never reuse the owner session.
