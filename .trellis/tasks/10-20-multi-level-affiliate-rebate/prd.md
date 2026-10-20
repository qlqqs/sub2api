# 多级代理返利（最多三级）

## Goal

在现有单级邀请返利基础上，支持管理员开放 1–3 级代理层级，并为每一级单独配置按**直属用户充值流水**计算的返利比例。例如：一级代理拿直属用户流水的 8%，二级代理同样拿自己直属用户的 8%，同时其上级（一级）再拿 3%。

## Background

- 现有返利只向 `user_affiliates.inviter_id`（直接邀请人）入账（`AccrueInviteRebateForOrder`）。
- 全局比例 `affiliate_rebate_rate`、冻结期、有效期、单人上限、用户专属比例均已存在。
- 返利流水 `user_affiliate_ledger(action='accrue')` 以 `user_id`（受益人）+ `source_user_id`（充值用户）记录。

## Requirements

### R1. 层级与比例配置
- 新增全局设置：`affiliate_rebate_levels`（1–3，默认 1）、`affiliate_rebate_rate_l2`、`affiliate_rebate_rate_l3`（0–100，默认 0）。一级比例沿用 `affiliate_rebate_rate`。
- 默认值下行为与现状完全一致。

### R2. 入账规则
- 用户 U 充值 A：U 的直接邀请人按一级比例入账；邀请人的邀请人按二级比例入账；以此类推至开放层级数，每一级均以 A 为基数。
- 用户专属比例仅对直属关系（一级）生效；上级层级使用对应全局比例。
- 有效期以 U 的邀请关系绑定时间为准，对所有层级统一判断。
- 冻结期对各层级一致；单人上限按「受益人 × 充值用户」独立计算。
- 链路出现环或某一级无邀请人时停止向上。

### R3. 流水与展示
- `user_affiliate_ledger` 新增 `rebate_level`（accrue 行写入，历史 accrue 回填为 1）。
- 管理端返利记录展示层级列并可排序；用户端邀请页展示各级生效比例。

## Acceptance Criteria
- [ ] AC1：levels=1 时所有现有单元/集成测试保持通过，行为不变。
- [ ] AC2：levels=3、三级链路 D→C→B→A，D 充值 100，比例 8/3/1 时，C/B/A 分别入账 8/3/1，流水 rebate_level 为 1/2/3。
- [ ] AC3：中间某级邀请人缺失时上游不入账；自环不会死循环。
- [ ] AC4：管理端设置可保存并回显层级与各级比例；超范围被 clamp。
- [ ] AC5：后端 gofmt/单元/集成测试、前端 typecheck/test/lint/i18n 检查通过。

## Out of Scope
- 按层级设置用户专属比例、层级独立的冻结期/上限。
- 历史流水按新规则补算。
