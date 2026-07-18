---
name: telegram-drop-bridge
description: Use when deploying, configuring, diagnosing, or extending HomeHub's Telegram-to-Drop worker, including BotFather setup, allowlists, BWS secrets, polling, and attachment forwarding.
version: 1.0.0
author: HomeHub
license: MIT
metadata:
  hermes:
    tags: [homehub, telegram, drop, bot, deployment]
    related_skills: [homehub-housekeeper, drop-deployment]
---

# Telegram-to-Drop Bridge

## Overview

`telegram-bridge` receives Telegram Bot API updates through long polling and
forwards allowed messages to Drop. It is an internal worker with no public route.

Source of truth:

```text
/home/ubuntu/homehub/services/telegram-bridge/
```

## Setup

1. Obtain a bot token from `@BotFather` and store it in BWS as
   `telegram_bot_token`. Never paste it into logs, Git, Compose, or chat.
2. Create a HomeHub API token restricted to `drop.upload` and store it in BWS as
   `telegram_drop_token`. Do not use the Hermes `agent.root` token.
3. Run `cd /home/ubuntu/homehub && make secrets-sync`.
4. Start the service. With an empty allowlist it connects but forwards nothing.
5. Send `/whoami` privately and in the intended group, then configure the returned
   IDs in `deploy/compose/.env.example`.
6. To receive ordinary group messages, disable the bot's privacy mode through
   BotFather `/setprivacy`.

## Commands

```bash
cd /home/ubuntu/homehub
make test-telegram-bridge
make compose-config

docker compose \
  --env-file deploy/compose/.env.example \
  -f deploy/compose/compose.yaml \
  -f services/telegram-bridge/compose.homehub.yaml \
  up -d --build --wait telegram-bridge

docker compose \
  --env-file deploy/compose/.env.example \
  -f deploy/compose/compose.yaml \
  -f services/telegram-bridge/compose.homehub.yaml \
  logs --tail=200 telegram-bridge

curl -fsS -o /dev/null http://127.0.0.1:8730/health/ready
```

## Behavior

- Private acknowledgements are enabled by default; successful group messages are
  silent.
- A user ID allows that user in any chat. A chat ID allows every sender in that
  chat. Configure only the intended IDs.
- `/whoami`, `/start`, and `/help` are control commands and are not forwarded.
- Photos use Telegram's largest available rendition. Documents preserve the
  original Telegram file bytes and filename.
- Public Bot API downloads are limited to 20 MB.
- One Telegram message becomes one Drop item. Albums are separate items for now.
- Stable idempotency keys prevent duplicates during retries and restarts.

## Diagnosis

1. `401` from Drop: rotate or recreate the `drop.upload` HomeHub token, update BWS,
   and sync secrets.
2. Telegram polling error: verify Mihomo on `127.0.0.1:1081` and the bot token.
3. Bot sees commands but not group text: disable BotFather privacy mode and re-add
   the bot if needed.
4. Message ignored: check structured logs and allowed user/chat IDs; never log
   message bodies or tokens.
5. Duplicate concern: inspect the deterministic `chat_id + message_id` behavior
   before changing update acknowledgement order.

## Completion checklist

- [ ] Go tests and Compose validation pass
- [ ] Container is healthy
- [ ] Private text reaches Drop
- [ ] Group text reaches Drop after allowlisting
- [ ] A document reaches Drop with identical bytes
- [ ] No secret appears in process arguments, logs, Git, or output
