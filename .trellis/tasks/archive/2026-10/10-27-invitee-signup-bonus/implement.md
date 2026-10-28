# 实施与验证记录

## 一、调研（实施前）

已执行 `grep -rn invitee_bonus backend/internal frontend/src` 及本机 e2e 目录同名搜索（排除 runs/历史结果），并搜索全部 `BindInviterByCode` / `bindOAuthAffiliate` 调用。编译产物 `backend/internal/web/dist` 不手改，由本机构建更新且不纳入源代码差异。

### 改动清单与逐处理由

| 位置 | 处理与理由 |
|---|---|
| service/domain_constants.go | 新设置键、新 reward 类型，不能复用首充奖励，否则破坏两者独立幂等。 |
| service/settings_view.go、setting_parse.go、affiliate_reward_config.go | 管理设置结构、默认值、批量读键、解析/归一化/写回并列新增固定金额；首充模式/比例/门槛/封顶逻辑保持原样。 |
| handler/dto/settings.go、admin/setting_handler*.go、setting_handler.go | 管理员读取/更新/响应、审计和公开响应逐层透传；新增负值校验。 |
| service/setting_public.go | 两处公开结构/导出映射与填充函数新增金额；总开关关闭为 0；首充 off 不能提前截断新字段。 |
| service/affiliate_owner.go | 仅注册邀请码绑定返回 bound=true 后发放，读当前设置，失败日志降级。 |
| service/affiliate_service.go、affiliate_detail.go | 仓储端口、独立汇总字段及我的邀请页展示金额；首充尚可领取的 HasRewardForSource 仍只查 invitee_bonus，不并列，否则注册赠送会错误阻止首充。 |
| repository/affiliate_repo.go | 新赠送事务沿用 CreditInviteeBonus；SumRewardTotals 新增独立合计；记录列表 action 集合加入新类型。旧 invite_fixed / invitee_bonus 的 ON CONFLICT 保持不变，新类型用独立部分唯一索引。HasRewardForSource 已按参数泛化，无须改查询。 |
| service/affiliate_distribution.go | reward 类型筛选白名单并列加入。 |
| service/finance.go | financeReplays 把新 action 归免费，未知 accrue 排除新类型。 |
| service/finance_reports.go | 赠送合计单列新类型，日成本/分润 accrue 排除两个赠送类型。 |
| service/admin_user.go、repository/redeem_code_repo.go | 余额历史 action 集合增加，注册赠送独立类型标签；累计实收保持原算法，赠送汇总追加账本赠送。已有首充历史兼容不随意重命名。 |
| service/affiliate_first_recharge.go、payment_fulfillment.go、payment_offline_test.go | 仅首充履约和首充计算，不在此发注册赠送；保留原逻辑并验证叠加。 |
| service/affiliate_settle*.go | 消费基数为真实充值上限，不取余额，不增加赠送为返利基数；补充回归/e2e核对。 |
| server/api_contract_test.go | 设置快照补新字段默认 0。 |
| affiliate_two_tier_test.go、payment_fulfillment_test.go 等 stub | 接口实现补齐，并新增注册/首充独立、关闭、非法码、重复、失败、OAuth、管理员不补发测试。 |
| frontend/api/admin/settings.ts、types/index.ts | 管理/公开字段、奖励汇总及邀请详情并列扩展。 |
| frontend/api/admin/affiliates.ts、AdminAffiliateRecordsTable.vue | 类型白名单/筛选/标签/汇总增加新项，首充项保留。 |
| frontend/api/admin/finance.ts、FinanceRechargeTab.vue | 赠送 DTO 与默认解码、新分类卡片单列。 |
| frontend/views/admin/SettingsView.vue | 首充奖励旁新增卡片，默认/提交/非负校验串联。 |
| frontend/views/auth/RegisterView.vue | 带 aff 参数且功能启用且金额>0 才显示独立提示，不依赖首充开关。 |
| frontend/views/user/AffiliateView.vue | 我的邀请的好友收益说明与记录标签新增。 |
| frontend/components/admin/user/UserBalanceHistoryModal.vue | 注册赠送独立筛选/金额类型/标签，与累计赠送口径一致。 |
| frontend/i18n/locales/{zh,en} | 新提示/设置/类型/财务/汇总双语 key 对齐。 |
| frontend 既有测试 | 旧 invitee_bonus 断言不改含义，新增新字段与显示/隐藏/提交测试。 |
| 本机 e2e/p0_bootstrap.py | 基线设置新金额为 0，避免污染所有旧用例。 |
| 本机 e2e/p3_fr_lim.py、CASES.md | 新 SGB 组覆盖注册各分支、管理员补绑、首充叠加、关闭与消费基数。组结束恢复金额 0。 |
| 本机 e2e/lib.py | ledger_summary 默认 action 纳入新赠送；已有关闭基线不会改变旧用例结果。 |
| 本机 e2e/p4_admin.py、p4_offline.py | 旧首充专用操作保持不变；新测试结束复位保证无污染。 |
| 本机 e2e/p10_finance.py | 免费余额 SQL、赠送分类与合计增加新类型并制造真实注册赠送，单列验证。 |

### 全部邀请绑定调用方

- `auth_service.go:Register`：邮箱注册，适用。
- `auth_service.go:bindOAuthAffiliate`：OAuth 注册统一辅助入口，适用。
- OAuth 普通路径：`auth_service.go` 两个成功 Create 分支才调用辅助入口；已存在用户登录不调用。
- `auth_email_oauth_auto.go`：自动邮箱 OAuth 建号后调用，适用。
- `auth_oauth_email_flow.go`：pending 流程最终建号后调用，适用。
- `affiliate_two_tier_test.go`：测试绑定 helper；更新 fake 并补测试。
- `AdminBindInviter`：独立路径，不经过 `BindInviterByCode`，明确不发。
- 未发现其他非注册生产调用，不额外增加 source 参数；绑定结构已经固定 Source=registration。

## 二、执行顺序与不做事项

1. 后端配置/DTO/审计→发放与迁移→账务/历史→单元与集成回归。
2. 前端接口/展示/双语/测试。
3. 按 AGENTS.md 完整单测、构建、integration、前端三项检查，确认锁文件不变。
4. 本机构建重启，更新 e2e，fast（实际已含 p10）与独立 p10，三页截图。
5. 末尾追加实际验证命令、结果和截图，不把跳过当通过。

不修改现有迁移、Ent/Wire、支付首充算法或默认赠送；不提交、不推送、不操作一号机。不明确的金额规范沿用首充八位精度与上限；新增字段默认 0。

## 三、实施中补充决策与发现

- 注册响应要同步已入账的余额：邮箱与直接 OAuth 注册在绑定返回后读取实际余额；pending OAuth 已在外层事务内且响应只有令牌，不在该事务内调用普通 `UserRepository.GetByID`。该读取不沿事务客户端，SQLite 单连接测试会等待连接，PostgreSQL 也可能读取提交前的旧余额。pending 在提交后由客户端正常读取余额，不扩大范围重构用户仓储。
- `/redeem/history` 当前只返回兑换码，不包含 affiliate ledger，本任务不扩展该接口。本人在「我的邀请」页通过独立的 `reward_totals.invitee_signup_bonus` 卡片看到已领取金额，不引导去兑换页找不存在的流水。后台余额历史仍增加独立类型和筛选。
- 余额历史累计赠送原先遗漏了首充账本赠送，本次在同一聚合快照中纳入 `invitee_bonus` 与 `invitee_signup_bonus`，两种都不进入累计实收；保留旧首充记录的 `affiliate_balance` 类型兼容，不改名。
- 第一次 fast e2e 在新 SGB-03 因测试脚本把数据库快照辅助函数 `aff()` 当成详情接口而抛 `KeyError: reward_totals`；修正为真实 `GET /user/aff`，保留原失败结果目录，不删用例后完整重跑。
- pending OAuth 发放复用外层注册事务，单纯捕获仓储错误仍不足以 fail-open：PostgreSQL 在 SQL 失败后会标记整个事务为 aborted。新发放方法用保存点包住账本+余额，失败回滚到保存点再返回错误；新增触发器故障注入，验证赠送两步均回滚而外层仍能写入/提交。
- 根因复盘：跨层事务契约遗漏（普通 GetByID 不沿 tx、SQL 错误污染外层事务），不是奖励算法问题。最初直接注册/仓储幂等测试不足以覆盖 pending 的事务边界；现通过 pending 真实 handler 测试及数据库触发器故障注入固定防线。没有修改测试超时来掩盖问题，也没有删除任何测试。

## 四、实际改动文件清单

### 后端（33 个）

- `backend/internal/handler/admin/setting_handler.go`
- `backend/internal/handler/admin/setting_handler_audit.go`
- `backend/internal/handler/admin/setting_handler_update.go`
- `backend/internal/handler/admin/setting_signup_bonus_test.go`
- `backend/internal/handler/auth_email_oauth_test.go`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/handler/setting_handler.go`
- `backend/internal/repository/affiliate_repo.go`
- `backend/internal/repository/affiliate_signup_bonus_integration_test.go`
- `backend/internal/repository/redeem_code_repo.go`
- `backend/internal/repository/redeem_code_totals_test.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/service/admin_user.go`
- `backend/internal/service/affiliate_detail.go`
- `backend/internal/service/affiliate_distribution.go`
- `backend/internal/service/affiliate_owner.go`
- `backend/internal/service/affiliate_reward_config.go`
- `backend/internal/service/affiliate_service.go`
- `backend/internal/service/affiliate_signup_bonus_test.go`
- `backend/internal/service/affiliate_two_tier_test.go`
- `backend/internal/service/auth_email_oauth_auto.go`
- `backend/internal/service/auth_oauth_email_flow.go`
- `backend/internal/service/auth_service.go`
- `backend/internal/service/domain_constants.go`
- `backend/internal/service/finance.go`
- `backend/internal/service/finance_reports.go`
- `backend/internal/service/finance_signup_bonus_test.go`
- `backend/internal/service/payment_fulfillment_test.go`
- `backend/internal/service/setting_parse.go`
- `backend/internal/service/setting_public.go`
- `backend/internal/service/setting_update.go`
- `backend/internal/service/settings_view.go`
- `backend/migrations/262_affiliate_invitee_signup_bonus.sql`

### 前端（33 个）

- `frontend/src/api/__tests__/admin.finance.spec.ts`
- `frontend/src/api/__tests__/inviteeSignupBonus.spec.ts`
- `frontend/src/api/admin/affiliates.ts`
- `frontend/src/api/admin/finance.ts`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/api/auth.ts`
- `frontend/src/api/user.ts`
- `frontend/src/components/admin/finance/FinanceRechargeTab.vue`
- `frontend/src/components/admin/finance/__tests__/FinanceRechargeTab.spec.ts`
- `frontend/src/components/admin/user/UserBalanceHistoryModal.vue`
- `frontend/src/components/admin/user/__tests__/UserBalanceHistoryModal.spec.ts`
- `frontend/src/i18n/locales/en/admin/finance.ts`
- `frontend/src/i18n/locales/en/admin/overview.ts`
- `frontend/src/i18n/locales/en/admin/settings.ts`
- `frontend/src/i18n/locales/en/common.ts`
- `frontend/src/i18n/locales/en/dashboard.ts`
- `frontend/src/i18n/locales/zh/admin/finance.ts`
- `frontend/src/i18n/locales/zh/admin/overview.ts`
- `frontend/src/i18n/locales/zh/admin/settings.ts`
- `frontend/src/i18n/locales/zh/common.ts`
- `frontend/src/i18n/locales/zh/dashboard.ts`
- `frontend/src/types/index.ts`
- `frontend/src/utils/affiliateRewards.ts`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/__tests__/SettingsView.spec.ts`
- `frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue`
- `frontend/src/views/admin/affiliates/__tests__/AdminAffiliateRecordsTable.spec.ts`
- `frontend/src/views/auth/RegisterView.vue`
- `frontend/src/views/auth/__tests__/RegisterView.spec.ts`
- `frontend/src/views/user/AffiliateView.vue`
- `frontend/src/views/user/RedeemView.vue`
- `frontend/src/views/user/__tests__/AffiliateView.spec.ts`
- `frontend/src/views/user/__tests__/RedeemView.spec.ts`

### 规范（2 个）

- `.trellis/spec/backend/admin-offline-recharge.md`
- `.trellis/spec/backend/database-guidelines.md`

### 任务文档

- `.trellis/tasks/10-27-invitee-signup-bonus/design.md`
- `.trellis/tasks/10-27-invitee-signup-bonus/implement.md`

### 仓库外本机 e2e（6 个）

- `/home/qlqq/workspace/sub2api-local/e2e/p0_bootstrap.py`
- `/home/qlqq/workspace/sub2api-local/e2e/p3_fr_lim.py`
- `/home/qlqq/workspace/sub2api-local/e2e/p10_finance.py`
- `/home/qlqq/workspace/sub2api-local/e2e/lib.py`
- `/home/qlqq/workspace/sub2api-local/e2e/CASES.md`
- `/home/qlqq/workspace/sub2api-local/e2e/ui/shots.cjs`

本机截图仍通过 `ui/shots.sh` 加载 Playwright 环境；`shots.cjs` 仅新增匿名截图模式和定位设置卡片的可选分支，原有离线充值验收分支保持不变。

## 五、最终验证结果

### 命令与结果

| 范围 | 实际命令 | 最终结果 | 日志/证据 |
|---|---|---|---|
| 后端全量单元测试 | `cd backend && env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...` | 通过，退出码0；58个有测试的包通过，没有删测试或把跳过作为通过 | `/tmp/signup-bonus-backend-unit-latest.log` |
| 后端构建 | `cd backend && go build ./...` | 通过，退出码0 | `/tmp/signup-bonus-backend-build-final.log` |
| 既有 Affiliate 集成 | `cd backend && go test -tags=integration ./internal/repository/ -run Affiliate` | 通过，退出码0 | `/tmp/signup-bonus-integration.log` |
| 最终集成回归 | 见下方最终集成命令 | 通过，退出码0；真实 PostgreSQL，包含新增索引、12路并发幂等、非零初始余额、首充叠加、外层回滚、余额写入失败保存点恢复、记录/余额历史筛选及现金/赠送拆分 | `/tmp/signup-bonus-integration-final.log` |
| 新行为定向单测 | 见下方定向单测命令 | 通过，四个包全部通过；覆盖直接 OAuth、pending 最终注册/会话重放、异常降级、首充独立、设置审计/负值/公开开关、财务免费与独立分类 | `/tmp/signup-bonus-targeted-final.log` |
| 前端 lint | `cd frontend && npx -y pnpm@9 run lint:check` | 通过 | `/tmp/signup-final-lint.log` |
| 前端类型 | `cd frontend && npx -y pnpm@9 run typecheck` | 通过 | `/tmp/signup-final-typecheck.log` |
| 前端全量测试 | `cd frontend && npx -y pnpm@9 run test:run` | 439个文件、3498个测试通过 | `/tmp/signup-final-all.log` |
| 本机完整构建与启动 | `cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d` | 通过，包含 i18n 检查、vue-tsc、Vite、Go embed 构建、薄镜像；应用健康检查通过 | 工具会话实际退出码0；未使用仓库多阶段 Dockerfile |
| 最终后端镜像更新 | `cd ~/workspace/sub2api-local && ./build.sh --no-web && docker compose up -d` | 保存点修正后通过，使用上次已验证的前端产物；最终 e2e 在此镜像上重跑 | `/tmp/signup-bonus-local-backend-build.log` |
| fast e2e | `cd ~/workspace/sub2api-local/e2e && ./run_all.sh --fast` | **221/221通过，0失败，退出码0**；当前 fast 已含 p7 和 p10，新 SGB 组全绿 | `runs/20261027-202858/{report.md,results.jsonl,run.log,state.json}`；额外控制台日志 `/tmp/signup-bonus-e2e-fast-latest.log` |
| 独立 p10 | 将 fast 的 `state.json` 复制到独立结果目录，`E2E_RUN_DIR=<目录> python3 p10_finance.py`，然后 `python3 report.py <目录>` | **34/34通过，0失败**；脚本与报告退出码均0；FIN-16独立赠送、FIN-16b赠送合计、FIN-16c仅赠送用户对账通过 | `runs/20261027-203259-p10-only/{report.md,results.jsonl,run.log,state.json}` |
| 界面截图 | `cd ~/workspace/sub2api-local/e2e/ui && SIGNUP_BONUS_UI=1 ./shots.sh <管理员邮箱> /tmp/signup-bonus-admin /admin/settings`；`./shots.sh --anonymous /tmp/signup-bonus-register '/register?aff=<本轮N1邀请码>'`；`./shots.sh <本轮SGBN邮箱> /tmp/signup-bonus-user /affiliate` | 通过，真实登录/匿名浏览器；临时配置注册赠送5、首充3，截图后已恢复原设置（注册赠送0、首充off） | `/tmp/signup-bonus-ui-shots.log`；下面列出稳定截图路径 |
| 最终差异检查 | `git diff --check`；`git diff --exit-code -- frontend/pnpm-lock.yaml frontend/package.json`；确认不存在 `frontend/pnpm-workspace.yaml` | 全部通过，锁文件和依赖无改动；仅新增迁移262，旧迁移没有修改 | 最终 git status 核对 |

最终集成与定向单测的实际可复制命令：

```bash
cd /home/qlqq/workspace/sub2api/backend
go test -tags=integration ./internal/repository/ -run 'Affiliate|BalanceHistoryTotals'
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository -run 'InviteeSignupBonus|FinanceInvitee|SignupBonusSettingAudit|TestCompleteEmailOAuthRegistrationUsesAffiliateCode|BalanceHistoryTotals'
```

### 截图路径与人工视觉复核

已打开图片核对中文、金额、卡片顺序及展示内容：

- `/tmp/signup-bonus-admin-card.png`：同屏可见首充奖励卡片和新注册赠送卡片，金额5，说明含代理直接邀请、直接进余额、计赠送、补绑不发、0关闭。
- `/tmp/signup-bonus-admin-settings.png`：管理员功能设置完整页面。
- `/tmp/signup-bonus-register.png`：带邀请码的匿名注册页，注册赠送5与首充满10赠3提示同时显示。
- `/tmp/signup-bonus-my-invites.png`：邀请页提示好友注册即得5，本人“已领取注册赠送5”独立卡片；实际余额28（注册5+充值20+首充3）。

带原路由后缀的原始截图也保留在 `/tmp/signup-bonus-*`，稳定路径只是副本。截图由 `/tmp/run-signup-bonus-shots.py` 编排调用现有 `shots.sh`，仅加载本机隔离环境凭据且用 finally 恢复设置，没有新增远端目标。

### 过程中的失败及修复（不隐瞒）

1. 首次编译尝试的配置构建函数返回值数量不匹配，随后改为返回 `(nil,error)`；编译与全量测试最终通过。
2. 新财务定向测试首次把内部 `financeAmounts` 断言成 `map[string]string` 导致 panic；修正测试以实际十进制类型断言，没有删断言或降低金额精度。最终测试覆盖了两类赠送独立合计和非分润口径。
3. pending OAuth 普通余额重读造成单连接等待：初次全量测试在该真实 handler 测试默认10分钟超时失败，定向60秒诊断也复现。日志保留 `/tmp/signup-bonus-backend-unit.log`、`/tmp/signup-bonus-pending-final.log`。通过区分事务内 pending 与非事务注册修正，未扩大连接池或调高超时；修复后完整重跑两次通过（`...-unit-final.log`、`...-unit-latest.log`）。另补 SQL 故障保存点回归防止仅吞错但外层事务被污染。
4. 首次 fast 的新用例 SGB-03 读取错测试辅助函数，异常结果保留在 `runs/20261027-202205/`；修正成真实详情 API 后两次 fast 全绿：`runs/20261027-202317/`、最终镜像的 `runs/20261027-202858/`。

### 验证边界与遗留问题

- 无已知功能遗留问题。外部 OAuth 提供商真实授权没有在本机连接；直接 OAuth 与 pending 最终建号用仓库真实处理流程测试覆盖，数据库入账/幂等/故障隔离用真实 PostgreSQL 覆盖。
- `golangci-lint` 本机未安装，按 AGENTS.md 留给 CI；没有声称已经运行。Vite 的旧 Browserslist 数据、产物体积及 Node 弃用提示是非阻塞构建警告，本任务没有升级依赖。
- 本任务没有要求的完整模式 p5/p6/p8/p9 未额外重跑；消费基数已用单测、真实仓储 PaidTotal 查询与 SGB 用例核对，本次要求的 fast 与独立 p10 均已完成。
- 所有操作仅本机隔离环境。未 git commit、未 push、未部署一号机、未全局 Docker 清理；未使用全局 pnpm。代码保持未提交，任务不归档，以免触发 Trellis 自动提交。

## 六、后续授权发布（2026-10-28 UTC）

以上「未提交/未部署」为实现验收当时的记录。后续用户授权推送并部署；产品提交 `1dafd13b7` 已推送，CI `40624930942` 四个任务成功，一号机 sub2api-custom 已更新并完成只读验证，注册赠送保持0。详见 `release-report.md`。
