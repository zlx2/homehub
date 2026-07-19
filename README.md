# HomeHub

HomeHub 是部署在个人服务器上的服务聚合平台：统一公网入口、统一身份与权限、统一首页，每个业务能力保持独立开发部署的微服务。

公网入口：<https://zlx2.com>（Cloudflare Tunnel → Traefik → IAM → 微服务）

## 技术栈

- Traefik 3.7：公网路由、TLS 和前置鉴权
- Cloudflare Tunnel：公网入口
- Go 1.26.5：IAM、Control、AI Gateway、Drop、Telegram Bridge
- React 19 + TypeScript + Vite：Portal 与 Drop 独立前端
- PostgreSQL 18：持久数据，每服务独立数据库和用户
- OpenFGA：关系型授权
- Ed25519 JWT：短期、受众绑定的内部身份令牌
- Bitwarden Secrets Manager：生产密钥来源

## 目录

```text
apps/
  iam/                 身份、会话、通行密钥、授权和令牌签发
  control/             服务状态聚合
  portal/              HomeHub 登录页与聚合首页
services/
  ai-gateway/          内网模型路由与 SSE 代理
  drop/                原始文件中转服务和独立 React 页面
  telegram-bridge/     Telegram → Drop 转发
packages/
  go-sdk/              Go 服务身份验证 SDK
  rust-sdk/            Rust 服务身份验证 SDK
deploy/
  compose/             Docker Compose
  traefik/             公网路由配置
  postgres/            数据库初始化脚本
  scripts/             部署与密钥管理脚本
docs/                  架构、ADR、开发和运维文档
services.yaml          统一服务元数据（IAM manifest / Control catalog / Traefik routes 的事实源）
```

## 常用命令

```sh
cd /home/ubuntu/homehub

make config              # 校验 Compose
make build               # 只编译 Go/Rust/React，不运行测试
make up                  # 构建并启动完整栈
make check               # 健康检查
make logs                # 查看日志
make down                # 停止服务

make test-iam
make test-control
make test-drop
make test-telegram-bridge
make test-portal
```

## 文档入口

- [开发指南](docs/development-guide.md)
- [当前状态](docs/current-state.md)
- [后续路线](docs/next-steps.md)
- [架构总览](docs/architecture/overview.md)
- [组件边界](docs/architecture/component-boundaries.md)
- [网络架构](docs/architecture/networking.md)
- [ADR 目录](docs/adr/)

## 工程约束

- 公网 HTTP 流量统一进入 Traefik → Cloudflare Tunnel。
- IAM 是身份和权限事实来源；业务服务不信任客户端提供的身份头。
- 每个服务只访问自己的数据库和文件目录。
- Redis 只用于缓存或临时队列。
- 不向应用容器挂载 Docker Socket。
- 不引入 Kubernetes / Nacos / Service Mesh / 消息队列，除非 ADR 证明必要。
- MySQL 42061 / Redis 38291 是保留的既有端口，独立加固。
