# Server environment

- Captured: 2026-07-17
- Host: VM-0-15-ubuntu
- User: ubuntu
- OS: Ubuntu 22.04.5 LTS
- Architecture: x86_64
- Memory: 3.6 GiB total, approximately 2.3 GiB available at capture time
- Swap: 4.0 GiB total
- Root filesystem: 40 GiB total, 28 GiB used, 9.6 GiB available
- Time synchronization: enabled
- Git: 2.34.1
- Docker Engine: 29.4.3
- Docker Compose: 5.1.3
- Repository path: `/home/ubuntu/homehub`
- Persistent data root: `/srv/homehub`
- Git remote: `git@gitee.com:zlx23/homehub.git` (private)

## Existing published or bound TCP ports

- 22: SSH, public
- 42061: MySQL, public IPv4 and IPv6
- 38291: Redis, public IPv4 and IPv6
- 443: existing service on Tailscale IPv4 and IPv6
- 8098: Kobold Lite on Tailscale IPv4
- 19877: RoleChat on localhost and Tailscale
- 8080-8082: Drop on localhost
- 8642: existing Hermes Agent gateway on localhost
- 9090: Mihomo admin on localhost
- 1080-1081: Mihomo proxies on localhost
- 39174 and 65092: existing Tailscale-bound listeners
- 2222: existing SSH tarpit listener

## Existing Docker networks

- bridge
- drop_default
- host
- kobold-lite_default
- none
- rolechat_default

## Tooling note

`curl`, `jq`, `openssl`, and `make` are available. The Bitwarden Secrets Manager
CLI was not found during the initial non-invasive check and will be installed or
integrated before production secrets are needed.

The Docker daemon injects an HTTP proxy pointing at `127.0.0.1:1081` into new
containers. Platform containers that call other containers must set `NO_PROXY`
for their internal service names because a container's loopback address is not
the host loopback address.
