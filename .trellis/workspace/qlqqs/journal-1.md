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


## Session 14: 财务功能推送与一号机部署
<!-- trellis-session: v=2 fp=c5c799b8683f1120 -->

**Date**: 2026-10-26
**Task**: 财务功能推送与一号机部署
**Branch**: `sub2api-custom`

### Summary

2026-10-26 用户授权推送部署。仅推送财务提交及CSV lint修复，未推送客户端规划文档。CI 40285951095 全部通过；部署 custom-b2a740e 到一号机 sub2api-custom，校验镜像 revision、健康、财务迁移。

### Main Changes

- CSV导出检查Buffer写入错误与series类型断言，补充导出错误脱敏回归。
- 部署前备份 /root/sub2api-custom/backups/pre-finance-final-20261026T074227Z；旧镜像 fa1f1ff772f837afd874a862bb5ef74061ac3a3393134eb5af467a85af10929f；只重建应用容器。

### Git Commits

| Hash | Message |
|------|---------|
| `38532a554` | feat(finance): admin finance ledger page with revenue recognition, account costs and expenses |
| `b2a740e3e` | fix(finance): check CSV export writes and summary type assertion |

### Testing

- [OK] 本机Finance handler单测、go build、本机镜像CSV认证导出/BOM/13列/未认证401通过；CI后端单测集成/lint、前端检查测试、镜像构建全过。
- [OK] 线上内外网health正常；财务新表2张和支付新列3个存在；未认证财务接口401。未进行管理员登录后的线上账目验收。

### Status

[OK] **Completed**

### Next Steps

- 需管理员配置支付实例通道费率3.2%和固定成本账号1（当前费率0且固定配置不存在）；模型倍率、订阅支出待提供；历史usage 236补算仍待单独确认。


## Session 15: 稳定分组工作台与财务联合发布
<!-- trellis-session: v=2 fp=d6b7a408fd3ff206 -->

**Date**: 2026-10-26
**Task**: 稳定分组工作台与财务联合发布
**Branch**: `sub2api-custom`

### Summary

用户授权后提交推送三仓库，CI通过，备份后部署一号机三个配套应用。修复财务7项lint及处理Chat动态IP占用主服务固定IP；现健康、迁移259/260及group ID 2核验通过。Nova 8d649ed，Chat 550b06b；未执行历史重算，未动design-previews。全仓库OWUI/Nova独立类型检查限制详见任务validation。

### Git Commits

| Hash | Message |
|------|---------|
| `1a2b2736d` | feat(workbench): bootstrap signup keys and persist first-use defaults |
| `7beeb73fe` | fix(finance): clear release lint checks without changing behavior |

### Status

[OK] **Completed**


## Session 16: 历史重算UI审查提交与一号机更新
<!-- trellis-session: v=2 fp=27db74c2b02c5ffc -->

**Date**: 2026-10-26
**Task**: 历史重算UI审查提交与一号机更新
**Branch**: `sub2api-custom`

### Summary

用户授权提交更新部署；历史重算取消预览与简化文案已发布。一号机sub2api-custom运行99ab810ed，CI 40360471074全绿；本机后端全量unit、定向PG integration/build、golangci-lint 0 issues及前端3442测试通过。备份pre-recalc-ui-20261026T164930Z，生产健康/静态资源/鉴权/迁移261通过；其他容器未重启，未执行线上历史重算。design-previews保留未提交。

### Git Commits

| Hash | Message |
|------|---------|
| `99ab810ed` | fix(finance): simplify recalculation UI and allow cancelling previews |
| `85d31831d` | docs(finance): record recalculation UI production rollout |

### Status

[OK] **Completed**


## Session 17: 上游维护补丁移植与增量工作流（未提交，待质量跟进）
<!-- trellis-session: v=2 fp=2134ded8c236a9ee -->

**Date**: 2026-10-27
**Task**: 上游维护补丁移植与增量工作流（未提交，待质量跟进）
**Branch**: `sub2api-custom`（候选：`upstream-sync/2026-10-27-maintenance`）

### Summary

本地main快进至f178e5ae3；M1–M4共14产品文件从已验证候选移入custom工作区，基线a18a7a3a4为他人提交。长期台账覆盖35单元，新增工作流并强制收尾更新现有记录。任务仍in_progress，无本轮提交/推送/部署，模型工作排除。

### Main Changes

- 新增 .trellis/upstream-sync/{state.json,decisions.jsonl,README.md} 与 spec/guides/upstream-sync-guide.md；更新现有guides索引、backend工具链规范、本轮PRD/design/implement/task及验证摘要。

### Git Commits

本轮未创建提交；已批准实施，补丁留在工作区。

### Testing

- [OK] 后端定向283、race23、unit58包、build与当前工作区vet通过；pnpm9冻结安装/lint/typecheck及437文件3463前端测试通过。
- [WARNING] 本机生产构建通过；e2e按现有report规则汇总395项，394通过、FIN-24失败（旧断言与邮箱子串OR用户ID契约冲突），未改业务/测试掩盖。原始依赖审计exit1：2 high/13 moderate/2 low；既有例外检查exit0，未改例外。CI lint及integration-tag测试未运行。

### Status

**In progress — 工作区移植与本轮记录完成；质量跟进及提交授权待办。**

### Next Steps

- 独立跟进FIN-24、依赖审计风险和CI lint；门禁及用户授权满足后才提交，届时记录真实custom目标SHA；推送/生产部署分别授权。
- 下次从f178e5ae3后新增单元与触发重开项开始，不重复审查已处置35单元；详细证据见10-27-upstream-sync/verification/final-report.md。


## Session 18: 上游维护补丁本地提交与进度更新
<!-- trellis-session: v=2 fp=253b43691e393ec3 -->

**Date**: 2026-10-27
**Task**: 上游维护补丁本地提交与进度更新
**Branch**: `sub2api-custom`

### Summary

用户明确要求提交；M1–M4共14产品文件已提交b95be64f6，提交blob与已测试指纹一致。六个上游选入单元追加真实目标SHA映射，工作流/台账/现有任务及journal同步更新。不推送、不部署。

### Git Commits

| Hash | Message |
|------|---------|
| `b95be64f6` | fix(upstream): sync selected security and gateway reliability fixes |

### Testing

- [LIMITED] 指纹、暂存范围、diff检查通过；沿用前轮代码未变的测试证据。e2e仍394/395（FIN-24失败），原始依赖审计exit1，CI lint待验证；未声称全绿。

### Status

**In progress — 本地提交完成；质量跟进及推送/部署待办。**

### Next Steps

- 质量跟进仍待处理；任务保持in_progress，不归档。推送/部署须另获授权。


## Session 19: 上游维护发布：推送与一号机主应用部署
<!-- trellis-session: v=2 fp=51406152fc9bcfb4 -->

**Date**: 2026-10-27
**Task**: 上游维护发布：推送与一号机主应用部署
**Branch**: `sub2api-custom`

### Summary

用户明确授权推送更新部署；origin/main=f178e5ae3、custom发布revision=1fa919f68。CI 40546398450四任务成功（unit/integration/lint/前端/镜像）；备份后仅更新一号机sub2api-custom，2026-10-27 17:20:07 UTC健康。其他运行容器ID前后一致，未执行生产财务重算或付费推理。

### Git Commits

| Hash | Message |
|------|---------|
| `b95be64f6` | fix(upstream): sync selected security and gateway reliability fixes |
| `1fa919f68` | docs(upstream): persist selective sync workflow and integration ledger |

### Testing

- [OK] 生产只读health/首页/Nova页面及17个静态资源通过；管理/Chat/工作台未认证401；运行revision准确、healthy，无启动错误模式。备份数据库/data读取与SHA256验证通过。
- [LIMITED] 保留本机e2e394/395（FIN-24）及原始依赖审计exit1，不以CI/上线成功掩盖。CI lint/integration待办已补齐。

### Status

**发布完成；任务保留质量跟进（in_progress）。**

### Next Steps

- 任务仍保留测试fixture、原始依赖审计及未演练流程场景跟进，不归档。详细发布证据在10-27-upstream-sync/verification/release-report.md；下次仅增量审查新单元和触发重开的旧项。


## Session 20: 注册赠送推送与一号机发布
<!-- trellis-session: v=2 fp=f226c86447e7a6a9 -->

**Date**: 2026-10-28
**Task**: 注册赠送推送与一号机发布
**Branch**: `sub2api-custom`

### Summary

用户授权推送部署；Custom Image 40624930942四个任务全部成功。一号机sub2api-custom精确更新至1dafd13b7，备份校验、迁移262、健康及双域名只读验证通过，其他容器未变，注册赠送保持0；归档本任务。

### Main Changes

- 仅更新一号机sub2api-custom主应用；保留既有网络/认证/监听与业务设置

### Git Commits

| Hash | Message |
|------|---------|
| `1dafd13b7fa2645e54b2b9d07ad24c60840f488c` | Add invitee signup bonus |

### Testing

- [OK] 本机fast 221/221与独立财务34/34；CI unit/integration/lint/frontend/image全部通过
- [OK] 生产loopback与主站域名smoke通过；API域名直连TLS reset，使用既有网络代理完整重试通过

### Status

[OK] **Completed**

### Next Steps

- 如需开启注册赠送，在后台邀请返利设置填入金额；本次未开启


## Session 21: 注册赠送按邀请来源筛选：实现与本机验收
<!-- trellis-session: v=2 fp=3fb36493925ec39c -->

**Date**: 2026-10-29
**Task**: 注册赠送按邀请来源筛选：实现与本机验收
**Branch**: `sub2api-custom`

### Summary

完成全部/代理/普通邀请来源下拉、绑定快照发放、匿名金额预览与简短UI文案；本机验收通过，未提交或部署。

### Main Changes

- 新增 affiliate_invitee_signup_bonus_source，默认all，保留统一金额和历史账本。
- 注册页提示按邀请码查询，支持链接变化和手动编辑防抖/取消；邀请页按本人身份展示。

### Git Commits

(No commits - planning session)

### Testing

- [OK] 后端全量unit、Affiliate集成、go build通过；前端lint/typecheck与3512项测试通过。
- [OK] 本机fast e2e252/252，来源专项31/31，UI11/11；报告在任务verification.md。

### Status

[OK] **Completed**

### Next Steps

- 等待用户同意后再提交、推送、部署一号机sub2api-custom；当前不归档以避免自动提交。


## Session 22: 注册赠送来源：提交推送与一号机部署
<!-- trellis-session: v=2 fp=d0042b004da68b16 -->

**Date**: 2026-10-29
**Task**: 注册赠送来源：提交推送与一号机部署
**Branch**: `sub2api-custom`

### Summary

CI四项通过，已备份部署一号机sub2api-custom，运行revision核对及生产双域名只读检查通过。

### Main Changes

- 按用户授权提交、推送；只更新主应用，其他容器不变。任务归档，发布报告持久化。
- 赠送配置保留¥0.50及all；备份pre-invitee-signup-source-20261029T062225Z，停机约11秒。

### Git Commits

| Hash | Message |
|------|---------|
| `2ffc49db745c01351d0cef662c60053d7aebcfed` | Add invitation source filter for signup gifts |

### Testing

- [OK] Custom Image 40827319767：前端、后端unit/integration、backend-lint、image全部成功。
- [OK] 生产loopback/www/api健康、静态资源、新邀请金额预览与鉴权检查通过，启动无fatal模式。

### Status

[OK] **Completed**

### Next Steps

- 管理员可在邀请来源下拉框选择适用来源；无需进一步部署。


## Session 23: 上游六项维护合入工作区：功能验证通过，质量跟进待处理
<!-- trellis-session: v=2 fp=7cf6e2da19fc5514 -->

**Date**: 2026-10-31
**Task**: 上游六项维护合入工作区：功能验证通过，质量跟进待处理
**Branch**: `sub2api-custom`

### Summary

六项21产品文件已移入custom且与候选指纹一致；本地main快进至d851a3847。未提交推送部署，任务保持in_progress。

### Main Changes

- 保留Node24.22.1、自定义CI、套餐及财务/分润契约；更新增量台账、任务报告和三份规范，原有工作台文档修改不动。
- 存量frontend audit/例外策略均exit1（follow-redirects和xlsx）；正常缓存浏览器发现既有CSP nonce/304问题，均未扩大范围修复。

### Git Commits

(No commits - implementation changes remain uncommitted)

### Testing

- [OK] 后端module verify、定向/全量unit、缓存race、build、vet通过；缓存旧实现负向对照如期失败。
- [OK] Node24.22.1+pnpm9：前端443文件3524测试、lint/typecheck通过；生产构建通过；本机完整e2e442/442，Nova和Chat均执行。
- [OK] 浏览器35项功能检查通过；禁用HTTP缓存对照36/36且零CSP违规。正常缓存CSP检查失败另见csp-followup.md；不作为门禁通过。

### Status

**In progress** — 质量跟进与提交授权待完成。

### Next Steps

- 先决定follow-redirects与CSP/304跟进范围；另获提交、推送和七号机部署授权。本批不归档。


## Session 24: 六项上游维护更新推送与七号机部署
<!-- trellis-session: v=2 fp=1cb88f4d5ceea663 -->

**Date**: 2026-10-31
**Task**: 六项上游维护更新推送与七号机部署
**Branch**: `sub2api-custom`

### Summary

用户授权发布；核对已有提交和21文件指纹，推送custom。CI 41182664896四任务全通过，备份后仅更新七号机主应用，healthy；配置/迁移/其他容器ID不变，服务器只读资源及公网限定GET/HEAD检查通过。既有audit和CSP/304问题保留，任务in_progress。发布记录已同步，无新产品代码改动。

### Git Commits

| Hash | Message |
|------|---------|
| `5957c600052bee92c7feaf5fd22b7af9647413df` | Apply upstream maintenance fixes and regression coverage |

### Status

**发布完成**；audit及CSP/304质量跟进仍开放，任务保持in_progress。
