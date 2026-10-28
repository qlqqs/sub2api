# 管理员赠送与线下充值（fork）

## 1. 范围

仅管理员手动加余额及其历史补登；兑换码不分类。赠送永不发奖励，线下充值沿用支付订单的奖励与财务规则。`affiliate_admin_recharge_enabled` 已删除，数据库遗留键不再读取。

## 2. 签名

- `POST /api/v1/admin/users/:id/balance`：`kind?: gift|offline`、`operation`、`balance`、`paid_amount`、`offline_channel`、`trade_no`、`notes`。
- `POST /api/v1/admin/users/:id/balance-history/:redeem_id/offline`：收款字段同上，不接受新到账金额。
- `PaymentService.AdminCreateOfflineRecharge` / `AdminBackfillOfflineRecharge`，实现 `payment_offline.go`。
- 管理员历史附带 `has_payment_order` 和可选 `offline_order: {id,out_trade_no,paid_amount}`；普通兑换历史不变。

## 3. 合约

省略 kind 兼容为 gift；UI 无默认且显式发送。offline 只能 add。`amount=到账`、`pay_amount=实收`、`bonus_amount=max(到账−实收,0)`、本金 `min(到账,实收)`，通道费显式为字符串 `"0"`。创建即 COMPLETED，无支付实例或 snapshot，不走网关、不发成功邮件。收款方式仅审计存储，不混用 `payment_type=offline`。

调账记录、订单、原子余额增加、收款审计必须同一事务；`AdjustBalance` 必须沿 `dbent.NewTxContext` 走事务连接。提交后调用已有缓存失效、订单返利、首充、升级评估逻辑，不复制另一份奖励算法。奖励失败不回滚已入账收款。

补登锁历史行，时间使用原 `used_at`；不改余额、不发奖励、不评估等级。任何关联订单（不限 offline）都禁止重复补登。

流水号 trim 后全局检查：非空流水号在 PostgreSQL 通过 `pg_advisory_xact_lock(hashtextextended('offline-receipt:'+trade,0))` 串行化查重与插入；未来所有 offline 写入口必须遵守。无需 schema 或迁移。

## 4. 校验与错误

- 金额必须有限、正数、不超过 100000000、最多两位小数；额外小数不静默取整，含不满足精度的历史记录。`OFFLINE_INVALID_AMOUNT`（400）。
- 收款方式 wechat/alipay/bank/other，流水号最多128字符、备注最多10000字节；非法分别 `OFFLINE_INVALID_CHANNEL` / `OFFLINE_INVALID_RECEIPT`（400）。
- 非法类型/操作组合 `OFFLINE_INVALID_KIND`（400）；历史非本人或不存在返回404；非正数、非 admin_balance 或未使用返回 `OFFLINE_INVALID_HISTORY`（400）。
- 重复流水 `OFFLINE_DUPLICATE_TRADE`、已关联历史 `OFFLINE_ALREADY_LINKED`（409）。
- 用户退款 `OFFLINE_USER_REFUND_DISABLED`（403）；回调 `OFFLINE_CALLBACK_REJECTED`（400）；管理员退款绕过实例/网关并写 `REFUND_OFFLINE`。

## 5. 正例／基线／反例

实收100到账110：paid100/free10/cash100；实收100到账90：paid90/free0/cash100。赠送50仅 free50。

`refund_amount` 是余额额度而非现金。财务读取 offline 订单时使用 `ROUND(refund_amount*pay_amount/amount,2)`，`refund_balance_deducted` 仍是余额；非offline保留原有历史口径。实收100到账110全退扣110、现金100，半退扣55、现金50。

## 6. 必需回归

`payment_offline_test.go` 验证金额/类型、拆分、失败回滚、重复流水、补登条件/无副作用、订单奖励幂等/本金、升级、无网关退款、回调拒绝、看板、收入重放。`payment_offline_integration_test.go` 用真实 PostgreSQL 验证 AdjustBalance 事务传播、并发流水号/补登、财务关联排除和退款现金比例。

本机 e2e `p4_admin.py` ADM-01/02 与 `p4_offline.py` OFF，fast 模式必须执行；前端三个弹窗测试覆盖必选、参数、失败和重开清理。

## 7. 错误与正确

错：先增余额再建订单、把流水号交给在线网关退款、只在事务外查重、补登后重跑履约。

对：事务内写完整收款凭证和余额；事务锁后查重；offline 在 provider/callback 入口明确拦截；补登只补收款，奖励函数仅用于新入账。


## 8. 余额历史汇总（2026-10-27 追加）

`RedeemCodeRepository.GetBalanceHistoryTotals` 返回 `BalanceHistoryTotals`，替代误把赠送计充值的 `SumPositiveBalanceByUser`。同一 SQL 快照聚合，不受类型筛选/分页影响：

- API `total_recharged`：本人 balance 订单、`paidPaymentOrderStatuses` 集合，Σ pay_amount − 已完成退款的现金；与财务共享 `paymentRefundCashSQL`（退款状态与 refund_at 条件一致），offline 按实收比例折算，其他方式保持既有财务口径。不含订阅。
- API `total_gifted`：上述订单 bonus_amount + 本人正数 balance/admin_balance 兑换记录；`NOT EXISTS payment_orders.recharge_code=code` 排除所有已关联记录（不限订单类型/状态），避免内部兑换码/调账重复计。
- 赠送是累计发生额，不随退款扣减；非当前余额，也不是返利汇总。UI 显示「累计充值（实收）/累计赠送」与赠送说明，中英文齐全。
- `redeem_code_totals_test.go` 在 SQLite 与真实 PostgreSQL 共用用例：在线/线下、部分/全额退款及按分取整、订阅/未付/退款流程状态、正负兑换码/赠送、关联排除、用户隔离、空历史。service/handler/UI 测试覆盖全局汇总契约；OFF-09..11 覆盖真实接口与履约。

### 注册赠送的并列口径（2026-10-27）

- `GetBalanceHistoryTotals` 的同一快照额外汇总本人 ledger 中 `invitee_bonus` 与 `invitee_signup_bonus`，两者只进入 `total_gifted`，不进入 `total_recharged`；分润 `transfer` 不算赠送。
- 管理端余额历史将新 action 映射为独立 `type=invitee_signup_bonus`，支持独立筛选；旧首充记录仍保留 `affiliate_balance` 兼容类型，未筛选时合并两种来源。
- 财务 `gift.invitee_signup_bonus` 单列注册赠送，收入重放为免费余额，分润 accrue 排除该类型。消费返利上限只取真实充值订单，不取余额/赠送。
