# HomeHub 后续路线

本文不是承诺排期，而是从当前真实状态出发的优先级建议。

## P0：先处理运行风险

### 1. 磁盘空间

根磁盘已使用约 86%，剩余约 5.3 GiB。继续频繁 Docker 构建会快速消耗空间。

建议先做只读盘点：

```sh
docker system df
du -xh /home/ubuntu /srv/homehub-v2 2>/dev/null | sort -h | tail
```

确认后再清理“可重建”的 Build Cache 和无引用镜像。不要删除 PostgreSQL volume、Drop 数据目录、BWS runtime 或未知的用户目录。

## P1：让首页反映真实系统

### 2. 接入真实服务器指标

当前首页的 CPU、内存、磁盘和运行时间是静态占位值。建议增加一个只读 Node/Host Metrics 服务，或正式接回 Beszel，再由 Control 汇总最小指标给 Portal。

首版只需要：

- CPU 使用率；
- 内存使用率；
- 根磁盘使用率；
- uptime；
- Docker 容器健康数量。

不要把 Docker Socket 直接挂给 Portal 或普通业务容器。

### 3. 修正服务目录

Control catalog 目前包含未运行的 Hermes Terminal、Server Monitor 和 AI Gateway。应逐项决定：

- 纳入 `compose.v2.yaml` 并提供真实 health/route；或
- 暂时从 catalog 和首页移除。

首页只展示真实可用或明确“未部署”的状态，不再用静态文案代替服务发现。

## P1：完成核心能力闭环

### 4. Hermes 正式接入 V2

目标不是迁移或改写 `~/.hermes`，而是让独立 Hermes Agent 以 `agent:hermes` 身份安全调用 HomeHub 服务。

需要先写清：

- 机器凭据的保存和轮换；
- `system.root` 的解释和服务端实现；
- 直接行为与人类委托行为的审计；
- 网页终端是否重新上线；
- Agent 调 Drop、AI Gateway 和未来服务的统一 SDK/Skill。

### 5. AI Gateway 决定是否上线

代码与测试已存在，但当前 V2 未运行。若继续：

1. 加入 `compose.v2.yaml` 的 backend/egress 网络；
2. 用 BWS 挂载 DeepSeek 和 OpenCode Go 密钥；
3. 为 Hermes/业务服务签发 `ai.use` 及模型 allowlist；
4. 不开放公网通用代理入口；
5. 增加实际 provider 健康与额度错误展示。

如果短期没有消费者，则保持代码不运行，避免首页假装它在线。

## P2：开发体验和维护

### 6. 收敛 V1/V2 文件

当前 `compose.yaml` 与 `compose.v2.yaml` 并存，ADR 也存在重复编号。系统稳定后应：

- 把 V1 配置移动到明确的 `legacy/` 或历史分支；
- 将 V2 Compose 改为默认名称；
- 修复重复 ADR 编号和链接；
- 删除已经失效的说明与脚本。

在此之前，所有文档和命令必须明确写出 V2 文件名。

### 7. CI

建议 Gitee 流水线至少执行：

- Go 单元测试；
- React TypeScript 检查与构建；
- Rust SDK 测试；
- `docker compose config --quiet`；
- secret/大文件扫描。

真实集成测试仍可留在服务器手动执行，因为它依赖 BWS materialized credentials。

### 8. 备份与恢复

当前数据仍是测试性质，可以继续快速重建。在正式保存不可丢失内容前，再增加：

- PostgreSQL 定时逻辑备份；
- Drop 元数据与附件一致性备份；
- 恢复演练；
- 保留策略和磁盘水位告警。

不要为了“看起来完整”提前引入复杂备份平台。

## 暂不引入

现阶段没有证据需要：

- Kubernetes；
- Nacos；
- Service Mesh；
- RabbitMQ/Kafka；
- 多节点 PostgreSQL；
- 独立 API Gateway 产品替代 Traefik；
- 每个服务一套登录系统。

当某个具体问题无法由 Docker Compose、Traefik、IAM、REST/SSE 和 PostgreSQL 解决时，再通过 ADR 评估新基础设施。
