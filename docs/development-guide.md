# HomeHub 开发指南

## 1. 开发基线

- Monorepo，服务器工作区 `/home/ubuntu/homehub-v2`。
- 活跃分支：`codex/v2-architecture`。
- Gitee 远端：`git@gitee.com:zlx23/homehub.git`。
- 探索阶段，允许短暂停机和重建容器，但必须明确操作范围。
- 不要停止或改造仓库外的 MySQL、Redis、Hermes Agent 或其它既有容器。
- 不要读取、打印或提交 `.env`、BWS Token、Bot Token、机器凭据、证书私钥和数据库内容。

推荐直接在服务器开发和验证。

## 2. 部署入口

```sh
cd /home/ubuntu/homehub-v2

make config     # 校验配置
make build      # 只编译，不运行测试
make up         # 构建并启动
make check      # 健康检查
make logs       # 日志
make down       # 停止
```

开发单个组件时用窄范围重建：

```sh
docker compose \
  --env-file deploy/compose/.env \
  -f deploy/compose/compose.yaml \
  build drop portal

docker compose \
  --env-file deploy/compose/.env \
  -f deploy/compose/compose.yaml \
  up -d --no-deps drop portal
```

## 3. 服务元数据

所有服务信息集中在一份 `services.yaml`。IAM manifest、Control catalog、Traefik routes 都从它派生。

新增服务时：
1. 在 `services.yaml` 中声明 id / audience / permissions / route / visibility / health。
2. 在 `apps/iam/manifests/` 创建对应 JSON manifest。
3. 在 `deploy/traefik/dynamic/routes.yaml` 添加路由规则。
4. 在 `deploy/compose/compose.yaml` 添加服务定义。
5. 创建 Dockerfile 和业务代码。
6. 更新 `apps/control/catalog.json`（与 services.yaml 保持一致）。

## 4. 测试

| 范围 | 命令 | 说明 |
| --- | --- | --- |
| IAM | `make test-iam` | 会话、令牌、身份、授权 |
| IAM 集成 | `make test-iam-integration` | 真实机器凭据换 token |
| Control | `make test-control` | catalog 与 token 校验 |
| Control 集成 | `make test-control-integration` | audience 与权限隔离 |
| Portal | `make test-portal` | TS 检查与构建 |
| Drop | `make test-drop` | API、附件、权限、有效期 |
| Drop 集成 | `make test-drop-integration` | 实际上传读取删除 |
| AI Gateway | `make build-ai-gateway` | 编译内网模型路由 |
| Telegram Bridge | `make test-telegram-bridge` | allowlist、幂等 |
| Telegram 集成 | `make test-telegram-bridge-integration` | 只能创建不能读 |
| Go SDK | `make test-sdk-go` | token 验证 SDK |
| Rust SDK | `make test-sdk-rust` | Rust token 验证 SDK |

安全边界改动至少要有单元测试；audience/scope/delegation/机器身份改动要跑对应集成测试。

## 5. 前端开发

### Portal (`apps/portal`)

React 19 + TypeScript + Vite。负责登录、Passkey、聚合首页。

```sh
cd apps/portal
npm ci && npm run check && npm run build
```

### Drop (`services/drop/frontend`)

独立 React 页面，Go 二进制内嵌。API 基路径 `/drop/v1`。

```sh
cd services/drop
npm ci && npm run check && npm run build
```

## 6. 后端开发

默认 Go 1.26.5。每个服务应有：

1. 稳定的 `service_id` 和 token audience。
2. `<service>.<resource>.<action>` 权限集合。
3. `apps/iam/manifests/<service>.json` 服务清单。
4. OpenAPI 3.1 文档。
5. `/health/live` 和 `/health/ready`。
6. 本地验证 IAM JWKS / issuer / audience / expiry / permissions。
7. 自己的数据库、用户和迁移。
8. 结构化日志、request ID、超时和优雅退出。
9. Dockerfile、Compose 定义、Control catalog 项。
10. 单元测试 + 真实身份集成测试。

## 7. 身份接入

### 浏览器访问

```
Browser Cookie → Traefik → IAM ForwardAuth → audience-bound token → 服务本地验签
```

业务服务不解析 HomeHub Session Cookie，不信任客户端传来的 Authorization 头。

### Workload 调用

```
workload credential → IAM token exchange → short-lived target token → target service
```

### Hermes / Agent

Hermes 设计上是 `agent:hermes`，拥有 `system.root`。当前 Hermes Agent 仍独立运行，接入 V2 前需确认凭据保存与交换方式。

## 8. 密钥和配置

```sh
make install-bws
make secrets-sync
```

BWS 将 secret 写入 `/srv/homehub-v2/runtime`。容器只挂载自己需要的文件。

禁止：
- 把 secret 写进 Compose、Git、日志或聊天记录。
- 在命令输出中打印 secret。
- 多服务共享同一个机器凭据。
- 把短期 access token 当永久令牌。

## 9. 诊断

```sh
docker compose --env-file deploy/compose/.env -f deploy/compose/compose.yaml ps
docker compose --env-file deploy/compose/.env -f deploy/compose/compose.yaml logs --tail=200 drop

curl -fsS http://127.0.0.1:18100/health/ready
curl -fsS http://127.0.0.1:18110/health/ready
curl -fsS http://127.0.0.1:18120/health/ready

curl -kI https://zlx2.com/
```

## 10. 提交前清单

- [ ] 修改范围和服务边界清楚。
- [ ] 没有 secret / .env / 证书 / Token / 测试数据进入 diff。
- [ ] TS/Go/Rust 检查通过。
- [ ] 安全敏感行为有测试。
- [ ] `make config` 通过。
- [ ] 文档、catalog、manifest、OpenAPI 与代码一致。
- [ ] 服务器工作树干净，提交已推送 Gitee。
