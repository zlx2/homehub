# ADR 0006: Enroll friends through one-time invitations

- Status: Accepted
- Date: 2026-07-17

## Context

HomeHub needs named friend accounts without enabling open registration. Friends
must receive only explicitly selected services and should use the same strong
session and TOTP controls as the owner. Invitation URLs may pass through chat
applications and must be safe to revoke or expire.

## Decision

- Only an authenticated administrator may create or revoke an invitation.
- Invitation tokens contain 256 bits of randomness, are returned once, and are
  stored only as SHA-256 hashes.
- Invitations expire after 24 hours by default and may never exceed seven days.
- An invitation may be claimed by one active 15-minute enrollment attempt.
- The friend selects a unique username and a 12-256 character password, then
  provisions TOTP before the account is created.
- TOTP confirmation is limited to five failures per enrollment attempt.
- Account creation, invitation consumption, initial service grants, session
  creation, and audit recording commit in one database transaction.
- Open registration remains unavailable. A friend receives only `portal.view`
  plus grants for services selected by the administrator.

## Consequences

Possession of an unused invitation token is sufficient to begin enrollment, so
administrators must transmit it to the intended recipient and revoke it if that
channel is compromised. Expired attempts may be restarted with the same still
valid invitation. The upcoming portal UI should place the token in the URL
fragment so it does not enter HTTP request logs or Referer headers.
