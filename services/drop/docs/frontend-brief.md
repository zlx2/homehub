# Frontend brief

The frontend is a Vue 3 + TypeScript single-page interface built by Vite and embedded in the Go binary.

## Product rules

- Do not render a top navigation or brand bar. The page opens directly into the timeline.
- Keep refresh and connection state as small floating utilities, not navigation.
- Keep the composer fixed at the bottom and reachable with one hand on mobile.
- Attachment selection uses one icon-only `+` control. Do not add a paperclip or labelled add-file button.
- Expiry, device authorization, storage and traffic live behind the lower-left settings icon.
- Settings open as a vertical panel. Expiry choices are 24 hours, 3 days, 7 days and 30 days.
- Public guests manually opt into image previews. Owner/Tailnet sessions may lazy-load generated thumbnails.
- The visual system is quiet and content-first: neutral canvas, restrained blue accent, weak borders and soft shadows.
- Mobile is a first-class layout, not a compressed desktop view. Menus become bottom-safe sheets and cards use one attachment column.

## Upload interaction

- Validate 10-file, 500 MB per-file and 1 GB per-message limits before sending.
- Preserve text and selected files after network errors or user cancellation.
- Show distinct preparing, uploading and server-saving phases. Reaching 100% must not look stuck.
- One multipart request remains atomic. Resumable/chunked upload is a separate backend protocol decision.

## Source and build

- Source: `frontend/src`
- Production bundle: `internal/httpapi/web/app.js` and `app.css`
- Install: `pnpm install --frozen-lockfile`
- Type check: `pnpm run typecheck`
- Production build: `pnpm run build`

The generated assets are checked in so the deployment Docker build remains Go-only and works without npm network access. Do not edit generated assets directly.
