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


## Session 8: 人民币显示与代理上级提交、CI 与一号机更新
<!-- trellis-session: v=2 fp=b3a0319b0c4fbf9c -->

**Date**: 2026-10-22
**Task**: 人民币显示与代理上级提交、CI 与一号机更新
**Branch**: `sub2api-custom`

### Summary

两项功能分别提交并推送，任务归档。GitHub Custom Image 39737339650 四个任务全部通过；部署镜像 custom-673303a 到一号机，仅重建 custom 应用容器。部署前备份数据库至 /root/sub2api-custom-backups；迁移249两个新列存在，容器 healthy，内外网 /health 返回 ok。正式 sub2api 和 sub2api-adv 未改动。

### Git Commits

| Hash | Message |
|------|---------|
| `017854fff` | feat(ui): show balance in CNY with per-group ¥/$ ratio |
| `9b62c73b2` | feat(affiliate): allow admins to set level-two agent upline |

### Status

[OK] **Completed**


## Session 9: 管理端团队消费页：实现、提交、部署一号机
<!-- trellis-session: v=2 fp=82b2702f822496d4 -->

**Date**: 2026-10-23
**Task**: 管理端团队消费页：实现、提交、部署一号机
**Branch**: `sub2api-custom`

### Summary

实现管理端团队消费页（汇总页 + 任意代理视角）与迁移 252 分润日汇总；本机全量验证后提交、推送，CI 通过后部署一号机并核对迁移回填。

### Main Changes

- 迁移 252：affiliate_agent_reward_daily_rollups，语句级触发器维护（插入/删除/更新），rebuild 先加 SHARE 锁，迁移内回填
- 后端：affiliateTeamAudience 区分代理端/管理端；管理端汇总 overview / agents 与代理视角四个接口（/admin/affiliates/team/...）
- 前端：TeamConsumptionPage + provide/inject 上下文复用代理端组件；汇总页、代理视角四路由、侧边栏最长前缀高亮、代理管理「⋯」入口
- 规范：database-guidelines（汇总表与触发器、251 rebuild 避开当天）、组件与状态管理、目录结构

### Git Commits

| Hash | Message |
|------|---------|
| `ea8008afd` | feat(affiliate): add daily agent reward rollups |
| `e5ff63561` | feat(affiliate): admin team consumption page |

### Testing

- [OK] 后端 unit + integration（Affiliate）全部通过；前端 lint / typecheck / vitest 2996 / i18n / build 通过，锁文件未变
- [OK] 本机 e2e 全量 238/238（新增 TADM 16、TAUI 15、TROL-02）；性能：2.7M usage_logs / 2.6M 流水，接口均 < 1 s，252 回填约 3.5 s/百万行
- [OK] 一号机：CI 通过后部署 e5ff63561，迁移 251/252 应用，两张汇总表与原始数据逐行一致（0 差异），新路由未登录返回 401

### Status

[OK] **Completed**

### Next Steps

- 一号机登录态复测可用 10-22-e2e-affiliate-payment/scripts 远端脚本（跑完执行 cleanup）
- 部署前备份在一号机 /tmp/sub2api-custom-pre252-20261023081559.sql.gz，重启会被清掉


## Session 10: 分组内单个模型倍率与美元汇率
<!-- trellis-session: v=2 fp=8626c7c44aca092c -->

**Date**: 2026-10-23
**Task**: 分组内单个模型倍率与美元汇率
**Branch**: `sub2api-custom`

### Summary

分组可为单个模型设倍率（迁移 253，计费 / 预扣 / 利润门 / 模型广场按计费模型生效，优先于用户专属倍率）；美元汇率定时获取（默认 6.8 兜底）；模型广场按模型 ¥ 价与「相比官方价」折扣；用户端不再显示分组倍率、使用记录明细改 ¥。本机全量 e2e 291 条通过，CI 通过，已部署一号机 sub2api-custom（汇率 6.6702 来自 Frankfurter）。

### Git Commits

| Hash | Message |
|------|---------|
| `85ea1f415` | feat(billing): per-model rate multipliers within groups |

### Status

[OK] **Completed**


## Session 11: Qx 首页小修与首次登录侧栏图标修复
<!-- trellis-session: v=2 fp=c7c530d86f8479b2 -->

**Date**: 2026-10-24
**Task**: Qx 首页小修与首次登录侧栏图标修复
**Branch**: `sub2api-custom`

### Summary

完成 CC Switch 下载链接、三步与价格顺序、分叉标签宽度、Anthropic curl 示例及公开页 CSS 泄漏导致首次登录侧栏图标挤成点的问题；已推送。GitHub 手动触发 CI 返回 422 Actions has been disabled for this repository，故未更新一号机。

### Git Commits

| Hash | Message |
|------|---------|
| `00692db18` | fix(qx): polish home setup and prevent sidebar icon style leakage |

### Testing

- [OK] Qx 67 项测试、针对性 ESLint、本机 build.sh 通过；中英文 6 屏宽和首次登录/刷新 Playwright 回归通过。按用户要求未运行本机全量测试。

### Status

[OK] **Completed**

### Next Steps

- 恢复仓库 Actions 后为 00692db18 构建镜像，通过 CI 再更新一号机 /root/sub2api-custom。


## Session 12: 对话工作台生图修复发布与真机复测
<!-- trellis-session: v=2 fp=029fb5ce9d0264b6 -->

**Date**: 2026-10-25
**Task**: 对话工作台生图修复发布与真机复测
**Branch**: `sub2api-custom`

### Summary

发布 Open WebUI 59fd038/126b01b；CI 通过，只更新一号机配套 chat。生图刷新可见、搜索引用及价格布局通过；6 条真实记录共 $0.157474992（1 张图 $0.15）。撤销测试 Key/会话、轮换临时密码并删除临时凭据。用户最终视觉确认未单独收到，任务暂不归档。

### Git Commits

| Hash | Message |
|------|---------|
| `58ac00ad2c69b172449b1c3f8f9354bfb5546fe4` | docs(chat): record image fix rollout and verified production retest |

### Testing

- [OK] 本机 p0–p9 336/336；fork 后端 438、前端 65；真实网页 4 步通过，无 CSP/pageerror/外站请求；上游类型错误未新增。

### Status

[OK] **Completed**

### Next Steps

- 用户确认已展示的最终截图后，归档 10-24-chat-openwebui。


## Session 13: 对话工作台最终验收与归档
<!-- trellis-session: v=2 fp=bad426435bafbe12 -->

**Date**: 2026-10-25
**Task**: 对话工作台最终验收与归档
**Branch**: `sub2api-custom`

### Summary

用户在最终截图展示后明确要求归档，记录交付接受并关闭最后验收项；仅归档 chat-openwebui，其他任务和线上环境不变。任务直接在长期 sub2api-custom 分支跨仓库实现，归档使用 skip-branch-validation 保留真实分支记录。

### Git Commits

| Hash | Message |
|------|---------|
| `58ac00ad2` | docs(chat): record image fix rollout and verified production retest |

### Testing

- [OK] 沿用已记录的本机 336/336 回归与生产复测，本次仅归档文档，不重复运行；完整 Svelte 类型检查仍有基线错误，不宣称通过。

### Status

[OK] **Completed**

### Next Steps

- 本任务已完成，无待实施事项。
