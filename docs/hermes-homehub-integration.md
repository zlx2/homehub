# Hermes 与 HomeHub

Hermes Agent 是仓库外的独立宿主机系统，HomeHub Compose 不启动、停止或修改它，也不将其网页终端纳入平台。

未来接入只使用 IAM：

```text
agent:hermes 机器凭据
  -> IAM /v1/tokens/exchange
  -> 短期、audience-bound access token
  -> Control / Drop / AI Gateway
```

Hermes 拥有保留权限 `system.root`，但仍必须为每个目标 audience 交换短期 token。人类委托请求必须在 `act` 中保留实际执行者。机器凭据只能由 Bitwarden 物化到 Git 之外的受限文件。

当前仓库不包含可直接安装的 Hermes API wrapper；完成凭据轮换、目标 audience 选择和审计合约后再实现。
