# 设计

## 数据（迁移 247_affiliate_agent_level.sql）

```sql
ALTER TABLE user_affiliates
  ADD COLUMN IF NOT EXISTS agent_level SMALLINT NOT NULL DEFAULT 3,
  ADD COLUMN IF NOT EXISTS agent_level_locked BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS agent_level_source VARCHAR(16) NOT NULL DEFAULT 'default', -- default|auto|manual
  ADD COLUMN IF NOT EXISTS agent_level_updated_at TIMESTAMPTZ NULL,
  ADD COLUMN IF NOT EXISTS rebate_settled_usage_id BIGINT NOT NULL DEFAULT 0,      -- 消费结算水位（usage_logs.id）
  ADD COLUMN IF NOT EXISTS rebate_settled_consumption DECIMAL(20,8) NOT NULL DEFAULT 0; -- 已计入返利的累计消费
CREATE INDEX IF NOT EXISTS idx_user_affiliates_agent_level ON user_affiliates(agent_level);
```
- `user_affiliate_ledger.rebate_level` 已存在（246），复用；新增 `source_usage_to_id BIGINT NULL` 记录本次结算覆盖到的 usage_logs.id 上界，便于审计。
- 不删 `affiliate_rebate_rate_l3` 设置键的历史值，代码里不再读取。

## 设置键
- `affiliate_rebate_base`：`recharge|consumption`，默认 `consumption`
- `affiliate_rebate_levels`：clamp 到 1–2（原 1–3）
- `affiliate_agent_upgrade_mode`：`manual|auto`，默认 `manual`
- `affiliate_agent_level2_threshold` / `affiliate_agent_level1_threshold`：int ≥ 0，默认 0（0 表示该级不可自动升）
- `affiliate_agent_count_mode`：`all|recharged`，默认 `all`
- `affiliate_settle_interval_minutes`：int，默认 60，clamp 5–1440

## 等级与层级判定
```go
func agentLevelAllowsLayer(agentLevel, layer int) bool {
    // 3 级无返利；2 级只第 1 层；1 级第 1、2 层
    return layer <= 3-agentLevel
}
```
`AccrueInviteRebateForOrder` 的层循环里：受益人 `!agentLevelAllowsLayer` 时跳过入账，但继续向上。

## 消费结算 AffiliateRebateSettleService
- 模板：`payment_order_expiry_service.go`（ticker + `tryAcquireSingletonLeaderLock`）。间隔从设置读取，每轮 `getInterval()` 后 `timer.Reset`（参考 ops_alert_evaluator）。
- 每轮：`IsEnabled && base==consumption` 才跑。
- repo `ListUsersWithPendingConsumption(ctx, limit)`：有 inviter 且 `EXISTS usage_logs WHERE user_id=ua.user_id AND id>ua.rebate_settled_usage_id AND billing_type=0`，分批。
- 每个用户在一个事务里：
  1. `SELECT ... FOR UPDATE` 行锁 user_affiliates。
  2. `SELECT COALESCE(SUM(actual_cost),0), MAX(id) FROM usage_logs WHERE user_id=$1 AND id>$2 AND billing_type=0 AND created_at < NOW() - interval '1 minute'`（留 1 分钟避免乱序写入）。
  3. 真实充值上限 `paidTotal = SUM(amount-bonus_amount) - SUM(refund_amount)`（status in COMPLETED/PARTIALLY_REFUNDED/REFUNDED, order_type=balance）。
  4. `eligible = min(delta, max(0, paidTotal - rebate_settled_consumption))`。
  5. 调 `accrueUpChain(ctx, user, eligible, source=usage, upToID)` 复用层循环（抽成 `AccrueInviteRebateForOrder` 与它的公共函数）。
  6. 更新水位 `rebate_settled_usage_id=maxID`, `rebate_settled_consumption += eligible`。
- 所有用户不管有没有 eligible 都推进水位，避免反复扫描。

## 自动升级
- `AffiliateService.EvaluateAgentLevel(ctx, userID)`：mode==auto && !locked 时，取 count（all: `aff_count`；recharged: `COUNT(DISTINCT invitee) JOIN payment_orders completed`），按门槛求目标等级，只在目标更高时 `UPDATE ... SET agent_level, source='auto'`，并写 `user_affiliate_ledger`? —— 不写 ledger，另加轻量表？决定：不建新表，升级记录写 `slog` + `agent_level_updated_at`。
- 触发：`BindInviterByCode` / `AdminBindInviter` 成功后对 inviter 调用（best effort）。
- 管理端 `POST /admin/affiliates/agent-levels/reevaluate`：遍历所有 inviter（分批）调用。

## 管理端 API
- `PUT /admin/affiliates/users/:user_id` 请求增加 `agent_level *int`、`unlock_agent_level *bool`。
  - 设置比当前低 → `source=manual, locked=true`；比当前高 → `source=manual, locked=false`；相等不动。
- `POST /admin/affiliates/users/batch-agent-level` `{user_ids, agent_level}`，同样的锁定规则。
- `POST /admin/affiliates/agent-levels/reevaluate`。
- `AffiliateAdminEntry`/`AffiliateUserOverview`/`AffiliateDetail` 增加 `agent_level`, `agent_level_locked`, `agent_level_source`；Detail 再加 `next_level`, `next_level_threshold`, `current_invite_count`（auto 模式）。
- `ListUsersWithCustomSettings` 的 WHERE 增加 `OR ua.agent_level <> 3`。

## 用户端
AffiliateView 新增「代理等级」卡：当前等级、两层解锁状态、auto 下升级进度；比例卡按解锁状态灰显。

## 测试
- service 单测：等级矩阵、平台保留、结算金额计算（上限、订阅忽略、水位推进）、自动升级/锁定规则、基数开关。
- repo 集成：迁移列、FOR UPDATE 结算幂等、ListUsersWithPendingConsumption。
- handler：新请求字段 clamp；api_contract 快照。
- 前端：SettingsView 新字段、等级设置模态、AffiliateView 等级卡、records 表。
