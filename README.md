# HomeHub

HomeHub 是部署在个人服务器上的服务聚合平台：统一公网入口、统一身份与权限、统一首页，并让每个业务能力保持为可独立开发和部署的微服务。

当前版本是 V2 探索版。测试数据可以重建，目标优先级是快速迭代、边界清楚、后续容易增加服务，而不是复杂的高可用系统。

## 当前状态

- 公网入口：<https://zlx2.com>
- 代码仓库：`git@gitee.com:zlx23/homehub.git`（私有）
- 活跃分支：`codex/v2-architecture`
- 服务器工作区：`/home/ubuntu/homehub-v2`
- 部署方式：Docker Compose
- 公网链路：Cloudflare Tunnel → Traefik → IAM 前置鉴权 → Portal / 微服务

截至 2026-07-19，V2 正在运行：Traefik、Cloudflared、IAM、OpenFGA、Control、Portal、Drop、Telegram Bridge、PostgreSQL 和 ACME Challenge。Drop 已拥有独立的 React 页面；Portal 登录后首页提供服务状态与跳转入口。

Hermes Agent 仍是服务器上的独立系统。仓库中虽然保留 Hermes Web Terminal、Beszel 和 AI Gateway 的实现或配置，但它们尚未纳入当前 `compose.v2.yaml` 运行栈，不应视为已上线的 V2 服务。

完整盘点见 [当前项目状态](docs/current-state.md)。

## 技术栈

- Traefik 3.7：公网路由、TLS 和前置鉴权
- Cloudflare Tunnel：绕过源站域名访问限制并隐藏常规访问路径
- Go 1.26.5：IAM、Control、Drop、Telegram Bridge 等后端
- React 19 + TypeScript + Vite：Portal 与 Drop 独立前端
- PostgreSQL 18：持久数据，每个服务拥有独立数据库和数据库用户
- OpenFGA：关系型授权
- Ed25519 JWT：短期、受众绑定的内部身份令牌
- Bitwarden Secrets Manager：生产密钥来源
- Rust 1.97：预留给确有性能、内存或安全收益的业务服务，目前不是默认选择

## 目录

```text
apps/
  iam/                 身份、会话、通行密钥、授权和令牌签发
  control/             服务目录与健康状态聚合
  portal/              HomeHub 登录页与聚合首页
services/
  drop/                原始文件中转服务和独立 React 页面
  telegram-bridge/     Telegram → Drop 转发
  ai-gateway/          内部模型网关（代码存在，当前 V2 未运行）
  hermes-terminal-web/ Hermes 网页终端（代码存在，当前 V2 未运行）
packages/
  go-sdk/              Go 服务身份验证 SDK
  rust-sdk/            Rust 服务身份验证 SDK
deploy/
  compose/             V1 参考栈与当前 V2 Compose
  traefik-v2/          当前公网路由
  catalog/             Control 服务目录
  scripts/             部署、密钥和检查脚本
docs/                  架构、ADR、开发和运维文档
```

## 常用命令

命令应在服务器仓库根目录执行：

```sh
cd /home/ubuntu/homehub-v2

make v2-config          # 校验 Compose
make v2-up              # 构建并启动完整 V2
make v2-check           # 检查核心服务
make v2-logs            # 查看 V2 日志

make test-iam
make test-control
make test-drop
make test-telegram-bridge
make test-portal
```

生产密钥、证书、`.env.v2`、数据库和上传文件都不属于 Git 仓库。

## 文档入口

- [当前项目状态](docs/current-state.md)：现网组件、资源、路由、数据与已知问题
- [开发指南](docs/development-guide.md)：本地/服务器开发、测试、部署和新增服务流程
- [后续路线](docs/next-steps.md)：按优先级整理的待办与架构决策
- [V2 边界](docs/architecture/v2-boundaries.md)：IAM、Control、Portal 与业务服务职责
- [网络架构](docs/architecture/networking.md)：网络和端口约束
- [ADR 目录](docs/adr/)：已经做出的关键架构决策

## 不变的工程约束

- 公网 HTTP 流量统一进入 Traefik。
- IAM 是身份和权限事实来源；业务服务不信任客户端自行提供的身份头。
- 每个服务只访问自己的数据库和文件目录。
- Redis 只能用于缓存或临时队列，不能作为唯一持久数据源。
- 不向应用容器挂载不受限的 Docker Socket。
- 不引入 Kubernetes、Nacos、Service Mesh 或消息队列，除非先写 ADR 并证明当前方案不足。
- MySQL `42061` 与 Redis `38291` 是明确保留的既有公网端口，后续单独加固。
