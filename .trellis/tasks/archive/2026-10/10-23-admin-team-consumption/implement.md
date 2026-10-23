# 实施计划：管理端团队消费页

前置：代理端团队消费页的两个提交（`196ee2502`、`b62bc1536`）已在本地分支，未推送。本任务在其上继续，代理端行为不得改变。

## 步骤

### 1. 数据层：分润日汇总（迁移 252）
- [x] `migrations/252_affiliate_agent_reward_daily_rollups.sql`：表、索引、触发器函数（`EXCEPTION` 兜底、UPDATE 只处理关键列变化）、三个语句级触发器、重建函数、全量回填（design 第 1 节）。
- [x] `migrations/affiliate_agent_reward_rollup_migration_test.go`（unit）：关键 SQL 片段。
- [x] `repository/affiliate_agent_reward_rollup_trigger_integration_test.go`：design 第 6 节「触发器 integration」所列场景；另用仓储真实的入账方法 `AccrueQuota`（`affiliate_repo.go:190`）写两种代理分润，再核对汇总。

**检查点 A**：`go test -tags unit ./migrations/...`、`go test -tags=integration ./internal/repository/ -run 'AgentRewardRollup|UsageUserRollup'` 通过。

### 2. 后端：服务、仓储、接口
- [x] `service/affiliate_team.go`：受众分叉（准入、展示、搜索、`subject`），代理端公开方法签名与行为不变；`AffiliateTeamListQuery` 加 `Level`、`AdminSearch`。
- [x] `service/affiliate_team_admin.go`：`AdminOverview`、`AdminListOwners`、`AdminViewer*` 四个方法与新类型。
- [x] `repository/affiliate_team_repo.go`：`ListCustomers` 支持管理端搜索；新增 `GetViewerSubject`。
- [x] `repository/affiliate_team_admin_repo.go`：`GetPlatformAggregates`、`ListTeamOwners`（design 第 2 节）。
- [x] `handler/affiliate_team_admin_handler.go`：六个管理端方法，复用现有参数解析。
- [x] `routes/admin.go`：`registerAffiliateRoutes` 增加 `/team` 路由组。
- [x] 测试：
  - service unit：管理端受众矩阵、代理端响应不含 `subject`、平台总览补零与前 5 合并、参数透传。
  - repository integration：design 第 6 节所列 `ListTeamOwners` / `GetPlatformAggregates` 场景；每行与该代理 `GetAggregates` 摘要逐项相等。
  - routes / handler：管理员组注册、`:viewer_id` 校验、非管理员 403。

**检查点 B**：`gofmt -l`、`go vet ./...`、`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...`、`go test -tags=integration ./internal/repository/ -run 'Affiliate|UsageUserRollup|AgentRewardRollup'`、`go build ./...` 通过。

### 3. 前端：抽出共享页面（代理端行为不变）
- [x] `teamContext.ts`、`TeamConsumptionPage.vue`；`AffiliateTeamView.vue` 改为薄视图。
- [x] `TeamCustomerTable`、`TeamAgentTable`、`TeamSummaryStrip`、`TeamTrendChart`、`TeamUserCell` 改为从上下文取路由与文案。
- [x] 代理端原有 vitest 不改断言即通过。

**检查点 C1**：`npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run`。

### 4. 前端：管理端页面
- [x] `api/admin/affiliateTeam.ts`、`types/index.ts`。
- [x] `composables/useAdminAffiliateTeamQuery.ts`。
- [x] `views/admin/affiliates/AdminAffiliateTeamView.vue`、`AdminAffiliateTeamViewerView.vue`、`AdminTeamOwnerTable.vue`；`TeamSummaryStrip` 的 `platform` 视图；`TeamToolbar` 无面包屑时右对齐。
- [x] 路由 4 条、侧边栏「团队消费」、代理管理「⋯」菜单项。
- [x] 文案：`nav.affiliateTeam`、`admin.affiliates.team.*`（中英）。
- [x] vitest：design 第 6 节「前端 vitest」所列。

**检查点 C2**：在 `frontend/` 下 `npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run && npx -y pnpm@9 run check:i18n && npx -y pnpm@9 run build`；`git status` 确认 `pnpm-lock.yaml` 无改动。

### 5. 本机性能验证
- [x] 扩展造数脚本（design 第 7 节），写入本机 Postgres（127.0.0.1:25432）。
- [x] 记录 252 回填耗时、触发器开销；汇总页两个接口 7 / 90 天的 `EXPLAIN ANALYZE` 与 curl 耗时 → `research/perf.md`。不达标时回到第 2 步调整查询。

### 6. 本机端到端
- [x] `sub2api-local`：`./build.sh && ./reset.sh`，`e2e/run_all.sh` 全量。
- [x] p6 增加管理端接口组 `TADM` 与界面用例，`CASES.md` 同步；原有 206 条全部通过。
- [x] 截图：汇总页、标签筛选与搜索、代理视角三层、使用记录跳转、手机宽度、深色 → `screenshots/`。
- [x] 结果与发现写入 `research/e2e.md`。

### 7. 收尾
- [x] 更新 spec：`database-guidelines.md` 记录分润日汇总（第二个触发器汇总）；`frontend/component-guidelines.md` 或 `state-management.md` 记录「受众上下文」复用方式。
- [ ] 提交（由用户确认时机），迁移 252 单独一个提交。
- [ ] 部署一号机前：查 `usage_logs`（`billing_type = 0`）与 `user_affiliate_ledger`（两种代理分润）行数，估算 251、252 回填耗时，再由用户决定部署时间。

## 回滚点
- 第 1 步完成后单独提交：迁移在一号机执行后不能再改，本机发现问题只能在部署前修改 252。
- 第 3 步是纯重构：若代理端回归失败，先回到第 3 步修复，不进入第 4 步。
- 前后端可独立下线：前端隐藏入口与路由，后端摘掉 `/admin/affiliates/team` 路由组。
