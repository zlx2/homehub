# HomeHub Drop V2

Drop is a Go business microservice. PostgreSQL stores item metadata and the
service-owned file volume stores attachment bytes exactly as uploaded. Drop does
not resize, recompress, or transcode attachments.

Every non-health endpoint validates a short-lived `homehub-drop` token locally.
Permissions are `drop.item.create`, `drop.item.read`, `drop.item.list`, and
`drop.item.delete`; `system.root` implies all four. Drop has no user database,
cookie session, invitation flow, Tailscale identity, or permanent Hermes token.

The React Portal will provide the browser experience later. Workloads such as
Telegram Bridge exchange their own machine credential with IAM and send the
resulting audience-bound token to this API.
