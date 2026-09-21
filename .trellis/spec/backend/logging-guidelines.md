# 日志规范

## 日志设施

统一日志入口是 `internal/pkg/logger`，底层为 zap。它支持 console/JSON、stdout/stderr、文件轮转、采样和动态级别，并把标准库 `log` 与 `slog` 桥接到同一输出。新代码优先使用：

```go
logger.L().Error("image_task.complete_store_failed",
    zap.String("task_id", taskID),
    zap.Error(err),
)
```

HTTP handler 优先通过 `requestLogger(c, component, fields...)` 或 `logger.FromContext(ctx)` 取得请求 logger。`server/middleware/request_logger.go` 已注入 `request_id`、`client_request_id`、`path` 和 `method`，不要手工生成第二套 request ID。

## 级别

- `Debug`：高频诊断、选择过程或开发排查信息，生产环境通常关闭。
- `Info`：启动、配置完成、后台任务状态转换等正常但有运维价值的事件。
- `Warn`：可恢复降级、重试、可选依赖不可用或清理失败，主流程仍可继续。
- `Error`：请求/任务失败、状态无法持久化、数据一致性风险或不可恢复的后台错误。
- 不在业务路径调用 `Fatal` 或 panic；由入口返回启动错误并执行清理。

同一个错误只在真正拥有处理/降级决定的边界记录，避免 service、handler、middleware 层层重复打印。

## 结构化格式

- message 使用稳定、简短的事件名，现有新代码采用点分形式，如 `image_task.execution_panicked`、`grok_media.record_usage_failed`。
- 字段名使用 `snake_case`，选择稳定标识与上下文：`user_id`、`account_id`、`group_id`、`model`、耗时、状态。
- 通过 `zap.String`、`zap.Int64`、`zap.Duration`、`zap.Error` 等类型化字段记录，不把所有上下文拼进 message。
- component 表示代码/业务区域，例如 `handler.openai_gateway.grok_media`；请求日志已有 `component=http`，子流程可覆盖成更具体值。
- 不记录完整请求/响应体，除非确有诊断需要且已限制大小并脱敏。

`internal/handler/grok_media.go` 展示带业务标识的结构化错误；`internal/server/middleware/request_logger.go` 和 `internal/handler/logging.go` 展示请求上下文传播。

## 敏感信息

禁止输出明文 Authorization、API key、密码、OAuth code/verifier、access/refresh/id token、client secret、cookie、完整凭据 JSON 或支付敏感数据。

确需记录外部 payload 时先使用 `internal/util/logredact`：

- `RedactMap` 用于结构化 map；
- `RedactJSON` 用于 JSON bytes；
- `RedactText` 仅是非结构化文本兜底，可通过额外 key 扩展。

`repository/claude_oauth_service.go` 对响应 body 调用 `RedactJSON`，`response.ErrorFrom` 对内部错误文本调用 `RedactText`。脱敏 helper 不是允许随意记录 payload 的理由。

## Legacy 边界

仓库仍有 `logger.LegacyPrintf(component, ...)`，用于逐步迁移旧的 printf 日志；它会推断级别并标记 `legacy_printf=true`。修改已有 legacy 区域时可以保持局部一致，但新功能应使用 zap 字段。不要新增裸 `fmt.Printf`/`log.Printf` 作为应用日志，也不要依赖 message 文本推断级别。

## 检查清单

- 错误日志包含 `zap.Error(err)` 和足够定位对象的非敏感字段。
- 请求相关日志从 context 取得 logger，保留 request ID。
- 高频循环没有默认 Info/Error 洪泛；必要时使用 Debug、聚合或现有采样机制。
- 日志文案不承诺未发生的成功，也不把预期客户端校验失败提升成服务故障。
- 新的敏感字段加入脱敏测试；参考 `internal/util/logredact/redact_test.go` 和 `internal/pkg/logger/*_test.go`。
