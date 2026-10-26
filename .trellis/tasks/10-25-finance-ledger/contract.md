# 财务明细页 — 前后端接口契约

## 变更记录

- 2026-10-26（账号关联支出）：仅 account breakdown 新增字符串 expense_cost，计入区间内关联支出的逐日摊销；gross_profit 扣除此项，按 account_cost + expense_cost 倒序（同值按 key）。无用量但关联摊销额为正的账号也出行；其他维度、summary/export 不变。

- 2026-10-25（成本方式口径修正）：`breakdown?dim=account` 的 `cost_mode` 按当前配置判定，优先级 `fixed > per_image > model_rate > rate`；`per_image` 要求账号统计定价规则覆盖账号（account_ids 或当前所属任一 group_ids），且该规则的定价含 image/per_request，不再根据图片请求数量推断。金额仍使用用量快照。

- 2026-10-25（后端联调）：`GET /finance/recharge` 的 `orders[]` 新增字符串 `out_trade_no`，用于精确跳转订单页；历史无商户订单号时返回空字符串。
- 2026-10-25（后端联调）：`GET /finance/reconcile` 的 `search` 同时匹配邮箱（包含、不区分大小写）和用户 ID（仅 `0–9` 组成的纯数字时按数值精确匹配 `user_id`，允许前导零）；两者为 OR，非纯数字仅匹配邮箱，空字符串不过滤。

所有接口在 `/api/v1/admin` 下，仅管理员，统一响应包 `{ code, message, data }`（`response.Success`）。

## 通用约定

- **金额**：一律是字符串，十进制，最多 8 位小数（如 `"12.34000000"`）。前端只展示，展示时保留 2 位（单价类保留 4 位），不做加减运算；合计都由后端给。
- **比例**（利润率、费率）：字符串小数，`"0.3200"` 表示 32%；分母为 0 时为 `null`。
- **日期**：`start`、`end` 为 `YYYY-MM-DD`（北京时间，闭区间）；缺省为本月 1 日到今天。跨度上限 732 天，超出返回 400 `FINANCE_RANGE_TOO_LARGE`。
- **粒度**：`granularity=day|month`，默认 `day`。月粒度的 `period` 为 `YYYY-MM`，日粒度为 `YYYY-MM-DD`。
- **错误**：400 时 `message` 为可读中文原因，前端直接 toast。

## 1. 总览 `GET /finance/summary?start&end&granularity`

```jsonc
{
  "range": { "start": "2026-10-01", "end": "2026-10-25", "granularity": "day" },
  "totals": {
    "revenue": "0",                  // 已确认收入（付费余额被消耗 × 单位实收 + 分润转入被消耗）
    "revenue_paid": "0",             //   其中：付费余额部分
    "revenue_commission": "0",       //   其中：分润转入部分
    "free_consumed": "0",            // 不计收入余额被消耗（赠送 / 管理员加的），收入 0
    "overdraft_consumed": "0",       // 透支消耗，收入 0
    "consumption": "0",              // 余额消耗合计 = revenue_paid 对应的余额 + 分润转入 + free + overdraft（余额单位）
    "account_cost": "0",             // 按量账号成本（不含固定成本账号）
    "account_cost_internal": "0",    //   其中：内部/测试消耗（不计收入余额被消耗部分）分摊的成本
    "commission": "0",               // 分润支出（accrue）
    "expenses": "0",                 // 其他支出（摊销后）
    "profit": "0",                   // revenue − account_cost − commission − expenses
    "profit_margin": "0.0000"        // profit / revenue；revenue 为 0 时 null
  },
  "series": [                        // 按 period 升序，范围内每个 period 都有一行（无数据填 "0"）
    {
      "period": "2026-10-25",
      "revenue": "0", "revenue_paid": "0", "revenue_commission": "0",
      "free_consumed": "0", "overdraft_consumed": "0", "consumption": "0",
      "account_cost": "0", "account_cost_internal": "0",
      "commission": "0", "expenses": "0", "profit": "0", "profit_margin": null
    }
  ],
  "snapshot": {                      // 当前时刻，不随日期范围变化
    "balance_total": "0",            // SUM(users.balance)（未删除用户）
    "balance_paid": "0",             // 付费余额剩余（余额单位）
    "balance_paid_cash_value": "0",  //   对应的实收价值（¥）
    "balance_commission": "0",       // 分润转入剩余
    "balance_free": "0",             // 不计收入余额剩余
    "balance_unattributed": "0",     // 对账差额合计（实际 − 理论），正负都可能
    "frozen_balance": "0",           // SUM(users.frozen_balance)
    "affiliate_available": "0",      // SUM(aff_quota)
    "affiliate_frozen": "0"          // SUM(aff_frozen_quota)
  },
  "warnings": ["范围内有 2 笔订阅订单，未计入利润"],   // 可为空数组
  "computed_at": "2026-10-25T21:00:00+08:00"
}
```

## 2. 充值与赠送 `GET /finance/recharge?start&end`

```jsonc
{
  "totals": {
    "order_count": 4,
    "cash_received": "26.00",        // SUM(pay_amount) 已入账订单
    "channel_fee": "0.83",           // SUM(pay_amount × 通道费率)
    "refund_cash": "0",              // 期间退款现金
    "net_cash": "25.17",             // cash_received − channel_fee − refund_cash
    "principal": "26.00",            // 到账本金 SUM(amount − bonus_amount)
    "gift": {                        // 不计收入入账，按来源
      "recharge_bonus": "0", "invitee_bonus": "2.00", "promo": "0",
      "admin_adjust_in": "110.00", "admin_adjust_out": "0",   // out 为正数，表示扣掉的
      "redeem_code": "0", "total": "112.00"
    },
    "commission_transfer_in": "0"    // 分润转入余额
  },
  "orders": [                        // 期间已入账的余额订单，按 completed_at 倒序，最多 500 条
    {
      "id": 59, "user_id": 60, "user_email": "a@b.c",
      "out_trade_no": "S2A202610250001", // 商户订单号；字符串，历史无值时为 ""
      "completed_at": "2026-10-25T17:50:30+08:00",
      "payment_type": "wxpay", "pay_amount": "10.00",
      "channel_fee_rate": "0.0320", "channel_fee": "0.32",
      "principal": "10.00", "bonus": "0",
      "refund_cash": "0", "refund_balance_deducted": "0",
      "status": "COMPLETED"
    }
  ],
  "orders_truncated": false
}
```

## 3. 消耗与成本 `GET /finance/breakdown?start&end&dim=account|group|model|user&limit=200`

```jsonc
{
  "dim": "account",
  "rows": [                          // account 按 account_cost + expense_cost 倒序；user 按 consumption；其他按 account_cost；同值按 key
    {
      "key": "4",                    // account/group/user 为 id 字符串，model 为模型名
      "label": "www.ggwk1.online - image - 0.05",   // 已删除的对象 label 带「（已删除）」
      "cost_mode": "per_image",      // 仅 dim=account：rate | model_rate | per_image | fixed（按当前配置：fixed > per_image > model_rate > rate）
      "requests": 1, "images": 1,
      "consumption": "0.15",         // 余额计费的 actual_cost
      "subscription_consumption": "0",   // 订阅计费的 actual_cost（参考）
      "account_cost": "0.05",        // 固定成本账号为 "0"
      "expense_cost": "0.02",       // 仅 dim=account：关联该账号的全部类别未删除支出，在查询范围内的北京日摊销合计
      "gross_profit": "0.08"         // account：consumption − account_cost − expense_cost；其他维度仍为 consumption − account_cost
    }
  ],
  "truncated": false
}
```

仅账号维度补充无用量但关联支出摊销额 > 0 的账号（请求/图片/消耗/按量成本为 0，毛利为负的支出摊销额）。未关联账号的支出不进入 breakdown；summary/export 中支出仍仅在 expenses 计一次。

## 4. 分润 `GET /finance/affiliate?start&end`

```jsonc
{
  "accrued": { "agent_direct": "0", "agent_diff": "0", "invite_percent": "0", "invite_fixed": "0", "total": "0" },
  "transferred": "0",                // 转入余额
  "withdrawn": "0",                  // 提现
  "available": "0", "frozen": "0",   // 当前快照
  "top_beneficiaries": [             // 期间分润最多的 50 人
    { "user_id": 59, "user_email": "x", "accrued": "0", "transferred": "0", "withdrawn": "0", "available": "0" }
  ]
}
```

## 5. 用户对账 `GET /finance/reconcile?page=1&page_size=50&only_mismatch=false&search=`

全时段（不受日期范围影响）。`search` 为邮箱包含匹配（不区分大小写）OR 纯数字用户 ID 精确匹配（允许前导零）；非纯数字仅匹配邮箱，空字符串不过滤。分页结构沿用 `response.Paginated`（`items/total/page/page_size/pages`）。

```jsonc
{
  "items": [
    {
      "user_id": 60, "user_email": "x", "role": "user", "deleted": false,
      "paid_in": "10.00",            // 付费本金
      "paid_cash": "9.68",           // 对应实收（扣通道费、退款）
      "free_in": "1.00",             // 不计收入入账（含管理员加）
      "free_out": "0",               // 管理员扣回
      "commission_in": "0",
      "refund_deducted": "0",
      "consumption": "0.05",
      "overdraft": "0",
      "expected_balance": "10.95",
      "actual_balance": "10.95",
      "difference": "0",             // actual − expected；|difference| ≥ 0.01 视为不平
      "remaining": { "paid": "9.95", "commission": "0", "free": "1.00" }
    }
  ],
  "total": 9, "page": 1, "page_size": 50, "pages": 1
}
```

## 6. 导出 `GET /finance/export?start&end&granularity`

返回 `text/csv; charset=utf-8`（带 BOM，Excel 可直接打开），列与 `summary.series` 一致，表头中文。前端用 blob 下载，文件名 `finance-<start>-<end>.csv`。

## 7. 设置 `GET /finance/settings`、`PUT /finance/settings`

```jsonc
{ "fixed_cost_account_ids": [1] }   // PUT 同结构；不存在的账号 id 返回 400
```

GET 额外返回 `accounts: [{ "id": 1, "name": "vm2api - pro", "platform": "openai", "deleted": false }]` 供多选（全部未删除账号 + 已选中但已删除的）。

## 8. 其他支出 `/finance/expenses`

- `GET /finance/expenses?start&end&category&page&page_size` → 分页，按 `occurred_on` 倒序。范围按 `occurred_on` 过滤（不是摊销日）。
- `POST /finance/expenses`、`PUT /finance/expenses/:id` 请求体：

```jsonc
{
  "occurred_on": "2026-10-25",
  "category": "upstream_subscription",   // upstream_subscription | server | domain | other
  "amount": "1400.00",                   // > 0，最多 2 位小数
  "amortize_days": 30,                   // 1..3660
  "account_id": 1,                       // 可空
  "note": "ChatGPT Pro #3"               // ≤ 500
}
```

- 返回的条目额外有 `id`、`account_name`、`daily_amount`（摊销后每日金额）、`created_by_email`、`created_at`、`updated_at`。
- `DELETE /finance/expenses/:id` 软删除。
- 前端表单「摊销方式」二选一：**一次性扣除**（提交 `amortize_days = 1`，全额计入发生日）/ **按天摊销**（填 2–3660 天，预填 30）。选 `upstream_subscription` 时默认按天摊销 30 天，其他类别默认一次性扣除；列表 1 天显示「一次性」。接口不变。

## 9. 账号模型成本倍率 `/accounts/:id/model-cost-multipliers`

与分组模型倍率（`/admin/groups/:id/model-rates`）同构：

- `GET` → `{ "rules": [{ "model": "gpt-5.6-luna", "rate_multiplier": 0.3 }] }`（按 model 排序）
- `PUT` 请求 `{ "rules": [...] }` 整组替换，返回归一化后的 `{ "rules": [...] }`。
- 校验：model 非空、≤ 255、只允许末尾一个 `*`、大小写不敏感去重；倍率 0–10000，最多 4 位小数；最多 200 条。失败 400 `ACCOUNT_MODEL_COST_INVALID`，message 可读。
- `GET /accounts/model-cost-multipliers` → `{ "<account_id>": [rules] }`，只含有规则的账号（账号列表页显示「有模型成本倍率」提示用）。
- 匹配：按发往上游的模型，精确优先，其次最长前缀通配；命中时替换账号倍率（不是相乘）。

## 10. 支付实例通道费率

- 现有支付实例接口（`/admin/payment/providers`）的实例对象新增 `channel_fee_rate`：字符串，百分比的小数形式，`"0.0320"` = 3.2%，范围 0–0.2，默认 `"0"`。
- 前端在实例编辑表单里以百分比输入（输入 3.2 → 提交 `"0.0320"`），说明文字：「支付平台从每笔收款中抽取的比例，只用于财务统计；修改只影响之后的新订单」。
