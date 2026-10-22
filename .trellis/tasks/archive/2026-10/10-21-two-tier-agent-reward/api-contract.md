# 接口契约（前后端共用，实现以此为准）

金额单位 USD，比例单位为百分数（20 表示 20%）。所有接口沿用现有响应包装 `{code, message, data}` 与分页结构。

## 1. 管理端设置 GET/PUT `/api/v1/admin/settings`

保留：`affiliate_enabled`、`affiliate_rebate_rate`（含义改为**利润池 P / 一级比例**）、`affiliate_rebate_freeze_hours`、`affiliate_rebate_duration_days`（代理利润有效期）、`affiliate_rebate_per_invitee_cap`（代理利润单人上限）、`affiliate_admin_recharge_enabled`、`affiliate_rebate_base`、`affiliate_settle_interval_minutes`、`affiliate_agent_upgrade_mode`、`affiliate_agent_level2_threshold`、`affiliate_agent_level1_threshold`、`affiliate_agent_count_mode`。

**删除**：`affiliate_rebate_levels`、`affiliate_rebate_rate_l2`。

新增：

| 字段 | 类型 | 默认 | 说明 |
|---|---|---|---|
| affiliate_agent_level2_rate | number | 10 | 二级比例 R2，0–100 且 ≤ P；PUT 时 R2 > P 返回 400，reason `AFFILIATE_LEVEL2_RATE_EXCEEDS_POOL` |
| affiliate_agent_custom_rate_enabled | bool | false | 专属比例开关 |
| affiliate_agent_appoint_enabled | bool | true | 允许一级代理任命直属用户为二级 |
| affiliate_agent_appoint_max | int | 0 | 每个一级最多任命几个，0=不限 |
| affiliate_invite_reward_mode | `off`/`percent`/`first_recharge` | off | 普通用户邀请奖励模式 |
| affiliate_invite_reward_rate | number | 0 | percent 模式比例 0–100 |
| affiliate_invite_reward_duration_days | int | 0 | percent 模式有效期，0=永久 |
| affiliate_invite_reward_per_invitee_cap | number | 0 | percent 模式单人上限，0=不限 |
| affiliate_invite_reward_fixed_amount | number | 0 | first_recharge 模式奖励金额 |
| affiliate_invite_reward_min_recharge | number | 0 | first_recharge 模式首充门槛 |
| affiliate_invitee_bonus_mode | `off`/`percent`/`fixed` | off | 被邀请人首充奖励模式 |
| affiliate_invitee_bonus_value | number | 0 | percent 时为比例 0–100，fixed 时为金额 |
| affiliate_invitee_bonus_min_recharge | number | 0 | 门槛 |
| affiliate_invitee_bonus_max | number | 0 | percent 模式封顶，0=不封顶 |

## 2. 公开设置 GET `/api/v1/settings/public`（未登录可读）

新增（邀请返利总开关关闭时 mode 恒为 `off`）：
`affiliate_invitee_bonus_mode`、`affiliate_invitee_bonus_value`、`affiliate_invitee_bonus_min_recharge`、`affiliate_invitee_bonus_max`。
不暴露其它返利设置。

## 3. 用户端

### GET `/api/v1/user/aff`
```jsonc
{
  "user_id": 1, "aff_code": "XXXX", "inviter_id": 5,
  "aff_count": 3, "aff_quota": 2, "aff_frozen_quota": 0, "aff_history_quota": 6,
  "rebate_base": "consumption",
  "agent_level": 3,                 // 1 一级代理 / 2 二级代理 / 3 普通用户
  "agent_level_locked": false,
  "agent_upgrade_mode": "manual",   // manual | auto
  "next_agent_level": 2,            // 仅 auto 且未锁定且有可升等级时出现
  "next_level_threshold": 5,
  "current_invite_count": 3,
  "agent_rate_percent": 20,         // 仅代理（1/2）出现：已套专属比例的本档比例
  "owned_customer_count": 15,       // 归属我的客户数（agent_owner_id = 我）
  "invite_reward": {                // 仅普通用户且模式非 off 时出现：我邀请好友我能得什么
    "mode": "percent",              // percent | first_recharge
    "rate_percent": 5, "duration_days": 0, "per_invitee_cap": 0,
    "fixed_amount": 0, "min_recharge": 0
  },
  "invitee_bonus": {                // 模式非 off 时出现：好友（被邀请人）首充能得什么
    "mode": "percent", "value": 10, "min_recharge": 10, "max": 20
  },
  "first_recharge_bonus": {         // 仅当「我」有邀请人、模式非 off、且尚未发过时出现：我自己可领的首充奖励
    "mode": "percent", "value": 10, "min_recharge": 10, "max": 20
  },
  "reward_totals": { "agent": 90, "invite": 6, "invitee_bonus": 0 },
  "appoint": { "max": 0, "used": 2 },  // 仅一级代理且任命开关开启时出现；max=0 不限
  "invitees": [{
    "user_id": 9, "email": "b***@x.com", "username": "b", "created_at": "...",
    "total_rebate": 12.0,           // 该好友为我带来的全部收益
    "owned_by_me": true,            // 该好友归属代理是我
    "agent_level": 2,
    "appointed_by_me": true,        // 由我任命的二级
    "appointable": false            // 有资格「设为二级」（直属、归属我、普通用户、未被管理员锁定）；是否已达上限由前端按 appoint.used/max 判断
  }]
}
```
删除：`effective_rebate_rate_percent`、`rebate_levels`、`level_rebate_rate_percents`、`unlocked_layers`。

### POST `/api/v1/user/aff/invitees/:user_id/agent-level`
Body `{ "agent_level": 2 | 3 }`（2=设为二级，3=取消）。成功返回 `{ "user_id": 9, "agent_level": 2 }`。
错误 reason：`AFFILIATE_APPOINT_FORBIDDEN`(403：不是一级 / 开关关闭)、`AFFILIATE_APPOINT_TARGET_INVALID`(400：不是直属且归属自己 / 等级状态不允许 / 被管理员锁定)、`AFFILIATE_APPOINT_LIMIT_REACHED`(409)、`AFFILIATE_APPOINT_NOT_OWNER`(403：只能取消自己任命的)。

## 4. 管理端

### GET `/api/v1/admin/affiliates/users`
Query：`search`、`agent_level`（1/2/3，可空）、`owner_id`（归属代理 ID，可空）、`page`、`page_size`。
无 search、无 agent_level、无 owner_id 时只列代理与有专属设置的用户；任一筛选存在时按筛选查全部用户。
条目：
```jsonc
{
  "user_id": 2, "email": "b@x.com", "username": "b",
  "aff_code": "VIP", "aff_code_custom": true, "aff_rebate_rate_percent": 25,
  "aff_count": 8,
  "agent_level": 2, "agent_level_locked": true,
  "agent_level_source": "agent",      // default | auto | manual | agent
  "agent_level_updated_by": 1, "agent_level_updated_by_email": "a@x.com",
  "agent_owner_id": 1, "agent_owner_email": "a@x.com", "agent_owner_level": 1, "agent_owner_deleted": false,
  "agent_upline_id": null, "agent_upline_email": "", "agent_upline_deleted": false,
  "owned_customer_count": 15,
  "agent_profit_total": 96.0          // 作为受益人收到的 agent_direct + agent_diff 合计
}
```

### GET `/api/v1/admin/affiliates/users/:user_id/customers`
分页。条目：`{ user_id, email, username, agent_level, relation: "owner" | "upline", bound_at, contribution }`，
relation=owner 表示该代理是客户的归属代理，upline 表示该代理是客户的上方一级（拿差额）；contribution 是该客户为该代理带来的代理分润合计。

### GET `/api/v1/admin/affiliates/users/:user_id/inviter/preview?inviter_id=X`
返回绑定后会写入的归属快照：`{ agent_owner_id, agent_owner_email, agent_owner_level, agent_upline_id, agent_upline_email }`（无归属时 id 为 null）。成环、已绑定等错误与正式绑定一致（`AFFILIATE_CYCLE` 400、`AFFILIATE_ALREADY_BOUND` 409）。

### GET `/api/v1/admin/affiliates/users/:user_id/overview`
删除 `rebate_rate_percent`；新增 `agent_rate_percent`（仅代理，number|null）、`agent_owner_*`、`agent_upline_*`（同列表）、`owned_customer_count`、`reward_totals: {agent, invite, invitee_bonus}`。

### GET `/api/v1/admin/affiliates/rebates`
新增 query `reward_type`（可空）。条目删除 `rebate_level`，新增 `reward_type`：
`agent_direct`（代理直属）/ `agent_diff`（一级差额）/ `invite_percent`（普通邀请·比例）/ `invite_fixed`（首充·邀请人）/ `invitee_bonus`（首充·被邀请人，受益人即被邀请人，直接进余额）。
排序键 `rebate_level` 改为 `reward_type`。

### GET `/api/v1/admin/affiliates/invites`
条目新增 `agent_owner_id`、`agent_owner_email`、`agent_owner_level`、`agent_owner_deleted`、`agent_upline_id`、`agent_upline_email`、`agent_upline_deleted`。

其余管理端接口（PUT/DELETE 用户设置、批量等级、批量比例、重新评估、提现、绑定）请求与响应不变；`agent_level_source` 新增取值 `agent`。
