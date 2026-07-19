# HomeHub 当前状态

> 2026-07-19 · `/home/ubuntu/homehub` · `main`

HomeHub 单栈：Cloudflare Tunnel → Traefik → IAM ForwardAuth → Portal / Control / Drop。

## 运行组件

| 组件 | 职责 | 公网路由 |
| --- | --- | --- |
| IAM + OpenFGA | 身份、会话、机器凭据、授权和签名 | `/api/iam/*` |
| Control | 服务目录和健康聚合 | `/api/control/*` |
| Portal | React 登录和聚合首页 | `/` |
| Drop | 文本和原始文件 | `/drop/*` |
| Telegram Bridge | Telegram → Drop | 无 |
| AI Gateway | 模型别名、权限白名单和 SSE 代理 | 无，仅内网 |

PostgreSQL 持久化 IAM、OpenFGA 和 Drop 独立数据库。运行密钥位于 `/srv/homehub/runtime`，Drop 文件位于 `/srv/homehub/data/drop`。

## 已知缺口

1. Control `/v1/nodes` 尚未接入宿主机指标。
2. 没有 CI，构建和集成验证仍由服务器 Make targets 执行。
3. 测试数据暂无备份和恢复演练。
