# 错误处理规范

## 标准错误模型

对外可分类的错误使用 `ApplicationError`，定义在 `internal/pkg/errors/errors.go`。它携带 HTTP `Code`、稳定的机器可读 `Reason`、安全的 `Message`、可选 `Metadata`，并通过 `WithCause` 保留内部原因。`errors.Is` 按 code/reason 匹配，`errors.As` 和 `%w` 包装仍可工作。

```go
return infraerrors.Conflict(
    "AUTH_IDENTITY_OWNERSHIP_CONFLICT",
    "auth identity already belongs to another user",
).WithCause(err)
```

已有范例包括 `service.ErrTotpInvalidCode`、`service.ErrOpsDisabled`，以及 `admin_user.go` 中带 cause 的事务错误。

## 各层职责

### Repository

- 将 `ent.IsNotFound`、`sql.ErrNoRows` 和唯一约束映射成 service 的语义错误；复用 `repository/translatePersistenceError`。
- 无法分类的基础设施错误原样返回或用 `%w` 添加操作上下文，不能根据脆弱的完整错误字符串分支。
- 不在这里构造 Gin 响应，也不泄露 SQL、凭据或数据库拓扑。

### Service

- 校验业务前置条件并返回 `BadRequest`、`Forbidden`、`NotFound`、`Conflict`、`ServiceUnavailable` 等 `ApplicationError`。
- `Reason` 使用稳定的大写下划线标识符；前端可能据此选择流程，例如 `ADMIN_COMPLIANCE_ACK_REQUIRED`。
- 对未知下层错误使用 `fmt.Errorf("operation: %w", err)` 保留链；只有确实需要控制公开状态/文案时才转换为 `ApplicationError`。
- 内部 cause 供日志与调试使用，公开 message 不应包含 SQL、上游响应体、密钥或 token。

### Handler

- 请求绑定、path/query 解析失败可直接使用 `response.BadRequest` 等 helper；业务调用失败优先 `response.ErrorFrom(c, err)`。
- 成功统一使用 `response.Success`、`Created`、`Accepted` 或 `Paginated`，不要手写不同 envelope。
- 每次写入响应后立即 `return`，避免一个请求写两次。
- `response.ErrorFrom` 会把未知 error 降级为 `500` 和 `internal error`，并对服务端日志进行脱敏。

## HTTP 合约

标准 envelope 定义在 `internal/pkg/response/response.go`：

```json
{
  "code": 400,
  "message": "invalid input",
  "reason": "VALIDATION_ERROR",
  "metadata": {}
}
```

成功响应的 `code` 为 `0`，业务数据在 `data` 中。错误的 HTTP 状态与 body `code` 保持一致；`reason`/`metadata` 可省略。前端 `frontend/src/api/client.ts` 会解包成功 `data`，并保留错误的 `status`、`code`、`reason`、`message` 和 `metadata`。

## 清理与后台任务

- close、rollback、flush 等清理错误若无法作为主返回值传播，应显式处理或记录；不要无说明丢弃关键错误。
- goroutine/异步入口若可能 panic，应在拥有该任务生命周期的位置恢复、记录结构化错误并把任务标为失败；参见 `handler/image_task_handler.go`。
- context 取消和超时应沿调用链返回；不要包装后丢失 `errors.Is(err, context.Canceled)` 等语义。

## 测试与反例

- `internal/pkg/errors/errors_test.go` 覆盖包装、匹配、metadata 深拷贝和 HTTP 转换。
- `internal/pkg/response/response_test.go` 验证 envelope 与未知错误降级。
- handler 测试使用 `httptest.ResponseRecorder` 同时断言 HTTP 状态和 JSON body。

不要：把 `err.Error()` 原样作为 500 响应；在 handler 重复实现错误映射；丢掉 `%w`；用 panic 表达普通业务失败；为同一 reason 随意改变含义。
