# 后端日志规范

主日志实现是 Zap，封装位于 `backend/internal/pkg/logger`。业务代码优先使用 `logger.FromContext(ctx)` 或 `logger.L()` 与 typed `zap.Field`；不要新建独立 logger，也不要在新代码中扩散 `log.Printf`。

## 初始化与桥接

`backend/internal/pkg/logger/logger.go:Init` 构造全局 Zap logger，支持 console/JSON、stdout/stderr 分流、文件轮转、采样、caller 与 stacktrace，基础字段包含 `service`、`env`。`bridgeSlogLocked` 和 `bridgeStdLogLocked` 分别承接 slog 与标准库日志；`LegacyPrintf` 只用于迁移历史 printf，新代码直接使用 Zap。

---

## 日志等级

- `Debug`：仅用于高频内部诊断，生产默认 `info` 时不输出；不得依赖 debug 日志完成审计。
- `Info`：成功完成的请求、启动/停止、配置生效和重要状态转换。HTTP access 日志固定为 Info。
- `Warn`：请求仍可处理但发生降级、重试、failover、队列压力或可恢复异常。例：`tempUnscheduleOpenAITransportError` 的稳定事件名。
- `Error`：当前操作最终失败、数据可能不一致或需要人工处置。预期的校验失败和普通 4xx 不使用 Error。
- `Fatal` 不用于库或请求路径；返回错误交给进程入口决定退出。stacktrace 默认从 error 级别开启，可由配置关闭或改为 fatal。

日志等级由 `logger.SetLevel` 动态调整；只接受 debug/info/warn/error。不要在调用点自行比较字符串等级。

## 结构化字段与请求关联

使用稳定事件名和 typed `zap.Field`，例如：

```go
logger.FromContext(ctx).Warn(
    "openai.account_temp_unscheduled_transport",
    zap.Int64("account_id", account.ID),
    zap.String("platform", account.Platform),
    zap.Time("until", until),
    zap.Error(err),
)
```

字段使用小写 snake_case。常用字段包括 `component`、`request_id`、`client_request_id`、`user_id`、`api_key_id`、`account_id`、`platform`、`model`、`status_code`、`upstream_status_code`、`latency_ms`。事件名用于聚合告警，不把变量拼入事件名。

`backend/internal/server/middleware/request_logger.go:RequestLogger` 接受或生成 `X-Request-ID`，写回响应头，将其放入 `ctxkey.RequestID`，并通过 `logger.IntoContext` 注入带 `component=http`、request IDs、path、method 的 logger。请求路径内必须优先使用 `logger.FromContext(c.Request.Context())`，不要退回无 request ID 的全局 logger。

`backend/internal/server/middleware/logger.go:Logger` 在请求结束记录 `http request completed`，字段包括 `component=http.access`、`status_code`、`latency_ms`、`client_ip`、`protocol`、`method`、`path`，以及可用的 `account_id`、`platform`、`model`；跳过 `/health` 与 `/setup/status`。若 `c.Errors` 非空，额外写 Warn。

## 应记录的事件

- 服务启动、配置装载、动态重配置、后台任务启动/停止和清理结果。
- 请求完成摘要，以及最终失败、重试、降级、failover、临时摘除账号等可运维状态转换。
- 日志消息描述发生了什么；筛选维度放字段。需要告警依赖的事件采用稳定点分名，例如 `openai.account_temp_unscheduled_transport`、`..._memory_only`、`..._failed`。
- 只在最接近最终决策的边界记录错误，使用 `zap.Error(err)` 保留类型化字段；不要让 Repository、Service、Handler 对同一失败各写一次。
- `logger.WriteSinkEvent` 绕过全局等级门控并写入 Ops sink，仅用于明确需要“可观测性入库”与控制台等级解耦的场景；普通业务日志不得用它规避等级配置。

## 脱敏与禁止记录

`backend/internal/util/logredact/redact.go` 是日志脱敏入口：

- `RedactMap` 递归复制并脱敏 map；`RedactJSON` 解析 JSON 后脱敏；`RedactText` 处理 JSON-like、query-like、plain key/value 文本，并遮盖 `GOCSPX-...` 与 `AIza...`。
- 默认敏感键包括 `authorization_code`、`code`、`code_verifier`、`access_token`、`refresh_token`、`id_token`、`client_secret`、`password`；调用方可传额外键。递归深度限制为 32。
- 非 JSON payload 交给 `RedactJSON` 时整体替换为 `<non-json payload redacted>`；不要因为解析失败而回退记录原文。

不得记录 Authorization/Cookie、API key/token、OAuth code/verifier、密码、client secret、私钥、完整 DSN、完整请求或响应 body。上游错误体只有在 `Gateway.LogUpstreamErrorBody` 明确开启时才能按 `LogUpstreamErrorBodyMaxBytes` 截断记录，并继续经过协议 sanitize/redact。

用户邮箱、客户端 IP、User-Agent、账号名等属于可识别信息，只在已有产品/运维场景需要且有访问控制时作为结构化字段记录；不得扩大到通用 debug dump。`backend/internal/service/ops_user_error.go:UserErrorRequest` 的白名单视图说明了用户侧可见字段边界，不能直接暴露内部 Ops 记录。

## 评审清单

- 请求日志是否从 context 获取 logger，并继承 `request_id`。
- 是否使用稳定事件名、snake_case typed 字段和正确等级。
- 是否只记录一次最终错误，且没有字符串拼接敏感对象。
- 是否在任何 body、URL query、header、错误文本入日志前调用现有脱敏/截断 helper。
- 新增桥接、request logger 或脱敏规则时，扩展 `backend/internal/pkg/logger/*_test.go`、`request_access_logger_test.go` 或 `backend/internal/util/logredact/redact_test.go`。
