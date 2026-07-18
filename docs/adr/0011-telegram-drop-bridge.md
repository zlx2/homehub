# ADR 0011: Telegram-to-Drop bridge

- Status: accepted
- Date: 2026-07-18

## Context

The owner wants Telegram private messages and messages from a small group to
appear in Drop without opening another HomeHub page. The server is in mainland
China and already exposes a Mihomo HTTP proxy only on host loopback.

## Decision

Implement `telegram-bridge` as a Go 1.26.5 internal worker using Telegram Bot API
long polling.

- No Telegram webhook or HomeHub public route is created.
- The container uses host networking solely to reach `127.0.0.1:1081`; its own
  health endpoint binds `127.0.0.1:8730`.
- A dedicated revocable HomeHub token with only `drop.upload` is stored in BWS.
  The bridge calls the public Traefik route, where Control exchanges that token
  for a short-lived identity bound to the Drop audience.
- Telegram bot and Drop tokens are separate BWS values mounted as files.
- User and chat ID allowlists deny forwarding by default. `/whoami` provides the
  numeric identifiers required for initial setup but never forwards content.
- Each Telegram message maps to one Drop item. The idempotency key contains bot,
  chat, and message IDs so a crash between upload and update acknowledgement does
  not duplicate the item.
- Files are streamed from Telegram to Drop. The service does not recompress or
  transcode, but Telegram may already have compressed media sent in photo mode.

## Consequences

The worker has no user-facing module in the HomeHub catalog. Docker health and
structured logs provide operational visibility. Telegram's hosted Bot API limits
downloads to 20 MB; larger content must use Drop directly. Media albums initially
arrive as one Drop item per Telegram message and can be grouped later without
changing the authentication model.
