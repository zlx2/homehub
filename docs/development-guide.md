# HomeHub V2 开发指南

本文用于下一位开发者或 Agent 在不了解历史对话的情况下继续工作。

## 1. 开发基线

- Monorepo，当前主工作区是服务器 `/home/ubuntu/homehub-v2`。
- 活跃分支是 `codex/v2-architecture`。
- Gitee 私有远端是 `git@gitee.com:zlx23/homehub.git`。
- 当前是探索阶段，允许短暂停机、直接重建容器和清除测试数据，但必须明确操作范围。
- 不要停止或改造仓库范围外的 MySQL、Redis、Hermes Agent 或其他既有容器。
- 不要读取、打印或提交 `.env.v2`、BWS Token、Bot Token、机器凭据、证书私钥和数据库内容。

推荐直接在服务器开发和验证，减少本地/服务器工具链差异。Windows 本地副本可用于查看与编辑，但最终必须保证服务器工作树、提交和 Gitee 一致。

## 2. 当前部署入口

```sh
cd /home/ubuntu/homehub-v2

# 仅校验配置
make v2-config

# 构建并启动完整 V2
make v2-up

# 健康检查
make v2-check

# 查看日志
make v2-logs
```

`make v2-up` 会构建多个 Go/React 镜像，开发单个组件时优先使用窄范围命令：

```sh
docker compose \
  --env-file deploy/compose/.env.v2 \
  -f deploy/compose/compose.v2.yaml \
  build drop portal

docker compose \
  --env-file deploy/compose/.env.v2 \
  -f deploy/compose/compose.v2.yaml \
  up -d --no-deps drop portal
```

不要使用 `deploy/compose/compose.yaml` 部署 V2；它属于旧架构和过渡期参考。

## 3. 测试矩阵

所有 Go/Rust/Node 版本由 Dockerfile 和 Makefile 固定，不要求宿主机安装完整工具链。

| 范围 | 命令 | 内容 |
| --- | --- | --- |
| IAM 单元测试 | `make test-iam` | 会话、令牌、身份和授权逻辑 |
| IAM 集成测试 | `make test-iam-integration` | 真实机器凭据换取短期 token |
| Control 单元测试 | `make test-control` | catalog 和 token 校验 |
| Control 集成测试 | `make test-control-integration` | audience 与权限隔离 |
| Portal | `make test-portal` | TypeScript 检查和 Vite 构建 |
| Drop | `make test-drop` | API、原始附件、权限、有效期 |
| Drop 集成测试 | `make test-drop-integration` | 实际上传、读取和删除原始文件 |
| Telegram Bridge | `make test-telegram-bridge` | allowlist、媒体和幂等逻辑 |
| Telegram 集成测试 | `make test-telegram-bridge-integration` | 只能创建、不能读取 Drop |
| AI Gateway | `make test-ai-gateway` | 路由、安全边界和流式响应 |
| Go SDK | `make test-sdk-go` | token 验证 SDK |
| Rust SDK | `make test-sdk-rust` | Rust token 验证 SDK |

涉及安全边界的修改至少应有单元测试；涉及 audience、scope、delegation 或机器身份的修改应再跑对应集成测试。

## 4. 前端开发

### Portal

路径：`apps/portal`

- React 19、TypeScript、Vite。
- 负责登录、Passkey、聚合首页、分享和安全设置。
- `/` 是管理员首页。
- `/drop/` 不由 Portal 渲染，Traefik 将其交给 Drop。
- 首页服务状态来自 Control `/api/control/v1/overview`。
- 首页服务器资源指标当前是静态占位值。

```sh
cd apps/portal
npm ci
npm run check
npm run build
```

### Drop

路径：`services/drop/frontend`

- Drop 是独立 React 页面，不应重新塞回 Portal。
- 生产构建由 `services/drop/Dockerfile` 完成，并嵌入 Go 二进制。
- API 基础路径是 `/drop/v1`；Traefik 去掉 `/drop` 后转发给 Go 服务。
- 前端 mutation 必须发送 HomeHub CSRF Cookie 对应的 `X-CSRF-Token`。
- 附件预览和下载都读取同一份原始文件；不要在前端或后端压缩替换。

```sh
cd services/drop
npm ci
npm run check
npm run build
```

## 5. 后端开发

默认使用 Go 1.26.5。仅当服务在内存、解析安全或 CPU 热点上有明确收益时使用 Rust 1.97。

每个服务应具备：

1. 稳定 `service_id` 和 token audience。
2. `<service>.<resource>.<action>` 权限集合。
3. `apps/iam/manifests/<service>.json` 服务清单。
4. OpenAPI 3.1 文档。
5. `/health/live` 和 `/health/ready`。
6. 本地验证 IAM JWKS、issuer、audience、expiry 和 permissions。
7. 自己的数据库、用户和迁移。
8. 结构化日志、request ID、超时和优雅退出。
9. Dockerfile、Compose 定义和 Control catalog 项。
10. 单元测试以及一条真实身份集成测试。

可用脚手架：

```sh
make new-service NAME=notes LANG=go VISIBILITY=owner
# 或 LANG=rust
```

脚手架只是起点。提交前必须检查生成的 audience、权限、网络、密钥挂载和路由是否符合真实需求。

## 6. 身份接入模式

### 浏览器访问业务服务

```text
Browser Cookie
  → Traefik
  → IAM /v1/edge/authorize
  → audience-bound Authorization token
  → business service local verification
```

业务服务不解析 HomeHub Session Cookie，也不信任客户端传来的 `Authorization` 或 `X-HomeHub-*` 头。Traefik 在 ForwardAuth 前清理/覆盖这些内容。

### Workload 调用另一个服务

```text
workload credential
  → IAM token exchange
  → short-lived target token
  → target service
```

机器凭据是长期身份材料，access token 是短期调用凭证，两者不能混用。每个 workload 只获取完成任务所需的最小关系和 permissions。

### Hermes / Agent

Hermes 设计上是 `agent:hermes`，可以拥有 `system.root`。但当前运行中的 Nous Research Hermes Agent 仍是独立系统。真正接入 V2 前应确认：

- 凭据由谁保存和轮换；
- 如何换取 audience-bound token；
- 直接操作与代表人类操作时如何保留 `act`；
- 哪些服务允许 `system.root`；
- 网页终端与 API 管家身份是否分开。

不要为了方便直接把 IAM 签名私钥或人类会话 Cookie 交给 Hermes。

## 7. 数据规则

- IAM、OpenFGA、Drop 使用独立逻辑数据库和用户。
- 业务服务不能直接查询另一个服务的数据库。
- Drop 附件在文件卷，元数据在 PostgreSQL；删除必须同时处理两者。
- Redis 只做缓存、限流或可丢失队列。
- 数据库迁移归服务自身所有，并在服务启动前/启动时明确执行。
- 当前是测试数据阶段，但任何会删除 `/srv/homehub-v2` 或 Docker volume 的命令仍应明确确认目标路径。

## 8. 密钥和配置

正常流程：

```sh
make install-bws
make secrets-sync
```

BWS 负责把 secret 原子化写入 `/srv/homehub-v2/runtime`。容器只挂载自己需要的文件。

禁止：

- 把 secret 写进 Compose、Git、日志或聊天记录；
- 在命令输出中打印 secret；
- 让多个无关服务共享同一机器凭据；
- 把短期 access token 当作永久设备令牌。

## 9. 常见诊断

```sh
# 查看状态
docker compose --env-file deploy/compose/.env.v2 \
  -f deploy/compose/compose.v2.yaml ps

# 单服务日志
docker compose --env-file deploy/compose/.env.v2 \
  -f deploy/compose/compose.v2.yaml logs --tail=200 drop

# 回环健康检查
curl -fsS http://127.0.0.1:18100/health/ready
curl -fsS http://127.0.0.1:18110/health/ready
curl -fsS http://127.0.0.1:18120/health/ready

# 公网入口（匿名访问受保护路由返回 401 是正常行为）
curl -kI https://zlx2.com/
curl -kI https://zlx2.com/drop/
```

Docker daemon 会给新容器注入指向宿主机 `127.0.0.1:1081` 的代理。普通 bridge 容器里的 `127.0.0.1` 不是宿主机，因此内部调用必须清空代理或正确设置 `NO_PROXY`。Telegram Bridge 使用 host network 是为了访问 Mihomo 回环代理。

## 10. 提交前清单

- [ ] 修改范围和服务边界清楚。
- [ ] 没有 secret、`.env`、证书、Token 或测试数据进入 diff。
- [ ] TypeScript/Go/Rust 检查通过。
- [ ] 安全敏感行为有测试。
- [ ] Compose 可以 `config --quiet`。
- [ ] 只重建需要的容器。
- [ ] 容器健康且公网/回环路由符合预期。
- [ ] 文档、catalog、manifest、OpenAPI 与代码一致。
- [ ] 服务器工作树干净，提交已推送 Gitee。
