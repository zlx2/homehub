# ADR 0007: Use direct capability links for friend access

- Status: Accepted
- Date: 2026-07-17
- Supersedes: ADR 0006

## Context

HomeHub sharing is primarily short-lived access to one or two selected personal
services. Requiring a friend to create a username, password, and TOTP enrollment
adds account-management ceremony without improving the intended sharing flow.

## Decision

- An administrator creates a link containing a 256-bit random capability token,
  one or more explicitly selected shareable services, and an expiry no longer
  than seven days.
- The token remains in the URL fragment and is exchanged by the portal through a
  same-origin POST request. Only its SHA-256 digest is stored.
- Opening the link directly creates a restricted `portal.view` guest session. No
  registration, password, or TOTP is requested from the recipient.
- Reopening an active link may create another session for the same isolated guest
  principal. Session and service-grant expiry are capped by the link expiry.
- Revoking the link disables its guest principal and immediately revokes every
  associated session and service grant.
- Owner authentication continues to require password and TOTP.

## Consequences

The link is a bearer capability: anyone who obtains it can use the selected
services until it expires or is revoked. Administrators should therefore use
short expiries and revoke a link if it is sent to the wrong place. The simplified
flow deliberately trades named friend accounts for low-friction, tightly scoped
sharing.
