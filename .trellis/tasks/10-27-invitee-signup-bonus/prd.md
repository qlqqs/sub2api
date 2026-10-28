# 被邀请人注册赠送

## Goal

在「系统设置 → 邀请返利」中新增「被邀请人注册赠送」：用户通过邀请码注册并成功绑定邀请人时，直接给被邀请人余额加一笔固定金额。这笔钱记为赠送，与现有的「被邀请人首充奖励」互相独立，可以叠加。

## 已确认的决策（2026-10-27，用户）

| 问题 | 决定 |
|---|---|
| 适用范围 | 所有被邀请人，邀请人是代理还是普通用户都送，口径同「被邀请人首充奖励」 |
| 管理员补绑邀请人 | **不补发**。只在注册时（含 OAuth 注册，包括 pending OAuth 流程最终建号时）经邀请码绑定成功才发 |
| 防刷 | 不加额外限制，沿用现有的注册限流、邮箱验证和域名白名单 |
| 前台展示 | 展示。带邀请码的注册页、用户「我的邀请」页提示「好友注册即得 ¥X」，展示方式参照首充奖励 |

## Requirements

### 设置
- 新增设置键 `affiliate_invitee_signup_bonus`（float，¥，默认 0；0 表示关闭；不能为负）。
  - 和 `affiliate_invitee_bonus_*` 一样走管理员设置的读写、DTO、审计、校验、公开设置。
  - 邀请返利总开关关闭时，公开设置里返回 0，且不发放。
- 管理员设置页的「邀请返利」区块新增一个卡片「被邀请人注册赠送」，放在「被邀请人首充奖励」旁边，只有一个金额输入框。
  - 说明文字：被邀请人（含代理直接邀请的人）通过邀请码注册并绑定成功后立即赠送，直接进入余额，计为赠送；管理员补绑不发放；0 = 关闭。
  - 中英文 i18n 都要加。

### 发放
- 发放时机：`BindInviterByCode` 绑定成功（`bound == true`，Source 为 registration）之后。
  - 邮箱注册和 OAuth 注册（`bindOAuthAffiliate`）都走这条路径。
  - `AdminBindInviter` 不发放。
  - 如果 `BindInviterByCode` 还有其他非注册场景的调用方，必须排除。实现前先搜索全部调用方并确认。
- 新增 reward_type / action：`invitee_signup_bonus`，写入 `user_affiliate_ledger`，`user_id = source_user_id = 被邀请人`，同时 `users.balance += 金额`，两步在同一个事务里完成。
- 幂等：每个被邀请人最多发一次。
  - 现有部分唯一索引 `(source_user_id, reward_type) WHERE reward_type IN ('invite_fixed','invitee_bonus')` 需要通过**新迁移**覆盖 `invitee_signup_bonus`，可以扩展原索引，也可以新建索引。
  - 迁移编号接在现有最大编号之后，不要改已有迁移文件。
- 发放失败不能阻断注册：记日志后继续，和现有绑定失败的处理方式一致。
- 金额按当时设置的值发，四舍五入到现有精度。

### 计入赠送（与 invitee_bonus 同口径，逐处对齐）
- 凡是代码里把 `invitee_bonus` 当作赠送、非现金、不计返利基数处理的地方，`invitee_signup_bonus` 都要同样处理。至少包括：
  - `service/finance.go`、`service/finance_reports.go`：财务赠送分类新增独立一项「注册赠送」，不并入首充奖励；不计入分润 accrue。
  - `service/admin_user.go` 中余额历史的 `action IN ('transfer','invitee_bonus')` 及现金/赠送拆分。
  - `repository/affiliate_repo.go`：`SumRewardTotals`、记录列表（约 847 行 `action IN ('accrue','invitee_bonus')`）、`HasRewardForSource` 的用法。
  - `affiliate_distribution.go` 约 229 行的 reward 类型分支。
  - 前端：`AdminAffiliateRecordsTable.vue`、`FinanceRechargeTab.vue`、`AffiliateView.vue`、用户余额历史等处的类型标签和 i18n。
- 消费返利基数本来就以「真实充值总额」为上限，赠送部分不产生返利。请确认这一点仍然成立。
- 实现前用 `grep -rn invitee_bonus backend/internal frontend/src` 全量排查，逐处决定是否要并列处理，结论写进 `implement.md`。

### 前台展示
- 公开设置新增 `affiliate_invitee_signup_bonus`。
- 注册页带邀请码（aff）时，在现有首充奖励提示旁边显示「通过邀请注册即得 ¥X」。
- 用户「我的邀请」页向邀请人说明好友注册可得 ¥X。
- 被邀请人自己的邀请页或明细能看到这笔注册赠送记录（参照 invitee_bonus 的展示）。

## Acceptance Criteria

- [ ] 设置为 5 时，用邀请码注册的新用户余额 = 默认余额 + 5，流水里有一条 `invitee_signup_bonus`。
- [ ] 设置为 0 或邀请返利关闭时，不发放，注册页和邀请页不展示。
- [ ] 不带邀请码、邀请码无效、自邀：不发放，注册正常完成。
- [ ] OAuth 注册（含 pending 流程）带邀请码：发放一次。
- [ ] 管理员补绑邀请人：不发放。
- [ ] 同一用户重复触发（并发、重放）最多一条流水，余额只加一次。
- [ ] 之后的首充奖励照常发放，与注册赠送叠加。
- [ ] 财务报表和用户余额历史把它算作赠送，单列「注册赠送」；不计入分润 accrue；消费返利基数不变。
- [ ] 后端单测、仓库集成测试（新迁移 + 幂等）、前端 lint、typecheck、test:run 全部通过。
- [ ] 本机 e2e：p0 基线把该设置重置为 0；在 `p3_fr_lim.py` 的 FR 组（或新建 SGB 组）加用例；`CASES.md` 同步；p10 财务里如有赠送合计断言，一并更新。`./run_all.sh --fast` 全绿，p10 相关用例通过。

## Notes

- 参照实现：`AffiliateRepository.CreditInviteeBonus`（`backend/internal/repository/affiliate_repo.go`）、`applyPublicInviteeBonus`（`setting_public.go`）、设置键 `SettingKeyAffiliateInviteeBonus*`（`domain_constants.go`）。
- 改 Wire、Ent 时运行 `make generate`。
- 测试命令以仓库 `AGENTS.md`「本机测试环境与测试工作流」为准：pnpm@9，单测去掉代理和 `OPENAI_API_KEY`。
- 未经用户同意不提交、不推送、不部署。
