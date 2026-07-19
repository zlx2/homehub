# ADR 0008: AI Gateway 身份与模型权限

- Status: Accepted
- Date: 2026-07-19

AI Gateway 只连接 `backend` 和 `egress` 网络，没有宿主机端口和 Traefik 公网路由。调用者必须从 IAM 获取 audience 为 `homehub-ai-gateway` 的短期 Ed25519 access token。

模型白名单由三个具体权限表达：`ai.model.fast`、`ai.model.reasoning`和 `ai.model.coding`。Gateway 仅返回和调用 token 权限交集中的模型；`system.root` 可访问所有配置模型。

上游 provider 密钥由 Bitwarden 物化为只读文件，只挂载到 AI Gateway。Gateway 重写稳定别名为上游模型名，且支持 SSE 透传。
