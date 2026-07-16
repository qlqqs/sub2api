# 后端错误处理规范

本项目有两套客户端错误契约：管理与站内 API 使用 `internal/pkg/response` 的统一 envelope；OpenAI、Anthropic、Gemini 网关端点保持各自协议格式，不能混用。

## 标准应用错误

`backend/internal/pkg/errors/errors.go` 定义 `ApplicationError` 与 `Status`；未知错误经 `FromError` 统一为安全的 500，原始错误只保留在不可序列化的 cause 中。

---

## Repository 翻译边界

`backend/internal/repository/error_translate.go:translatePersistenceError` 是持久化错误的统一翻译入口：`sql.ErrNoRows` 与 `ent.IsNotFound` 映射为调用方提供的 `notFound`；PostgreSQL `pq.Error` 的 `23505` 优先映射为 `conflict`，字符串匹配只作兼容回退；两类映射都用 `WithCause` 保留原错误。未命中的基础设施错误原样向上传播。

事务查询沿用同文件的 `clientFromContext` 获取 Ent 事务 client。不要在 Handler 或新增 Service 中判断 pq/Ent 具体错误。`backend/internal/service/sql_errors.go:isSQLNoRowsError` 是兼容辅助，不是绕过 Repository 翻译的新范式。

## Service 与网关错误链

Service 返回错误，不负责写管理 API 响应。输入或状态错误应尽早转换为 `ApplicationError`。例如 `backend/internal/service/ops_errors.go:OpsService.GetErrorTrend`、`GetErrorDistribution` 对缺少过滤器返回 `BadRequest`，对未注入 Repository 返回 `ServiceUnavailable`，其他 Repository 错误继续返回。

网关有独立的重试和提交规则：

- `backend/internal/service/openai_upstream_transport_error.go:handleOpenAIUpstreamTransportError` 区分 `context.Canceled`、持久网络故障和可 failover 故障；客户端取消不切换账号，其余传输故障返回 `UpstreamFailoverError`。
- `backend/internal/service/openai_gateway_upstream_errors.go:handleErrorResponse` 有界读取错误体，清洗上游消息，执行透传规则，记录 Ops 事件，再决定直接响应或 failover。
- Service 已写协议响应时必须调用 `MarkResponseCommitted`；Handler 检查该状态，避免二次兜底写入。
- `backend/internal/service/model_not_found_error.go` 结合状态码与正文分类上游模型不存在，不能把任意内部 404 混入该语义。
- 流式响应开始后不能再写普通 JSON；沿用 `backend/internal/handler/stream_error_event.go` 和 streaming-aware Handler 分支。

同一错误通常只由拥有最终响应、重试决策或失败语义的层记录一次，避免 Repository、Service、Handler 重复打印。

## HTTP 响应契约

`backend/internal/pkg/response/response.go:Response` 是管理 API envelope：`code`、`message`、可选 `reason`、`metadata`、`data`。`Success`/`Created` 返回 HTTP 200/201、`code: 0`、`message: "success"`；`Accepted` 返回 202 与 `message: "accepted"`。`Paginated` 将 `items/total/page/page_size/pages` 放入 `data`。

`ErrorFrom` 调用 `errors.ToHTTP`，只序列化 `Status`，绝不序列化 cause；nil 错误不写响应并返回 false。Handler 已有普通 `error` 时优先调用 `response.ErrorFrom`，不要重复实现 `errors.As` 与 envelope。

网关协议使用 `backend/internal/handler/gateway_handler.go:GatewayHandler.errorResponse`、OpenAI Handler helper 和透传规则输出兼容结构，例如 `{"error":{"type":...,"message":...}}`。修改时覆盖非流式、流式已开始、failover 耗尽、响应已提交；参考 `gateway_handler_error_fallback_test.go`、`stream_error_event_test.go`、`error_policy_integration_test.go`。

## Recovery、脱敏与禁止做法

- `response.ErrorFrom` 仅对最终 5xx 调用 `backend/internal/util/logredact.RedactText` 后记录内部错误；4xx 预期错误不在这里升级为 error 日志。
- 上游错误体只在配置允许时按最大字节数记录，并复用现有 sanitize/truncate 路径。不得直接记录请求体、Authorization、token、OAuth code 或 DSN。
- HTTP server 组装必须保留 Gin recovery 兜底；常规校验、数据库和网络失败使用 `error`，不用 `panic`。局部隔离 panic 可参考 `backend/internal/handler/admin/account_data.go`，且响应提交后不得再写 JSON。
- 不向客户端返回 `err.Error()`、SQL/Ent/pq 文本、堆栈、凭证或未清洗的上游正文。
- 不在 Repository 中依赖 Gin，不在普通 Service 中写管理 API response。
- 不在 `c.JSON` 后返回会触发二次写入的错误；Service 直接提交协议响应时设置 committed 标记。
- 新增映射应测试 `errors.Is/As` 链、HTTP 状态、reason/metadata、未知错误 500 脱敏，以及协议的流式与非流式格式。

## 上游余额查询错误

管理端 `POST /admin/accounts/:id/upstream-balance` 使用统一 envelope。协议不匹配在**明确平台**上应返回业务 data `status=unsupported`（成功 envelope），不要把内部 `errUpstreamBalanceProtocolMismatch` 泄漏为 500。凭证不足使用稳定 reason `UPSTREAM_BALANCE_CREDENTIAL_REQUIRED`，禁止零余额伪装。完整矩阵见 [上游账号余额查询](./upstream-balance.md)。
