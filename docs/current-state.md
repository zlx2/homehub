# HomeHub 当前项目状态

> 快照日期：2026-07-19（Asia/Hong_Kong）<br>
> 服务器：`VM-0-15-ubuntu` / Ubuntu 22.04 / x86_64<br>
> 活跃分支：`codex/v2-architecture`

本文描述“现在真实运行的系统”，不是最终愿景。若本文与早期 ADR、V1 Compose 或历史对话冲突，以当前代码、`deploy/compose/compose.v2.yaml` 和现网检查结果为准。

## 1. 服务器资源

| 项目 | 当前值 | 备注 |
| --- | --- | --- |
| CPU | 4 vCPU | 足够支撑当前个人服务规模 |
| 内存 | 3.6 GiB，总可用约 2.5 GiB | 当前压力不高 |
| Swap | 4 GiB，已用约 1.2 GiB | 需要观察是否持续增长 |
| 根磁盘 | 40 GiB，已用 33 GiB（86%） | 当前最明确的资源风险 |
| 运行时间 | 约 11 天 | 快照时数值 |

磁盘剩余约 5.3 GiB。继续构建镜像前应定期清理可回收的 Build Cache 和废弃镜像，但不要误删 `/srv/homehub-v2`、数据库卷或用户上传文件。

## 2. 公网访问链路

```text
浏览器
  → zlx2.com / Cloudflare
  → Cloudflare Tunnel
  → Traefik :443
  → IAM ForwardAuth（受保护路由）
  → Portal / Control / Drop
```

当前域名入口是 `https://zlx2.com`。`www.zlx2.com` 在 Traefik 层重定向到主域名。Cloudflared 通过 token file 启动，不在仓库中保存 Tunnel Token。

Traefik 同时监听宿主机 `80/443`。V2 管理和服务调试端口仅绑定回环地址：

| 地址 | 用途 |
| --- | --- |
| `127.0.0.1:18080` | Portal |
| `127.0.0.1:18100` | IAM |
| `127.0.0.1:18101` | OpenFGA HTTP |
| `127.0.0.1:18110` | Control |
| `127.0.0.1:18120` | Drop |
| `127.0.0.1:18181` | Traefik 管理入口 |
| `127.0.0.1:8730` | Telegram Bridge 健康检查 |

## 3. 当前运行组件

2026-07-19 检查时，下列 V2 容器均处于运行状态，带健康检查的组件均为 healthy：

| 组件 | 定位 | 持久数据 | 公网路由 |
| --- | --- | --- | --- |
| Traefik | TLS、路由、ForwardAuth | 域名证书（只读挂载） | `/`、`/api/*`、`/drop/*` |
| Cloudflared | Cloudflare Tunnel 客户端 | Tunnel Token（只读挂载） | 无独立路由 |
| ACME Challenge | 证书续期挑战文件 | ACME webroot | `/.well-known/acme-challenge/` |
| IAM | 用户、会话、TOTP、Passkey、机器身份、令牌 | PostgreSQL + 签名/加密密钥 | `/api/iam/*` |
| OpenFGA | 关系授权模型与 tuples | PostgreSQL | 仅内部/回环 |
| Control | 服务目录和健康聚合 | 当前主要读取静态 catalog | `/api/control/*` |
| Portal | 登录页和聚合首页 | 无服务端持久数据 | `/` |
| Drop | 文本、原始文件、独立 React UI | PostgreSQL + `/srv/homehub-v2/data/drop` | `/drop/*` |
| Telegram Bridge | Telegram 消息转发到 Drop | 无业务数据库 | 无公网路由 |
| PostgreSQL | IAM、OpenFGA、Drop 数据库 | Docker 卷 `homehub-v2-postgres` | 不公开 |

### Portal

- React 19 + TypeScript + Vite。
- 未登录时提供用户名/密码/TOTP 和 Passkey 登录。
- 管理员登录后的 `/` 是服务聚合首页。
- 首页搜索和状态筛选已经可用。
- Drop 卡片进入独立的 `/drop/` 页面。
- 首页的 CPU、内存、磁盘、运行时间仍是第一版静态展示值，不是真实监控数据。

### Drop

- Go API + 独立 React 页面，不再依附 Portal 的简化表单。
- PostgreSQL 保存条目元数据；文件卷保存上传原始字节。
- 图片不压缩、不转码；下载仍是原文件。
- 支持文本、多个附件、1/3/7 天有效期、删除、存储状态、SSE 实时刷新。
- API 在本地验证 IAM 签发的 `homehub-drop` audience token。
- 主要权限：`drop.item.create/read/list/delete`。

### Telegram Bridge

- 使用 Telegram Bot API long polling，不需要公网 webhook。
- 允许的私聊/群组消息被转换为 Drop 条目。
- 机器身份只能换取 `drop.item.create`，不能读取、列举或删除 Drop 内容。
- Telegram 以“照片”发送的图片可能已被 Telegram 压缩；要求原图时应以文件发送。

## 4. 身份与权限现状

浏览器会话由 IAM 管理。Traefik 在把请求交给 Control 或 Drop 前调用 IAM 的 `/v1/edge/authorize`：

1. IAM 验证 HomeHub 会话或分享能力。
2. IAM 根据目标 audience 签发短期 Ed25519 访问令牌。
3. Traefik把令牌放入受信任的 `Authorization` 头。
4. 业务服务验证签名、issuer、audience、过期时间和权限。

稳定主体类型为：`human`、`guest`、`device`、`node`、`workload`、`agent`。

机器服务不复用人类 Cookie，也不保存长期 bearer access token。它们保存自己的机器凭据，仅在需要时向 IAM 换取短期、受众绑定的访问令牌。

Hermes 的“系统管家”身份和 `system.root` 设计已经有代码与 ADR，但当前 Hermes Agent 仍在 `~/.hermes` 独立运行，尚未作为 V2 Compose 服务接入现网链路。

## 5. 数据与密钥

| 数据 | 位置/所有者 |
| --- | --- |
| IAM / OpenFGA / Drop 数据库 | PostgreSQL 18；逻辑数据库和用户相互独立 |
| PostgreSQL 文件 | Docker 卷 `homehub-v2-postgres` |
| Drop 原始附件 | `/srv/homehub-v2/data/drop` |
| V2 运行密钥 | `/srv/homehub-v2/runtime` |
| Bitwarden 机器 Token | 宿主机受限文件，不进入仓库 |
| TLS 证书 | V2 runtime 与既有 ACME 目录，只读挂载到 Traefik |

生产 secret 的来源是 Bitwarden Secrets Manager。仓库只保存 secret 名称和挂载约定，不保存值。当前 `.env.v2` 仍承载 Compose 插值所需的数据库密码和非敏感配置，因此它必须继续保持 Git 忽略状态。

## 6. 与 V2 并存的既有服务

以下既有端口按用户要求继续公开，不属于 V2 PostgreSQL：

- MySQL 8：`0.0.0.0:42061 → 3306`
- Redis 7：`0.0.0.0:38291 → 6379`

它们应单独做访问控制和密码加固，不能误认为已受 HomeHub IAM 保护。

## 7. 代码存在但当前未上线的组件

| 组件 | 当前事实 |
| --- | --- |
| Hermes Web Terminal | 仓库有实现和旧部署脚本；不在 `compose.v2.yaml` |
| Beszel 服务器面板 | 仓库有 V1 Compose/配置；当前 V2 没有运行容器和路由 |
| AI Gateway | Go 服务、测试和 providers 配置存在；当前 V2 没有运行容器 |
| Remote Browser / Files | 首页概念阶段出现过，尚无 V2 服务实现 |

Control catalog 仍列出其中部分组件，因此首页可能显示“不可用”。后续应选择“正式接入”或“从当前 catalog 移除”，不要继续用静态状态掩盖真实运行情况。

## 8. 仓库和部署事实

- Gitee：`git@gitee.com:zlx23/homehub.git`
- 服务器仓库：`/home/ubuntu/homehub-v2`
- 当前活动分支：`codex/v2-architecture`
- 当前生效 Compose：`deploy/compose/compose.v2.yaml`
- 当前路由配置：`deploy/traefik-v2/`
- `deploy/compose/compose.yaml` 与 `deploy/traefik/` 主要是 V1/过渡期参考，不能和 V2 命令混用。

## 9. 已知问题

1. 根磁盘使用率 86%，是近期最高优先级的运维风险。
2. Portal 首页服务器资源数据还是静态值。
3. Catalog 与当前 V2 运行组件不完全一致。
4. Hermes、服务器面板、AI Gateway 尚未重新纳入 V2。
5. V1 和 V2 配置同时留在仓库，容易选错 Compose。
6. 没有正式 CI 流水线；测试依赖服务器上的 Make targets。
7. 当前数据仍为测试性质，尚未建立强制备份/恢复演练。
8. ADR 编号存在两个 `0011`，后续整理时应统一编号和链接。

下一阶段建议见 [next-steps.md](next-steps.md)。
