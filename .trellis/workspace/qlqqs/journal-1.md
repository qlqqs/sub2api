# Journal - qlqqs (Part 1)

> AI development session journal
> Started: 2026-09-04

---



## Session 5: 同步上游并合并定制分支
<!-- trellis-session: v=2 fp=f57486390b716d0a -->

**Date**: 2026-10-20
**Task**: 同步上游并合并定制分支
**Branch**: `sub2api-custom`

### Summary

保持 upstream/main、origin/main、本地 main 完全一致；仅将上游改动合并进 sub2api-custom。

### Main Changes

- 合并上游，保留补绑及提现功能与测试，统一 Gin user_id 动态参数并增加共存回归测试。

### Git Commits

| Hash | Message |
|------|---------|
| `8f2cbb53d` | Merge pull request #8177 from feeeei/fix/gemini-negative-input-tokens |

### Testing

- [OK] 前端 2873 项测试、类型检查、构建和相关 ESLint 通过；后端定向单元测试、25 项邀请返利集成测试和构建通过。
- [OK] 代理导致验证码本地请求超时，清除测试进程代理后 repository 全包通过；真实 OpenAI API 对比测试不作为离线验证通过项。

### Status

[OK] **Completed**


## Session 6: 多级代理返利（最多三级）实现
<!-- trellis-session: v=2 fp=cbce7bc908f84931 -->

**Date**: 2026-10-20
**Task**: 多级代理返利（最多三级）实现
**Branch**: `sub2api-custom`

### Summary

新增 affiliate_rebate_levels / affiliate_rebate_rate_l2 / _l3 设置与迁移 246（ledger.rebate_level）；AccrueInviteRebateForOrder 沿邀请链逐级入账；管理端设置与返利记录、用户端邀请页展示层级比例；后端 unit/integration、前端 lint/typecheck/test/build 通过。

### Git Commits

| Hash | Message |
|------|---------|
| `adb0db824` | Merge upstream main into sub2api-custom |

### Status

[OK] **Completed**


## Session 7: 代理等级与按消费结算的两层返利（实现完成，待提交）
<!-- trellis-session: v=2 fp=aa4cc9c53e210de8 -->

**Date**: 2026-10-21
**Task**: 代理等级与按消费结算的两层返利（实现完成，待提交）
**Branch**: `sub2api-custom`

### Summary

user_affiliates 新增 agent_level/locked/source 与消费结算水位；两层返利受等级限制；新增消费定时结算任务、自动升级、管理端等级接口与前端页面

### Main Changes

- 迁移 247：agent_level、agent_level_locked、agent_level_source、rebate_settled_at、rebate_settled_consumption、ledger.source_usage_until
- 设置：affiliate_rebate_base、affiliate_agent_upgrade_mode、level1/level2 阈值、count_mode、settle_interval；levels 上限改为 2，移除 L3
- AccrueInviteRebate 仅在 recharge 基数生效；accrueRebateUpChain 按等级限制层数；SettleConsumptionRebates + AffiliateRebateSettleService 定时结算
- 管理端：PUT users/:id 支持 agent_level/agent_level_locked，新增 batch-agent-level 与 agent-levels/reevaluate
- 前端：设置页代理等级区块、专属用户等级列/批量设置/解锁、用户邀请页等级卡

### Git Commits

(No commits - planning session)

### Testing

- [OK] go test ./... 通过（Aliyun captcha 3 个用例为环境原因预先失败）
- [OK] 集成测试 Affiliate 相关通过（Docker Postgres）
- [OK] 前端 typecheck/lint/check:i18n/全部 2876 用例/build 通过

### Status

[OK] **Completed**

### Next Steps

- 用户确认后提交并归档任务
