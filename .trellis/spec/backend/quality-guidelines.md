# 后端质量规范

## 必须保持的约束

- 使用 Go 1.26 module 语义和仓库已有依赖，不为小功能引入重型框架。
- 所有 Go 改动经过 `gofmt`。`backend/.golangci.yml` 启用 `depguard`、`errcheck`、`gosec`、`govet`、`ineffassign`、`staticcheck` 和 `unused`。
- handler 不依赖 repository/数据库客户端；service 通过自己定义的最小接口依赖基础设施。
- 公共方法接收的 `context.Context` 沿 repository、网络和缓存调用传播。
- 错误增加动作上下文时使用 `%w`；API 错误和日志遵守本目录相应规范。
- 修改 interface 后搜索所有实现、编译期断言、测试 stub 和 mock；`DEV_GUIDE.md` 将漏改 stub 列为已发生过的问题。
- 修改 Ent schema 或 Wire provider 后运行 `make generate`，审查并提交 `ent/`/`wire_gen.go` 的生成差异。

## 测试策略

测试与被测代码同 package/目录，主要使用标准 `testing` 和 `testify/require`。

| 类型 | 方式 | 代表文件 |
|------|------|----------|
| 纯逻辑单元测试 | 表驱动、stub/fake、`//go:build unit`（部分基础包无 tag） | `internal/pkg/errors/errors_test.go`, `internal/service/proxy_test.go` |
| HTTP handler/middleware | Gin test mode + `httptest`，断言状态和 body | `internal/handler/auth_wechat_oauth_test.go`, `internal/middleware/rate_limiter_test.go` |
| Repository 集成 | `//go:build integration`，真实 PostgreSQL/Redis/testcontainers | `internal/repository/user_repo_integration_test.go` |
| 迁移 | sqlmock 验证 runner，集成测试验证最终 schema/数据 | `migrations_runner_notx_test.go`, `migrations_schema_integration_test.go` |
| 跨模块 E2E | `//go:build e2e` | `internal/integration/e2e_user_flow_test.go` |

新增行为至少覆盖成功、输入/权限失败和关键下游失败。并发、幂等、缓存或事务改动还要覆盖重复调用、取消/回滚和竞争边界。优先使用可控时钟、channel 或显式状态，避免依赖脆弱的短时间 `Sleep`。

## 验证命令

从 `backend/` 运行：

```bash
go test ./...
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...
go build ./cmd/server
```

可先用 `go test -run TestName ./internal/<package>` 快速迭代，但交付前扩大到受影响 package；数据库、路由、共享接口或生成代码的改动应运行全量检查。`Makefile` 也提供 `make test-unit`、`make test-integration` 和 `make test-e2e-local`。

## 审查清单

- 层次和依赖方向没有被绕过，接口放在使用者一侧。
- handler 只处理协议，业务不变量在 service，持久化细节在 repository。
- 事务内所有操作使用同一 tx context/client，失败路径可回滚。
- 查询有界且避免明显 N+1；批量路径、分页和空输入行为明确。
- goroutine、ticker、HTTP body、rows、事务和其他资源都有停止/关闭路径。
- 用户输入、动态 URL、SQL 和日志字段经过对应的验证、参数化或脱敏。
- 生成差异与 schema/provider 改动一致，没有手工修改生成文件。
- 测试断言可观察行为，而不是只验证 mock 调用；失败消息足以定位原因。

## 禁止复制的模式

- 忽略关键返回错误、用 `_` 吞掉提交/写入失败，或用 panic 处理普通失败。
- 在新代码中继续扩散 `LegacyPrintf`、裸标准日志或非结构化大 payload。
- 为绕过 lint 在 handler/service 直接引入数据库实现。
- 修改历史迁移，或只改 schema 不更新迁移/生成代码。
- 在测试中依赖共享外部状态、未清理全局 logger/timer，或因本机无 Docker 就宣称集成测试通过。

## Gin 同级路由参数兼容

- **适用范围**：合并上游或新增同一 HTTP method 的同级动态路由时。
- **签名**：邀请管理路由统一使用 `/api/v1/admin/affiliates/users/:user_id`；补绑为 `POST .../:user_id/inviter`，提现为 `POST .../:user_id/withdraw`。
- **合约**：handler 读取 `c.Param("user_id")`；审计 action override 使用相同模板，补绑动作仍为 `admin.affiliate.inviter.bind`，响应字段仍为 `invitee_id`。
- **错误边界**：同级动态段名称不一致会使 Gin 在注册时 panic，而不是返回 HTTP 错误；合法路由应能同时注册并解析用户 ID。
- **正反例**：同为 `:user_id` 可共存；单独注册补绑无法覆盖共存问题；补绑 `:invitee_id` 与提现 `:user_id` 混用是错误模式。
- **必要测试**：`internal/server/routes/affiliate_routes_test.go` 同时注册补绑/提现并验证匹配路径和 `user_id`；handler 和审计测试分别验证认证操作者与审计参数。
- **错误与正确**：不要只消除文本冲突并保留不同 wildcard；应统一路由、handler、审计和测试中的参数名，不改变客户端实际 URL。

## 计费倍率解析（分组模型倍率，fork）

- **适用范围**：新增或修改任何「乘倍率」的路径：网关结算、报价、按响应模型重算、预扣冻结、批量生图、利润门阈值、利润预览、模型广场展示。任务 `10-23-group-model-rate`。
- **签名**：`resolveBaseRate(ctx, rates *GroupModelRateService, groupID int64, model string, fallback func() float64) float64`（`service/group_model_rate.go`）；`(*GroupModelRateService).Match(ctx, groupID, model) (float64, bool)`；表 `group_model_rate_multipliers`（迁移 253）。
- **合约**：基础倍率 = 分组模型倍率（按**计费模型**、`normalizeChannelPricingModelName` 后匹配：精确优先，其次前缀最长的末尾 `*`）?? `fallback()`（原「用户专属 ?? 分组倍率」）。高峰因子、渠道分时、生图 / 视频独立倍率在基础倍率之外叠加；独立倍率开启时独立倍率优先。`usage_logs.rate_multiplier` 写入实际采用的倍率。
- **错误边界**：服务未注入（nil）或规则快照从未加载成功时一律视为未命中，行为与无此功能时相同；加载失败沿用旧快照并 warn。管理端 PUT 校验失败返回 400、reason `GROUP_MODEL_RATE_INVALID`（`*` 只能在末尾且前面至少一个字符、归一化后不重复、0 < 倍率 ≤ 10000、单组 ≤ 200 条）。
- **正反例**：命中规则 → 不调用 `ResolveUserGroupRateMultiplier`；未命中 → 经 `fallback` 原样调用。按响应模型重算时两侧倍率可能不同，`responseModelBillingAdoptable` 还要比较实扣金额（`responseModelBillingActualCostAdoptable`）。预扣冻结按候选模型逐个算；免费分组只有候选模型命中 > 0 的规则时才预扣。利润门复用键为「分组 + 模型」。
- **必要测试**：`group_model_rate_test.go`（匹配、校验、快照失效）、`group_model_rate_billing_test.go`（命中时不调用专属解析、同一用户 A→B 不串值、生图独立倍率开 / 关、响应模型实扣比较、预扣逐候选、利润门阈值与复用）；本机 e2e `p7_model_rate.py`（MRT 组）。
- **错误与正确**：不要把模型倍率当默认值传给 `ResolveUserGroupRateMultiplier`——它会把传入的默认值缓存在 `user:group` 键下，污染该用户在同组其他模型的倍率；应先 `Match`，未命中再走原解析（即 `resolveBaseRate` 的 `fallback`）。

## HTTP 网关响应模型名（fork）

- **适用范围**：网关 POST 成功响应（JSON / SSE），包括根路径别名、Codex HTTP 与 Gemini/Antigravity；WebSocket 帧不经过此 HTTP 中间件。
- **签名**：`handler.ClientResponseModelMiddleware() gin.HandlerFunc`；`setOpsRequestContext` 首次收到非空模型时调用 `setClientResponseModel`，合成路由优先使用 `RequestedPublicModelFromContext`。
- **合约**：仅在客户端写出边界将已存在的字符串 `model`、`response.model`、`message.model`、`modelVersion`、`response.modelVersion` 还原为请求模型；Gemini JSON 数组处理各项的 `modelVersion`。不递归替换正文、工具参数或 metadata，不新增模型字段。上游观察、路由、计费和 `usage_logs.upstream_response_model` 保持原样。
- **校验与错误边界**：非 2xx、无请求模型、非 JSON/SSE 和压缩字节原样转发；无效 JSON 或非字符串字段不改；写出错误保留。模型字节长度可能改变，写头前移除原 Content-Length。SSE 跨 Write 的半行暂存，但完整事件、注释心跳及 JSON 空白心跳不能等待请求结束才发送。
- **正反例**：请求 `public`、上游返回 `vendor/version` → 客户端模型为 `public`，内部仍记录 `vendor/version`；即使请求没有配置模型映射也要还原。GET 模型列表与错误消息不改。
- **必要测试**：`client_response_model_test.go` 覆盖协议字段、精确保留其他字节、合成路由、分片/SSE/心跳、错误与压缩绕过；路由覆盖见 `TestGatewayRoutesClientResponseModelMountedOnEveryHTTPChain`。本机模拟上游还需验证客户端模型与 usage 原始上游模型分离。
- **错误与正确**：不要改写上游响应后再送入计费观察器，也不要仅以“请求模型与映射模型不同”作为还原条件；应在所有解析/观察之后的 HTTP 写出边界处理。
