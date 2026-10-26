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

## 嵌入对话附件请求体（2026-10-25）

- **Scope / Trigger**：修改 `/_chat` 反代大小限制或 fork 的附件处理时。
- **Signatures**：`server.limitChatBody(*gin.Context) bool`；fork `src/lib/embed/image-upload.ts::{readImageDataURL,imageDataURLToFile,canvasImageFile}`。
- **Contracts**：`/_chat/api/v1/files/*` 请求体上限 26 MiB = 默认 25 MiB 文件 + 1 MiB multipart 余量；其他请求 5 MiB。fork `rag.file.max_size` 保持 25 MiB，两个上限不可混淆。严格 `connect-src 'self'` 不变。
- **Validation / Errors**：已知/未知 Content-Length 同样有界读取；超限返回 413 `BODY_TOO_LARGE` 且不得触达上游。文件本身 >25 MiB 即便请求 <26 MiB 也由 fork 返回 413。FileReader error/abort 或编码失败须给上传失败提示，不留未处理 Promise。
- **Good / Base / Bad**：25 MiB 文件附 multipart 头允许；普通 5 MiB 请求允许；26 MiB+1 上传请求拒绝。无压缩图片原 File 复用；压缩结果 MIME 来自实际编码；截图直接 toBlob。
- **Tests Required**：`TestChatProxyMultipartFileLimit` 覆盖实际 multipart 文件大小与已知/未知长度；`TestChatProxyLocationCookiesAndBodyLimits` 覆盖请求边界和其他路由不变。浏览器覆盖图片选择/粘贴/拖拽、压缩、临时聊天、发送回显与失败重试，捕获 CSP 违规；只测文件 API 不能证明图片选择流程可用。
- **Wrong vs Correct**：错：把请求上限和文件上限都设为 25 MiB，或为修复 fetch(data:) 开放 CSP；对：为封装留有界余量，并在浏览器本地转换数据，不做虚假的网络请求。

## 财务明细与账号成本快照（fork，2026-10-25）

1. **范围**：`finance*.go` 的收入重放/财务查询、支付费率快照、`account_model_cost.go` 的模型成本覆盖；不改变真实用户余额扣费、转发和调度。
2. **签名与存储**：`ReplayFinanceRevenue(events []FinanceEvent, actual decimal.Decimal) FinanceRevenue` 是纯函数；`FinanceRepository.Load(ctx, FinanceRange)` 用只读 REPEATABLE READ 取全部收入事件和区间成本；迁移 254/255 为原生 SQL 自有表，256 的支付 numeric 字段使用 Ent String 映射保证字符串 API 与精度。
3. **合约**：API 在 `/api/v1/admin/finance`，完整 DTO 见任务 `10-25-finance-ledger/contract.md`。金额字符串、北京时间闭区间、最大 732 天；免费优先，再付费/分润 FIFO；管理员扣回不确认收入。历史费率 NULL 按实例当前费率，退款扣回 NULL 按 refund_amount；显式 0 不能当 NULL。管理写接口必须挂既有审计中间件。
4. **校验与错误**：费用 >0、最多 2 位小数，摊销 1–3660 天；`FINANCE_EXPENSE_INVALID`/`FINANCE_RANGE_TOO_LARGE` 为 400，缺失支出为 404。账号模型倍率 0–10000、最多 4 位、每账号最多 200 条，缺少 rate_multiplier 不得默认为 0；失败 `ACCOUNT_MODEL_COST_INVALID`。
5. **正例/基线/反例**：上游模型命中规则仅替换成本快照；无配置时逐位等于 `account.BillingRateMultiplier()`；退款先减少该订单剩余现金，超额退款在退款日冲减收入，不能串扣其他订单现金。推断免费期初必须同时保留实际−已登记净流量差额，不能伪造对账平衡。
6. **回归点**：`account_model_cost_test.go` 覆盖两个网关的真实用量快照及 batch 的价格/冻结不变；`finance_revenue_test.go` 覆盖免费优先、跨天 FIFO、退款、扣回、分润、透支、期初与摊销尾差；`finance_repo_integration_test.go` 用 testcontainers 验证 SQL/decimal/迁移/退款；`finance_audit_test.go` 验证支出写审计。
7. **错误与正确**：批量生图的旧局部 `accountMultiplier` 还参与售价与冻结，直接把它改成模型成本倍率会误改客户扣费；应保留原售价公式，只覆盖 `BatchImagePricingSnapshot.AccountRateMultiplier`。`settings` 表没有 `created_at`，原生 upsert 只写 key/value/updated_at。报表可不缓存以避免补录/退款水位遗漏，但不得用 float64 累加金额或把已清理的历史数据当完整账本。

8. **成本方式标签回归**：账号 breakdown 的 cost_mode 只看当前配置，优先级 fixed > per_image > model_rate > rate；按张/次需同一自定义统计规则覆盖账号或其当前所属任一分组并含 image/per_request 定价。不能以图片流量占比或历史用量 group_id 代替配置/归属判断，标签变化不得重算历史快照金额。`TestFinanceBreakdownCostModeUsesConfiguration` 与 `TestFinanceCostModeCurrentPricingConfiguration` 分别覆盖优先级和真实 SQL 匹配。

## 历史成本重算与显式现存明细基线（fork，2026-10-26）

1. **范围/执行前提**：仅账号成本快照及支付通道费率，复用当前配置；运维先停止请求并等在途结束、备份，再预览/确认、核对、恢复。不是在线重算产品，不修改余额、实际扣费/退款、分润或运行时配额，不提供撤销。
2. **签名/存储**：`POST /admin/finance/recalculations` 的可选 `accept_retained_baseline` 默认为 false，仅 `usage_cost` 可用；独立确认仍须 `acknowledged:true` 和预览指纹。迁移257的 `filter` 和 `config_snapshot` 保存接受选择及白名单 `FinanceRetainedBaseline`，含版本/时区/删除版本、整日来源数和摘要、旧小时/日桶；公开 DTO 不泄露原始快照。
3. **合约**：账号/模型筛选只限定改动记录，基线核对必须包含受影响整天的所有账号。最多31个涉及日期、整日总计2000条来源；超限不截断。不接受时旧日期仍 `unproven_aggregate_coverage` 跳过。接受是账务口径，不是从行数证明从未清理。原值/NULL、规则和新旧差额均留审计；迁移/启动不自动重算。
4. **校验/错误**：缺失非空桶、请求数/金额不符返回 `FINANCE_RECALCULATION_COVERAGE_UNKNOWN`；规则/来源漂移拒绝并要求新预览。小时金额按数据库scale10比较；日金额只接受已知两种写入公式（round全天明细，或相加已逐个核对的小时round值），不能用任意epsilon掩盖差异。Apply锁内复核后在一个事务写成本及汇总，失败全部回滚；缓存同步独立可重试且不得重写成本。
5. **正例/基线/反例**：两天176条且逐桶相符可由用户显式接受后重算；不接受不改变旧历史；筛选外明细或汇总不一致则整批拒绝。基线不得仅回拨全局 `complete_from` 冒充证据。
6. **必需测试**：`TestFinanceRetainedBaselineHistoricalApplyAuditAndFullDay`、`...RejectsReconciliationMismatch`、`...AuditAndWritesRollbackTogether`；恢复请求后的 `...LaterInsertsAggregateThenDeletionPreserves`；`TestFinanceBaselineDeletionGeneration` 验证真实删除/无命中删除/TRUNCATE/回滚。运行本机p11证明非空旧明细→财务与账号API及小时/日汇总同步，不能把空批次或支付通过称为历史请求成功。
7. **错误与正确**：旧日期写repaired标记却仍按前瞻边界一律冻结，会漏掉恢复流量后的请求；260用删除版本区分已接受且尚未失效的基线，后续删除保守失效。不支持后台自动协调停流量。`SHARE usage_logs` 与持有ROW SHARE/advisory的清理升级可能死锁；限定基线使用配置表→source EXCLUSIVE→advisory，普通SELECT仍可读，锁只是误操作防护，不承诺在线重算。
