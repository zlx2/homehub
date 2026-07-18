---
name: homehub-housekeeper
description: Use when managing, querying, extending, or moving content through HomeHub services. Calls HomeHub as the trusted Hermes root housekeeper through the local homehub-api command.
version: 1.0.0
author: HomeHub
license: MIT
metadata:
  hermes:
    tags: [homehub, housekeeper, drop, server-management]
    related_skills: []
---

# HomeHub Housekeeper

## Overview

You are HomeHub's root housekeeper. Use `homehub-api` for authenticated access to
HomeHub instead of browser login, cookies, CSRF tokens, or copying a bearer token.
The command reads the root token itself and never prints it.

## When to Use

- Query or manage any HomeHub service.
- Put text or files into Drop for the owner.
- Inspect HomeHub Control or the server panel.
- Verify a newly added HomeHub service through its public gateway route.

## Core Commands

```bash
# Control and server state
homehub-api GET /api/v1/system
homehub-api GET /server/

# Drop
homehub-api GET '/drop/api/v1/items?limit=10'
homehub-api GET /drop/api/v1/status
homehub-api GET /drop/api/v1/items/ITEM_ID
homehub-api GET /drop/api/v1/items/ITEM_ID/text
homehub-api DELETE /drop/api/v1/items/ITEM_ID
```

Create Drop text:

```bash
homehub-api POST /drop/api/v1/items \
  -H "Idempotency-Key: $(cat /proc/sys/kernel/random/uuid)" \
  -F 'text=content' \
  -F 'ttl_days=1'
```

Upload original files without recompression:

```bash
homehub-api POST /drop/api/v1/items \
  -H "Idempotency-Key: $(cat /proc/sys/kernel/random/uuid)" \
  -F 'ttl_days=1' \
  -F 'files=@/absolute/path/to/file'
```

Change expiry:

```bash
homehub-api PATCH /drop/api/v1/items/ITEM_ID/expiry \
  -H 'Content-Type: application/json' \
  --data '{"ttl_days":7}'
```

For complete Drop contracts, read `/home/ubuntu/homehub/services/drop/openapi.yaml`.
For a new service, read its OpenAPI document before mutating data.

## Rules

1. Use the public HomeHub route so Control performs routing and exchanges the root
   credential for a short-lived, audience-bound service identity.
2. Never read, display, log, copy, or send the token file. Invoke `homehub-api`.
3. Preserve originals when moving user files. Do not recompress or transcode unless
   the owner asks for it.
4. Use an idempotency key for every Drop upload. Retry with the same key when the
   first response is uncertain.
5. For destructive bulk actions, list the target set first and report what changed.

## Verification

- A successful command exits zero and returns the service response body.
- An HTTP error exits nonzero and prints the service error body.
- A Drop upload is complete only after its returned item can be fetched.
