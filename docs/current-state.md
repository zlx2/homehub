# HomeHub 当前项目状态

> 快照日期：2026-07-19<br>
> 服务器：`VM-0-15-ubuntu` / Ubuntu 22.04 / x86_64<br>
> 活跃分支：`codex/v2-architecture`

## 1. 公网入口

```
浏览器 → zlx2.com / Cloudflare → Cloudflare Tunnel → Traefik :443 → IAM ForwardAuth → Portal / Drop
```

域名入口：`https://zlx2.com`。IP 直连路由已移除。Cloudflared 通过 BWS 管理的 token file 启动。

## 2. 运行组件

| 组件 | 定位 | 持久数据 | 公网路由 |
| --- | --- | --- | --- |
| Traefik | TLS、路由、ForwardAuth | 域名证书（只读） | `/`、`/api/*`、`/drop/*` |
| Cloudflared | Cloudflare Tunnel | Token（只读） | 无独立路由 |
| ACME Challenge | 证书续期 | ACME webroot | `/.well-known/acme-challenge/` |
| IAM | 用户、会话、Passkey、机器身份、令牌 | PostgreSQL | `/api/iam/*` |
| OpenFGA | 关系授权模型 | PostgreSQL | 仅内部 |
| Control | 服务状态聚合 | catalog 只读 | `/api/control/*` |
| Portal | 登录页和聚合首页 | 无 | `/` |
| Drop | 文本、原始文件 | PostgreSQL + `/srv/homehub-v2/data/drop` | `/drop/*` |
| Telegram Bridge | Telegram → Drop | 无 | 无 |
| PostgreSQL | IAM、OpenFGA、Drop 数据库 | Docker 卷 `homehub-v2-postgres` | 不公开 |

## 3. 服务端口（仅回环）

| 地址 | 用途 |
| --- | --- |
| `127.0.0.1:18080` | Portal |
| `127.0.0.1:18100` | IAM |
| `127.0.0.1:18101` | OpenFGA |
| `127.0.0.1:18110` | Control |
| `127.0.0.1:18120` | Drop |
| `127.0.0.1:18181` | Traefik admin |
| `127.0.0.1:8730` | Telegram Bridge health |

## 4. 服务元数据

所有服务信息集中在根目录 `services.yaml`。IAM manifest、Control catalog 从它派生。不在 services.yaml 中声明的服务不应存在于 Control catalog 或首页。

## 5. 数据与密钥

| 数据 | 位置 |
| --- | --- |
| IAM / OpenFGA / Drop 数据库 | PostgreSQL 18；独立逻辑库和用户 |
| PostgreSQL 文件 | Docker 卷 `homehub-v2-postgres` |
| Drop 附件 | `/srv/homehub-v2/data/drop` |
| 运行密钥 | `/srv/homehub-v2/runtime`（BWS 管理） |
| TLS 证书 | V2 runtime，只读挂载到 Traefik |

## 6. 与 HomeHub 并存的既有服务

- MySQL 8：`0.0.0.0:42061`（独立加固）
- Redis 7：`0.0.0.0:38291`（独立加固）

## 7. V1 已归档

V1 配置、Compose、Traefik、Beszel、Hermes Terminal、AI Gateway 代码已移入 `legacy/` 目录，不再维护。历史 ADR 仍保留在 `docs/adr/` 中以供参考。

## 8. 已知问题

1. 根磁盘使用率 86%，是近期最高优先级的运维风险。
2. Portal 首页服务器资源数据还是静态占位值。
3. Control `/v1/nodes` 仍返回空数组，节点监控未实现。
4. 没有 CI 流水线；测试依赖服务器上的 Make targets。
5. 数据为测试性质，无备份/恢复演练。
6. ADR 编号存在两个 `0011`。
7. `make new-service` 脚手架已禁用，等待基于 services.yaml 重写。
