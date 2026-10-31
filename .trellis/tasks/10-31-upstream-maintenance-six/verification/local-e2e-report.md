# e2e 结果（20261031-072240）

共 442 条检查，通过 442，失败 0。


## CFG

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| CFG-01 | ✅ | 管理端 PUT 基线设置后库中值一致 | `{}` | `{}` |
| CFG-02 | ✅ | R2>P 被拒绝或收敛 | `"R2<=P"` | `"rejected 400 level-2 agent rate cannot exceed the profit pool rate"` |
| CFG-03 | ✅ | 公开设置含被邀请人奖励字段、不暴露利润池/二级比例/邀请人奖励 | `{"has_invitee_bonus": true, "leaked": []}` | `{"has_invitee_bonus": true, "leaked": [], "keys": ["affiliate_enabled", "affiliate_invitee_bonus_max", "affiliate_invitee_bonus_min_recharge", "affiliate_invitee_bonus_mode", "affiliate_invitee_bonus_value", "affiliate_invitee_signup_bonus"]}` |

## REL

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| REL-01 | ✅ | A 用 B 的码注册：归属 B 一级、无上方一级、邀请人是代理 | `{"inviter": 2458, "owner": 2458, "owner_level": 1, "upline": null, "inviter_was_agent": true}` | `{"inviter": 2458, "owner": 2458, "owner_level": 1, "upline": null, "inviter_was_agent": true}` |
| REL-02 | ✅ | 一级 B 任命 A 为二级（用户端） | `{"level": 2, "source": "agent", "updated_by": 2458}` | `{"level": 2, "source": "agent", "updated_by": 2458}` |
| REL-03 | ✅ | U1 用二级 A 的码注册：归属 A 二级、上方一级 B | `{"inviter": 2459, "owner": 2459, "owner_level": 2, "upline": 2458, "inviter_was_agent": true}` | `{"inviter": 2459, "owner": 2459, "owner_level": 2, "upline": 2458, "inviter_was_agent": true}` |
| REL-04 | ✅ | U2 用普通用户 U1 的码注册：沿链归属 A 二级、上方 B，邀请人非代理 | `{"inviter": 2460, "owner": 2459, "owner_level": 2, "upline": 2458, "inviter_was_agent": false}` | `{"inviter": 2460, "owner": 2459, "owner_level": 2, "upline": 2458, "inviter_was_agent": false}` |
| REL-05 | ✅ | 管理端邀请记录显示双方当前等级（1/2/3） | `[["A", 2, "U1", 3], ["B", 1, "A", 2], ["U1", 3, "U2", 3]]` | `[["A", 2, "U1", 3], ["B", 1, "A", 2], ["U1", 3, "U2", 3]]` |

## PAY

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| PAY-01a | ✅ | U2 下单 ¥100：返回支付链接，网关回调 success，订单完成，余额 +100 | `{"pending": "PENDING", "notify": [[200, "success"]], "status": "COMPLETED", "delta": 100.0}` | `{"pending": "PENDING", "pay_url": "https://mock-pay.invalid/pay/MOCK202610310722420001", "notify": [[200, "success"]], "status": "COMPLETED", "delta": 100.0}` |
| PAY-01b | ✅ | U2 充值 100 分润：A 20 direct、B 10 diff、U1 5 invite_percent | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` |
| PAY-01c | ✅ | 流水记录 base_amount=100 与订单号 | `{"base": [100.0, 100.0, 100.0], "order": [1231, 1231, 1231]}` | `{"base": [100, 100, 100], "order": [1231, 1231, 1231]}` |
| PAY-02 | ✅ | 同一成功回调重放 3 次：幂等，余额与流水不变 | `{"delta": 0.0, "new_ledger": 0}` | `{"delta": 0.0, "new_ledger": 0, "notify": [[200, "success"], [200, "success"], [200, "success"]]}` |
| PAY-03 | ✅ | 伪造签名回调被拒，订单仍 PENDING | `{"http": 400, "status": "PENDING"}` | `{"http": 400, "body": "verify failed", "status": "PENDING"}` |
| PAY-04 | ✅ | 签名正确但金额被篡改（10→1000）：拒绝，订单不完成 | `{"http": ">=400", "status": "PENDING"}` | `{"http": 502, "body": "confirm failed", "status": "PENDING"}` |
| PAY-05 | ✅ | 网关查单显示未支付却收到成功回调：上游确认失败，订单不完成 | `{"http": ">=400", "status": "PENDING"}` | `{"http": 502, "body": "confirm failed", "status": "PENDING"}` |
| PAY-07 | ✅ | 回调被拒但网关已支付：用户取消时查单补单，按网关金额 10 入账（非篡改的 1000） | `{"status": "COMPLETED", "X_delta": 10.0}` | `{"status": "COMPLETED", "X_delta": 10.0, "cancel": "accepted"}` |
| PAY-06 | ✅ | 用户取消待支付订单（网关未支付） | `"CANCELLED"` | `"CANCELLED"` |

## DIST

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| DIST-01 | ✅ | 二级 A 本人充 100：上方一级 B 拿满 30，A 不拿 | `[["B", "agent_direct", 30]]` | `[["B", "agent_direct", 30]]` |
| DIST-02 | ✅ | 一级 B 本人充 100：不产生代理利润 | `[]` | `[]` |
| DIST-03 | ✅ | U1（邀请人是二级代理）充 100：A 20、B 10，无普通邀请奖励 | `[["A", "agent_direct", 20], ["B", "agent_diff", 10]]` | `[["A", "agent_direct", 20], ["B", "agent_diff", 10]]` |
| DIST-04 | ✅ | 无邀请人用户 X 充 100：无流水 | `[]` | `[]` |
| DIST-05a | ✅ | U1 升二级后 U4 注册：归属 U1 二级，上方一级沿 U1→A(二级)→B 找到 B | `{"owner": 2460, "owner_level": 2, "upline": 2458, "inviter_was_agent": true}` | `{"owner": 2460, "owner_level": 2, "upline": 2458, "inviter_was_agent": true}` |
| DIST-05b | ✅ | U4 充 100：U1 20 direct、B 10 diff | `[["B", "agent_diff", 10], ["U1", "agent_direct", 20]]` | `[["B", "agent_diff", 10], ["U1", "agent_direct", 20]]` |
| DIST-06 | ✅ | U1 升级后 U2 再充 100：快照不变，A 20、B 10、U1 仍拿 5 普通邀请 | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` |
| DIST-07 | ✅ | U1（现二级）本人充 100：快照 owner=A 二级 / upline=B → B 拿 30 | `[["B", "agent_direct", 30]]` | `[["B", "agent_direct", 30]]` |
| DIST-08 | ✅ | A 升一级后新客户 U5 充 100：A 30 | `[["A", "agent_direct", 30]]` | `[["A", "agent_direct", 30]]` |
| DIST-09 | ✅ | A 升一级后 U2 再充 100：仍 A 20、B 10、U1 5 | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` | `[["A", "agent_direct", 20], ["B", "agent_diff", 10], ["U1", "invite_percent", 5]]` |
| DIST-10a | ✅ | 管理员把 B2 降为普通：等级 3 且锁定自动升级 | `{"level": 3, "locked": true}` | `{"level": 3, "locked": true}` |
| DIST-10b | ✅ | B2 降级前绑定的 C7 充 100：快照不变，A2 20、B2 10 | `[["A2", "agent_direct", 20], ["B2", "agent_diff", 10]]` | `[["A2", "agent_direct", 20], ["B2", "agent_diff", 10]]` |
| DIST-11a | ✅ | B2 降级后 U6 注册：归属 A2 二级，上方无一级 | `{"owner": 2466, "owner_level": 2, "upline": null}` | `{"owner": 2466, "owner_level": 2, "upline": null}` |
| DIST-11b | ✅ | U6 充 100：A2 拿满利润池 30 | `[["A2", "agent_direct", 30]]` | `[["A2", "agent_direct", 30]]` |

## CUS

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| CUS-01 | ✅ | 专属开：A2 专属 25，C7 充 100 → A2 25、B2 5 | `[["A2", "agent_direct", 25], ["B2", "agent_diff", 5]]` | `[["A2", "agent_direct", 25], ["B2", "agent_diff", 5]]` |
| CUS-02 | ✅ | B2 专属 15 < A2 25：差额为负不发，只有 A2 25 | `[["A2", "agent_direct", 25]]` | `[["A2", "agent_direct", 25]]` |
| CUS-03 | ✅ | 专属开关关：按默认 A2 20、B2 10 | `[["A2", "agent_direct", 20], ["B2", "agent_diff", 10]]` | `[["A2", "agent_direct", 20], ["B2", "agent_diff", 10]]` |

## FR

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

## LIM

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| LIM-01a | ✅ | 代理单人上限 25：W1 充 100 → B 30 截断为 25 | `[["B", "agent_direct", 25]]` | `[["B", "agent_direct", 25]]` |
| LIM-01b | ✅ | W1 再充 100：已达上限，无流水 | `[]` | `[]` |
| LIM-02a | ✅ | W2 用普通用户 U2 的码注册：沿 U2 快照归属 A（A 现为一级） | `{"owner": 2459, "owner_level": 1, "upline": null, "inviter_was_agent": false}` | `{"owner": 2459, "owner_level": 1, "upline": null, "inviter_was_agent": false}` |
| LIM-02b | ✅ | 两类上限独立：W2 充 100 → A 30 截为 25、U2 5 截为 3 | `[["A", "agent_direct", 25], ["U2", "invite_percent", 3]]` | `[["A", "agent_direct", 25], ["U2", "invite_percent", 3]]` |
| LIM-02c | ✅ | W2 再充 100：两类都已到顶 | `[]` | `[]` |
| LIM-03a | ✅ | 代理有效期 1 天，W3 绑定时间改为 2 天前：充 100 无代理利润 | `[]` | `[]` |
| LIM-03b | ✅ | 绑定时间 20 小时前（仍在有效期内）：B 30 | `[["B", "agent_direct", 30]]` | `[["B", "agent_direct", 30]]` |
| LIM-04a | ✅ | 冻结 24h：B 30 进入冻结额度，可用额度不变 | `{"rows": [["B", "agent_direct", 30.0]], "frozen_delta": 30.0, "quota_delta": 0.0, "frozen_hours≈": 24}` | `{"rows": [["B", "agent_direct", 30]], "frozen_delta": 30, "quota_delta": 0, "frozen_hours": 24.0}` |
| LIM-05 | ✅ | 冻结到期后再次转余额：解冻 30 并转入 | `{"balance_delta": 30.0, "frozen": 0.0, "aff_quota": 0.0}` | `{"balance_delta": 30.0, "frozen": 0, "aff_quota": 0}` |

## QUO

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| QUO-01 | ✅ | B 把返利额度转余额：只转可用部分，冻结部分保留，写 transfer 流水 | `{"transferred": 201, "balance_delta": 201, "aff_quota": 0.0, "frozen": 30.0}` | `{"transferred": 201, "balance_delta": 201.0, "aff_quota": 0, "frozen": 30, "ledger": ["transfer"]}` |
| QUO-02a | ✅ | 管理员登记线下提现 5，同 Idempotency-Key 重放不重复扣 | `{"delta": -5.0}` | `{"delta": -5, "first": {"ledger_id": 1783, "user_id": 2459, "amount": 5, "available_quota_after": 130, "frozen_quota_after": 0, "history_quota_after": 135}, "replay": {"ledger_id": 1783, "user_id": 2459, "amount": 5, "available_quota_after": 130, "frozen_quota` |
| QUO-02b | ✅ | 提现金额超过可用额度被拒 | `"rejected"` | `"rejected 400 400 insufficient available affiliate quota"` |

## SGB

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| SGB-01 | ✅ | 普通用户邀请：注册即得默认余额+5，自己作为受益人且无订单 | `5.0` | `5.0` |
| SGB-01b | ✅ | 注册赠送独立 action/reward_type，双方都是被邀请人 | `true` | `true` |
| SGB-02 | ✅ | 代理直接邀请也赠送5 | `5.0` | `5.0` |
| SGB-03 | ✅ | 我的邀请页可看到已领取金额及好友注册赠送 | `[5.0, 5.0]` | `[5, 5]` |
| SGB-04 | ✅ | 余额历史注册赠送独立类型/赠送汇总，实收为0 | `[1, "invitee_signup_bonus", 0, 5]` | `[1, "invitee_signup_bonus", 0, 5]` |
| SGB-05 | ✅ | 首充依然可领3且注册赠送不会重复 | `28.0` | `28.0` |
| SGB-05b | ✅ | 叠加后两种被邀请人赠送各一条 | `["invitee_bonus", "invitee_signup_bonus"]` | `["invitee_bonus", "invitee_signup_bonus"]` |
| SGB-06 | ✅ | 无邀请码不赠送 | `0.0` | `0.0` |
| SGB-07 | ✅ | 管理员补绑后不补发 | `0.0` | `0.0` |
| SGB-08 | ✅ | 无效邀请码正常注册，不赠送且无绑定 | `[0.0, 0]` | `[0, 0]` |
| SGB-09 | ✅ | 金额0不发放且公开设置为0 | `[0.0, 0.0]` | `[0.0, 0]` |
| SGB-10 | ✅ | 总开关关闭不绑定、不发放且公开金额0 | `[0.0, 0.0]` | `[0.0, 0]` |
| SGB-11 | ✅ | 设置负数金额被拒绝 | `"rejected"` | `"rejected 400 400 注册赠送金额必须为非负有限数值"` |
| SGB-12 | ✅ | 赠送不计消费返利真实充值基数 | `20.0` | `20` |

## SGS

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| SGS-01-all-SGSAGENT | ✅ | 匿名预览只返回适用金额 | `{"amount": 5}` | `{"amount": 5}` |
| SGS-02-all-SGSAGENT | ✅ | 我的邀请提示与来源匹配 | `5` | `5` |
| SGS-03-all-SGSAGENT | ✅ | 真实注册发放与来源匹配 | `5` | `5.0` |
| SGS-04-all-SGSAGENT | ✅ | 账本条数与赠送一致 | `1` | `1` |
| SGS-01-all-SGSUSER | ✅ | 匿名预览只返回适用金额 | `{"amount": 5}` | `{"amount": 5}` |
| SGS-02-all-SGSUSER | ✅ | 我的邀请提示与来源匹配 | `5` | `5` |
| SGS-03-all-SGSUSER | ✅ | 真实注册发放与来源匹配 | `5` | `5.0` |
| SGS-04-all-SGSUSER | ✅ | 账本条数与赠送一致 | `1` | `1` |
| SGS-01-agent-SGSAGENT | ✅ | 匿名预览只返回适用金额 | `{"amount": 5}` | `{"amount": 5}` |
| SGS-02-agent-SGSAGENT | ✅ | 我的邀请提示与来源匹配 | `5` | `5` |
| SGS-03-agent-SGSAGENT | ✅ | 真实注册发放与来源匹配 | `5` | `5.0` |
| SGS-04-agent-SGSAGENT | ✅ | 账本条数与赠送一致 | `1` | `1` |
| SGS-01-agent-SGSUSER | ✅ | 匿名预览只返回适用金额 | `{"amount": 0}` | `{"amount": 0}` |
| SGS-02-agent-SGSUSER | ✅ | 我的邀请提示与来源匹配 | `0` | `0` |
| SGS-03-agent-SGSUSER | ✅ | 真实注册发放与来源匹配 | `0` | `0.0` |
| SGS-04-agent-SGSUSER | ✅ | 账本条数与赠送一致 | `0` | `0` |
| SGS-01-user-SGSAGENT | ✅ | 匿名预览只返回适用金额 | `{"amount": 0}` | `{"amount": 0}` |
| SGS-02-user-SGSAGENT | ✅ | 我的邀请提示与来源匹配 | `0` | `0` |
| SGS-03-user-SGSAGENT | ✅ | 真实注册发放与来源匹配 | `0` | `0.0` |
| SGS-04-user-SGSAGENT | ✅ | 账本条数与赠送一致 | `0` | `0` |
| SGS-01-user-SGSUSER | ✅ | 匿名预览只返回适用金额 | `{"amount": 5}` | `{"amount": 5}` |
| SGS-02-user-SGSUSER | ✅ | 我的邀请提示与来源匹配 | `5` | `5` |
| SGS-03-user-SGSUSER | ✅ | 真实注册发放与来源匹配 | `5` | `5.0` |
| SGS-04-user-SGSUSER | ✅ | 账本条数与赠送一致 | `1` | `1` |
| SGS-05 | ✅ | 修改金额后预览更新 | `{"amount": 10}` | `{"amount": 10}` |
| SGS-06 | ✅ | 已领取金额保持5 | `5.0` | `5.0` |
| SGS-07 | ✅ | 之后注册领取10 | `10.0` | `10.0` |
| SGS-08 | ✅ | 无效邀请码不展示金额 | `{"amount": 0}` | `{"amount": 0}` |
| SGS-09 | ✅ | 旧客户端省略来源仍保留agent | `"agent"` | `"agent"` |
| SGS-10 | ✅ | 非法来源被拒绝 | `"rejected"` | `"rejected 400 400 Invalid signup gift invitation source"` |
| SGS-11 | ✅ | 触发升级的邀请仍按普通用户赠送 | `[5.0, false, 2]` | `[5.0, false, 2]` |

## ADM

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| ADM-01 | ✅ | 赠送加款 50 永不产生返利 | `[]` | `[]` |
| ADM-02 | ✅ | 线下充值 50 与在线相同：A 10、B 5、U1 2.5 | `[["A", "agent_direct", 10], ["B", "agent_diff", 5], ["U1", "invite_percent", 2.5]]` | `[["A", "agent_direct", 10], ["B", "agent_diff", 5], ["U1", "invite_percent", 2.5]]` |
| ADM-02b | ✅ | 线下订单完成、关联调账，返利都有 source order | `true` | `true` |
| ADM-03 | ✅ | 管理员退款 U2 的 100 元订单（扣余额）：订单 REFUNDED、余额 −100、网关收到退款、已发返利不回收 | `{"status": "REFUNDED", "refund_amount": 100.0, "balance_delta": -100.0, "gateway_refunds": 1, "ledger_changes": []}` | `{"status": "REFUNDED", "refund_amount": 100, "balance_delta": -100.0, "gateway_refunds": 1, "gateway_last": {"money": "100.00", "out_trade_no": "sub2_20261031vOlFrQth"}, "ledger_changes": []}` |
| ADM-04a | ✅ | 订阅订单支付成功：开通订阅，无任何代理 / 邀请 / 首充奖励 | `{"status": "COMPLETED", "subscriptions": 1, "ledger": []}` | `{"status": "COMPLETED", "subscriptions": 1, "ledger": []}` |
| ADM-04b | ✅ | 订阅不消耗首充资格：V3 随后余额充 20 → V3 首充 3、B 分润 6 | `[["B", "agent_direct", 6], ["V3", "invitee_bonus", 3]]` | `[["B", "agent_direct", 6], ["V3", "invitee_bonus", 3]]` |

## APT

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| APT-01 | ✅ | 一级 K 任命直属客户 K1 为二级 | `{"level": 2, "source": "agent", "by": 2497}` | `{"level": 2, "source": "agent", "by": 2497}` |
| APT-02 | ✅ | 任命上限 1 已用满，再任命 K2 被拒 | `"rejected"` | `"rejected 409 409 appointment limit reached"` |
| APT-03 | ✅ | K 取消自己任命的 K1：回普通并锁定自动升级 | `{"level": 3, "locked": true}` | `{"level": 3, "locked": true}` |
| APT-04 | ✅ | K 取消管理员设的二级 K3 被拒 | `"rejected"` | `"rejected 403 403 only the appointing agent can revoke this level-2 agent"` |
| APT-05 | ✅ | 普通用户 K2 任命自己的邀请人 K4 被拒 | `"rejected"` | `"rejected 403 403 only level-1 agents can appoint level-2 agents"` |
| APT-07a | ✅ | K 任命成为代理前邀请的 K0（不归属 K）被拒 | `"rejected"` | `"rejected 400 400 this user cannot be appointed or revoked"` |
| APT-07b | ✅ | K 任命非直属用户 X 被拒 | `"rejected"` | `"rejected 400 400 this user cannot be appointed or revoked"` |
| APT-06 | ✅ | 任命开关关闭：K 任命 K2 被拒 | `"rejected"` | `"rejected 403 403 only level-1 agents can appoint level-2 agents"` |

## OWN

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| OWN-01 | ✅ | 无上级的二级 S 的客户 S1 充 100：S 拿满 30 | `[["S", "agent_direct", 30]]` | `[["S", "agent_direct", 30]]` |
| OWN-02a | ✅ | 管理员给 S 设上级一级 K：只改 S 的归属快照，邀请关系不变 | `{"owner": 2497, "owner_level": 1, "upline": null, "inviter": null}` | `{"owner": 2497, "owner_level": 1, "upline": null, "inviter": null}` |
| OWN-02b | ✅ | S1 是设置前的客户：快照不变，仍 S 30 | `[["S", "agent_direct", 30]]` | `[["S", "agent_direct", 30]]` |
| OWN-02c | ✅ | 设置后新客户 S2 充 100：S 20、K 10 | `[["K", "agent_diff", 10], ["S", "agent_direct", 20]]` | `[["K", "agent_diff", 10], ["S", "agent_direct", 20]]` |
| OWN-02d | ✅ | S 本人充 100：上级 K 拿满 30 | `[["K", "agent_direct", 30]]` | `[["K", "agent_direct", 30]]` |
| OWN-03a | ✅ | 目标不是二级（S1 普通）被拒 | `"rejected"` | `"rejected 400 400 only level-2 agents can have a level-1 upline set"` |
| OWN-03b | ✅ | 上级不是一级（K2 普通）被拒 | `"rejected"` | `"rejected 400 400 upline must be an active level-1 agent"` |
| OWN-03c | ✅ | 上级设为自己被拒 | `"rejected"` | `"rejected 400 400 user cannot invite themselves"` |
| OWN-03d | ✅ | 成环（KS 归属 S，再把 S 的上级设为 KS）被拒 | `"rejected"` | `"rejected 400 400 binding would create an affiliate cycle"` |
| OWN-04 | ✅ | 清除上级后新客户 S3 充 100：S 拿满 30 | `[["S", "agent_direct", 30]]` | `[["S", "agent_direct", 30]]` |
| OWN-05a | ✅ | 管理员给无邀请人 Z 补绑邀请人 N1 | `{"inviter": 2469, "inviter_was_agent": false}` | `{"inviter": 2469, "inviter_was_agent": false}` |
| OWN-05b | ✅ | 已绑定后再次补绑被拒（一次性） | `"rejected"` | `"rejected 409 409 affiliate inviter already bound"` |
| OWN-06 | ✅ | 补绑成环（B 的邀请人设为其下游 U2）被拒 | `"rejected"` | `"rejected 400 400 binding would create an affiliate cycle"` |

## UPG

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| UPG-01 | ✅ | 自动升级（口径全部）：Q 邀请 2 人升二级 | `{"level": 2, "source": "auto"}` | `{"level": 2, "source": "auto"}` |
| UPG-02 | ✅ | Q 邀请第 3 人升一级 | `1` | `1` |
| UPG-04a | ✅ | 被降级锁定的 K1 邀请满 2 人也不自动升级 | `{"level": 3, "locked": true}` | `{"level": 3, "locked": true}` |
| UPG-04b | ✅ | 解锁后点「重新评估」：K1 升二级 | `2` | `2` |
| UPG-03a | ✅ | 口径仅充值：R 邀请 2 人但都未充值，不升级 | `3` | `3` |
| UPG-03b | ✅ | 两名直属完成真实充值后 R 升二级 | `2` | `2` |
| UPG-05 | ✅ | 删除过有下级的账号（B2/A2）后点「重新评估」：跳过已删除用户，正常完成 | `"ok, evaluated>0"` | `{"evaluated": 392, "promoted": 0}` |

## DEL

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| DEL-01 | ✅ | 删除一级 B2 后 C7 充 100：A2 仍 20，B2 的差额不再发放 | `[["A2", "agent_direct", 20]]` | `[["A2", "agent_direct", 20]]` |
| DEL-02 | ✅ | 再删除归属代理 A2：C7 充 100 无代理利润 | `[]` | `[]` |
| DEL-03 | ✅ | 管理端总览：C7 的归属代理 / 上方一级显示为已删除 | `"deleted flags"` | `{"agent_owner_id": 2466, "agent_owner_email": "e2e-072240-a2@e2e.test", "agent_owner_level": 2, "agent_owner_deleted": true, "agent_upline_id": 2465, "agent_upline_email": "e2e-072240-b2@e2e.test", "agent_upline_deleted": true}` |

## OFF

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| OFF-01 | ✅ | 实收100到账110：订单本金100、赠送10、零费率；财务 paid100/free10/cash100 | `[110, 100, 10, 0, 0, "offline", "offline", null, "COMPLETED", 100, 10, 100]` | `[110, 100, 10, 0, 0, "offline", "offline", null, "COMPLETED", 100.0, 10.0, 100.0]` |
| OFF-01b | ✅ | 入账时间一致 | `true` | `true` |
| OFF-09a | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[100, 10]` | `[100, 10]` |
| OFF-02 | ✅ | 相同流水号（含首尾空格）拒绝409 | `409` | `409` |
| OFF-03a | ✅ | 历史赠送入free且没有奖励 | `[30, []]` | `[30.0, []]` |
| OFF-09b | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[100, 30]` | `[100, 30]` |
| OFF-03 | ✅ | 补登不改余额不发奖励，财务该笔从free20转paid15/free5 | `[130.0, [], 115, 15, 115]` | `[130.0, [], 115.0, 15.0, 115.0]` |
| OFF-09c | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[115, 15]` | `[115, 15]` |
| OFF-04 | ✅ | 重复补登409 | `409` | `409` |
| OFF-05 | ✅ | 用户不能申请线下订单退款 | `403` | `403` |
| OFF-06 | ✅ | 管理员退款扣回110、无网关退款、无奖励回收 | `["REFUNDED", 110, -110, 0, []]` | `["REFUNDED", 110, -110.0, 0, []]` |
| OFF-06b | ✅ | 退款现金按实收比例100而不是到账110 | `100` | `100.0` |
| OFF-09d | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[15, 15]` | `[15, 15]` |
| OFF-07 | ✅ | 实收大于到账：本金90、现金100、赠送0 | `[90, 100, 0]` | `[90.0, 100.0, 0.0]` |
| OFF-08a | ✅ | 仅充值口径：注册尚不升级 | `3` | `3` |
| OFF-08 | ✅ | 只有线下充值也计入有效邀请并按本金发首充10 | `[2, 120]` | `[2, 120.0]` |
| OFF-08b | ✅ | 首充奖励关联线下订单 | `true` | `true` |
| OFF-08c | ✅ | 再次线下充值不重复首充奖励 | `130` | `130.0` |
| OFF-09e | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[15, 20]` | `[15, 20]` |
| OFF-09-filter | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[15, 20]` | `[15, 20]` |
| OFF-09-page | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[15, 20]` | `[15, 20]` |
| OFF-09-subscription | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[15, 20]` | `[15, 20]` |
| OFF-09-affiliate | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[15, 20]` | `[15, 20]` |
| OFF-10 | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[50, 0]` | `[50, 0]` |
| OFF-11a | ✅ | 在线订单已履约 | `"COMPLETED"` | `"COMPLETED"` |
| OFF-11 | ✅ | 余额历史累计实收/赠送（不从当前页求和） | `[70, 0]` | `[70, 0]` |

## MRT

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| MRT-01 | ✅ | PUT 模型单独倍率：模型名归一化（小写、claude 点号换横线），按模型名排序返回 | `[{"model": "claude-haiku-*", "rate_multiplier": 0.8}, {"model": "claude-haiku-4-5", "rate_multiplier": 0.7}, {"model": "claude-opus-4-5", "rate_multiplier": 2.0}]` | `[{"model": "claude-haiku-*", "rate_multiplier": 0.8}, {"model": "claude-haiku-4-5", "rate_multiplier": 0.7}, {"model": "claude-opus-4-5", "rate_multiplier": 2}]` |
| MRT-02 | ✅ | MU 请求 claude-opus-4-5：命中精确规则 2.0，专属倍率 0.5 不再作用 | `{"rate_multiplier": 2.0, "actual_cost": "total_cost × 2"}` | `{"model": "claude-opus-4-5", "rate": 2, "total": 0.01, "actual": 0.02}` |
| MRT-03 | ✅ | MU 请求 claude-sonnet-4-5：未命中规则 → 用户专属倍率 0.5 | `{"rate_multiplier": 0.5, "actual_cost": "total_cost × 0.5"}` | `{"model": "claude-sonnet-4-5", "rate": 0.5, "total": 0.006, "actual": 0.003}` |
| MRT-04 | ✅ | MU 请求 claude-haiku-4-5：精确规则 0.7 优先于通配 claude-haiku-* 0.8 | `{"rate_multiplier": 0.7, "actual_cost": "total_cost × 0.7"}` | `{"model": "claude-haiku-4-5", "rate": 0.7, "total": 0.002, "actual": 0.0014}` |
| MRT-05 | ✅ | MU 请求 claude-haiku-4-5-20251001：带日期的模型名命中通配 claude-haiku-* 0.8 | `{"rate_multiplier": 0.8, "actual_cost": "total_cost × 0.8"}` | `{"model": "claude-haiku-4-5-20251001", "rate": 0.8, "total": 0.002, "actual": 0.0016}` |
| MRT-06 | ✅ | MV 请求 claude-sonnet-4-5：无专属倍率的用户未命中规则 → 分组倍率 1.0 | `{"rate_multiplier": 1.0, "actual_cost": "total_cost × 1"}` | `{"model": "claude-sonnet-4-5", "rate": 1, "total": 0.006, "actual": 0.006}` |
| MRT-07 | ✅ | MV 请求 claude-opus-4-5：无专属倍率的用户命中规则 2.0 | `{"rate_multiplier": 2.0, "actual_cost": "total_cost × 2"}` | `{"model": "claude-opus-4-5", "rate": 2, "total": 0.01, "actual": 0.02}` |
| MRT-08 | ✅ | MU 交替请求 opus / sonnet：倍率不串（缓存按用户×分组，不被模型倍率污染） | `[["claude-opus-4-5", 2.0], ["claude-sonnet-4-5", 0.5], ["claude-opus-4-5", 2.0], ["claude-sonnet-4-5", 0.5]]` | `[["claude-opus-4-5", 2], ["claude-sonnet-4-5", 0.5], ["claude-opus-4-5", 2], ["claude-sonnet-4-5", 0.5]]` |
| MRT-09 | ✅ | 规则 opus 改为 3.0 后立即生效（保存时清本进程快照） | `{"rate_multiplier": 3.0, "actual_cost": "total_cost × 3"}` | `{"model": "claude-opus-4-5", "rate": 3, "total": 0.01, "actual": 0.03}` |
| MRT-10 | ✅ | 生图独立倍率关闭：gpt-image-1 用模型单独倍率 2.0 | `{"rate_multiplier": 2.0, "actual_cost": "total_cost × 2"}` | `{"model": "gpt-image-1", "rate": 2, "total": 0.134, "actual": 0.268}` |
| MRT-11 | ✅ | 生图独立倍率开启（1.5）：独立倍率优先，模型单独倍率不作用 | `{"rate_multiplier": 1.5, "actual_cost": "total_cost × 1.5"}` | `{"model": "gpt-image-1", "rate": 1.5, "total": 0.134, "actual": 0.201}` |
| MRT-12 | ✅ | 去掉 MU 专属倍率（等缓存过期）后 sonnet 回到分组倍率 1.0 | `{"rate_multiplier": 1.0, "actual_cost": "total_cost × 1"}` | `{"model": "claude-sonnet-4-5", "rate": 1, "total": 0.006, "actual": 0.006}` |
| MRT-13 | ✅ | 去掉专属倍率后 opus 仍为模型单独倍率 2.0 | `{"rate_multiplier": 2.0, "actual_cost": "total_cost × 2"}` | `{"model": "claude-opus-4-5", "rate": 2, "total": 0.01, "actual": 0.02}` |
| MRT-20 | ✅ | PUT 校验：通配符不在末尾 → 400 | `400` | `400` |
| MRT-21 | ✅ | PUT 校验：只有通配符 → 400 | `400` | `400` |
| MRT-22 | ✅ | PUT 校验：归一化后重复（大小写 / 点号） → 400 | `400` | `400` |
| MRT-23 | ✅ | PUT 校验：倍率为 0 → 400 | `400` | `400` |
| MRT-24 | ✅ | PUT 校验：倍率为负 → 400 | `400` | `400` |
| MRT-25 | ✅ | PUT 校验：倍率超过 10000 → 400 | `400` | `400` |
| MRT-26 | ✅ | PUT 校验：空模型名 → 400 | `400` | `400` |
| MRT-27 | ✅ | 校验失败不改动已有规则 | `[{"model": "claude-haiku-*", "rate_multiplier": 0.8}, {"model": "claude-haiku-4-5", "rate_multiplier": 0.7}, {"model": "claude-opus-4-5", "rate_multiplier": 2.0}]` | `[{"model": "claude-haiku-*", "rate_multiplier": 0.8}, {"model": "claude-haiku-4-5", "rate_multiplier": 0.7}, {"model": "claude-opus-4-5", "rate_multiplier": 2}]` |
| MRT-28 | ✅ | 不存在的分组 → 404 | `404` | `404` |
| MRT-29 | ✅ | 普通用户访问管理端接口被拒 | `true` | `true` |
| MRT-30 | ✅ | GET /admin/groups/model-rates 含本组规则（键为分组 id） | `[{"model": "claude-haiku-*", "rate_multiplier": 0.8}, {"model": "claude-haiku-4-5", "rate_multiplier": 0.7}, {"model": "claude-opus-4-5", "rate_multiplier": 2.0}]` | `[{"model": "claude-haiku-*", "rate_multiplier": 0.8}, {"model": "claude-haiku-4-5", "rate_multiplier": 0.7}, {"model": "claude-opus-4-5", "rate_multiplier": 2}]` |
| MRT-31 | ✅ | 复制分组后模型单独倍率一并复制 | `[{"model": "claude-haiku-*", "rate_multiplier": 0.8}, {"model": "claude-haiku-4-5", "rate_multiplier": 0.7}, {"model": "claude-opus-4-5", "rate_multiplier": 2.0}]` | `[{"model": "claude-haiku-*", "rate_multiplier": 0.8}, {"model": "claude-haiku-4-5", "rate_multiplier": 0.7}, {"model": "claude-opus-4-5", "rate_multiplier": 2}]` |
| MRT-40 | ✅ | 模型广场（MU 已登录）：opus 公开倍率 2.0、无专属倍率字段（单独倍率优先） | `{"rate_multiplier": 2.0, "user_rate_multiplier": null}` | `{"rate_multiplier": 2, "user_rate_multiplier": null}` |
| MRT-41 | ✅ | 模型广场（MU）：sonnet 公开倍率 1.0、专属倍率 0.5 | `{"rate_multiplier": 1.0, "user_rate_multiplier": 0.5}` | `{"rate_multiplier": 1, "user_rate_multiplier": 0.5}` |
| MRT-42 | ✅ | 模型广场（MU）：haiku 精确 0.7、带日期的命中通配 0.8 | `[0.7, 0.8]` | `[0.7, 0.8]` |
| MRT-43 | ✅ | 模型广场（匿名）：sonnet 无专属倍率字段 | `{"rate_multiplier": 1.0, "user_rate_multiplier": null}` | `{"rate_multiplier": 1, "user_rate_multiplier": null}` |
| MRT-50 | ✅ | 页面已滚动（scrollY > 0），分组区块在视口顶部（0 ≤ top < 视口高度的 40%） | `"scrolled into view"` | `{"top": 96, "vh": 480, "scrollY": 345}` |
| MRT-51 | ✅ | opus 显示 -75%（¥2 = $1，按 $1 = ¥8.1），无划线公开价 | `{"text": "-75%", "publicText": null}` | `{"text": "-75%", "publicText": null, "title": "¥2 = $1，按 $1 = ¥8.1 折算"}` |
| MRT-52 | ✅ | sonnet 显示划线公开折扣 -88% + 专属折扣 -94% | `{"text": "-94%", "publicText": "-88%"}` | `{"text": "-94%", "publicText": "-88%"}` |
| MRT-53 | ✅ | 分组标签与表格文字中没有「= $1」 | `null` | `null` |
| MRT-54 | ✅ | /keys：没有「= $1」「价格比例」「高峰价格」 | `{"ratio": null, "label": null}` | `{"ratio": null, "label": null}` |
| MRT-55 | ✅ | /available-channels：没有「= $1」「价格比例」「高峰价格」 | `{"ratio": null, "label": null}` | `{"ratio": null, "label": null}` |
| MRT-56 | ✅ | /purchase：没有「= $1」「价格比例」「高峰价格」 | `{"ratio": null, "label": null}` | `{"ratio": null, "label": null}` |
| MRT-57 | ✅ | /subscriptions：没有「= $1」「价格比例」「高峰价格」 | `{"ratio": null, "label": null}` | `{"ratio": null, "label": null}` |
| MRT-58 | ✅ | 点击后打开 /model-plaza，带 embedded=1 与本账号密钥所在分组 group | `{"embedded": "1", "group": "std\|img"}` | `{"embedded": "1", "group": "230"}` |
| MRT-59 | ✅ | 明细为 ¥、有「折扣 -75%」，官方价（原始费用）仍为 $ | `{"input": "¥…", "discount": "-75%", "original": "$…"}` | `{"tip": "费用明细 输入费用 ¥0.01000000 输出费用 ¥0.01000000 输入单价 ¥10.0000 / 1M Token 输出单价 ¥50.0000 / 1M Token 服务档位 Standard 折扣 -75% 官方价 $0.01000000 用户扣费 ¥0.02000000"}` |
| MRT-60 | ✅ | sonnet 显示公开折扣 -88%；登录跳转 redirect 含 group=229 | `{"text": "-88%", "publicText": null, "redirectHasGroup": true}` | `{"text": "-88%", "publicText": null, "redirect": "/model-plaza?group=229"}` |
| MRT-61 | ✅ | std 分组倍率列显示「3 个模型单独倍率」 | `"3 个模型单独倍率"` | `"3 个模型单独倍率"` |
| MRT-62 | ✅ | 管理端明细为 $、有「价格比例」、无「折扣」 | `{"input": "$…", "label": "价格比例"}` | `{"tip": "费用明细 输入费用 $0.00500000 输出费用 $0.00500000 输入单价 $5.0000 / 1M Token 输出单价 $25.0000 / 1M Token 服务档位 Standard 价格比例 ¥2 = $1 官方价 $0.01000000 用户扣费 ¥0.02000000 账号倍率 1.00x 账号计费 $0.01000000"}` |
| MRT-63 | ✅ | 卡片显示当前生效汇率 8.1 | `"8.1"` | `"$1 = ¥8.1 自动获取"` |

## FX

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| FX-01 | ✅ | 全部来源失败：立即更新 ok=false，生效值为默认 6.8，提示未更新成功 | `{"ok": false, "effective_rate": 6.8, "effective_from": "default", "stale": true, "has_error": true}` | `{"ok": false, "effective_rate": 6.8, "effective_from": "default", "stale": true, "has_error": true}` |
| FX-02 | ✅ | 公开设置 usd_cny_rate = 6.8（默认兜底） | `6.8` | `6.8` |
| FX-03 | ✅ | 首选来源正常：采用 7.10，来源为首选主机，记录获取时间 | `{"ok": true, "effective_rate": 7.1, "effective_from": "fetched", "source": "sub2api-local-upstream-mock", "stale": false}` | `{"ok": true, "effective_rate": 7.1, "effective_from": "fetched", "source": "sub2api-local-upstream-mock", "stale": false}` |
| FX-04 | ✅ | 公开设置 usd_cny_rate 跟随为 7.10 | `7.1` | `7.1` |
| FX-05 | ✅ | 首选失败 → 备选来源 7.20（变化 1.4%，在 10% 内） | `{"ok": true, "effective_rate": 7.2, "source": "upstream-mock"}` | `{"ok": true, "effective_rate": 7.2, "source": "upstream-mock:18998"}` |
| FX-06 | ✅ | 异常值（超出 5–9、变化超过 10%）被丢弃：ok=false，生效值仍为 7.20 | `{"ok": false, "effective_rate": 7.2, "has_error": true}` | `{"ok": false, "effective_rate": 7.2, "has_error": true}` |
| FX-06b | ✅ | 两个来源一致的大幅变动（8.10 / 8.12，相对 7.20 变化 12.5%）被采用：取首选来源 8.10 | `{"ok": true, "effective_rate": 8.1}` | `{"ok": true, "effective_rate": 8.1}` |
| FX-07 | ✅ | 默认汇率 0 超出 5–9 → 400 | `400` | `400` |
| FX-08 | ✅ | 关闭自动更新、默认改为 7.0：生效值 7.0（来自默认），公开设置同步 | `{"effective_rate": 7.0, "effective_from": "default", "public": 7.0}` | `{"effective_rate": 7, "effective_from": "default", "public": 7}` |
| FX-09 | ✅ | 重新开启自动更新：生效值回到最近一次成功获取的 8.10 | `8.1` | `8.1` |

## FIN

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| FIN-01 | ✅ | PUT 账号模型成本倍率：模型名归一化为小写，原样返回倍率 | `[{"model": "claude-sonnet-*", "rate_multiplier": 0.3}]` | `[{"model": "claude-sonnet-*", "rate_multiplier": 0.3}]` |
| FIN-02 | ✅ | 账号模型成本倍率超出上限 10000 → 400 | `"rejected"` | `"rejected 400 400 第 1 条（x）：倍率不能超过 10000"` |
| FIN-03 | ✅ | GET 全部账号的模型成本倍率含本账号 | `true` | `true` |
| FIN-04 | ✅ | PUT 财务设置：固定成本账号列表加入本阶段账号 | `true` | `true` |
| FIN-05 | ✅ | 固定成本账号含不存在的账号 id → 400 | `"rejected"` | `"rejected 400 400 包含不存在的账号"` |
| FIN-06 | ✅ | 日期跨度超过 732 天 → 400 | `"rejected"` | `"rejected 400 400 日期范围不能超过 732 天"` |
| FIN-07 | ✅ | 新订单快照通道费率 3.2% | `0.032` | `0.032` |
| FIN-08 | ✅ | 未配置模型成本倍率的账号：成本倍率快照 = 账号倍率 0.5（与改动前一致） | `[0.5, 0.5, 0.5]` | `[0.5, 0.5, 0.5]` |
| FIN-09 | ✅ | 模型成本倍率命中 claude-sonnet-*：快照 0.3；未命中的 haiku 沿用账号倍率 0.5 | `{"claude-sonnet-4-5": 0.3, "claude-haiku-4-5": 0.5}` | `{"claude-sonnet-4-5": 0.3, "claude-haiku-4-5": 0.5}` |
| FIN-10 | ✅ | 按张成本规则：2 张图 account_stats_cost = 0.05 × 2 | `[0.1]` | `[0.1]` |
| FIN-11 | ✅ | 按账号拆分：成本 = Σ(account_stats_cost ?? total_cost) × 成本倍率；固定成本账号为 0 | `{"185": 2.445, "186": 0.42, "187": 0.0, "188": 0.1}` | `{"185": 2.445, "186": 0.42, "187": 0.0, "188": 0.1}` |
| FIN-12 | ✅ | 按账号拆分的成本方式标签 | `{"rate": "rate", "model": "model_rate", "fixed": "fixed", "img": "per_image"}` | `{"rate": "rate", "model": "model_rate", "fixed": "fixed", "img": "per_image"}` |
| FIN-13 | ✅ | 总览差值：收入按「赠送优先 + 单位实收」确认；不计收入消耗、账号成本、内部成本与手算一致 | `{"revenue": 0.349424, "revenue_commission": 0.09, "free_consumed": 6.9, "consumption": 7.258, "account_cost": 2.965, "account_cost_internal": 2.794909}` | `{"revenue": 0.349424, "revenue_commission": 0.09, "free_consumed": 6.9, "consumption": 7.258, "account_cost": 2.965, "account_cost_internal": 2.794909}` |
| FIN-14 | ✅ | 分润支出差值 = 期间新增 accrue 流水（首充奖励不算分润） | `0.5` | `0.5` |
| FIN-15 | ✅ | 利润 = 收入 − 账号成本 − 分润 − 其他支出 | `-765.854576` | `-765.854576` |
| FIN-16 | ✅ | 充值与赠送差值：实收、通道费 3.2%、退款现金、本金、管理员加余额 15、首充奖励、独立注册赠送 | `{"cash_received": 130, "channel_fee": 4.16, "refund_cash": 10.0, "principal": 130, "admin_adjust_in": 15.0, "invitee_bonus": 1.0, "invitee_signup_bonus": 5.0}` | `{"cash_received": 130.0, "channel_fee": 4.16, "refund_cash": 10.0, "principal": 130.0, "admin_adjust_in": 15.0, "invitee_bonus": 1.0, "invitee_signup_bonus": 5.0}` |
| FIN-16b | ✅ | 赠送总计包含独立注册赠送且不重复计首充 | `21.0` | `21.0` |
| FIN-16c | ✅ | 只有注册赠送的用户对账：免费5、付费0、差额0 | `[5.0, 0.0, 0.0]` | `[5.0, 0.0, 0.0]` |
| FIN-17 | ✅ | 充值订单明细：FD 订单带单号、通道费、退款现金 10、扣回余额 10 | `{"has_out_trade_no": true, "channel_fee_rate": 0.032, "refund_cash": 10.0, "refund_balance_deducted": 10.0}` | `{"has_out_trade_no": true, "channel_fee_rate": 0.032, "refund_cash": 10.0, "refund_balance_deducted": 10.0}` |
| FIN-18 | ✅ | 用户对账 FA：差额 0、消耗与 usage_logs 一致、不计收入余额剩余 = 入账 − 已消耗 | `{"difference": 0, "consumption": 5.068, "free_left": 0.0}` | `{"difference": 0.0, "consumption": 5.068, "free_left": 0.0, "remaining": {"commission": "0", "free": "0", "paid": "99.932"}}` |
| FIN-19 | ✅ | 用户对账 FB：差额 0、消耗与 usage_logs 一致、不计收入余额剩余 = 入账 − 已消耗 | `{"difference": 0, "consumption": 1.2, "free_left": 0.0}` | `{"difference": 0.0, "consumption": 1.2, "free_left": 0.0, "remaining": {"commission": "0", "free": "0", "paid": "9.8"}}` |
| FIN-20 | ✅ | 用户对账 FC：差额 0、消耗与 usage_logs 一致、不计收入余额剩余 = 入账 − 已消耗 | `{"difference": 0, "consumption": 0.9, "free_left": 9.1}` | `{"difference": 0.0, "consumption": 0.9, "free_left": 9.1, "remaining": {"commission": "0", "free": "9.1", "paid": "0"}}` |
| FIN-21 | ✅ | 用户对账 FD：差额 0、消耗与 usage_logs 一致、不计收入余额剩余 = 入账 − 已消耗 | `{"difference": 0, "consumption": 0, "free_left": 0.0}` | `{"difference": 0.0, "consumption": 0.0, "free_left": 0.0, "remaining": {"commission": "0", "free": "0", "paid": "10"}}` |
| FIN-22 | ✅ | 用户对账 FI：差额 0、消耗与 usage_logs 一致、不计收入余额剩余 = 入账 − 已消耗 | `{"difference": 0, "consumption": 0.09, "free_left": 0.0}` | `{"difference": 0.0, "consumption": 0.09, "free_left": 0.0, "remaining": {"commission": "0.41", "free": "0", "paid": "0"}}` |
| FIN-23 | ✅ | 退款对账：FD 扣回 10、付费剩余 10、实收 = 20×(1−3.2%) − 10 | `{"refund_deducted": 10.0, "paid_left": 10.0, "paid_cash": 9.36}` | `{"refund_deducted": 10.0, "paid_left": 10.0, "paid_cash": 9.36}` |
| FIN-24 | ✅ | 对账搜索支持纯数字用户 ID | `[2525]` | `[2525]` |
| FIN-25 | ✅ | 支出按天摊销跨月：2020-01-20 起 30 天 ¥30 → 1 月 12 天、2 月 18 天 | `{"2020-01": 12.0, "2020-02": 18.0}` | `{"2020-01": 12.0, "2020-02": 18.0}` |
| FIN-26 | ✅ | 支出返回每日摊销额与关联账号名 | `{"daily_amount": 1.0, "account_linked": true}` | `{"daily_amount": 1.0, "account_linked": true}` |
| FIN-31 | ✅ | 按账号拆分：关联支出按范围内摊销额计入该账号（无用量也出行），毛利扣除 | `{"expense_cost": 12.0, "gross_profit": -12.0, "requests": 0, "cost_mode": "fixed"}` | `{"expense_cost": 12.0, "gross_profit": -12.0, "requests": 0, "cost_mode": "fixed"}` |
| FIN-27 | ✅ | 修改支出后重新摊销：¥31 / 31 天 → 1 月 12、2 月 19 | `{"2020-01": 12.0, "2020-02": 19.0}` | `{"2020-01": 12.0, "2020-02": 19.0}` |
| FIN-32 | ✅ | 一次性扣除（1 天）：全额计入发生日；关联账号只计关联的那笔 | `{"months": {"2020-01": 12.0, "2020-02": 26.0}, "expense_cost": 7.0}` | `{"months": {"2020-01": 12.0, "2020-02": 26.0}, "expense_cost": 7.0}` |
| FIN-28 | ✅ | 支出金额为 0 → 400 | `"rejected"` | `"rejected 400 400 支出日期、类别、金额、摊销天数或备注无效"` |
| FIN-29 | ✅ | 删除支出后不再计入 | `{"2020-01": 0.0, "2020-02": 0.0}` | `{"2020-01": 0.0, "2020-02": 0.0}` |
| FIN-30 | ✅ | CSV 导出：text/csv、带 BOM、表头 + 当天一行 | `{"status": 200, "bom": true, "csv": true, "has_today": true}` | `{"status": 200, "bom": true, "csv": true, "has_today": true}` |

## CON

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

## TSCO

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TSCO-00 | ✅ | 团队拓扑的归属快照符合设计（注册带邀请码、用户端任命 / 取消、管理员改挂） | `{"TB": {"level": 1, "owner": "TO", "upline": null, "inviter": "TO"}, "X0": {"level": 3, "owner": "TO", "upline": null, "inviter": "TB"}, "TA": {"level": 2, "owner": "TB", "upline": null, "inviter": "TB"}, "TZ": {"level": 2, "owner": "TB", "upline": null, "invi` | `{"TB": {"level": 1, "owner": "TO", "upline": null, "inviter": "TO"}, "X0": {"level": 3, "owner": "TO", "upline": null, "inviter": "TB"}, "TA": {"level": 2, "owner": "TB", "upline": null, "inviter": "TB"}, "TZ": {"level": 2, "owner": "TB", "upline": null, "invi` |
| TSCO-01 | ✅ | TB 的「我的客户」= 归属快照 owner=TB（112 人；含 TA 本人，不含 TA1/TM/X0） | `{"total": 112, "users": ["TA", "TC1", "TC1x", "TD", "TF001", "TF002", "TF003", "TF004", "TF005", "TF006", "TF007", "TF008", "TF009", "TF010", "TF011", "TF012", "TF013", "TF014", "TF015", "TF016", "TF017", "TF018", "TF019", "TF020", "TF021", "TF022", "TF023", "` | `{"total": 112, "users": ["TA", "TC1", "TC1x", "TD", "TF001", "TF002", "TF003", "TF004", "TF005", "TF006", "TF007", "TF008", "TF009", "TF010", "TF011", "TF012", "TF013", "TF014", "TF015", "TF016", "TF017", "TF018", "TF019", "TF020", "TF021", "TF022", "TF023", "` |
| TSCO-01b | ✅ | 客户行的代理身份：TA/TZ 二级（带标记），TH 已降为普通 | `{"TA": 2, "TZ": 2, "TH": 3, "TC1": 3}` | `{"TC1": 3, "TA": 2, "TH": 3, "TZ": 2}` |
| TSCO-02 | ✅ | TB 的下级代理：TA、TZ 当前；TH（取消任命）、TM（改挂）历史；客户数 3/0/1/1 | `{"total": 4, "agents": {"TA": {"historical": false, "customers": 3}, "TZ": {"historical": false, "customers": 0}, "TH": {"historical": true, "customers": 1}, "TM": {"historical": true, "customers": 1}}}` | `{"total": 4, "agents": {"TA": {"historical": false, "customers": 3}, "TH": {"historical": true, "customers": 1}, "TM": {"historical": true, "customers": 1}, "TZ": {"historical": false, "customers": 0}}}` |
| TSCO-02b | ✅ | TB 摘要计数：直属客户 112、下级代理 4 | `{"direct_customers": 112, "sub_agents": 4}` | `{"direct_customers": 112, "sub_agents": 4}` |
| TSCO-03 | ✅ | TA 的团队（TB 视角，agent_id=TA）= TA1、TA2、TA1x，不含 TA 本人 | `["TA1", "TA1x", "TA2"]` | `["TA1", "TA1x", "TA2"]` |
| TSCO-04 | ✅ | TA 视角：我的客户 = TA1、TA2、TA1x；没有下级代理 | `{"customers": ["TA1", "TA1x", "TA2"], "counts": {"direct_customers": 3, "sub_agents": 0}, "agents_total": 0}` | `{"customers": ["TA1", "TA1x", "TA2"], "counts": {"direct_customers": 3, "sub_agents": 0}, "agents_total": 0}` |
| TSCO-05 | ✅ | 团队合计 117 人；直属 / 下级消费拆分与 SQL 一致 | `{"customers": 117, "direct_cost": 0.9110715, "sub_cost": 0.76925}` | `{"customers": 117, "direct_cost": 0.9110715, "sub_cost": 0.76925}` |
| TSCO-06 | ✅ | TA 本人消费只计入 TB 的「我的客户」：不在 TA 的团队里，也不计入 TA 自己的团队消费 | `{"tb_row_cost": 0.01413, "positive": true, "in_ta_team": false, "ta_view_cost": 0.73415}` | `{"tb_row_cost": 0.01413, "positive": true, "in_ta_team": false, "ta_view_cost": 0.7341500000000001}` |
| TSCO-07 | ✅ | TO 视角：我的客户 = TB、X0、TM；下级代理 TM（当前，团队 0 人） | `{"customers": ["TB", "TM", "X0"], "agents": {"TM": {"historical": false, "customers": 0}}}` | `{"customers": ["TB", "TM", "X0"], "agents": {"TM": {"historical": false, "customers": 0}}}` |

## TGW

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TGW-00 | ✅ | 消费客户经易支付模拟网关充值 100、TC1 购买订阅，订单全部完成 | `{"TA": "COMPLETED", "TA1": "COMPLETED", "TA1x": "COMPLETED", "TB": "COMPLETED", "TC1": "COMPLETED", "TC1x": "COMPLETED", "TD": "COMPLETED", "TH1": "COMPLETED", "TH2": "COMPLETED", "TM1": "COMPLETED", "TN": "COMPLETED", "X0": "COMPLETED", "TC1-sub": "COMPLETED"` | `{"TA": "COMPLETED", "TA1": "COMPLETED", "TA1x": "COMPLETED", "TB": "COMPLETED", "TC1": "COMPLETED", "TC1x": "COMPLETED", "TD": "COMPLETED", "TH1": "COMPLETED", "TH2": "COMPLETED", "TM1": "COMPLETED", "TN": "COMPLETED", "X0": "COMPLETED", "TC1-sub": "COMPLETED"` |
| TGW-01a | ✅ | 经网关的 21 次请求全部 200 并写入 usage_logs | `21` | `21` |
| TGW-01 | ✅ | 网关写入的 usage_logs 与上游返回的用量逐条一致（token、缓存、生图张数、订阅 billing_type=1） | `[["TA", "claude-haiku-4-5", 800, 100, 200, 0, 0, 0], ["TA", "claude-sonnet-4-5", 1200, 300, 0, 0, 0, 0], ["TA1", "claude-opus-4-1", 1000, 100, 0, 0, 0, 0], ["TA1", "claude-sonnet-4-5", 3000, 500, 1000, 200, 0, 0], ["TA1", "gpt-image-1", 10, 0, 0, 0, 2, 0], ["T` | `[["TA", "claude-haiku-4-5", 800, 100, 200, 0, 0, 0], ["TA", "claude-sonnet-4-5", 1200, 300, 0, 0, 0, 0], ["TA1", "claude-opus-4-1", 1000, 100, 0, 0, 0, 0], ["TA1", "claude-sonnet-4-5", 3000, 500, 1000, 200, 0, 0], ["TA1", "gpt-image-1", 10, 0, 0, 0, 2, 0], ["T` |
| TGW-03a | ✅ | 模型映射：上游收到映射后的模型，usage 记录请求模型 | `{"upstream_got_mapped": true, "upstream_got_requested": false, "usage": {"model": "claude-opus-4-1", "requested_model": "claude-opus-4-1", "upstream_model": "claude-haiku-4-5-20251001"}}` | `{"upstream_got_mapped": true, "upstream_got_requested": false, "usage": {"model": "claude-opus-4-1", "requested_model": "claude-opus-4-1", "upstream_model": "claude-haiku-4-5-20251001"}}` |
| TGW-02 | ✅ | 应用经网关写入（批量写入路径）与直接写入的 usage_logs，汇总表与明细重算逐行一致（621 行） | `{"missing": 0, "extra": 0, "mismatched": 0, "last_used_diff": 0}` | `{"missing": 0, "extra": 0, "mismatched": 0, "last_used_diff": 0}` |

## TMET

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TMET-09 | ✅ | TC1 新请求写入后立即反映在 TB 的团队摘要（增量 = 该请求 actual_cost，不等任何任务） | `0.005229` | `0.005229` |
| TMET-10a | ✅ | 结算前：今天的消费已全部显示，收益与当时已入账的流水一致 | `{"cost": 1.6803215, "earnings": 0}` | `{"cost": 1.6803214999999998, "earnings": 0}` |
| TMET-10b | ✅ | 定时结算任务处理完全部有归属客户的余额消费 | `[]` | `[]` |
| TMET-10c | ✅ | 按归属快照入账代理分润（降级 TH、改挂 TM 的老客户照常给 TH/TM 直推、TB 差额） | `{"TA": [["TB", "agent_direct"]], "TA1": [["TA", "agent_direct"], ["TB", "agent_diff"]], "TA1x": [["TA", "agent_direct"], ["TB", "agent_diff"]], "TC1": [["TB", "agent_direct"]], "TC1x": [["TB", "agent_direct"]], "TD": [["TB", "agent_direct"]], "TH1": [["TB", "a` | `{"TA": [["TB", "agent_direct"]], "TA1": [["TA", "agent_direct"], ["TB", "agent_diff"]], "TA1x": [["TA", "agent_direct"], ["TB", "agent_diff"]], "TC1": [["TB", "agent_direct"]], "TC1x": [["TB", "agent_direct"]], "TD": [["TB", "agent_direct"]], "TH1": [["TB", "a` |
| TMET-10d | ✅ | （记录）本轮其他奖励流水：普通邀请人的比例奖励等，不计入团队收益 | `"info"` | `[["TA1", "invite_percent", "TA1x"], ["TB", "invite_percent", "X0"], ["TC1", "invite_percent", "TC1x"], ["TH", "invite_percent", "TH2"]]` |
| TMET-10e | ✅ | TA1 的分润：base = 绑定后余额消费，TA 直推 20%、TB 差额 10% | `{"base": 0.328075, "direct": 0.065615, "diff": 0.032808}` | `{"base": 0.328075, "direct": 0.065615, "diff": 0.032808}` |
| TMET-10f | ✅ | 结算后：TB 收益 = 流水汇总，且大于结算前（收益滞后于消费） | `{"earnings": 0.08524645, "grew": true}` | `{"earnings": 0.08524645, "grew": true}` |
| TMET-01 | ✅ | TB 团队摘要（默认 7 天）与直接对 usage_logs / 流水的 SQL 一致 | `{"cost": 1.6803215, "direct_cost": 0.9110715, "sub_cost": 0.76925, "earnings": 0.08524645, "active_customers": 9, "customers": 117, "requests": 21, "tokens": 39064}` | `{"cost": 1.6803214999999998, "direct_cost": 0.9110715, "sub_cost": 0.76925, "earnings": 0.08524645, "active_customers": 9, "customers": 117, "requests": 21, "tokens": 39064}` |
| TMET-02 | ✅ | 趋势 7 个点（含今天、补零），每天消费与收益与 SQL 一致 | `[{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": 0, "earnings": 0}, {"date": "2026-10-27", "cost": 0, "earnings": 0}, {"date": "2026-10-28", "cost": 0, "earnings": 0}, {"date": "2026-10-29", "cost": 0.4, "earnings": 0}, {"date"` | `[{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": 0, "earnings": 0}, {"date": "2026-10-27", "cost": 0, "earnings": 0}, {"date": "2026-10-28", "cost": 0, "earnings": 0}, {"date": "2026-10-29", "cost": 0.4, "earnings": 0}, {"date"` |
| TMET-03 | ✅ | 模型分布：前 5 名 + 其他，金额、占比与 SQL 一致 | `[{"model": "claude-sonnet-4-5", "cost": 0.8892165, "share": 0.5291942643119188}, {"model": "sora-2", "cost": 0.4, "share": 0.2380496827541634}, {"model": "gpt-image-1", "cost": 0.268, "share": 0.15949328744528948}, {"model": "claude-opus-4-1", "cost": 0.03375,` | `[{"model": "claude-sonnet-4-5", "cost": 0.8892165, "share": 0.5291942643119188}, {"model": "sora-2", "cost": 0.4, "share": 0.2380496827541634}, {"model": "gpt-image-1", "cost": 0.268, "share": 0.15949328744528948}, {"model": "claude-opus-4-1", "cost": 0.03375,` |
| TMET-03b | ✅ | 模型分布占比合计为 1、按请求模型统计（映射前的 claude-opus-4-1 在列或并入其他） | `true` | `true` |
| TMET-04 | ✅ | TC1 的订阅消费（billing_type=1）不计入请求数与明细 | `{"subscription_rows": 1, "requests": 8, "list_total": 8}` | `{"subscription_rows": 1, "requests": 8, "list_total": 8}` |
| TMET-05 | ✅ | TB 客户列表每行（112 行）：消费、请求、收益、最近使用、占比、身份、已删除与 SQL 一致 | `{"2550": {"user_id": 2550, "agent_level": 3, "deleted": false, "nickname": "Mike Chen", "cost": 0.0165, "requests": 1, "earnings": 0.00495, "last_used_at": "2026-10-31 15:27:01.518118+08:00", "cost_share": 0.01811054346448111}, "2591": {"user_id": 2591, "agent` | `{"2543": {"user_id": 2543, "agent_level": 3, "deleted": false, "nickname": "张三丰", "cost": 0.8682915, "requests": 8, "earnings": 0.03548745, "last_used_at": "2026-10-31 15:27:02.710012+08:00", "cost_share": 0.9530443000357272}, "2550": {"user_id": 2550, "agent_` |
| TMET-06 | ✅ | TB 下级代理列表每行：客户数、活跃、团队消费、收益、占比与 SQL 一致 | `{"2539": {"agent_id": 2539, "historical": false, "customers": 3, "active_customers": 2, "team_cost": 0.73415, "earnings": 0.033415, "cost_share": 0.9543711407214819}, "2540": {"agent_id": 2540, "historical": false, "customers": 0, "active_customers": 0, "team_` | `{"2539": {"agent_id": 2539, "historical": false, "customers": 3, "active_customers": 2, "team_cost": 0.73415, "earnings": 0.033415, "cost_share": 0.954371140721482}, "2541": {"agent_id": 2541, "historical": true, "customers": 1, "active_customers": 1, "team_co` |
| TMET-07 | ✅ | 客户详情：TC1 我的客户、TA1 / TH1 / TM1 为下级代理的客户、TA 本人是二级；绑定时间与累计收益与库一致 | `{"TC1": {"relation": "owner", "owner_agent": null, "is_sub_agent": false, "bound_at": "2026-10-31 15:26:33.818907+08:00", "lifetime_earnings": 0.03548745}, "TA1": {"relation": "upline", "owner_agent": "TA", "is_sub_agent": false, "bound_at": "2026-10-31 15:26:` | `{"TC1": {"relation": "owner", "owner_agent": null, "is_sub_agent": false, "bound_at": "2026-10-31 15:26:33.818907+08:00", "lifetime_earnings": 0.03548745}, "TA1": {"relation": "upline", "owner_agent": "TA", "is_sub_agent": false, "bound_at": "2026-10-31 15:26:` |
| TMET-08 | ✅ | 北京时间零点：昨天 23:59:30 的 0.5 归昨天，今天 00:00:30 的 0.25 归今天（两者 UTC 同一天） | `{"yesterday_cost": 0.5, "yesterday_rows": ["2026-10-30 23:59:30+08:00"], "today_oldest": "2026-10-31 00:00:30+08:00", "today_cost": 0.7803215}` | `{"yesterday_cost": 0.5, "yesterday_rows": ["2026-10-30 23:59:30+08:00"], "today_oldest": "2026-10-31 00:00:30+08:00", "today_cost": 0.7803215}` |
| TMET-11a | ✅ | TA 视角的摘要、趋势、模型与 SQL 一致 | `{"summary": {"cost": 0.73415, "direct_cost": 0.73415, "sub_cost": 0, "earnings": 0.06683, "active_customers": 2, "customers": 3, "requests": 5, "tokens": 6910}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": 0, "earn` | `{"summary": {"cost": 0.7341500000000001, "direct_cost": 0.7341500000000001, "sub_cost": 0, "earnings": 0.06683, "active_customers": 2, "customers": 3, "requests": 5, "tokens": 6910}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10` |
| TMET-11b | ✅ | TH 视角的摘要、趋势、模型与 SQL 一致 | `{"summary": {"cost": 0.02565, "direct_cost": 0.02565, "sub_cost": 0, "earnings": 0.00513, "active_customers": 1, "customers": 1, "requests": 2, "tokens": 4180}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": 0, "earn` | `{"summary": {"cost": 0.02565, "direct_cost": 0.02565, "sub_cost": 0, "earnings": 0.00513, "active_customers": 1, "customers": 1, "requests": 2, "tokens": 4180}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": 0, "earn` |
| TMET-11c | ✅ | TO 视角的摘要、趋势、模型与 SQL 一致 | `{"summary": {"cost": 0.0135, "direct_cost": 0.0135, "sub_cost": 0, "earnings": 0.002025, "active_customers": 2, "customers": 3, "requests": 2, "tokens": 2200}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": 0, "earni` | `{"summary": {"cost": 0.0135, "direct_cost": 0.0135, "sub_cost": 0, "earnings": 0.002025, "active_customers": 2, "customers": 3, "requests": 2, "tokens": 2200}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": 0, "earni` |
| TMET-11d | ✅ | TA 的团队（TB 视角，agent_id=TA）摘要与 SQL 一致 | `{"summary": {"cost": 0.73415, "direct_cost": 0, "sub_cost": 0.73415, "earnings": 0.033415, "active_customers": 2, "customers": 3, "requests": 5, "tokens": 6910}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": 0, "ear` | `{"summary": {"cost": 0.7341500000000001, "direct_cost": 0, "sub_cost": 0.7341500000000001, "earnings": 0.033415, "active_customers": 2, "customers": 3, "requests": 5, "tokens": 6910}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-1` |
| TMET-11e | ✅ | 单个客户 TC1（TB 视角）摘要、趋势、模型与 SQL 一致 | `{"summary": {"cost": 0.8682915, "direct_cost": 0.8682915, "sub_cost": 0, "earnings": 0.03548745, "active_customers": 1, "customers": 1, "requests": 8, "tokens": 20094}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": ` | `{"summary": {"cost": 0.8682915, "direct_cost": 0.8682915, "sub_cost": 0, "earnings": 0.03548745, "active_customers": 1, "customers": 1, "requests": 8, "tokens": 20094}, "trend": [{"date": "2026-10-25", "cost": 0, "earnings": 0}, {"date": "2026-10-26", "cost": ` |

## TACC

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TACC-01 | ✅ | TB 可进入团队消费页 | `{"team_access": true, "agent_level": 1, "downgraded": false, "settle_interval_minutes": 5, "invitees_in_detail": false}` | `{"team_access": true, "invitees_in_detail": false, "agent_level": 1, "downgraded": false, "settle_interval_minutes": 5}` |
| TACC-02 | ✅ | TA 可进入团队消费页 | `{"team_access": true, "agent_level": 2, "downgraded": false, "settle_interval_minutes": 5, "invitees_in_detail": false}` | `{"team_access": true, "invitees_in_detail": false, "agent_level": 2, "downgraded": false, "settle_interval_minutes": 5}` |
| TACC-03 | ✅ | TH 可进入团队消费页 | `{"team_access": true, "agent_level": 3, "downgraded": true, "settle_interval_minutes": 5, "invitees_in_detail": false}` | `{"team_access": true, "invitees_in_detail": false, "agent_level": 3, "downgraded": true, "settle_interval_minutes": 5}` |
| TACC-04a | ✅ | TN（无快照客户的普通用户）无权访问：入口不显示、四个接口 403 | `{"team_access": false, "overview": [403, "AFFILIATE_TEAM_FORBIDDEN"], "customers": [403, "AFFILIATE_TEAM_FORBIDDEN"], "agents": [403, "AFFILIATE_TEAM_FORBIDDEN"], "requests": [403, "AFFILIATE_TEAM_FORBIDDEN"]}` | `{"team_access": false, "overview": [403, "AFFILIATE_TEAM_FORBIDDEN"], "customers": [403, "AFFILIATE_TEAM_FORBIDDEN"], "agents": [403, "AFFILIATE_TEAM_FORBIDDEN"], "requests": [403, "AFFILIATE_TEAM_FORBIDDEN"]}` |
| TACC-04b | ✅ | TC1（无快照客户的普通用户）无权访问：入口不显示、四个接口 403 | `{"team_access": false, "overview": [403, "AFFILIATE_TEAM_FORBIDDEN"], "customers": [403, "AFFILIATE_TEAM_FORBIDDEN"], "agents": [403, "AFFILIATE_TEAM_FORBIDDEN"], "requests": [403, "AFFILIATE_TEAM_FORBIDDEN"]}` | `{"team_access": false, "overview": [403, "AFFILIATE_TEAM_FORBIDDEN"], "customers": [403, "AFFILIATE_TEAM_FORBIDDEN"], "agents": [403, "AFFILIATE_TEAM_FORBIDDEN"], "requests": [403, "AFFILIATE_TEAM_FORBIDDEN"]}` |
| TACC-05 | ✅ | 返利功能关闭：TB 的团队接口 403，详情不再给出 team_access | `{"overview": [403, "AFFILIATE_TEAM_FORBIDDEN"], "team_access_not_true": true}` | `{"overview": [403, "AFFILIATE_TEAM_FORBIDDEN"], "team_access_not_true": true, "detail": false}` |
| TACC-05b | ✅ | 重新开启后 TB 恢复访问 | `200` | `200` |
| TACC-06 | ✅ | 未登录访问团队接口 | `401` | `401` |

## TRNG

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TRNG-01 | ✅ | 不传日期 = 北京时间最近 7 天（含今天） | `{"first": "2026-10-25", "last": "2026-10-31", "n": 7}` | `{"first": "2026-10-25", "last": "2026-10-31", "n": 7}` |
| TRNG-02 | ✅ | 90 天通过；91 天、结束早于开始、格式错误、只传一端均 400 | `{"90d": 200, "91d": [400, "AFFILIATE_TEAM_INVALID_RANGE"], "reversed": [400, "AFFILIATE_TEAM_INVALID_RANGE"], "bad_format": [400, "AFFILIATE_TEAM_INVALID_RANGE"], "start_only": [400, "AFFILIATE_TEAM_INVALID_RANGE"], "end_only": [400, "AFFILIATE_TEAM_INVALID_RA` | `{"90d": 200, "91d": [400, "AFFILIATE_TEAM_INVALID_RANGE"], "reversed": [400, "AFFILIATE_TEAM_INVALID_RANGE"], "bad_format": [400, "AFFILIATE_TEAM_INVALID_RANGE"], "start_only": [400, "AFFILIATE_TEAM_INVALID_RANGE"], "end_only": [400, "AFFILIATE_TEAM_INVALID_RA` |
| TRNG-03 | ✅ | 30 天范围包含 TC1 10 天前的 0.8，7 天范围不包含 | `{"diff": 0.8, "sql30": 1.6682915}` | `{"diff": 0.8, "sql30": 1.6682915}` |
| TRNG-04 | ✅ | 无数据的日期：摘要为 0、客户数不变、趋势 6 天补零、模型为空 | `{"cost": 0, "requests": 0, "earnings": 0, "active": 0, "customers": 117, "trend": [0, 0, 0, 0, 0, 0], "models": []}` | `{"cost": 0, "requests": 0, "earnings": 0, "active": 0, "customers": 117, "trend": [0, 0, 0, 0, 0, 0], "models": []}` |

## TLST

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TLST-01 | ✅ | 客户列表 4 个排序键 × 升降序：前 100 名顺序与 SQL 一致（空值在后、同值按 user_id 降序） | `{"cost.desc": [2543, 2550, 2539, 2544, 2657, 2656, 2655, 2654, 2653, 2652, 2651, 2650, 2649, 2648, 2647, 2646, 2645, 2644, 2643, 2642, 2641, 2640, 2639, 2638, 2637, 2636, 2635, 2634, 2633, 2632, 2631, 2630, 2629, 2628, 2627, 2626, 2625, 2624, 2623, 2622, 2621,` | `{"cost.desc": [2543, 2550, 2539, 2544, 2657, 2656, 2655, 2654, 2653, 2652, 2651, 2650, 2649, 2648, 2647, 2646, 2645, 2644, 2643, 2642, 2641, 2640, 2639, 2638, 2637, 2636, 2635, 2634, 2633, 2632, 2631, 2630, 2629, 2628, 2627, 2626, 2625, 2624, 2623, 2622, 2621,` |
| TLST-02 | ✅ | 分页：默认 20 条；page_size=500 截为 100；越界页为空；两页合起来不重不漏 | `{"default": [20, 20, 112, 6], "capped": [100, 100], "far": [0, 112], "all": [2543, 2550, 2539, 2544, 2657, 2656, 2655, 2654, 2653, 2652, 2651, 2650, 2649, 2648, 2647, 2646, 2645, 2644, 2643, 2642, 2641, 2640, 2639, 2638, 2637, 2636, 2635, 2634, 2633, 2632, 263` | `{"default": [20, 20, 112, 6], "capped": [100, 100], "far": [0, 112], "all": [2543, 2550, 2539, 2544, 2657, 2656, 2655, 2654, 2653, 2652, 2651, 2650, 2649, 2648, 2647, 2646, 2645, 2644, 2643, 2642, 2641, 2640, 2639, 2638, 2637, 2636, 2635, 2634, 2633, 2632, 263` |
| TLST-03 | ✅ | 非法 sort_by 回落到按消费降序 | `[2543, 2550, 2539, 2544, 2657, 2656, 2655, 2654, 2653, 2652, 2651, 2650, 2649, 2648, 2647, 2646, 2645, 2644, 2643, 2642]` | `[2543, 2550, 2539, 2544, 2657, 2656, 2655, 2654, 2653, 2652, 2651, 2650, 2649, 2648, 2647, 2646, 2645, 2644, 2643, 2642]` |
| TLST-04 | ✅ | 下级代理列表 3 个排序键 × 升降序与 SQL 一致 | `{"team_cost.desc": [2539, 2541, 2542, 2540], "team_cost.asc": [2540, 2542, 2541, 2539], "earnings.desc": [2539, 2541, 2542, 2540], "earnings.asc": [2540, 2542, 2541, 2539], "active_customers.desc": [2539, 2542, 2541, 2540], "active_customers.asc": [2540, 2542,` | `{"team_cost.desc": [2539, 2541, 2542, 2540], "team_cost.asc": [2540, 2542, 2541, 2539], "earnings.desc": [2539, 2541, 2542, 2540], "earnings.asc": [2540, 2542, 2541, 2539], "active_customers.desc": [2539, 2542, 2541, 2540], "active_customers.asc": [2540, 2542,` |
| TLST-05 | ✅ | 客户搜索：昵称模糊（去空格、不分大小写）、完整邮箱精确；邮箱前缀 / 域名、%、_ 只按字面；已删除与范围外搜不到 | `{"nickname": ["TC1"], "nickname_trim": ["TC1"], "nickname_ci": ["TC1x"], "email_exact_ci": ["TC1"], "email_prefix": [], "email_domain": [], "percent": ["TF007"], "underscore": ["TF007"], "deleted": [], "other_scope": [], "in_agent_team": ["TA1"]}` | `{"nickname": ["TC1"], "nickname_trim": ["TC1"], "nickname_ci": ["TC1x"], "email_exact_ci": ["TC1"], "email_prefix": [], "email_domain": [], "percent": ["TF007"], "underscore": ["TF007"], "deleted": [], "other_scope": [], "in_agent_team": ["TA1"]}` |

## TREQ

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TREQ-01 | ✅ | TC1 明细与 SQL 逐行一致（时间倒序），字段只有时间、模型、分组、计费方式、token、张数 / 条数、扣费 | `{"rows": [{"created_at": "2026-10-31 15:27:02.710012+08:00", "model": "claude-sonnet-4-5", "group_name": "e2e-team-std-072240", "billing_mode": "token", "input_tokens": 777, "output_tokens": 77, "cache_creation_tokens": 0, "cache_read_tokens": 0, "image_count"` | `{"rows": [{"created_at": "2026-10-31 15:27:02.710012+08:00", "model": "claude-sonnet-4-5", "group_name": "e2e-team-std-072240", "billing_mode": "token", "input_tokens": 777, "output_tokens": 77, "cache_creation_tokens": 0, "cache_read_tokens": 0, "image_count"` |
| TREQ-02a | ✅ | 明细筛选项：区间内的请求模型与分组（含无分组 id=0） | `{"models": ["claude-3-7-sonnet-20250219", "claude-haiku-4-5", "claude-opus-4-5", "claude-sonnet-4-5"], "groups": [{"id": 0, "name": ""}, {"id": 236, "name": "e2e-team-std-072240"}, {"id": 237, "name": "e2e-team-alt-072240"}]}` | `{"models": ["claude-3-7-sonnet-20250219", "claude-haiku-4-5", "claude-opus-4-5", "claude-sonnet-4-5"], "groups": [{"id": 0, "name": ""}, {"id": 236, "name": "e2e-team-std-072240"}, {"id": 237, "name": "e2e-team-alt-072240"}]}` |
| TREQ-02b | ✅ | 按模型、分组、无分组、模型 + 分组筛选的结果与 SQL 一致且非空 | `{"model": true, "group_alt": true, "no_group": true, "model_and_group": true}` | `{"model": true, "group_alt": true, "no_group": true, "model_and_group": true}` |
| TREQ-03 | ✅ | TA1 明细：生图行 2 张（gpt-image-1），视频行 1 条（sora-2，requested_model 为空时显示 model） | `{"images": [["gpt-image-1", 2]], "videos": [["sora-2", 1]]}` | `{"images": [["gpt-image-1", 2]], "videos": [["sora-2", 1]]}` |
| TREQ-04 | ✅ | 明细 page_size=2 逐页翻完：不重不漏、顺序不变 | `[{"created_at": "2026-10-31 15:27:02.710012+08:00", "model": "claude-sonnet-4-5", "group_name": "e2e-team-std-072240", "billing_mode": "token", "input_tokens": 777, "output_tokens": 77, "cache_creation_tokens": 0, "cache_read_tokens": 0, "image_count": 0, "vid` | `[{"created_at": "2026-10-31 15:27:02.710012+08:00", "model": "claude-sonnet-4-5", "group_name": "e2e-team-std-072240", "billing_mode": "token", "input_tokens": 777, "output_tokens": 77, "cache_creation_tokens": 0, "cache_read_tokens": 0, "image_count": 0, "vid` |

## TOOS

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TOOS-01 | ✅ | 越权：范围外的二级代理 / 客户 / 明细（含改挂后 TO 看 TM1）一律 404，响应不含目标数据 | `{"00 TB /overview {'agent_id': 2543}": [404, "AFFILIATE_TEAM_NOT_FOUND", false], "01 TB /overview {'agent_id': 2536}": [404, "AFFILIATE_TEAM_NOT_FOUND", false], "02 TB /overview {'agent_id': 99999999}": [404, "AFFILIATE_TEAM_NOT_FOUND", false], "03 TB /custome` | `{"00 TB /overview {'agent_id': 2543}": [404, "AFFILIATE_TEAM_NOT_FOUND", false], "01 TB /overview {'agent_id': 2536}": [404, "AFFILIATE_TEAM_NOT_FOUND", false], "02 TB /overview {'agent_id': 99999999}": [404, "AFFILIATE_TEAM_NOT_FOUND", false], "03 TB /custome` |
| TOOS-04 | ✅ | 伪造参数：agent_id 与 user_id 同传、非数字、0、负数、group_id=-1 均 400 | `{"both": 400, "agent_abc": 400, "agent_zero": 400, "user_neg": 400, "requests_abc": 400, "group_neg": 400, "customers_agent_abc": 400}` | `{"both": 400, "agent_abc": 400, "agent_zero": 400, "user_neg": 400, "requests_abc": 400, "group_neg": 400, "customers_agent_abc": 400}` |

## THIS

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| THIS-01 | ✅ | 取消任命的 TH：TB 列表中为历史，TH1 的消费与 TB 的差额收益照常统计 | `{"historical": true, "team_cost": 0.02565, "earnings": 0.002565, "earned": true, "subject_historical": true}` | `{"historical": true, "team_cost": 0.02565, "earnings": 0.002565, "earned": true, "subject_historical": true}` |
| THIS-02 | ✅ | 取消后用 TH 的码注册的 TH2 归属 TB，出现在 TB 的「我的客户」且有消费 | `{"listed": true, "cost_positive": true}` | `{"listed": true, "cost_positive": true}` |
| THIS-03 | ✅ | 改挂到 TO 的 TM：TB 列表中为历史，TM1 仍在 TB 团队；TM 本人移出 TB 的客户、进入 TO 的客户 | `{"historical": true, "tm1_relation": "upline", "tm1_owner": "TM", "tm_in_tb": false, "tm_in_to": true}` | `{"historical": true, "tm1_relation": "upline", "tm1_owner": "TM", "tm_in_tb": false, "tm_in_to": true}` |
| THIS-04 | ✅ | 管理员删除的 TD：昵称与邮箱为空、deleted=true；消费与收益保留 | `{"user": {"user_id": 2544, "nickname": "", "email": "", "deleted": true}, "cost": 0.0108, "earnings": 0.00324, "positive": true, "detail_user": {"user_id": 2544, "nickname": "", "email": "", "deleted": true}}` | `{"user": {"user_id": 2544, "nickname": "", "email": "", "deleted": true}, "cost": 0.0108, "earnings": 0.00324, "positive": true, "detail_user": {"user_id": 2544, "nickname": "", "email": "", "deleted": true}}` |
| THIS-05 | ✅ | TA 升为一级：TB 列表中 TA 变历史，TA 的老客户仍计入 TB 团队；TA 视角为一级、客户不变 | `{"historical": true, "team_customers": 117, "ta1_relation": "upline", "ta_level": 1, "ta_customers": 3}` | `{"historical": true, "team_customers": 117, "ta1_relation": "upline", "ta_level": 1, "ta_customers": 3}` |

## TINV

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TINV-01 | ✅ | TB 邀请列表：共 112（含归属他人的 X0、已删除的 TD），每页 20，按绑定时间倒序翻完与 SQL 一致 | `{"total": 112, "page_size": 20, "pages": 6, "order": [2656, 2655, 2654, 2653, 2652, 2651, 2650, 2649, 2648, 2647, 2646, 2645, 2644, 2643, 2642, 2641, 2640, 2639, 2638, 2637, 2636, 2635, 2634, 2633, 2632, 2631, 2630, 2629, 2628, 2627, 2626, 2625, 2624, 2623, 26` | `{"total": 112, "page_size": 20, "pages": 6, "order": [2656, 2655, 2654, 2653, 2652, 2651, 2650, 2649, 2648, 2647, 2646, 2645, 2644, 2643, 2642, 2641, 2640, 2639, 2638, 2637, 2636, 2635, 2634, 2633, 2632, 2631, 2630, 2629, 2628, 2627, 2626, 2625, 2624, 2623, 26` |
| TINV-02 | ✅ | 邀请列表搜索：昵称模糊命中第 100 名以后的 TF003；完整邮箱精确；邮箱前缀 / 域名、%、_ 只按字面；已删除搜不到 | `{"nickname": ["TF003"], "nickname_prefix": ["TF003", "TF004"], "email_exact_ci": ["TF003"], "email_prefix": [], "percent": ["TF007"], "underscore": ["TF007"], "deleted": [], "domain": []}` | `{"nickname": ["TF003"], "nickname_prefix": ["TF003", "TF004"], "email_exact_ci": ["TF003"], "email_prefix": [], "percent": ["TF007"], "underscore": ["TF007"], "deleted": [], "domain": []}` |
| TINV-03 | ✅ | 归属与任命标记：X0 归属他人不可任命；TC1 可任命；TA、TZ 由 TB 任命；TH 已取消（锁定） | `{"X0": {"owned_by_me": false, "appointable": false, "appointed_by_me": false, "agent_level": 3}, "TC1": {"owned_by_me": true, "appointable": true, "appointed_by_me": false, "agent_level": 3}, "TA": {"owned_by_me": true, "appointable": false, "appointed_by_me":` | `{"X0": {"owned_by_me": false, "appointable": false, "appointed_by_me": false, "agent_level": 3}, "TC1": {"owned_by_me": true, "appointable": true, "appointed_by_me": false, "agent_level": 3}, "TA": {"owned_by_me": true, "appointable": false, "appointed_by_me":` |
| TINV-03b | ✅ | （记录）TH（被 TB 取消任命、锁定自动升级）在 TB 邀请列表中的标记 | `"info"` | `{"owned_by_me": true, "appointable": true, "appointed_by_me": false, "agent_level": 3}` |
| TINV-03c | ✅ | 被管理员改挂到 TO 的 TM：TB 不能再取消，列表不显示「取消」（与任命接口校验一致） | `{"owned_by_me": false, "appointable": false, "appointed_by_me": false, "agent_level": 2, "revoke": [400, "AFFILIATE_APPOINT_TARGET_INVALID"]}` | `{"owned_by_me": false, "appointable": false, "appointed_by_me": false, "agent_level": 2, "revoke": [400, "AFFILIATE_APPOINT_TARGET_INVALID"]}` |
| TINV-04 | ✅ | 接口任命 / 取消 TF004：列表标记与 appoint.used 同步变化 | `{"appointed": [2, true, 3], "revoked": [3, false, 2]}` | `{"appointed": [2, true, 3], "revoked": [3, false, 2]}` |
| TINV-05 | ✅ | 邀请列表不含余额等字段，邮箱均脱敏 | `{"leaked": [], "masked": true}` | `{"leaked": [], "masked": true}` |
| TINV-06 | ✅ | 普通用户 / 二级也能用邀请列表接口（TM 自己的邀请列表含 TM1） | `[200, ["TM1"]]` | `[200, ["TM1"]]` |
| TINV-07 | ✅ | 任命名额只计 TB 还能取消的二级：已任命数 = 列表中可「取消」的 TA、TZ；改挂到 TO 的 TM 仍记着由 TB 任命，但不占名额 | `{"used": 2, "revocable": ["TA", "TZ"], "tm_recorded_as_tb_appointee": 1}` | `{"used": 2, "revocable": ["TA", "TZ"], "tm_recorded_as_tb_appointee": 1}` |
| TINV-08 | ✅ | 已删除的客户 TD 仍在列表里，但不显示「设为二级」；接口任命返回 404 | `{"row": {"owned_by_me": true, "appointable": false, "appointed_by_me": false, "agent_level": 3}, "appoint": [404, "USER_NOT_FOUND"]}` | `{"row": {"owned_by_me": true, "appointable": false, "appointed_by_me": false, "agent_level": 3}, "appoint": [404, "USER_NOT_FOUND"]}` |
| TINV-09 | ✅ | 上限设为已用 + 1：任命新邀请的 Q1 后用满，再任命 Q2 被拒；删除 Q1 后名额释放、Q2 可任命；Q1 那一行不再显示「取消」（取消返回 404） | `{"full": [409, "AFFILIATE_APPOINT_LIMIT_REACHED"], "used_after_delete": 1, "row": {"agent_level": 2, "appointed_by_me": false, "appointable": false}, "revoke": [404, "USER_NOT_FOUND"], "again": 2}` | `{"full": [409, "AFFILIATE_APPOINT_LIMIT_REACHED"], "used_after_delete": 1, "row": {"agent_level": 2, "appointed_by_me": false, "appointable": false}, "revoke": [404, "USER_NOT_FOUND"], "again": 2}` |

## TPRV

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TPRV-01 | ✅ | 全部 85 个团队接口响应均不含余额、IP、UA、Key、账号、上游模型、费用拆分、倍率、请求 / 会话 ID | `{}` | `{}` |
| TPRV-02 | ✅ | 响应原文不出现任何完整邮箱（含邀请列表）；email 字段均为脱敏形式；不出现上游模型名 | `{"full_email_in": [], "bad_mask": [], "upstream_model_in": []}` | `{"full_email_in": [], "bad_mask": [], "upstream_model_in": []}` |

## TADM

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TADM-01 | ✅ | 六个管理端接口：未登录 401、代理（非管理员）403、管理员 200 | `{"/overview": [401, 403, 200], "/agents": [401, 403, 200], "/2537/overview": [401, 403, 200], "/2537/customers": [401, 403, 200], "/2537/agents": [401, 403, 200], "/2537/customers/2543/requests": [401, 403, 200]}` | `{"/overview": [401, 403, 200], "/agents": [401, 403, 200], "/2537/overview": [401, 403, 200], "/2537/customers": [401, 403, 200], "/2537/agents": [401, 403, 200], "/2537/customers/2543/requests": [401, 403, 200]}` |
| TADM-02 | ✅ | 返利功能关闭时管理端仍可查看（与其他管理端邀请返利接口一致），代理端 403 | `{"admin_viewer": 200, "admin_summary": 200, "agent": 403}` | `{"admin_viewer": 200, "admin_summary": 200, "agent": 403}` |
| TADM-03 | ✅ | 管理端接口不加重型限流：1 分钟内 70 次全部 200 | `[200]` | `[200]` |
| TADM-04 | ✅ | 收录：与 SQL 的「有团队的人」完全相同（一级、二级、已降级有老客户、已删除的二级）；客户与普通用户不收；等级计数一致 | `{"ids": [2, 3, 4, 9, 10, 13, 24, 26, 28, 30, 33, 36, 42, 47, 48, 52, 53, 55, 56, 57, 58, 174, 175, 177, 178, 179, 180, 181, 186, 187, 190, 201, 203, 205, 207, 210, 213, 219, 224, 225, 229, 230, 232, 233, 234, 235, 351, 352, 354, 355, 356, 357, 358, 363, 364, 3` | `{"ids": [2, 3, 4, 9, 10, 13, 24, 26, 28, 30, 33, 36, 42, 47, 48, 52, 53, 55, 56, 57, 58, 174, 175, 177, 178, 179, 180, 181, 186, 187, 190, 201, 203, 205, 207, 210, 213, 219, 224, 225, 229, 230, 232, 233, 234, 235, 351, 352, 354, 355, 356, 357, 358, 363, 364, 3` |
| TADM-05 | ✅ | 汇总条与直接 SQL 一致：渠道客户消费（一级与其二级重叠的客户只算一次）、全站余额消费、全部代理分润（不限来源）、活跃 / 客户；趋势与模型分布一致 | `{"summary": {"cost": 810.7568275, "platform_cost": 945.7550895, "earnings": 11633.98802325, "active_customers": 74, "customers": 1996}, "trend": [{"date": "2026-10-25", "cost": 325.197563, "earnings": 2287.2096129}, {"date": "2026-10-26", "cost": 161.8827215, ` | `{"summary": {"cost": 810.7568275000002, "platform_cost": 945.7550895000005, "earnings": 11633.98802325, "active_customers": 74, "customers": 1996}, "trend": [{"date": "2026-10-25", "cost": 325.197563, "earnings": 2287.2096129}, {"date": "2026-10-26", "cost": 1` |
| TADM-06 | ✅ | 每行的团队消费、活跃 / 客户、分润与该代理自己 overview 的摘要相同；全部行与 SQL 一致；TADMO 来自团队外 TN 的 0.123 分润不计入其行、计入汇总条 | `{"vs_own_overview": {}, "vs_sql": {}, "tadmo_row": 0.0, "tadmo_excluded": 0.123, "ta_upline": "TB", "tadmd_customers": 1}` | `{"vs_own_overview": {}, "vs_sql": {}, "tadmo_row": 0, "tadmo_excluded": 0.123, "ta_upline": "TB", "tadmd_customers": 1}` |
| TADM-07 | ✅ | 汇总页 3 个排序键 × 升降序与 SQL 一致（同值按 user_id 降序）；默认按团队消费降序 | `{"team_cost.desc": [1344, 1166, 2089, 1548, 2532, 2531, 2090, 1549, 1345, 1167, 2095, 2537, 1554, 1350, 1172, 2097, 2539, 1556, 1352, 1174, 2541, 2099, 1558, 1354, 1176, 2536, 2094, 1553, 1349, 1171, 2542, 2100, 1559, 1355, 1177, 2659, 2658, 2540, 2521, 2515, ` | `{"team_cost.desc": [1344, 1166, 2089, 1548, 2532, 2531, 2090, 1549, 1345, 1167, 2095, 2537, 1554, 1350, 1172, 2097, 2539, 1556, 1352, 1174, 2541, 2099, 1558, 1354, 1176, 2536, 2094, 1553, 1349, 1171, 2542, 2100, 1559, 1355, 1177, 2659, 2658, 2540, 2521, 2515, ` |
| TADM-08 | ✅ | 等级筛选：一级 / 二级 / 已降级与 SQL 一致；非法值按全部 | `{"1": [2, 3, 9, 13, 24, 30, 33, 36, 47, 52, 53, 55, 174, 179, 180, 186, 190, 201, 207, 210, 213, 224, 229, 230, 232, 351, 356, 357, 363, 367, 378, 384, 387, 390, 401, 406, 407, 409, 528, 538, 539, 545, 549, 560, 566, 569, 572, 583, 588, 589, 591, 710, 715, 716` | `{"1": [2, 3, 9, 13, 24, 30, 33, 36, 47, 52, 53, 55, 174, 179, 180, 186, 190, 201, 207, 210, 213, 224, 229, 230, 232, 351, 356, 357, 363, 367, 378, 384, 387, 390, 401, 406, 407, 409, 528, 538, 539, 545, 549, 560, 566, 569, 572, 583, 588, 589, 591, 710, 715, 716` |
| TADM-09 | ✅ | 搜索：昵称 / 邮箱模糊（不分大小写、去首尾空格）、纯数字另按 ID 精确（已删除也能找到）；已删除账号不参与名称搜索；%、_ 只按字面 | `{"nickname": [52, 53, 229, 230, 406, 407, 588, 589, 765, 766, 942, 943, 1171, 1172, 1349, 1350, 1553, 1554, 2094, 2095, 2536, 2537], "nickname_literal": [174, 351, 528, 710, 887, 1064, 1293, 1471, 1675, 2216, 2658], "email_fragment": [2537], "email_upper": [25` | `{"nickname": [52, 53, 229, 230, 406, 407, 588, 589, 765, 766, 942, 943, 1171, 1172, 1349, 1350, 1553, 1554, 2094, 2095, 2536, 2537], "nickname_literal": [174, 351, 528, 710, 887, 1064, 1293, 1471, 1675, 2216, 2658], "email_fragment": [2537], "email_upper": [25` |
| TADM-10 | ✅ | 分页：page_size=3 第 2 页为默认排序的第 4–6 名；越界页为空、total 不变 | `{"page2": [1548, 2532, 2531], "far": [0, 429]}` | `{"page2": [1548, 2532, 2531], "far": [0, 429]}` |
| TADM-11 | ✅ | 代理视角与该代理自己的接口逐项相同（总览三层、客户列表含二级团队与排序、下级代理、请求明细），只是邮箱不脱敏；代理端响应不含 subject | `{}` | `{}` |
| TADM-12 | ✅ | 管理端邮箱为完整原文（与注册邮箱一致），代理端同一行仍为脱敏形式；搜索支持邮箱片段与 ID | `{"admin": {"2543": "e2e-072240-tc1@e2e.test", "2550": "e2e-072240-tc1x@e2e.test", "2539": "e2e-072240-ta@e2e.test", "2657": "e2e-072240-th2@e2e.test"}, "agent_masked": true, "search_email": [2543], "search_id": [2657]}` | `{"admin": {"2543": "e2e-072240-tc1@e2e.test", "2550": "e2e-072240-tc1x@e2e.test", "2539": "e2e-072240-ta@e2e.test", "2657": "e2e-072240-th2@e2e.test"}, "agent_masked": true, "search_email": [2543], "search_id": [2657]}` |
| TADM-13 | ✅ | subject：完整邮箱、上级（仅二级）、累计分润（全部历史，与流水一致）、等级与降级标记；已删除的二级只留 ID | `{"TB": {"user_id": 2537, "email": "e2e-072240-tb@e2e.test", "deleted": false, "upline": null, "lifetime": 0.08524645, "level": 1, "downgraded": false}, "TA": {"user_id": 2539, "email": "e2e-072240-ta@e2e.test", "deleted": false, "upline": 2537, "lifetime": 0.0` | `{"TB": {"user_id": 2537, "email": "e2e-072240-tb@e2e.test", "deleted": false, "upline": null, "lifetime": 0.08524645, "level": 1, "downgraded": false}, "TA": {"user_id": 2539, "email": "e2e-072240-ta@e2e.test", "deleted": false, "upline": 2537, "lifetime": 0.0` |
| TADM-14 | ✅ | 越权组合与非法参数：普通用户 / 客户 / 不存在的 ID 404；非数字与 0 为 400；范围外的客户、二级与明细 404；agent_id 与 user_id 互斥；超过 90 天 400、90 天正常 | `{"viewer_normal_user": [404, "AFFILIATE_TEAM_NOT_FOUND"], "viewer_customer": [404, "AFFILIATE_TEAM_NOT_FOUND"], "viewer_missing": [404, "AFFILIATE_TEAM_NOT_FOUND"], "viewer_not_number": [400, null], "viewer_zero": [400, null], "customer_out_of_scope": [404, "A` | `{"viewer_normal_user": [404, "AFFILIATE_TEAM_NOT_FOUND"], "viewer_customer": [404, "AFFILIATE_TEAM_NOT_FOUND"], "viewer_missing": [404, "AFFILIATE_TEAM_NOT_FOUND"], "viewer_not_number": [400, null], "viewer_zero": [400, null], "customer_out_of_scope": [404, "A` |
| TADM-15 | ✅ | 全部 162 个管理端团队接口响应与代理端字段集合相同：不含余额、IP、UA、Key、账号、上游模型、费用拆分、倍率等 | `{}` | `{}` |
| TADM-16 | ✅ | 分润日汇总与流水重算逐行一致（金额与条数，含已删除来源记为 0）（749 行） | `{"missing": 0, "extra": 0, "mismatched": 0}` | `{"missing": 0, "extra": 0, "mismatched": 0}` |

## TUI

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TUI-01 | ✅ | TB 的邀请页：有「邀请 / 团队消费」标签与昵称列；每页 20 行；空昵称显示 - | `{"tabs": 1, "nicknameCol": 1, "rows": 20, "total": 112, "firstNick": "-"}` | `{"tabs": 1, "nicknameCol": 1, "rows": 20, "total": 112, "firstNick": "-"}` |
| TUI-02 | ✅ | 搜索第 100 名以后的 TF003 → 设为分销代理 → 确认：任命计数 +1，行内显示分销代理与「取消」 | `{"rows": 1, "nick": "待任命小王", "delta": 1, "role": "分销代理 取消"}` | `{"rows": 1, "nick": "待任命小王", "delta": 1, "role": "分销代理 取消"}` |
| TUI-03 | ✅ | 取消任命 → 确认：计数 −1，行内不再显示「取消」（记录取消后的身份列） | `{"delta": -1, "revokeButtons": 0}` | `{"delta": -1, "revokeButtons": 0, "role": "普通 设为分销代理"}` |
| TUI-04 | ✅ | 按邮箱前缀搜索：显示「没有匹配的好友」 | `"没有匹配的好友"` | `"没有匹配的好友"` |
| TUI-05 | ✅ | 邀请列表翻到第 2 页：行变化，接口 page=2 | `{"changed": true, "page": 2, "rows": 20}` | `{"changed": true, "page": 2, "rows": 20}` |
| TUI-05b | ✅ | 翻到已删除的 TD 所在页：那一行身份列显示「— 不可任命」，没有「设为分销代理」/「取消」按钮 | `{"buttons": 0, "role": "— 不可任命"}` | `{"page": 6, "buttons": 0, "role": "— 不可任命"}` |
| TUI-06 | ✅ | 切到团队消费：摘要与页面收到的接口数据一致；客户表 20 行；分销代理标签数 4 | `{"path": "/affiliate/team", "cost": 1.6803214999999998, "earnings": 0.08524645, "active": "9 / 117", "rows": 20, "agentsTab": 4}` | `{"path": "/affiliate/team", "cost": "¥1.68", "earnings": "¥0.09", "active": "9 / 117", "rows": 20, "agentsTab": 4}` |
| TUI-07 | ✅ | 分销代理：TH、TM 带「历史」，TA 不带；URL tab=agents | `{"TH": true, "TM": true, "TA": false, "tab": "agents", "rows": 4}` | `{"TH": true, "TM": true, "TA": false, "tab": "agents", "rows": 4}` |
| TUI-08 | ✅ | 点 TA → 点 TA1：面包屑逐级；TA 团队 3 人；明细行数与接口一致；生图显示张数、视频显示条数 | `{"agentPath": "/affiliate/team/agents/2539", "crumb1": "团队消费 / A二级", "title": "A二级的客户 3", "path": "/affiliate/team/customers/2545", "crumb2": "团队消费 / A二级 / 甲客户", "rows": 4, "images": ["2 张"], "videos": ["1 条"]}` | `{"agentPath": "/affiliate/team/agents/2539", "crumb1": "团队消费 / A二级", "title": "A二级的客户 3", "path": "/affiliate/team/customers/2545", "crumb2": "团队消费 / A二级 / 甲客户", "rows": 4, "images": ["2 张"], "videos": ["1 条"]}` |
| TUI-09 | ✅ | 按模型筛选：URL 带 model、每行都是该模型；再点模型分布条取消筛选 | `{"urlModel": "claude-sonnet-4-5", "rowsModel": ["claude-sonnet-4-5"], "cleared": null, "allRows": true}` | `{"urlModel": "claude-sonnet-4-5", "rowsModel": ["claude-sonnet-4-5"], "cleared": null, "allRows": true}` |
| TUI-10 | ✅ | 浏览器后退：依次回到 TA 团队页、tab=agents 的团队页（筛选用 replace，不产生历史） | `{"first": "/affiliate/team/agents/2539", "second": "/affiliate/team", "tab": "agents", "selected": "true"}` | `{"first": "/affiliate/team/agents/2539", "second": "/affiliate/team", "tab": "agents", "selected": "true"}` |
| TUI-11 | ✅ | 日期预设「近 30 天」：URL start/end 更新；摘要与接口、SQL 一致；趋势 30 个点 | `{"start": "2026-10-02", "end": "2026-10-31", "cost": 2.48, "trend": 30}` | `{"start": "2026-10-02", "end": "2026-10-31", "cost": 2.48, "trend": 30}` |
| TUI-12 | ✅ | 客户搜索「张三」：URL 带 q、1 行；刷新页面后仍是过滤结果 | `{"before": {"q": "张三", "rows": 1}, "after": {"q": "张三", "rows": 1, "input": "张三", "start": "2026-10-02"}}` | `{"before": {"q": "张三", "rows": 1}, "after": {"q": "张三", "rows": 1, "input": "张三", "start": "2026-10-02"}}` |
| TUI-13 | ✅ | 点「请求」列：URL sort=requests、降序；首行为请求最多的客户 | `{"sort": "requests", "first": "张三丰"}` | `{"sort": "requests", "order": "desc", "first": "张三丰 e***@e***.test ¥1.67 9 ¥0.04 2分钟前"}` |
| TUI-14 | ✅ | 已删除客户行显示「已删除」，点击不跳转 | `{"stays": true}` | `{"stays": true, "text": "已删除 ¥0.01 1 <¥0.01 2分钟前"}` |
| TUI-15 | ✅ | 打开范围外客户 X0 的链接：显示「客户不存在或无权查看」与返回链接 | `"客户不存在或无权查看 返回团队消费"` | `"客户不存在或无权查看 返回团队消费"` |
| TUI-16b | ✅ | TC1 明细：token 明细与分组名显示（含「无分组」） | `{"tokens": true, "groups": true}` | `{"tokens": true, "groups": true, "sample": ["e2e-team-std-072240", "e2e-team-alt-072240", "无分组"]}` |
| TUI-16 | ✅ | 普通用户 TN：邀请页无标签；直接打开团队页显示无权限卡片 | `{"tabs": 0, "card": "团队消费仅对代理开放 返回邀请返利"}` | `{"tabs": 0, "card": "团队消费仅对代理开放 返回邀请返利"}` |
| TUI-17 | ✅ | 降级用户 TH：团队页显示降级提示 | `"你当前不是代理，以下为代理期间归属你的客户，分润照常"` | `"你当前不是代理，以下为代理期间归属你的客户，分润照常"` |
| TUI-18 | ✅ | 手机宽度 390px：团队页、客户页、邀请页均无横向溢出（表格内部滚动） | `{"team": 390, "customer": 390, "invite": 390}` | `{"team": 390, "customer": 390, "invite": 390}` |
| TUI-00 | ✅ | 界面脚本正常结束并输出结果 | `{"rc": 0, "results>=18": true}` | `{"rc": 0, "results>=18": true, "stderr": ""}` |

## TAUI

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TAUI-01 | ✅ | 侧边栏「邀请返利」分组：团队消费排在代理管理之后；点击进入汇总页，顶部栏标题为「团队消费」 | `{"order": ["/admin/affiliates/agents", "/admin/affiliates/team", "/admin/affiliates/invites"], "path": "/admin/affiliates/team", "title": true, "loaded": true}` | `{"order": ["/admin/affiliates/agents", "/admin/affiliates/team", "/admin/affiliates/invites"], "path": "/admin/affiliates/team", "title": true, "loaded": true}` |
| TAUI-02 | ✅ | 汇总条三项与接口一致（消费附全站占比、分润附团队消费占比）；等级标签带人数；无面包屑；首行为团队消费最多的代理 | `{"cost": 810.7568275000002, "earnings": 11633.98802325, "active": "74 / 1,996", "costSub": "占全站余额消费", "earningsSub": "占团队消费", "tabs": ["全部 429", "渠道经理 229", "分销代理 189", "已降级 11"], "breadcrumb": 0, "firstRow": "-"}` | `{"cost": "¥810.76", "earnings": "¥11,633.99", "active": "74 / 1,996", "costSub": "占全站余额消费 86%", "earningsSub": "占团队消费 1435.0%", "tabs": ["全部 429", "渠道经理 229", "分销代理 189", "已降级 11"], "breadcrumb": 0, "firstRow": "- e2e-044448-cb@e2e.test 渠道经理 4 / 4 ¥161.44 ¥40.` |
| TAUI-03 | ✅ | 搜索「总代」命中 TB、TO；切到「渠道经理」页码归 1、搜索保留、只列渠道经理；按 TH 的 ID 搜到 TH；「分销代理」标签只列分销代理（等级列为「分销代理 · 上级」） | `{"byName": {"q": "总代", "total": 22, "hasTB": true, "hasTO": true}, "level1": {"level": "1", "q": "总代", "page": null, "levels": ["渠道经理"]}, "byId": true, "level2": ["分销代理"]}` | `{"byName": {"q": "总代", "total": 22, "hasTB": true, "hasTO": true}, "level1": {"level": "1", "q": "总代", "page": null, "levels": ["渠道经理"]}, "byId": true, "level2": ["分销代理"]}` |
| TAUI-04 | ✅ | 点 TB 的行：进入 TB 的视角，下钻只带日期；面包屑「团队消费 / B总代」；元信息行有完整邮箱、#ID、渠道经理、累计分润；标签为「直属客户 / 分销代理」；侧边栏仍高亮「团队消费」 | `{"path": "/admin/affiliates/team/2537", "query": ["end", "start"], "crumbs": "团队消费 / B总代", "meta": ["e2e-072240-tb@e2e.test", "#2537", "渠道经理", "累计分润"], "tabs": ["直属客户", "分销代理"], "lifetime": true, "sidebarActive": true}` | `{"path": "/admin/affiliates/team/2537", "query": ["end", "start"], "crumbs": "团队消费 / B总代", "meta": "e2e-072240-tb@e2e.test · #2537 · 渠道经理 · 累计分润 ¥0.09", "tabs": ["直属客户", "分销代理"], "lifetime": true, "sidebarActive": true}` |
| TAUI-05 | ✅ | 分销代理 → TA：面包屑「团队消费 / B总代 / A二级」；元信息行有 TA 的完整邮箱与 #ID | `{"path": "/admin/affiliates/team/2537/agents/2539", "crumbs": "团队消费 / B总代 / A二级", "meta": true}` | `{"path": "/admin/affiliates/team/2537/agents/2539", "crumbs": "团队消费 / B总代 / A二级", "meta": true}` |
| TAUI-06 | ✅ | 点 TA1：面包屑四级；元信息行有完整邮箱、#ID、「A二级的客户」、累计分润；「使用记录 →」本页跳到管理端使用记录（带用户与日期），浏览器返回回到客户页 | `{"crumbs": "团队消费 / B总代 / A二级 / 甲客户", "meta": ["e2e-072240-ta1@e2e.test", "#2545", "A二级的客户", "累计分润", "使用记录"], "usage": {"user_id": "2545", "start_date": "2026-10-25", "end_date": "2026-10-31"}, "request": {"user_id": "2545", "start_date": "2026-10-25", "end_dat` | `{"crumbs": "团队消费 / B总代 / A二级 / 甲客户", "meta": "e2e-072240-ta1@e2e.test · #2545 · A二级的客户 · 2026-10-31 绑定 · 累计分润 ¥0.03 · 使用记录 →", "usage": {"user_id": "2545", "start_date": "2026-10-25", "end_date": "2026-10-31"}, "request": {"user_id": "2545", "start_date": "202` |
| TAUI-07 | ✅ | 面包屑首项「团队消费」回到汇总页：带回离开时的等级、搜索与排序；搜索框与标签状态恢复 | `{"path": "/admin/affiliates/team", "level": "1", "q": "总代", "sort": "earnings", "input": "总代", "selected": "true"}` | `{"path": "/admin/affiliates/team", "level": "1", "q": "总代", "sort": "earnings", "input": "总代", "selected": "true"}` |
| TAUI-08 | ✅ | 浏览器返回逐层回退：客户 → 二级团队 → TB 视角（tab=agents）→ 汇总页（筛选保持） | `{"tab": "agents", "summary": {"level": "1", "q": "总代", "sort": "earnings"}}` | `{"tab": "agents", "summary": {"level": "1", "q": "总代", "sort": "earnings"}}` |
| TAUI-09 | ✅ | TM 的视角：元信息行「分销代理 · 上级 O总代」；点上级进入 TO 的视角 | `{"meta": ["分销代理", "上级 O总代"], "path": "/admin/affiliates/team/2536", "crumbs": "团队消费 / O总代"}` | `{"meta": ["分销代理", "上级 O总代"], "path": "/admin/affiliates/team/2536", "crumbs": "团队消费 / O总代"}` |
| TAUI-10 | ✅ | 已降级的 TH：元信息行为「已降级 · 代理期间的客户仍产生分润」，没有代理端的降级提示；已删除的二级显示「已删除 #ID」且可点进，看到其名下客户 | `{"th": "已降级 · 代理期间的客户仍产生分润", "notice": 0, "label": "已删除 #2659", "crumbs": "团队消费 / 已删除 #2659", "customer": true}` | `{"th": "已降级 · 代理期间的客户仍产生分润", "notice": 0, "label": "已删除 #2659", "crumbs": "团队消费 / 已删除 #2659", "customer": true}` |
| TAUI-11 | ✅ | 打开普通用户 TN 的视角：显示「代理或客户不存在」与「返回团队消费」（回到汇总页） | `{"card": "代理或客户不存在 返回团队消费", "link": true}` | `{"card": "代理或客户不存在 返回团队消费", "link": true}` |
| TAUI-12 | ✅ | 代理管理「⋯」→「团队消费」：直达 TB 的视角 | `{"path": "/admin/affiliates/team/2537", "crumbs": "团队消费 / B总代"}` | `{"path": "/admin/affiliates/team/2537", "crumbs": "团队消费 / B总代"}` |
| TAUI-13 | ✅ | 手机宽度 390px：汇总页、代理视角、客户页均无横向溢出（表格内部滚动） | `{"summary": 390, "viewer": 390, "customer": 390}` | `{"summary": 390, "viewer": 390, "customer": 390}` |
| TAUI-14 | ✅ | 深色模式下汇总页与代理视角正常渲染（截图人工复核） | `{"dark": true}` | `{"dark": true}` |
| TAUI-00 | ✅ | 管理端界面脚本正常结束并输出结果 | `{"rc": 0, "results>=14": true}` | `{"rc": 0, "results>=14": true, "stderr": ""}` |

## TRL

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TRL-01 | ✅ | 同一用户 1 分钟内超过 60 次团队接口被 429；其他用户不受影响 | `{"first_429_at": 61, "other_user": 200}` | `{"first_429_at": 61, "other_user": 200, "codes": [200, 429]}` |

## TRET

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TRET-01 | ✅ | 重启后首轮保留期清理：100 天前的明细与汇总行删除，89 天前的保留并计入 90 天范围 | `{"healthy": true, "before": {"d100": {"n": 1, "c": 0.3}, "d89": {"n": 1, "c": 0.2}}, "expired_left": 0, "after": {"d100": {"n": 0, "c": 0}, "d89": {"n": 1, "c": 0.2}}, "c90": 1.8682915}` | `{"healthy": true, "before": {"d100": {"n": 1, "c": 0.3}, "d89": {"n": 1, "c": 0.2}}, "expired_left": 0, "after": {"d100": {"n": 0, "c": 0}, "d89": {"n": 1, "c": 0.2}}, "c90": 1.8682915}` |

## TROL

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| TROL-01 | ✅ | 测试结束时全表核对：汇总表与 usage_logs 重算完全一致（622 行） | `{"missing": 0, "extra": 0, "mismatched": 0, "last_used_diff": 0}` | `{"missing": 0, "extra": 0, "mismatched": 0, "last_used_diff": 0}` |
| TROL-02 | ✅ | 测试结束时全表核对：分润日汇总与流水重算完全一致（749 行） | `{"missing": 0, "extra": 0, "mismatched": 0}` | `{"missing": 0, "extra": 0, "mismatched": 0}` |

## STU

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| STU-00 | ✅ | 直调网关生图成功 | `200` | `200` |
| STU-01 | ✅ | 直调网关生图产生 1 条 usage（基准） | `1` | `1` |
| STU-02 | ✅ | POST /api/nova/tasks 接受任务（202） | `202` | `202` |
| STU-03 | ✅ | 任务完成并有图片结果 | `true` | `true` |
| STU-04 | ✅ | 经工作台生图产生 1 条 usage，归属 TA1 与所选 Key | `[[2545, 711, "gpt-image-1", 1]]` | `[[2545, 711, "gpt-image-1", 1]]` |
| STU-05 | ✅ | 扣费与直调网关一致 | `0.134` | `0.134` |
| STU-06 | ✅ | usage 记录的客户端 IP = 用户 IP，与直调一致，不是 Nova 容器 IP | `["198.51.100.23", "198.51.100.23", true]` | `["198.51.100.23", "198.51.100.23", true]` |
| STU-07 | ✅ | 上游模拟收到这次生图（客户端填的 api.openai.com 被忽略，走了网关） | `true` | `true` |
| STU-08 | ✅ | 其他用户读取该任务 → 404 | `404` | `404` |
| STU-09 | ✅ | 其他用户读取该任务图片 → 404 | `404` | `404` |
| STU-10 | ✅ | 未登录读取 → 401 | `401` | `401` |
| STU-11 | ✅ | /_nova/ 用嵌入专用策略：仅同源嵌入、浏览器只连本站 | `[200, "SAMEORIGIN", true, true]` | `[200, "SAMEORIGIN", true, true]` |
| STU-12 | ✅ | 插件重载仅管理员（普通用户 403，管理员 200）；普通用户可读列表 | `[403, 200, 200]` | `[403, 200, 200]` |
| STU-13 | ✅ | 视频 / 浏览器自动化接口在嵌入模式关闭（管理员也 404） | `[404, 404, 404]` | `[404, 404, 404]` |
| STU-14 | ✅ | 提示词广场未登录 → 401 | `401` | `401` |
| STU-15 | ✅ | 提示词广场拒绝白名单外的仓库 / 路径 / 类型 / 图床（404，未发起外部请求） | `[404, 404, 404, 404, 404]` | `[404, 404, 404, 404, 404]` |
| STU-16a | ✅ | Nova 停机：/_nova 与 /api/nova 返回 503 STUDIO_UNAVAILABLE，入口状态仍为已启用 | `[503, 503, "STUDIO_UNAVAILABLE", true]` | `[503, 503, "STUDIO_UNAVAILABLE", true]` |
| STU-16b | ✅ | Nova 恢复后接口可用 | `200` | `200` |

## CHAT

| 用例 | 结论 | 说明 | 预期 | 实际 |
|---|---|---|---|---|
| CHAT-00 | ✅ | 入口已启用；签发 s2a_chat Cookie（Path=/_chat、HttpOnly、SameSite=Strict） | `[true, 200, true, true, true]` | `[true, 200, true, true, true]` |
| CHAT-01 | ✅ | 无 Cookie：接口 401 CHAT_SESSION_INVALID；页面可加载，严格 CSP、仅同源嵌入、无内联脚本 | `[401, "CHAT_SESSION_INVALID", 200, true, true, "SAMEORIGIN", 0]` | `[401, "CHAT_SESSION_INVALID", 200, true, true, "SAMEORIGIN", 0]` |
| CHAT-01b | ✅ | Artifact 沙箱页单独 CSP：禁止网络连接、表单与外部框架 | `[200, true, true]` | `[200, true, true]` |
| CHAT-02 | ✅ | Key 列表只含本人 Key、不含明文；生图分组标记支持生图 | `[200, [712, 713], false, {"712": false, "713": true}]` | `[200, [712, 713], false, {"712": false, "713": true}]` |
| CHAT-03 | ✅ | 可信头自动登录：合成邮箱、角色按 sub2api 同步（普通用户 user、管理员 admin） | `["u2532@sub2api.internal", "user", "user", "admin"]` | `["u2532@sub2api.internal", "user", "user", "admin"]` |
| CHAT-04 | ✅ | 模型列表来自所选 Key（网关该分组可用模型） | `[200, true]` | `[200, true]` |
| CHAT-04b | ✅ | 对话列表隐藏内部/生图模型，生图 Key 的网关原始列表仍完整 | `[200, 200, true, ["gpt-5.2", "gpt-5.3-codex-spark", "gpt-5.4", "gpt-5.4-mini", "gpt-5.5", "gpt-5.6", "gpt-5.6-luna", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-6", "gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-6.1-sol"]]` | `[200, 200, true, ["gpt-5.2", "gpt-5.3-codex-spark", "gpt-5.4", "gpt-5.4-mini", "gpt-5.5", "gpt-5.6", "gpt-5.6-luna", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-6", "gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-6.1-sol"]]` |
| CHAT-05 | ✅ | 经对话工作台对话：成功，usage 1 条，归属 CA 与所选 Key | `[200, 200, [[2532, 712, "claude-sonnet-4-5", 1200, 300]]]` | `[200, 200, [[2532, 712, "claude-sonnet-4-5", 1200, 300]]]` |
| CHAT-06 | ✅ | 扣费与直调网关一致；usage IP = 用户 IP（不是容器 IP） | `[0.0081, "198.51.100.31"]` | `[0.0081, "198.51.100.31"]` |
| CHAT-07 | ✅ | 标题生成计在同一 Key（不落到其他 Key） | `[200, true, [712]]` | `[200, true, [712]]` |
| CHAT-08 | ✅ | 原生搜索：上游 web_search 与 url_citation 引用，计在所选 Key | `[200, true, true, [712]]` | `[200, true, true, [712]]` |
| CHAT-09 | ✅ | 生图：计在生图 Key（不是对话 Key）、1 张、扣费与直调一致；结果存为本站文件 | `[200, 200, [[713, "gpt-image-2", 1]], true, true]` | `[200, 200, [[713, "gpt-image-2", 1]], true, true]` |
| CHAT-09b | ✅ | 生图响应同时带外站 url 与 b64_json：用 b64 存为本站文件（不拉外站、不报错），计费 1 张 | `[200, true, [[713, 1]]]` | `[200, true, [[713, 1]]]` |
| CHAT-09c | ✅ | 混合 URL/base64 生图结果可从本站读取且字节与模拟原图一致 | `true` | `true` |
| CHAT-10 | ✅ | 本人可读自己的会话 / 文件 / 笔记 / 日历 | `{"chat": 200, "file": 200, "file-content": 200, "note": 200, "calendar": 200}` | `{"chat": 200, "file": 200, "file-content": 200, "note": 200, "calendar": 200}` |
| CHAT-11 | ✅ | 其他用户按 ID 访问：与不存在相同（拒绝） | `{"chat": 401, "file": 404, "file-content": 404, "note": 404, "calendar": 404}` | `{"chat": 401, "file": 404, "file-content": 404, "note": 404, "calendar": 404}` |
| CHAT-11b | ✅ | 管理员按 ID 访问用户的会话 / 文件 / 笔记 / 日历同样被拒（无管理员特权） | `{"chat": 401, "file": 404, "file-content": 404, "note": 404, "calendar": 404}` | `{"chat": 401, "file": 404, "file-content": 404, "note": 404, "calendar": 404}` |
| CHAT-11c | ✅ | 其他用户的会话列表不含 CA 的会话 | `false` | `false` |
| CHAT-12 | ✅ | 屏蔽清单接口对普通用户与管理员都不可用（401 / 403 / 404） | `{}` | `{}` |
| CHAT-13 | ✅ | 插件仅管理员：普通用户创建工具 / 改工具服务器被拒，管理员可创建工具 | `[true, 200, true]` | `[true, 200, true]` |
| CHAT-14 | ✅ | 伪造身份 / 角色头无效；用他人 Key ID 或塞明文 Key 不会被注入（请选择可用的 Key） | `[200, "u2532@sub2api.internal", "user", 400, true]` | `[200, "u2532@sub2api.internal", "user", 400, true]` |
| CHAT-15 | ✅ | 换账号后旧 token 401；不可信对端直连 Open WebUI 伪造身份头 → 403 | `[401, "403"]` | `[401, "403"]` |
| CHAT-16 | ✅ | Open WebUI 自带注册 / LDAP / 添加用户接口不可用 | `true` | `true` |
| CHAT-17 | ✅ | 出网：白名单外域名经出站代理被拒（代理记录 DENY、无 ALLOW）；绕过代理无路由；对话网络为 internal | `["URLError", true, 0, "URLError", "true"]` | `["URLError", true, 0, "URLError", "true"]` |
| CHAT-18 | ✅ | 退出登录：DELETE /api/v1/chat/session 清 Cookie 后接口 401 | `[200, 200, 401]` | `[200, 200, 401]` |
| CHAT-19a | ✅ | Open WebUI 停机：/_chat 页面与接口 503；入口状态仍为启用；站点其他页面正常 | `[503, 503, true, 200]` | `[503, 503, true, 200]` |
| CHAT-19b | ✅ | Open WebUI 恢复后可用 | `200` | `200` |
