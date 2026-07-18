# Server environment

- Captured: 2026-07-19
- Host: `VM-0-15-ubuntu`
- User: `ubuntu`
- OS: Ubuntu 22.04, Linux 5.15
- Architecture: x86_64
- CPU: 4 vCPU
- Memory: 3.6 GiB total, approximately 2.5 GiB available at capture time
- Swap: 4.0 GiB total, approximately 1.2 GiB used
- Root filesystem: 40 GiB total, 33 GiB used, 5.3 GiB available (86% used)
- V2 repository: `/home/ubuntu/homehub-v2`
- V2 data root: `/srv/homehub-v2`
- Git branch: `codex/v2-architecture`
- Git remote: `git@gitee.com:zlx23/homehub.git` (private)
- Public URL: `https://zlx2.com`

## V2 loopback and public ports

| Port | Binding | Purpose |
| --- | --- | --- |
| 80 | public | Traefik HTTP redirect / ACME challenge |
| 443 | public | Traefik HTTPS origin |
| 8730 | loopback | Telegram Bridge health |
| 18080 | loopback | Portal |
| 18100 | loopback | IAM |
| 18101 | loopback | OpenFGA HTTP |
| 18110 | loopback | Control |
| 18120 | loopback | Drop |
| 18181 | loopback | Traefik admin/ping |

Existing public services outside V2 remain on MySQL `42061` and Redis `38291`.

## Runtime notes

- The active V2 deployment is `deploy/compose/compose.v2.yaml` with the ignored
  environment file `deploy/compose/.env.v2`.
- The Docker daemon may inject an HTTP proxy pointing at host
  `127.0.0.1:1081`. Bridge containers cannot use that address as the host unless
  they use host networking; otherwise internal names must be in `NO_PROXY` or
  proxy variables must be cleared.
- Production secrets are materialized from Bitwarden Secrets Manager into
  `/srv/homehub-v2/runtime`. Never print or commit those files.
- The root disk is the immediate capacity constraint. Inspect Docker build cache
  and unused images before large rebuild cycles.
