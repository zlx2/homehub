# HomeHub 后续路线

## P0

1. 监控根磁盘水位，仅清理可重建的 Docker 缓存和无引用镜像。
2. 为 AI Gateway 完成工作负载凭据签发，并验证 `ai.model.*` 权限的最小集合。

## P1

1. 实现只读 Host Metrics 服务，由 Control 聚合 CPU、内存、根磁盘和 uptime。
2. 为 `agent:hermes` 提供 IAM 凭据交换客户端；Hermes 运行时仍与 HomeHub 独立。
3. 建立最小 CI：Go/Rust 编译、React 构建、Compose 配置校验和 secret 扫描。

## P2

1. 添加 PostgreSQL 和 Drop 文件的备份/恢复演练。
2. 用 `services.yaml` 生成 IAM manifest、Control catalog 和 Traefik 路由，减少手工同步。
