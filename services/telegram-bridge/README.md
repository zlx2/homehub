# Telegram Bridge

Telegram Bridge is an internal HomeHub worker that forwards allowed Telegram
private or group messages to Drop. It uses Bot API long polling, so no public
webhook route or additional public port is required.

The worker owns a HomeHub `workload` identity with only Drop's `caller`
relationship. It exchanges that machine credential with IAM for a two-minute,
audience-bound `drop.item.create` access token and caches it briefly. It cannot
list, read, or delete Drop items and stores no permanent Drop token.

## Behavior

- Text and captions become Drop text.
- The largest Telegram photo rendition is forwarded as JPEG.
- Documents, videos, animations, audio, voice, video notes, and stickers are
  streamed to Drop without another transcoding or recompression step.
- A deterministic idempotency key derived from bot, chat, and message IDs makes
  Telegram retries safe.
- Private chats receive a success acknowledgement by default; groups stay quiet.
- `/whoami` returns the sender and chat IDs even before allowlisting, making the
  initial setup possible without logging message content.
- Empty allowlists deny all forwarding.

Telegram's hosted Bot API currently limits bot downloads to 20 MB. Photos sent
using Telegram's photo mode may already have been compressed by Telegram; send
the image as a document when the original bytes are required.

## Secrets

Create these values in the `HomeHub Production` Bitwarden Secrets Manager project:

| BWS key | Value |
| --- | --- |
| `telegram_bot_token` | Token issued by `@BotFather` |
| `telegram_bridge_credential` | Credential issued once by HomeHub IAM for the Telegram workload |

Never put either credential in Compose, `.env`, logs, Git, or chat messages. The
machine credential is not an access token: IAM validates it and issues a short-
lived token for each active upload window.

## Allowlist and group setup

1. Start a private chat with the bot and send `/whoami`.
2. Put the returned user ID in `TELEGRAM_BRIDGE_ALLOWED_USER_IDS`.
3. Add the bot to the group, send `/whoami`, and put the negative group ID in
   `TELEGRAM_BRIDGE_ALLOWED_CHAT_IDS` to forward messages from every member.
4. If every ordinary group message should be delivered, use BotFather `/setprivacy`
   and disable privacy mode for this bot, then remove/re-add it to the group if
   Telegram does not apply the change immediately.

Comma-separated IDs are accepted. The server currently loads these non-secret
values from the ignored `deploy/compose/.env` file.

## Development

```bash
cd /home/ubuntu/homehub-v2/services/telegram-bridge
go test ./...

cd /home/ubuntu/homehub-v2
make v2-up
make test-telegram-bridge-integration
```

The container uses host networking only to reach the loopback Mihomo proxy on
`127.0.0.1:1081`. IAM and Drop are also reached only through their loopback V2
ports. The health server binds only `127.0.0.1:8730`; it does not register a
Traefik route.
