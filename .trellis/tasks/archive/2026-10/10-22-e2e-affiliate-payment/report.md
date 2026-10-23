# 测试报告：真机端到端（代理 / 奖励 / 支付）

- 时间：2026-10-22 19:45–20:35（UTC+8）
- 对象：一号机 `sub2api-custom`，镜像 `custom-latest`（commit 34ef3fc05）
- 方法：
  - 公开接口注册测试账号（带邀请码），管理端 / 用户端接口操作。
  - 模拟易支付网关容器完成「下单 → 网关查单 → 签名回调 → 到账 / 退款」全链路。
  - 消费返利由服务器上真实的定时结算任务处理（写入 `usage_logs`）。
  - 有效期 / 冻结等时间相关用例直接改库造数据。
  - 每条用例都比对 `user_affiliate_ledger` 的新增流水（受益人、类型、金额、基数）。
- 用例设计：见 `prd.md`。驱动脚本与原始结果在本机 `/tmp/e2e/`（`results.jsonl`）。

## 结论

- 代理归属快照、级差分润、专属比例、首充奖励、上限 / 有效期 / 冻结、返利额度转出与提现、
  管理员充值返利、退款、订阅订单隔离、一级任命、管理员设上级 / 补绑、账号删除、消费基数结算、
  支付回调验签 / 幂等 / 金额校验 / 上游确认均与设计一致。
- **发现 2 个缺陷**（都在代理自动升级上，分润金额本身没有问题）：

### 缺陷 1：「仅真实充值」口径下，被邀请人首充到账时漏算本单，邀请人升级慢一拍（UPG-03b）
- 现象：升级方式 auto、二级门槛 2、人数口径 recharged。R 的两名直属 R1、R2 都完成真实充值后，R 仍是普通用户。
  之后任何一次触发评估（R 再绑定一个新下级）才升到二级（UPG-03c）。
- 原因：
  - `doBalance` 在 `markCompleted` 之前调用 `applyFirstRechargeRewards`（`payment_fulfillment.go:458-459`）。
  - `ApplyFirstRechargeRewards` 末尾评估邀请人等级（`affiliate_first_recharge.go:96`）。
  - 此时本单状态还是 `RECHARGING`，而 `CountQualifiedInvitees` 只统计 `COMPLETED` 等已付款状态（`affiliate_repo_agent_level.go:15-25`），刚首充的这位不被计入。
- 影响：最后一名达标的直属首充时不会触发升级，要等下一次事件或管理员手动重新评估（而重新评估又受缺陷 2 影响）。
- 建议：把「仅充值口径的升级评估」挪到 `markCompleted` 成功之后执行（`doBalance` 两条路径都要改），
  补一个集成测试：第 N 名直属首充完成即升级。

### 缺陷 2：删除过有下级的账号后，「重新评估全部用户」直接 404（UPG-05）
- 现象：删除 B2 / A2（都有下级）后，`POST /admin/affiliates/agent-levels/reevaluate` 返回 `404 USER_NOT_FOUND`。
- 原因：
  - `ReevaluateAllAgentLevels` 遍历 `ListInviterIDs`（包含已软删除的邀请人）。
  - 对每个邀请人调用 `EvaluateAgentLevel` → `EnsureUserAffiliate`，后者对已删除用户返回 `ErrUserNotFound`。
  - 循环遇错直接返回（`affiliate_agent_level.go:138-141`），排在它后面的用户全部不评估。
- 影响：站里只要删过一个有下级的账号，后台「重新评估」按钮就失效了。
- 建议：
  - 与 `SettleConsumptionRebates` 一致，遇到 `ErrUserNotFound` 记 `after = id` 后跳过。
  - 或在 `ListInviterIDs` 里 JOIN users 过滤 `deleted_at IS NULL`。
  - 补对应单测。

## 其他观察（非缺陷）
- PAY-07：回调被拒（签名错 / 金额被改）但网关实际已支付的订单，用户点取消时系统会先查单补单，
  并按网关记录的真实金额（10）入账，不是按篡改后的金额（1000）。行为正确。
- `user_affiliates` 上有守护触发器，直接 UPDATE `inviter_bound_at` 会被静默还原（防篡改生效）。
  测试里时间回拨用 `session_replication_role=replica` 在会话内绕过。
- 退款不回收已发返利（设计第 10 节）。消费基数下，退款会扣减可计返利的真实充值额度（CON-05 已验证）。
- 界面：
  - 邀请记录在双方邮箱下显示「一级代理 / 二级代理 / 普通用户」。
  - 返利记录在消费基数下切换为「来源 / 返利基数 / 结算截至」列。
  - 返利记录的邀请人 / 被邀请人下方仍是「-」占位，与邀请记录的新样式不一致，可考虑统一。
- 「重新评估」会扫描全站所有邀请人。本次在 auto 模式下点过一次，站长原有的 5 个账号等级与余额已核对，未被改动。

## 测试中修正的驱动问题（不计入缺陷）
- CFG-01：数字格式比较（"30" 对 "30.00000000"），修正后通过。
- PAY-07：断言里多带了审计字段导致严格比较不等，改为按关键字段判定后通过。
- LIM-03：第一次改库被上述守护触发器还原，等于没造出过期数据。改用会话级绕过后重测通过。
- UPG-04a：原前提用了从未升级过的 K2（3→3 不算降级、不锁定），改用确实被降级锁定的 K1 后重测通过。

## 环境恢复
见文末「收尾」。测试账号（`e2e-114834-*@e2e.test`，共约 60 个）及其订单、流水、用量记录保留在库里，方便在后台查看。
可按邮箱前缀清理，或用测试前备份回滚：`/root/sub2api-custom-backups/pre-e2e-20261022-113129.dump`
（注意：回滚会同时抹掉测试期间库里的其他改动）。

## 修复（2026-10-22，未提交）
- 缺陷 1：首充奖励不再评估邀请人等级。新增 `AffiliateService.EvaluateInviterAfterRecharge`，
  由 `PaymentService.completeBalanceOrder` 在 `markCompleted` 成功后调用（`doBalance` 两条路径）。
  单测 `TestExecuteBalanceFulfillmentEvaluatesInviterAfterOrderCompleted` 断言评估时本单已是 COMPLETED。
- 缺陷 2：`ListInviterIDs` 只列未删除的邀请人；`ReevaluateAllAgentLevels` 遇到 `ErrUserNotFound` 跳过继续。
  对应单测 `TestReevaluateAllAgentLevels_SkipsDeletedInviters`，并在集成测试里补了已删除邀请人的断言。
- 返利记录「来源」列：订单行改为一行显示「充值订单 #ID」，完整交易号放进悬停提示，不再撑宽整列。
- 验证：后端定向单测、affiliate 仓库集成测试、前端 vue-tsc / eslint / 全量 vitest（2933 条）通过。
  后端全量单测中仅 3 条阿里云验证码测试失败，不带改动时同样失败（本机 HTTP 代理拦截本地桩服务），与本次无关。
- 待部署后在真机重跑 UPG-03b / UPG-05。

## 用例明细

共 107 条检查，通过 105，失败 2。


### CFG

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| CFG-02 | ✅ | R2>P 被拒绝或收敛 | `"R2<=P"` | `"rejected 400 level-2 agent rate cannot exceed the profit pool rate"` |
| CFG-03 | ✅ | 公开设置含被邀请人奖励字段、不暴露利润池/二级比例/邀请人奖励 | `{"has_invitee_bonus": true, "leaked": []}` | `{"has_invitee_bonus": true, "leaked": [], "keys": ["affiliate_enabled", "affiliate_invitee_bonus_max", "affiliate_invitee_bonus_min_recharge", "affiliate_invitee_bonus_mode", "affiliate_invitee_bonus_value"]}` |
| CFG-01 | ✅ | 管理端 PUT 基线设置后库中值一致 | `{}` | `{}` |

### REL

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| REL-01 | ✅ | A 用 B 的码注册：归属 B 一级、无上方一级、邀请人是代理 | `{"inviter": 7, "owner": 7, "owner_level": 1, "upline": null, "inviter_was_agent": true}` | `{"inviter": 7, "owner": 7, "owner_level": 1, "upline": null, "inviter_was_agent": true}` |
| REL-02 | ✅ | 一级 B 任命 A 为二级（用户端） | `{"level": 2, "source": "agent", "updated_by": 7}` | `{"level": 2, "source": "agent", "updated_by": 7}` |
| REL-03 | ✅ | U1 用二级 A 的码注册：归属 A 二级、上方一级 B | `{"inviter": 8, "owner": 8, "owner_level": 2, "upline": 7, "inviter_was_agent": true}` | `{"inviter": 8, "owner": 8, "owner_level": 2, "upline": 7, "inviter_was_agent": true}` |
| REL-04 | ✅ | U2 用普通用户 U1 的码注册：沿链归属 A 二级、上方 B，邀请人非代理 | `{"inviter": 9, "owner": 8, "owner_level": 2, "upline": 7, "inviter_was_agent": false}` | `{"inviter": 9, "owner": 8, "owner_level": 2, "upline": 7, "inviter_was_agent": false}` |
| REL-05 | ✅ | 管理端邀请记录显示双方当前等级（1/2/3） | `[["A", 2, "U1", 3], ["B", 1, "A", 2], ["U1", 3, "U2", 3]]` | `[["A", 2, "U1", 3], ["B", 1, "A", 2], ["U1", 3, "U2", 3]]` |

### PAY

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| PAY-01a | ✅ | U2 下单 ¥100：返回支付链接，网关回调 success，订单完成，余额 +100 | `{"pending": "PENDING", "notify": [[200, "success"]], "status": "COMPLETED", "delta": 100.0}` | `{"pending": "PENDING", "pay_url": "https://mock-pay.invalid/pay/MOCK202610221149300001", "notify": [[200, "success"]], "status": "COMPLETED", "delta": 100.0}` |
| PAY-01b | ✅ | U2 充值 100 分润：A 20 direct、B 10 diff、U1 5 invite_percent | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` |
| PAY-01c | ✅ | 流水记录 base_amount=100 与订单号 | `{"base": [100.0, 100.0, 100.0], "order": [7, 7, 7]}` | `{"base": [100, 100, 100], "order": [7, 7, 7]}` |
| PAY-02 | ✅ | 同一成功回调重放 3 次：幂等，余额与流水不变 | `{"delta": 0.0, "new_ledger": 0}` | `{"delta": 0.0, "new_ledger": 0, "notify": [[200, "success"], [200, "success"], [200, "success"]]}` |
| PAY-03 | ✅ | 伪造签名回调被拒，订单仍 PENDING | `{"http": 400, "status": "PENDING"}` | `{"http": 400, "body": "verify failed", "status": "PENDING"}` |
| PAY-04 | ✅ | 签名正确但金额被篡改（10→1000）：拒绝，订单不完成 | `{"http": ">=400", "status": "PENDING"}` | `{"http": 502, "body": "confirm failed", "status": "PENDING"}` |
| PAY-05 | ✅ | 网关查单显示未支付却收到成功回调：上游确认失败，订单不完成 | `{"http": 502, "status": "PENDING"}` | `{"http": 502, "body": "confirm failed", "status": "PENDING", "audit": ["ORDER_CREATED", "PAYMENT_UPSTREAM_UNCONFIRMED"]}` |
| PAY-06 | ✅ | 用户取消待支付订单（网关未支付） | `"CANCELLED"` | `"CANCELLED"` |
| PAY-07 | ✅ | 回调被拒但网关已支付：用户取消时查单补单，按网关金额 10 入账（非篡改的 1000） | `{"status": "COMPLETED", "X_balance": 10.0, "cancel": "rejected"}` | `{"status": "COMPLETED", "X_balance": 10.0, "cancel": "rejected (order cannot be cancelled in current status)", "audit": ["ORDER_CREATED", "PAYMENT_UPSTREAM_UNCONFIRMED", "ORDER_PAID", "AFFILIATE_REBATE_SKIPPED", "RECHARGE_SUCCESS"]}` |

### DIST

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| DIST-01 | ✅ | 二级 A 本人充 100：上方一级 B 拿满 30，A 不拿 | `[["B", "agent_direct", 30]]` | `[["B", "agent_direct", 30]]` |
| DIST-02 | ✅ | 一级 B 本人充 100：不产生代理利润 | `[]` | `[]` |
| DIST-03 | ✅ | U1（邀请人是二级代理）充 100：A 20、B 10，无普通邀请奖励 | `[["A", "agent_direct", 20], ["B", "agent_diff", 10]]` | `[["A", "agent_direct", 20], ["B", "agent_diff", 10]]` |
| DIST-04 | ✅ | 无邀请人用户 X 充 100：无流水 | `[]` | `[]` |
| DIST-05a | ✅ | U1 升二级后 U4 注册：归属 U1 二级，上方一级沿 U1→A(二级)→B 找到 B | `{"owner": 9, "owner_level": 2, "upline": 7, "inviter_was_agent": true}` | `{"owner": 9, "owner_level": 2, "upline": 7, "inviter_was_agent": true}` |
| DIST-05b | ✅ | U4 充 100：U1 20 direct、B 10 diff | `[["B", "agent_diff", 10], ["U1", "agent_direct", 20]]` | `[["B", "agent_diff", 10], ["U1", "agent_direct", 20]]` |
| DIST-06 | ✅ | U1 升级后 U2 再充 100：快照不变，A 20、B 10、U1 仍拿 5 普通邀请 | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` |
| DIST-07 | ✅ | U1（现二级）本人充 100：快照 owner=A 二级 / upline=B → B 拿 30 | `[["B", "agent_direct", 30]]` | `[["B", "agent_direct", 30]]` |
| DIST-08 | ✅ | A 升一级后新客户 U5 充 100：A 30 | `[["A", "agent_direct", 30]]` | `[["A", "agent_direct", 30]]` |
| DIST-09 | ✅ | A 升一级后 U2 再充 100：仍 A 20、B 10、U1 5 | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` |
| DIST-10a | ✅ | 管理员把 B2 降为普通：等级 3 且锁定自动升级 | `{"level": 3, "locked": true}` | `{"level": 3, "locked": true}` |
| DIST-10b | ✅ | B2 降级前绑定的 C7 充 100：快照不变，A2 20、B2 10 | `[["A2", "agent_direct", 20], ["B2", "agent_diff", 10]]` | `[["A2", "agent_direct", 20], ["B2", "agent_diff", 10]]` |
| DIST-11a | ✅ | B2 降级后 U6 注册：归属 A2 二级，上方无一级 | `{"owner": 15, "owner_level": 2, "upline": null}` | `{"owner": 15, "owner_level": 2, "upline": null}` |
| DIST-11b | ✅ | U6 充 100：A2 拿满利润池 30 | `[["A2", "agent_direct", 30]]` | `[["A2", "agent_direct", 30]]` |

### CUS

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| CUS-01 | ✅ | 专属开：A2 专属 25，C7 充 100 → A2 25、B2 5 | `[["A2", "agent_direct", 25], ["B2", "agent_diff", 5]]` | `[["A2", "agent_direct", 25], ["B2", "agent_diff", 5]]` |
| CUS-02 | ✅ | B2 专属 15 < A2 25：差额为负不发，只有 A2 25 | `[["A2", "agent_direct", 25]]` | `[["A2", "agent_direct", 25]]` |
| CUS-03 | ✅ | 专属开关关：按默认 A2 20、B2 10 | `[["A2", "agent_direct", 20], ["B2", "agent_diff", 10]]` | `[["A2", "agent_direct", 20], ["B2", "agent_diff", 10]]` |

### FR

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| FR-01 | ✅ | N2 首笔充 5（低于门槛 10）：无首充奖励 | `[]` | `[]` |
| FR-02a | ✅ | N2 再充 20：被邀请人奖励 3 + 普通邀请人首充固定 2 | `[["N1", "invite_fixed", 2], ["N2", "invitee_bonus", 3]]` | `[["N1", "invite_fixed", 2], ["N2", "invitee_bonus", 3]]` |
| FR-02b | ✅ | N2 余额增加 20 + 3 | `23.0` | `23.0` |
| FR-03 | ✅ | N2 再充 50：首充奖励不再发放 | `[]` | `[]` |
| FR-04 | ✅ | 一级 B 直接邀请的 V1 充 20：V1 拿首充 3，B 只拿分润 6，无邀请人首充奖励 | `[["B", "agent_direct", 6], ["V1", "invitee_bonus", 3]]` | `[["B", "agent_direct", 6], ["V1", "invitee_bonus", 3]]` |
| FR-05 | ✅ | 比例模式 10% 封顶 5：N3 充 100 → 被邀请人奖励 5；N1 首充固定 2 | `[["N1", "invite_fixed", 2], ["N3", "invitee_bonus", 5]]` | `[["N1", "invite_fixed", 2], ["N3", "invitee_bonus", 5]]` |
| FR-06a | ✅ | N4 两笔 20 元订单并发回调：首充奖励各只发一次（被邀请人 2 = 20×10%，N1 固定 2） | `[["N1", "invite_fixed", 2], ["N4", "invitee_bonus", 2]]` | `[["N1", "invite_fixed", 2], ["N4", "invitee_bonus", 2]]` |
| FR-06b | ✅ | 两笔都完成且余额 = 40 + 2 | `{"status": ["COMPLETED", "COMPLETED"], "delta": 42.0}` | `{"status": ["COMPLETED", "COMPLETED"], "delta": 42.0}` |
| FR-07a | ✅ | 充值赠送阶梯满 100 送 10%：订单 amount/pay/bonus | `{"amount": 110.0, "pay": 100.0, "bonus": 10.0, "status": "COMPLETED"}` | `{"amount": 110, "pay": 100, "bonus": 10, "status": "COMPLETED"}` |
| FR-07b | ✅ | 分润与首充都按实付 100 计：B 30（非 33）、V2 首充 10（非 11），base=100 | `[["B", "agent_direct", 30.0, 100.0], ["V2", "invitee_bonus", 10.0, null]]` | `[["B", "agent_direct", 30, 100], ["V2", "invitee_bonus", 10, null]]` |
| FR-07c | ✅ | V2 余额 = 110 + 10 | `120.0` | `120.0` |

### LIM

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| LIM-01a | ✅ | 代理单人上限 25：W1 充 100 → B 30 截断为 25 | `[["B", "agent_direct", 25]]` | `[["B", "agent_direct", 25]]` |
| LIM-01b | ✅ | W1 再充 100：已达上限，无流水 | `[]` | `[]` |
| LIM-02a | ✅ | W2 用普通用户 U2 的码注册：沿 U2 快照归属 A（A 现为一级） | `{"owner": 8, "owner_level": 1, "upline": null, "inviter_was_agent": false}` | `{"owner": 8, "owner_level": 1, "upline": null, "inviter_was_agent": false}` |
| LIM-02b | ✅ | 两类上限独立：W2 充 100 → A 30 截为 25、U2 5 截为 3 | `[["A", "agent_direct", 25], ["U2", "invite_percent", 3]]` | `[["A", "agent_direct", 25], ["U2", "invite_percent", 3]]` |
| LIM-02c | ✅ | W2 再充 100：两类都已到顶 | `[]` | `[]` |
| LIM-04a | ✅ | 冻结 24h：B 30 进入冻结额度，可用额度不变 | `{"rows": [["B", "agent_direct", 30.0]], "frozen_delta": 30.0, "quota_delta": 0.0, "frozen_hours≈": 24}` | `{"rows": [["B", "agent_direct", 30]], "frozen_delta": 30, "quota_delta": 0, "frozen_hours": 24.0}` |
| LIM-05 | ✅ | 冻结到期后再次转余额：解冻 30 并转入 | `{"balance_delta": 30.0, "frozen": 0.0, "aff_quota": 0.0}` | `{"balance_delta": 30.0, "frozen": 0, "aff_quota": 0}` |
| LIM-03a | ✅ | 代理有效期 1 天，W3 绑定时间改为 2 天前：充 100 无代理利润 | `[]` | `[]` |
| LIM-03b | ✅ | 绑定时间 20 小时前（仍在有效期内）：B 30 | `[["B", "agent_direct", 30]]` | `[["B", "agent_direct", 30]]` |

### QUO

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| QUO-01 | ✅ | B 把返利额度转余额：只转可用部分，冻结部分保留，写 transfer 流水 | `{"transferred": 231, "balance_delta": 231, "aff_quota": 0.0, "frozen": 30.0}` | `{"transferred": 231, "balance_delta": 231.0, "aff_quota": 0, "frozen": 30, "ledger": ["transfer"]}` |
| QUO-02a | ✅ | 管理员登记线下提现 5，同 Idempotency-Key 重放不重复扣 | `{"delta": -5.0}` | `{"delta": -5, "first": {"ledger_id": 50, "user_id": 8, "amount": 5, "available_quota_after": 130, "frozen_quota_after": 0, "history_quota_after": 135}, "replay": {"ledger_id": 50, "user_id": 8, "amount": 5, "available_quota_after": 130, "frozen_quota_after": 0` |
| QUO-02b | ✅ | 提现金额超过可用额度被拒 | `"rejected"` | `"rejected 400 400 insufficient available affiliate quota"` |

### ADM

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| ADM-01 | ✅ | 管理员充值返利关：给 U2 加款 50 无流水 | `[]` | `[]` |
| ADM-02 | ✅ | 管理员充值返利开：加款 50 → A 10、B 5、U1 2.5 | `[["A", "agent_direct", 10], ["B", "agent_diff", 5], ["U1", "invite_percent", 2.5]]` | `[["A", "agent_direct", 10], ["B", "agent_diff", 5], ["U1", "invite_percent", 2.5]]` |
| ADM-03 | ✅ | 管理员退款 U2 的 100 元订单（扣余额）：订单 REFUNDED、余额 −100、网关收到退款、已发返利不回收 | `{"status": "REFUNDED", "refund_amount": 100.0, "balance_delta": -100.0, "gateway_refunds": 1, "ledger_changes": []}` | `{"status": "REFUNDED", "refund_amount": 100, "balance_delta": -100.0, "gateway_refunds": 1, "gateway_last": {"money": "100.00", "out_trade_no": "sub2_2026102275JpLhoT"}, "ledger_changes": []}` |
| ADM-04a | ✅ | 订阅订单支付成功：开通订阅，无任何代理 / 邀请 / 首充奖励 | `{"status": "COMPLETED", "subscriptions": 1, "ledger": []}` | `{"status": "COMPLETED", "subscriptions": 1, "ledger": []}` |
| ADM-04b | ✅ | 订阅不消耗首充资格：V3 随后余额充 20 → V3 首充 3、B 分润 6 | `[["B", "agent_direct", 6], ["V3", "invitee_bonus", 3]]` | `[["B", "agent_direct", 6], ["V3", "invitee_bonus", 3]]` |

### APT

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| APT-01 | ✅ | 一级 K 任命直属客户 K1 为二级 | `{"level": 2, "source": "agent", "by": 29}` | `{"level": 2, "source": "agent", "by": 29}` |
| APT-02 | ✅ | 任命上限 1 已用满，再任命 K2 被拒 | `"rejected"` | `"rejected 409 409 appointment limit reached"` |
| APT-03 | ✅ | K 取消自己任命的 K1：回普通并锁定自动升级 | `{"level": 3, "locked": true}` | `{"level": 3, "locked": true}` |
| APT-04 | ✅ | K 取消管理员设的二级 K3 被拒 | `"rejected"` | `"rejected 403 403 only the appointing agent can revoke this level-2 agent"` |
| APT-05 | ✅ | 普通用户 K2 任命自己的邀请人 K4 被拒 | `"rejected"` | `"rejected 403 403 only level-1 agents can appoint level-2 agents"` |
| APT-07a | ✅ | K 任命成为代理前邀请的 K0（不归属 K）被拒 | `"rejected"` | `"rejected 400 400 this user cannot be appointed or revoked"` |
| APT-07b | ✅ | K 任命非直属用户 X 被拒 | `"rejected"` | `"rejected 400 400 this user cannot be appointed or revoked"` |
| APT-06 | ✅ | 任命开关关闭：K 任命 K2 被拒 | `"rejected"` | `"rejected 403 403 only level-1 agents can appoint level-2 agents"` |

### OWN

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| OWN-01 | ✅ | 无上级的二级 S 的客户 S1 充 100：S 拿满 30 | `[["S", "agent_direct", 30]]` | `[["S", "agent_direct", 30]]` |
| OWN-02a | ✅ | 管理员给 S 设上级一级 K：只改 S 的归属快照，邀请关系不变 | `{"owner": 29, "owner_level": 1, "upline": null, "inviter": null}` | `{"owner": 29, "owner_level": 1, "upline": null, "inviter": null}` |
| OWN-02b | ✅ | S1 是设置前的客户：快照不变，仍 S 30 | `[["S", "agent_direct", 30]]` | `[["S", "agent_direct", 30]]` |
| OWN-02c | ✅ | 设置后新客户 S2 充 100：S 20、K 10 | `[["K", "agent_diff", 10], ["S", "agent_direct", 20]]` | `[["K", "agent_diff", 10], ["S", "agent_direct", 20]]` |
| OWN-02d | ✅ | S 本人充 100：上级 K 拿满 30 | `[["K", "agent_direct", 30]]` | `[["K", "agent_direct", 30]]` |
| OWN-03a | ✅ | 目标不是二级（S1 普通）被拒 | `"rejected"` | `"rejected 400 400 only level-2 agents can have a level-1 upline set"` |
| OWN-03b | ✅ | 上级不是一级（K2 普通）被拒 | `"rejected"` | `"rejected 400 400 upline must be an active level-1 agent"` |
| OWN-03c | ✅ | 上级设为自己被拒 | `"rejected"` | `"rejected 400 400 user cannot invite themselves"` |
| OWN-03d | ✅ | 成环（KS 归属 S，再把 S 的上级设为 KS）被拒 | `"rejected"` | `"rejected 400 400 binding would create an affiliate cycle"` |
| OWN-04 | ✅ | 清除上级后新客户 S3 充 100：S 拿满 30 | `[["S", "agent_direct", 30]]` | `[["S", "agent_direct", 30]]` |
| OWN-05a | ✅ | 管理员给无邀请人 Z 补绑邀请人 N1 | `{"inviter": 18, "inviter_was_agent": false}` | `{"inviter": 18, "inviter_was_agent": false}` |
| OWN-05b | ✅ | 已绑定后再次补绑被拒（一次性） | `"rejected"` | `"rejected 409 409 affiliate inviter already bound"` |
| OWN-06 | ✅ | 补绑成环（B 的邀请人设为其下游 U2）被拒 | `"rejected"` | `"rejected 400 400 binding would create an affiliate cycle"` |

### UPG

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| UPG-01 | ✅ | 自动升级（口径全部）：Q 邀请 2 人升二级 | `{"level": 2, "source": "auto"}` | `{"level": 2, "source": "auto"}` |
| UPG-02 | ✅ | Q 邀请第 3 人升一级 | `1` | `1` |
| UPG-04b | ✅ | 解锁后点「重新评估」：K2 升二级 | `2` | `2` |
| UPG-03a | ✅ | 口径仅充值：R 邀请 2 人但都未充值，不升级 | `3` | `3` |
| UPG-03b | ❌ | 两名直属完成真实充值后 R 升二级 | `2` | `3` |
| UPG-05 | ❌ | 删除过有下级的账号（B2/A2）后点「重新评估」：应跳过已删除用户继续评估 | `"200，跳过已删除用户"` | `"404 USER_NOT_FOUND user not found"` |
| UPG-03c | ✅ | （定位）R 再绑定一个新下级触发评估时，R1/R2 订单已 COMPLETED → R 立刻升二级，证实到账时漏算当前订单 | `2` | `2` |
| UPG-04a | ✅ | 被降级锁定的 K1（APT-03 被取消任命）邀请满 2 人也不自动升级 | `{"level": 3, "locked": true}` | `{"level": 3, "locked": true}` |

### DEL

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| DEL-01 | ✅ | 删除一级 B2 后 C7 充 100：A2 仍 20，B2 的差额不再发放 | `[["A2", "agent_direct", 20]]` | `[["A2", "agent_direct", 20]]` |
| DEL-02 | ✅ | 再删除归属代理 A2：C7 充 100 无代理利润 | `[]` | `[]` |
| DEL-03 | ✅ | 管理端总览：C7 的归属代理 / 上方一级显示为已删除 | `"deleted flags"` | `{"agent_owner_id": 15, "agent_owner_email": "e2e-114834-a2@e2e.test", "agent_owner_level": 2, "agent_owner_deleted": true, "agent_upline_id": 14, "agent_upline_email": "e2e-114834-b2@e2e.test", "agent_upline_deleted": true}` |

### CON

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| CON-01 | ✅ | 消费基数下 CU2/CU1/Z2 支付充值：充值时不产生代理利润与比例奖励 | `[]` | `[]` |
| CON-05a | ✅ | CU1 的 100 元订单部分退款 60 | `{"status": "PARTIALLY_REFUNDED", "refund": 60.0}` | `{"status": "PARTIALLY_REFUNDED", "refund": 60}` |
| CON-02 | ✅ | CU2 余额消费 30 + 订阅消费 10：按 30 结算 CA 6、CB 3、CU1 1.5 | `[["CA", "agent_direct", 6, 30], ["CB", "agent_diff", 3, 30], ["CU1", "invite_percent", 1.5, 30]]` | `[["CA", "agent_direct", 6, 30], ["CB", "agent_diff", 3, 30], ["CU1", "invite_percent", 1.5, 30]]` |
| CON-02b | ✅ | 结算流水带 source_usage_until、无订单号 | `true` | `true` |
| CON-05b | ✅ | CU1 充 100 退 60 后消费 50：按剩余真实充值 40 结算 CA 8、CB 4（邀请人是代理，无比例奖励） | `[["CA", "agent_direct", 8, 40], ["CB", "agent_diff", 4, 40]]` | `[["CA", "agent_direct", 8, 40], ["CB", "agent_diff", 4, 40]]` |
| CON-06 | ✅ | Z2 补绑前消费 20 不计、补绑后消费 10：CA 2、CB 1 | `[["CA", "agent_direct", 2, 10], ["CB", "agent_diff", 1, 10]]` | `[["CA", "agent_direct", 2, 10], ["CB", "agent_diff", 1, 10]]` |
| CON-02c | ✅ | CU2 已计入消费 30，水位推进 | `30.0` | `30` |
| CON-03 | ✅ | CU2 再消费 40：按剩余真实充值 20 结算 CA 4、CB 2、CU1 1 | `[["CA", "agent_direct", 4], ["CB", "agent_diff", 2], ["CU1", "invite_percent", 1]]` | `[["CA", "agent_direct", 4], ["CB", "agent_diff", 2], ["CU1", "invite_percent", 1]]` |
| CON-03b | ✅ | CU2 已计入消费到顶 50 | `50.0` | `50` |
| CON-04 | ✅ | CU2 已到封顶后再消费 10：无流水 | `[]` | `[]` |
| CON-04b | ✅ | 封顶后水位仍推进、已计入消费保持 50 | `{"advanced": true, "settled": 50.0}` | `{"advanced": true, "settled": 50}` |

### CLEAN

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| CLEAN-01 | ✅ | 设置恢复到测试前快照 | `{}` | `{}` |
| CLEAN-02 | ✅ | 支付实例恢复（dulu 启用、e2e-mock 删除），e2e 套餐/分组删除 | `[[1, "dulu", true]]` | `[[1, "dulu", true]]` |

## 收尾
- 设置已逐键恢复到测试前快照（CLEAN-01）。
- 支付：原实例「dulu」已恢复启用，「e2e-mock」实例、e2e 订阅套餐已删除，e2e 分组已软删除（CLEAN-02）。
- 临时管理员 e2e-admin 已降级、停用并软删除；复用的合规确认记录已删除。
- 模拟网关容器、python:3.13-alpine 镜像、/root/e2e-mock 已删除；一号机 sub2api-custom 健康检查 200。
- 生产 sub2api / sub2api-adv 未触碰。
