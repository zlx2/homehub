# HomeHub Drop V2

Drop is an independently deployable HomeHub business service with a Go API and
its own React 19 frontend at `/drop/`.

PostgreSQL stores item metadata. The service-owned file volume stores attachment
bytes exactly as uploaded; Drop does not resize, recompress, or transcode files.

## Browser experience

The React application preserves the original Drop timeline layout:

- chronological message groups;
- fixed bottom composer;
- text, paste, drag/drop, and multi-file uploads;
- image preview, lightbox, video playback, and original downloads;
- 1/3/7-day expiry;
- copy, delete, storage status, and SSE refresh.

The frontend build runs in the Drop Dockerfile and is embedded into the final Go
binary. It is not a Portal feature module. Portal links to `/drop/`, and Traefik
routes the page and `/drop/v1/*` API requests to this service.

## Authorization

Every non-health API endpoint validates a short-lived `homehub-drop` token
locally. Permissions are:

- `drop.item.create`
- `drop.item.read`
- `drop.item.list`
- `drop.item.delete`

`system.root` implies all service permissions. Drop has no user database, login
form, permanent device token, or Hermes-specific bypass. Browser tokens come
from IAM ForwardAuth; workloads exchange their own machine credential with IAM.

Telegram Bridge has only `drop.item.create`, so it can forward content but
cannot list, read, modify, or delete Drop items.

## Development

```sh
cd /home/ubuntu/homehub-v2/services/drop
npm ci
npm run check
npm run build

cd /home/ubuntu/homehub-v2
make test-drop
make test-drop-integration
```

The production image runs the React build, Go tests, and Go binary build in
multi-stage Docker build steps.
