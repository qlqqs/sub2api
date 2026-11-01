# 合伙人 — 前后端接口契约

前后端都以本文件为准。任何一方要改契约，先在这里的「变更记录」写明，再通知主代理。

## 变更记录

- 2026-10-31：初版。
- 2026-10-31（第二轮确认）：
  - 成本按分组 + 模型拆分（`cost_by_group` / `cost_by_model`）已定，不做供应商维度。
  - 1.2 预览增加 `current_region`。
  - 3.1 合伙人的 `appoint` 为 null，邀请列表的 `appointable` 一律为 false。

## 通用约定

- 统一响应包 `{ code, message, data }`。管理端接口在 `/api/v1/admin` 下，用户端在 `/api/v1/user` 下。
- **金额**：字符串，十进制，最多 8 位小数。`share_amount` 和调整项金额固定 2 位。前端只展示（保留 2 位），不做运算。
- **比例**：字符串小数，`"0.4000"` 表示 40%。请求里也用字符串或数字 0–1，例如 `0.4`。
- **月份**：`"YYYY-MM"`，北京时间自然月。日期为 `"YYYY-MM-DD"`，时间为 RFC3339。
- **错误**：4xx 时 `message` 是可读的中文原因，前端直接 toast。各接口的 `code` 写在对应条目里。
- **用户对象 `PartnerUserRef`**：`{ "id": 75, "email": "1185393366@qq.com", "username": "malaixin", "deleted": false }`。
  - 管理端返回邮箱原文。
  - 合伙人端返回脱敏邮箱，规则同代理团队页（`maskEmail`）。

### 月度数字 `PartnerMonthTotals`

```jsonc
{
  "month": "2026-11",
  "status": "current",          // current（当月预估）| draft（已结束未确认）| confirmed | paid
  "share_rate": "0.4000",
  "consumption": "0",           // 区域余额消耗（billing_type=0 actual_cost，起点之后）
  "revenue": "0",
  "account_cost": "0",
  "commission": "0",
  "region_expense": "0",
  "adjustment": "0",
  "profit": "0",                // revenue − account_cost − commission − region_expense + adjustment
  "loss_before": "0",           // 期初累计亏损（≥ 0）
  "loss_after": "0",            // 期末累计亏损（≥ 0）
  "share_base": "0",
  "share_amount": "0.00",
  "confirmed_at": null,         // confirmed / paid 时有值
  "paid_on": null,              // "YYYY-MM-DD"，paid 时有值
  "paid_note": null
}
```

### 结算单明细 `PartnerStatementDetail`

```jsonc
{
  "totals": { /* PartnerMonthTotals */ },
  "lines": [                    // 按 revenue 倒序；line_agent 为 null 表示「合伙人直属团队」
    {
      "line_agent": null,       // 或 PartnerUserRef（挂靠的渠道经理）
      "members": 3,
      "consumption": "0", "revenue": "0", "account_cost": "0", "commission": "0", "profit": "0"
    }
  ],
  "cost_by_group": [ { "group_id": 2, "group_name": "Codex 稳定分组", "consumption": "0", "account_cost": "0" } ],
  "cost_by_model": [ { "model": "gpt-6-sol", "consumption": "0", "account_cost": "0" } ],
  "adjustments": [ { "id": 1, "amount": "-5.00", "note": "…", "created_at": "…", "created_by_email": "admin@…" } ],
  "warnings": [ "…" ]
}
```

- 合伙人端返回的 `adjustments[]` 不含 `created_by_email`。
- 合伙人端的任何接口、任何字段，都不出现账号名称、账号 ID、供应商、单价。

## 1. 管理端 `/api/v1/admin/affiliates/partners`

### 1.1 列表 `GET /affiliates/partners`

```jsonc
{ "items": [ {
  "user": { /* PartnerUserRef */ },
  "region_name": "华东",
  "share_rate": "0.4000",
  "next_share_rate": null,      // 已设置下月新比例时有值
  "next_rate_month": null,      // "2026-12"
  "since": "2026-11-02T10:00:00+08:00",
  "agent_count": 2,
  "member_count": 15,
  "loss_carried": "0",          // 截至最后一个已结束月份的期末累计亏损
  "unconfirmed_months": 1,      // 已结束但未确认的月份数
  "current_month": { /* PartnerMonthTotals，status=current */ }
} ] }
```

### 1.2 设为合伙人：预览 `GET /affiliates/partners/preview?user_id=62`

```jsonc
{ "user": { /* PartnerUserRef */ }, "agent_level": 3, "is_partner": false, "is_attached": false,
  "team": { "owned": 0, "via_upline": 0 },
  "current_region": null,       // 或 { "partner": PartnerUserRef, "region_name": "…" }：按快照当前所属区域，仅作提示
  "can_create": true, "reason": null }
```

`reason` 是不能设立时的中文原因。

### 1.3 设为合伙人 `POST /affiliates/partners`

- 请求：`{ "user_id": 62, "region_name": "华南", "share_rate": "0.40" }`
- 返回：1.1 的单项。
- 错误：
  - 400 `PARTNER_INVALID`：区域名为空或超过 64 字，或比例越界。
  - 404 `USER_NOT_FOUND`
  - 409 `PARTNER_EXISTS`
  - 409 `PARTNER_AGENT_ATTACHED`

### 1.4 详情 `GET /affiliates/partners/:user_id`

返回 1.1 单项 + `"agents": [PartnerAgentItem]`：

```jsonc
{ "user": { /* PartnerUserRef */ }, "attached_at": "…", "source": "manual",   // 或 promote
  "attached_by_email": "admin@…",                                             // promote 且自动升级时为 null
  "agent_level": 1, "team": { "owned": 6, "via_upline": 3 },
  "current_month": { "consumption": "0", "revenue": "0", "account_cost": "0", "commission": "0", "profit": "0" } }
```

### 1.5 修改 `PUT /affiliates/partners/:user_id`

- 请求：`{ "region_name": "华南", "share_rate": "0.35" }`，两个字段都可以省略。
- 比例写入下月生效，返回 1.1 单项（含 `next_share_rate` / `next_rate_month`）。
- 如果新比例等于当前比例，删除已设置的下月比例。

### 1.6 挂靠：预览 `GET /affiliates/partners/:user_id/agents/preview?agent_user_id=59`

```jsonc
{ "agent": { /* PartnerUserRef */ }, "agent_level": 1, "team": { "owned": 6, "via_upline": 3 },
  "current_region": null,       // 或 { "partner": PartnerUserRef, "region_name": "…" }：按快照当前所属区域，仅作提示
  "can_attach": true, "reason": null }
```

### 1.7 挂靠 `POST /affiliates/partners/:user_id/agents`

- 请求：`{ "agent_user_id": 59 }`
- 返回：`PartnerAgentItem`。
- 错误：
  - 404：合伙人不存在。
  - 400 `PARTNER_AGENT_NOT_LEVEL1`
  - 409 `PARTNER_AGENT_IS_PARTNER`
  - 409 `PARTNER_AGENT_ATTACHED`
- 没有解除挂靠或改挂的接口。

### 1.8 结算单列表 `GET /affiliates/partners/:user_id/statements`

返回 `{ "items": [PartnerMonthTotals] }`，从设立当月到当月，按月份倒序。

### 1.9 结算单明细 `GET /affiliates/partners/:user_id/statements/:month`

- 返回 `PartnerStatementDetail`。
- 已确认和已付的读快照，其余实时计算。

### 1.10 确认 `POST /affiliates/partners/:user_id/statements/:month/confirm`

- 返回 `PartnerStatementDetail`。
- 错误：
  - 400 `PARTNER_MONTH_NOT_ENDED`：当月或未来月份。
  - 400 `PARTNER_PREVIOUS_UNCONFIRMED`
  - 400 `PARTNER_MONTH_OUT_OF_RANGE`：早于设立当月。
  - 409 `PARTNER_MONTH_CONFIRMED`

### 1.11 标记已付 `POST /affiliates/partners/:user_id/statements/:month/pay`

- 请求：`{ "paid_on": "2026-12-03", "note": "招行转账 尾号1234" }`。`paid_on` 可省略，默认今天（北京时间）；`note` 必填，最长 500 字。
- 返回：`PartnerStatementDetail`。
- 错误：
  - 400 `PARTNER_MONTH_NOT_CONFIRMED`
  - 409 `PARTNER_MONTH_PAID`

### 1.12 调整项

- `POST /affiliates/partners/:user_id/adjustments`
  - 请求：`{ "month": "2026-11", "amount": "-5.00", "note": "…" }`
  - 返回：调整项对象。
- `DELETE /affiliates/partners/:user_id/adjustments/:id`
- 错误：
  - 400 `PARTNER_ADJUSTMENT_INVALID`：金额为 0、超过 2 位小数，或说明为空。
  - 400 `PARTNER_MONTH_OUT_OF_RANGE`
  - 409 `PARTNER_MONTH_CONFIRMED`

### 1.13 导出 `GET /affiliates/partners/:user_id/statements/:month/export`

- `text/csv`，UTF-8 BOM。
- 文件名 `partner-<user_id>-<YYYY-MM>.csv`。

## 2. 管理端现有接口的变化

### 2.1 代理列表 `GET /admin/affiliates/users`

- 每项增加：
  - `"is_partner": bool`
  - `"partner": { "user_id": 75, "region_name": "华东" } | null`：该用户所属的区域。合伙人本人这里填自己。
- 新增查询参数 `partner_id`，只返回该区域的用户。

### 2.2 设档位 `POST /admin/affiliates/users/batch-agent-level` 与 `PUT /admin/affiliates/users/:user_id`

- 目标里有合伙人时，返回 409 `PARTNER_LEVEL_FIXED`，整批不改。
- 设为 1 级时，响应 data 增加 `"auto_attached": [ { "agent_user_id": 80, "partner": PartnerUserRef, "region_name": "华东" } ]`。没有自动挂靠时为空数组。
  - 批量接口原响应是 `{ "affected": n }`，在它上面加这个字段。
  - `PUT /:user_id` 也返回同样的字段。

### 2.3 财务

**支出 `GET/POST/PUT /admin/finance/expenses`**
- 输入输出增加 `"partner_user_id": 75 | null`。
- 输出增加 `"partner_region_name": "华东" | null`。
- 合伙人不存在时返回 400 `FINANCE_EXPENSE_PARTNER_INVALID`。

**总览 `GET /admin/finance/summary`**
- `totals` 增加：
  - `"partner_share": "0"`、`"net_profit": "0"`
  - `"partner_share_estimated": bool`
  - `"partner_share_scope": "whole_months"`
- `granularity=month` 时，`series[]` 每行增加 `"partner_share"`、`"net_profit"`、`"partner_share_estimated"`。
- `granularity=day` 时，`series[]` 每行不含这三项。

**导出 `GET /admin/finance/export`**
- 月粒度增加「合伙人分成」「平台净利润」两列。

## 3. 用户端

### 3.1 `GET /user/aff`（现有）

增加：

```jsonc
"partner": null   // 或 { "region_name": "华东", "share_rate": "0.4000", "next_share_rate": null, "next_rate_month": null, "since": "…" }
```

合伙人的 `appoint` 为 null（与不能任命的账号一致）；`GET /user/aff/invitees` 每项的 `appointable` 一律为 false。

### 3.2 合伙人接口 `/api/v1/user/aff/partner/*`

- 所有接口挂重型限流，受返利功能开关约束。
- 非合伙人访问返回 403 `PARTNER_FORBIDDEN`。

| 方法 | 路径 | 返回 |
|---|---|---|
| GET | `/overview` | `{ "region_name", "share_rate", "next_share_rate", "next_rate_month", "since", "loss_carried", "agent_count", "member_count", "current_month": PartnerMonthTotals, "lines": [...] }`，`lines` 同明细结构，取当月 |
| GET | `/statements` | `{ "items": [PartnerMonthTotals] }`，从设立当月到当月，按月份倒序 |
| GET | `/statements/:month` | `PartnerStatementDetail`：脱敏，不含 `created_by_email` |

### 3.3 区域团队消费 `/api/v1/user/aff/partner/team/*`

查询参数与现有 `/user/aff/team/*` 一致（`start`、`end` 或 `days`、分页、搜索），范围是整个区域，不受计入起点限制。

| 方法 | 路径 | 返回 |
|---|---|---|
| GET | `/team/overview` | 现有 `AffiliateTeamOverview` 结构（summary、trend、models），范围为区域；不含 `earnings` 类字段 |
| GET | `/team/customers` | `{ items: [AffiliateTeamCustomerEntry + "owner": PartnerUserRef \| null, "line_agent": PartnerUserRef \| null], total }`：`owner` 是客户的归属代理，`line_agent` 是所在渠道经理线，为 null 时表示合伙人直属团队；不返回 `earnings` |
| GET | `/team/lines` | `{ items: [ { "line_agent": PartnerUserRef \| null, "customers", "active_customers", "requests", "team_cost", "cost_share" } ] }`：渠道经理线 + 直属团队一行 |
| GET | `/team/customers/:user_id/requests` | 现有请求明细结构；客户必须在区域内，否则 403 |
| GET | `/team/agents/:agent_id/overview` | 以挂靠渠道经理为 viewer 的现有结构，四个子路径原样复用现有代理端返回：`overview`、`customers`、`agents`、`customers/:user_id/requests` |

- `agent_id` 不是本合伙人挂靠的渠道经理时，返回 403 `PARTNER_FORBIDDEN`。
- 合伙人直属团队直接用现有 `/user/aff/team/*`：合伙人内部是 1 级，viewer 为合伙人本人。
