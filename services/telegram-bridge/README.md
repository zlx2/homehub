# Telegram Bridge

Telegram Bridge is an internal HomeHub worker that forwards allowed Telegram
private or group messages to Drop. It uses Bot API long polling, so no public
webhook route or additional public port is required.

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
| `telegram_drop_token` | HomeHub API token with only `drop.upload` scope |

Run `make secrets-sync` after both values exist. Never put either token in Compose,
`.env`, logs, Git, or chat messages.

## Allowlist and group setup

1. Start a private chat with the bot and send `/whoami`.
2. Put the returned user ID in `TELEGRAM_BRIDGE_ALLOWED_USER_IDS`.
3. Add the bot to the group, send `/whoami`, and put the negative group ID in
   `TELEGRAM_BRIDGE_ALLOWED_CHAT_IDS` to forward messages from every member.
4. If every ordinary group message should be delivered, use BotFather `/setprivacy`
   and disable privacy mode for this bot, then remove/re-add it to the group if
   Telegram does not apply the change immediately.

Comma-separated IDs are accepted. The server currently loads these non-secret
values from `deploy/compose/.env.example`.

## Development

```bash
cd /home/ubuntu/homehub/services/telegram-bridge
go test ./...

cd /home/ubuntu/homehub
docker compose \
  --env-file deploy/compose/.env.example \
  -f deploy/compose/compose.yaml \
  -f services/telegram-bridge/compose.homehub.yaml \
  up -d --build --wait telegram-bridge
```

The container uses host networking only to reach the loopback Mihomo proxy on
`127.0.0.1:1081`. Its health server binds only `127.0.0.1:8730`; it does not
register a Traefik route.
