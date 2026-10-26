# 财务明细页 — 技术设计

## 1. 总体思路

- 只读统计为主：收入、赠送、分润、成本都从现有表推导，**不改扣费、转发、调度路径**。
- 写路径只有一处改动：请求记录时，账号倍率快照可被「账号模型成本倍率」覆盖（§3）。
- 余额三分类不落库，每次查询按用户重放（§4），这样改费率快照、补录数据后结果自动一致；数据量变大后再改为物化表（§9）。
- 所有新代码按 fork 惯例标 `// CUSTOM(fork):`，新表用原生 SQL（与 `group_model_rate_multipliers` 同样做法）。

## 2. 数据变更（迁移 254+）

| 迁移 | 内容 |
|---|---|
| `254_account_model_cost_multipliers.sql` | 新表 `account_model_cost_multipliers(account_id FK accounts ON DELETE CASCADE, model_pattern varchar(255), rate_multiplier numeric(10,4), created_at, updated_at, PK(account_id, model_pattern))` |
| `255_finance_expenses.sql` | 新表 `finance_expenses(id, occurred_on date, category varchar(32), amount numeric(20,2) > 0, amortize_days int ≥ 1 default 1, account_id bigint null FK SET NULL, note varchar(500), created_by, created_at, updated_at, deleted_at)`，索引 `(occurred_on) WHERE deleted_at IS NULL` |
| `256_payment_channel_fee_rate.sql` | `payment_provider_instances.channel_fee_rate numeric(6,4) not null default 0`；`payment_orders.channel_fee_rate numeric(6,4) null`（下单快照）、`payment_orders.refund_balance_deducted numeric(20,8) null`（退款实际扣回余额） |

- 设置项 `finance_fixed_cost_account_ids`：JSON 数组，缺省 `[]`。部署后由用户在页面上设为 `[1]`。
- `payment_orders.fee_rate` 已存在，是**用户承担**的充值手续费，与新增的通道费率无关，命名上注意区分。
- 历史订单 `channel_fee_rate` 为 NULL 时，按其支付实例的当前费率计（一号机现有 4 笔已完成订单都会按 3.2% 算）。
- 历史退款 `refund_balance_deducted` 为 NULL 时按 `refund_amount` 计；对不上的部分会在用户对账里体现为差额。

## 3. 账号模型成本倍率（写路径）

- 新文件 `internal/service/account_model_cost.go`，结构照抄 `group_model_rate.go`：整表进程内快照、30 秒过期、singleflight、管理端保存后清本进程快照；匹配规则一致（精确优先，其次最长前缀的末尾 `*`）。
- 匹配用**发往上游的模型**（`upstream_model`，为空时退回请求模型），因为成本由上游计价决定。
- 接入点：`gateway_usage_billing.go:841`、`openai_gateway_usage.go:334`、`batch_image_public.go:1077` 三处 `account.BillingRateMultiplier()`，改为 `resolveAccountCostMultiplier(ctx, account, upstreamModel)`：命中则用覆盖值，否则原值。
  - 下游自然跟着变：`usage_logs.account_rate_multiplier` 快照、账号额度计数 `AccountQuotaCost`、所有看板里的账号成本。
  - **不动** `openai_account_scheduler.go:2994`（调度按账号整体倍率）和 `upstream_billing_probe.go:718`。
- 接口：`GET/PUT /admin/accounts/:id/model-cost-multipliers`，整组替换；上限 200 条、倍率 0–10000，与分组模型倍率相同。

## 4. 收入确认引擎（核心）

`internal/service/finance_revenue.go`，纯函数 + 仓储取数，便于单测。

### 4.1 事件（按用户收集，按时间排序）

| 事件 | 来源 | 处理 |
|---|---|---|
| 付费入账 | `payment_orders` balance 类、已入账（COMPLETED 及之后的退款状态），时间 `completed_at` | 新建付费批次：单位 = 本金，现金 = `pay_amount × (1 − 通道费率)` |
| 充值赠送 | 同上订单的 `bonus_amount` | 不计收入桶 + |
| 首充奖励 | ledger `invitee_bonus` | 不计收入桶 + |
| 分润转入 | ledger `transfer` | 新建分润批次：单位 = 现金 = amount |
| 优惠码 | `promo_code_usages.bonus_amount` | 不计收入桶 + |
| 管理员调整 | `redeem_codes` type `admin_balance`，正数 + / 负数 − | 正：不计收入桶 +；负：先扣不计收入桶，再按先进先出扣批次（扣掉的现金价值记为「管理员扣回」，不算收入） |
| 兑换码 | `redeem_codes` type `balance` 且 code 不是任何订单的 `recharge_code` | 不计收入桶 + |
| 退款 | 订单 `refund_at`、`refund_amount`、`refund_balance_deducted` | 该订单批次：单位 −= 扣回余额，现金 −= 退款现金；不足部分从其他批次扣 |
| 消耗 | `usage_user_daily_rollups`（北京日、仅余额计费）`actual_cost` | 先扣不计收入桶，再按先进先出扣批次；从批次扣掉的部分 × 批次单位实收 = 当日确认收入 |

- 注册赠送 `default_balance` 没有流水：用户首个事件前的「期初」按 `users.balance` 倒推的差额处理——重放到当前后与实际余额比较，多出来的部分记为「未登记入账（视同不计收入）」，并在对账里标出。一号机该设置为 0。
- 日粒度：同一天内先处理所有入账，再处理消耗（用户先充后用是常态）；时间精度在财务报表里足够。
- 余额不足仍有消耗（透支）：超出部分记为「透支消耗」，收入 0，对账标红。

### 4.2 输出

- 按日：已确认收入、赠送消耗（不计收入桶被消耗的金额）、分润转入消耗、透支消耗。
- 当前快照：每用户各桶剩余、付费批次剩余的实收价值。
- 用户对账：理论余额 vs `users.balance`。

### 4.3 性能

- 一次查询重放全部用户的全部历史，复杂度 O(事件数)。消耗按「用户 × 日」取自 rollup，体量 = 活跃用户日数。
- 结果按（数据水位）进程内缓存 60 秒；页面提供「刷新」。一号机目前数据量可忽略；若单次超过 2 秒再做物化（§9）。

## 5. 账号成本聚合

- 按量成本：`usage_logs` 全部记录（含订阅计费、扣费失败），`COALESCE(account_stats_cost, total_cost) × COALESCE(account_rate_multiplier, 1)`，与现有看板同一公式；固定成本账号排除。
- 维度：日 × 账号 × 分组 × 模型 × 用户 一次 GROUP BY 出来，前端各视图自行汇总；时间条件走 `created_at` 索引。
- 「内部/测试消耗成本」：消耗全部来自不计收入桶的用户当天的成本。简化为：按 §4 每用户每日「不计收入消耗占比」分摊该用户当日成本。

## 6. 分润

- 支出：ledger `accrue`，按 `created_at`，按 `reward_type` 拆（agent_direct / agent_diff / invite_percent / invite_fixed）。
- 去向：`transfer`、`withdraw` 汇总；未提 = `user_affiliates.aff_quota + aff_frozen_quota`。

## 7. 其他支出与摊销

- 摊销：金额 ÷ 天数，从 `occurred_on` 起逐日分摊，最后一天吃掉舍入误差，保证合计精确等于金额。
- 类别枚举：`upstream_subscription`（上游订阅账号）、`server`、`domain`、`other`。前端选订阅类别时摊销天数默认 30。
- 接口：`GET/POST/PUT/DELETE /admin/finance/expenses`（软删除），写审计日志（沿用 `audit_logs`）。

## 8. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/admin/finance/summary?start&end&granularity=day\|month` | 总览卡片 + 趋势序列 + 利润明细表 |
| GET | `/admin/finance/breakdown?start&end&dim=account\|group\|model\|user` | 消耗与成本拆分 |
| GET | `/admin/finance/affiliate?start&end` | 分润 |
| GET | `/admin/finance/recharge?start&end` | 充值与赠送 |
| GET | `/admin/finance/reconcile?page&only_mismatch` | 用户对账 |
| GET | `/admin/finance/export?start&end&granularity` | CSV |
| GET/PUT | `/admin/finance/settings` | 固定成本账号 |
| CRUD | `/admin/finance/expenses` | 其他支出 |

金额在接口里用字符串保留两位到八位小数（避免 JS 浮点），前端只做展示。

## 9. 后续扩展（不在本期）

- 物化：`finance_user_daily(user_id, day, 各桶增减, 确认收入)` 由后台任务按水位增量计算。
- 历史成本按当前规则重算（管理端按时间段 + 账号触发）。

## 10. 兼容与回滚

- 三个迁移均为加表 / 加可空列，回滚时直接回退镜像即可，旧代码忽略新列新表。
- 账号模型成本倍率没有配置时写路径行为完全不变；配置后影响之后的请求，删配置即恢复。
- 页面只读，不影响业务数据。

## 11. 后端实施口径补充（2026-10-25）

- 批量生图原账号倍率同时参与客户售价和冻结额，不能直接替换该局部变量。保留原售价计算，只覆盖 `AccountRateMultiplier` 成本快照；当前 batch provider 原样发送 job.Model，因此以请求模型匹配，不擅自增加模型映射。
- 退款按退款日冲减对应订单剩余现金；超过未确认现金的部分在退款日冲减已确认收入，不回写历史日。扣回余额优先该订单，缺口先赠送再其他 FIFO；现金不能串扣其他订单。已无单位但仍有现金的尾差不作为收入（保守处理）。
- 未登记期初 = 实际余额 − 全部已登记净流量，仅正差作为首事件前免费余额补入；对账 `expected_balance` 仍为已登记净流量，保留 `difference` 告警，不能用推断掩盖缺流水。透支单独留债，后续入账先抵债，不补确认历史透支收入。
- 全部来源在同一只读 REPEATABLE READ 事务中取快照；一期不缓存（刷新即重算，避免删除/补录/退款水位遗漏导致过时报表）。原始历史被保留期清理后不能完整重放，明确 warnings，不声称恢复丢失流水。
- 旧分润 `reward_type IS NULL` 保守归入 invite_percent（旧版仅百分比邀请奖励）；未知非空类型仍计 total，并提示。已完成退款只在终态 REFUNDED/PARTIALLY_REFUNDED 且有 refund_at 时计现金退款；pending 的暂扣尚未终态，以对账差额提示。
- 账号成本方式按当前配置判定：固定成本账号 → fixed；账号统计定价自定义规则的 account_ids 含该账号，或 group_ids 含该账号当前所属的任一分组，且该规则的定价含 billing_mode=image/per_request → per_image；否则有账号模型成本倍率 → model_rate；否则 rate。按张规则与模型倍率同时存在时 per_image 优先，不依赖期间图片请求占比或期间请求分组。不据标签重算历史成本，金额始终读取用量快照。
- 金额使用 decimal，计算精度保留到 16 位，API 最多 8 位。摊销按 8 位截断，最后一天补足。支出走既有管理面审计中间件（记录请求、操作者、结果），不重复异步写日志。

- 2026-10-26：仅账号 breakdown 将关联的所有类别未删除支出，按 FinanceAmortization 逐北京日截取 [start,end] 后汇总为 expense_cost；毛利扣除该值，按按量成本加关联支出倒序、同值按 key。无用量但摊销额为正的账号补行；Load 在同一只读事务批量补取支出关联账号的名称、软删除状态及完整 per_image/model_rate 当前配置，沿用 fixed > per_image > model_rate > rate，不降级标签。物理删除账号的支出 account_id 已 SET NULL，不再归属账号；软删除账号保留名称并标记。其他维度、summary/export 不变，支出在总览中仍只计一次。
