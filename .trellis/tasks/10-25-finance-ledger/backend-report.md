# 财务明细后端交付记录

## 范围

完成 implement.md 第 1–5 步后端：迁移、账号模型成本快照、支付通道费率与退款扣回快照、收入重放、全部财务接口、CSV、支出审计。
未修改 frontend/、本机部署目录或两个禁止改动的调度/探针文件；未提交、推送、SSH、部署或进行 Docker 全局清理。

## 迁移

- 254：`account_model_cost_multipliers`，账号 FK 级联删除，倍率 numeric(10,4)，支持显式 0，限制 0–10000。
- 255：`finance_expenses`，金额 numeric(20,2)，摊销 1–3660 天，类别 CHECK，账号/操作者 FK SET NULL，软删除与日期部分索引。
- 256：支付实例 numeric(6,4) `channel_fee_rate`（默认 0），订单可空费率快照及 numeric(20,8) `refund_balance_deducted`。历史行保持 NULL，不擅自补历史值。
- 支付字段同步 Ent schema 并运行生成器；Wire 同样通过生成器更新。新自有配置/支出表没有进入 Ent。

## 契约与口径

首次交付时 `contract.md` 无变更；2026-10-25 联调补充已更新契约顶部变更记录（见本文末尾）。接口金额为字符串，只有契约明确指定的模型成本倍率为数值。

保守口径已追加至 `design.md` §11：

1. batch 原账号倍率兼用于客户售价/冻结，保留原公式，仅替换成本快照；其 provider 原样发送 job.Model。
2. 上游模型为空时，两个实时网关回退映射前 RequestedModel；有上游模型时不被 OriginalModel 干扰。
3. 退款先冲减对应订单未确认现金，超过部分在退款日冲减收入，不篡改历史日期；余额不足从免费桶和其他 FIFO 批次扣，现金不跨订单。
4. 正对账差额推断为免费期初，但保留已登记理论余额与实际余额差额，不掩盖缺流水；透支后入账先抵债，历史透支不补认收入。
5. 本期使用只读 REPEATABLE READ 实时查询，不做 60 秒缓存；手动刷新会重新取数。
6. 旧分润空类型归 invite_percent，未知类型计 total 并告警；退款 pending 暂扣尚未终态，显示为对账差额。
7. 账号成本方式按当前配置以 fixed > per_image > model_rate > rate 判定。per_image 要求账号统计定价自定义规则覆盖该账号或其当前所属任一分组，且匹配规则含 image/per_request 定价；不以图片请求占比推断，不用于重算成本，金额仍来自 usage_logs 快照。

## 验证

最终校验全部通过（2026-10-25）：全量 unit、Finance/AccountModelCost 仓储 integration、build、vet 退出码均为 0。所有 Go 构建/测试使用任务专属 `GOTMPDIR=/dev/shm/sub2api-finance-go-tmp`、`GOCACHE=/dev/shm/sub2api-finance-go-cache`、`GOMAXPROCS=4`、`-p 2`，避免根分区临时产物爆满。未清理其他任务缓存或容器。

- 全量 unit：去掉 http_proxy/https_proxy/HTTP_PROXY/HTTPS_PROXY/OPENAI_API_KEY。
- integration：`go test -tags=integration ./internal/repository/ -run 'Finance|AccountModelCost'`，`CI=true` 确保没有 Docker 时不会假成功；实际启动 testcontainers PostgreSQL/Redis。
- `go build ./...`。
- `go vet ./...`。
- `git diff --check -- backend .trellis/spec/backend/quality-guidelines.md`：通过。
- 未运行 golangci-lint（本机未安装）。未运行本机镜像/e2e（按分工交主会话）。

覆盖点：收入纯函数、退款扣回/不扣回/超出本批余额、免费优先、FIFO、跨天、管理员扣余额、分润转入、透支、未登记期初、同日入账优先、跨月摊销尾差；两个网关和 batch 的无配置快照与客户扣费/冻结不变；上游模型及原请求回退；支付 CRUD 和快照；SQL 精确聚合与过滤、软删除、迁移字段类型/约束、替换事务回滚；HTTP 输入/错误脱敏、管理员权限、CSV BOM、支出审计。

## 已知限制

- 日重放而非逐请求账本，同日先入账后消耗；不改变现有扣费。
- 原始用量/兑换流水被删除或保留期清理后，无法完整恢复过去收入；页面明确告警，差额保留。
- 全历史重放未做物化或缓存，大数据量时需要测量并优化；本期未做生产规模性能基准。
- sub2api 订阅套餐收入不计利润（有提示）；按量成本含订阅、内部/测试请求，固定成本账号排除后由手工支出计入。
- 本机 e2e、前后端浏览器联调和真实本机利润抽查由主会话完成，本交付没有声称完成它们。

## 文件清单

- `backend/cmd/server/wire_gen.go`
- `backend/ent/migrate/schema.go`
- `backend/ent/mutation.go`
- `backend/ent/paymentorder.go`
- `backend/ent/paymentorder/paymentorder.go`
- `backend/ent/paymentorder/where.go`
- `backend/ent/paymentorder_create.go`
- `backend/ent/paymentorder_query.go`
- `backend/ent/paymentorder_update.go`
- `backend/ent/paymentproviderinstance.go`
- `backend/ent/paymentproviderinstance/paymentproviderinstance.go`
- `backend/ent/paymentproviderinstance/where.go`
- `backend/ent/paymentproviderinstance_create.go`
- `backend/ent/paymentproviderinstance_query.go`
- `backend/ent/paymentproviderinstance_update.go`
- `backend/ent/runtime/runtime.go`
- `backend/ent/schema/payment_order.go`
- `backend/ent/schema/payment_provider_instance.go`
- `backend/internal/handler/admin/account_model_cost_handler.go`
- `backend/internal/handler/admin/account_model_cost_handler_test.go`
- `backend/internal/handler/admin/finance_handler.go`
- `backend/internal/handler/admin/finance_handler_test.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/wire.go`
- `backend/internal/repository/account_model_cost_repo.go`
- `backend/internal/repository/finance_repo.go`
- `backend/internal/repository/finance_repo_integration_test.go`
- `backend/internal/repository/wire.go`
- `backend/internal/server/middleware/finance_audit_test.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/server/routes/finance.go`
- `backend/internal/server/routes/finance_routes_test.go`
- `backend/internal/service/account_model_cost.go`
- `backend/internal/service/account_model_cost_test.go`
- `backend/internal/service/batch_image_public.go`
- `backend/internal/service/finance.go`
- `backend/internal/service/finance_payment.go`
- `backend/internal/service/finance_payment_test.go`
- `backend/internal/service/finance_reports.go`
- `backend/internal/service/finance_revenue.go`
- `backend/internal/service/finance_revenue_test.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/gateway_usage_billing.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_usage.go`
- `backend/internal/service/payment_config_providers.go`
- `backend/internal/service/payment_config_service.go`
- `backend/internal/service/payment_order.go`
- `backend/internal/service/payment_refund.go`
- `backend/internal/service/wire.go`
- `backend/migrations/254_account_model_cost_multipliers.sql`
- `backend/migrations/255_finance_expenses.sql`
- `backend/migrations/256_payment_channel_fee_rate.sql`
- `backend/migrations/finance_migration_test.go`

任务/规范文档：`design.md` §11、本文件、`implement.md` 交付链接、`.trellis/spec/backend/quality-guidelines.md` 财务章节。

### 最终命令与日志

```bash
cd backend
export GOTMPDIR=/dev/shm/sub2api-finance-go-tmp
export GOCACHE=/dev/shm/sub2api-finance-go-cache
export GOMAXPROCS=4
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY   go test -p 2 -tags unit ./...                  # PASS，退出码 0
CI=true go test -p 2 -tags=integration ./internal/repository/   -run 'Finance|AccountModelCost' -count=1      # PASS，退出码 0
go build -p 2 ./...                            # PASS，退出码 0
go vet -p 2 ./...                              # PASS，退出码 0
```

本轮日志：`/tmp/finance-unit-delivery.log`、`/tmp/finance-integration-delivery.log`、`/tmp/finance-build-delivery.log`、`/tmp/finance-vet-delivery.log`。

首次默认磁盘缓存编译遇到根分区空间不足，改用上述专属内存目录后完成所有验证。早期集成用例暴露了测试订单必填字段和 settings 无 created_at 的 SQL 问题，已修正并以真实数据库重跑通过。没有因其他 agent 的改动产生未解决的 build 错误。

## 2026-10-25 联调补充

- `/finance/recharge` 的 `orders[]` 新增 `out_trade_no` 字符串，原生 SQL → FinanceOrder → HTTP 输出完整透传，历史空值返回空字符串。
- `/finance/reconcile` 的 search 同时匹配邮箱（不区分大小写的包含匹配）OR 纯数字 user_id（数值精确匹配，允许前导零）；不按 ID 前缀匹配，不接受带正负号、小数、科学计数法或全角数字的 ID。
- 已先更新 `contract.md` 顶部变更记录、订单示例及搜索说明。
- 相关单测：`go test -p 2 -tags unit ./internal/service ./internal/handler/admin -run Finance -count=1` 通过。
- SQL 透传集成测试：`go test -p 2 -tags integration ./internal/repository -run TestFinanceRepositorySnapshotExpensesAndReports -count=1` 通过。
- `go build -p 2 ./...` 通过；`git diff --check -- backend` 通过。
- 命令使用之前的任务专属 Go 临时/缓存目录，新增 `GOFLAGS=-mod=readonly`；单测继续移除代理与 OPENAI_API_KEY。未运行生成器，`backend/go.sum` 前后 SHA256 均为 `b17668c6300c04027bd397650e60e28e514dc4abb9ab113e3cea634fdd708109`，没有恢复用户已撤回的依赖校验和。
- 本轮改动：`service/finance.go`、`service/finance_reports.go`、`service/finance_revenue_test.go`、`repository/finance_repo.go`、`repository/finance_repo_integration_test.go`、`handler/admin/finance_handler_test.go` 及契约/本交付记录；未修改前端、未提交。
- 本轮日志：`/tmp/finance-followup-unit.log`、`/tmp/finance-followup-integration.log`、`/tmp/finance-followup-build.log`。

## 2026-10-25 成本方式联调修正

- 修复根因：图片流量类型不等于成本定价配置，原“图片请求占多数”推断会误标默认倍率账号。
- 仓储从当前账号统计定价自定义规则查询账号直接匹配或当前任一所属分组匹配，并确认同一规则下存在 image/per_request 定价；不依赖用量的 group_id，不额外以渠道默认定价开关限制自定义规则。
- 服务按 fixed > per_image > model_rate > rate 输出账号 cost_mode；成本 SQL 与历史快照金额不变，固定成本账号原有排除逻辑不变。
- 已更新 design.md §11、contract.md 顶部变更记录及 cost_mode 说明。
- HTTP 单测覆盖全图片但未配置、无图片但有配置、模型倍率、按张优先、固定优先；仓储集成覆盖直接账号、第二所属分组、非所属分组、token/image/per_request 变化，并验证切换配置后成本仍为历史快照 0.6。
- 以下命令全部通过（退出码 0），沿用任务专属 GOTMPDIR/GOCACHE、GOMAXPROCS=4、GOFLAGS=-mod=readonly；单测移除代理和 OPENAI_API_KEY，集成设置 CI=true：
  - `go test -p 2 -tags unit ./internal/service ./internal/handler/admin ./internal/server/routes ./internal/server/middleware -run Finance -count=1`
  - `go test -p 2 -tags integration ./internal/repository -run Finance -count=1`（testcontainers）
  - `go build -p 2 ./...`
- 本轮代码文件：`service/finance.go`、`service/finance_reports.go`、`repository/finance_repo.go`、`repository/finance_repo_integration_test.go`、`handler/admin/finance_handler_test.go`。无新增迁移，未运行生成器，go.sum 校验和保持不变；未改前端、未提交。
- 日志：`/tmp/finance-costmode-unit.log`、`/tmp/finance-costmode-integration.log`、`/tmp/finance-costmode-build.log`。

## 2026-10-26 账号关联支出摊销

- 仅 account breakdown 新增字符串 expense_cost；全部类别的未删除关联支出按 FinanceAmortization 逐北京日截取查询范围并汇总。账号毛利扣除此项，排序按 account_cost + expense_cost 倒序、同值按 key。
- 区间无用量但关联摊销额为正的账号补行；Load 在同一只读事务批量取得名称、软删除标记、完整当前成本配置，未降级成本方式优先级。其他维度、summary/export 不变，总览支出只计一次。
- 单测覆盖跨月范围截断与末日尾差、无用量账号及删除标记、固定/按张/模型倍率优先级、总成本排序及截断、未关联支出排除、其他维度不变、总览不重复计支出；testcontainers 验证补取配置 SQL、无用量行、软删除支出排除。
- 改动文件：backend/internal/service/finance.go、finance_reports.go、新增 finance_breakdown_test.go；backend/internal/repository/finance_repo.go、finance_repo_integration_test.go；任务 contract.md、design.md §11、本记录。无迁移、无前端改动、无提交。
- 以下验证全部通过（退出码 0）：
  - `env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -p 2 -tags unit ./internal/service/ ./internal/repository/ ./internal/handler/...`
  - `CI=true go test -p 2 -tags=integration ./internal/repository/ -run Finance -count=1`
  - `go build -p 2 ./...`
- 命令沿用 GOTMPDIR=/dev/shm/sub2api-finance-go-tmp、GOCACHE=/dev/shm/sub2api-finance-go-cache、GOMAXPROCS=4、GOFLAGS=-mod=readonly。日志为 /tmp/finance-expense-{unit,integration,build}.log；go.sum SHA256 仍为 b17668c6300c04027bd397650e60e28e514dc4abb9ab113e3cea634fdd708109。
