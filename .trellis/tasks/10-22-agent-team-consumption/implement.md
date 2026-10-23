# 实施计划：代理团队消费页

前置：工作区里还有 e2e 任务未提交的自动升级修复（`affiliate_agent_level.go` 等 11 个文件），开工前先由用户决定提交，避免和本任务的改动混在一起。

## 步骤

### 1. 数据层（迁移 + 触发器）
- [x] `migrations/251_usage_user_daily_rollups.sql`：表、索引、三个触发器函数（带 `EXCEPTION` 兜底）、触发器、回填（design 第 1 节）。
- [x] `migrations/251_affiliate_ledger_user_created_index_notx.sql`（design 第 2 节）。
- [x] 迁移 unit 测试：关键 SQL 片段。
- [x] 触发器集成测试，参照 `group_usage_rollup_trigger_integration_test.go`，普通表和分区表两种变体各跑一遍：
  - 插入、批量插入同键、删除（含删到 0 后删掉汇总行）、UPDATE（改 `group_id`、改 `actual_cost`）。
  - `billing_type=1` 不计入；跨北京时间零点的归日。
  - 迁移重复执行结果一致；回填后与原始明细重算一致。
  - 区间修复 SQL 与并发插入同时执行时结果正确。
- [x] `CleanupUsageLogs` 追加删除过期汇总，并补单元测试。

**检查点 A**：`make test-unit`、`make test-integration` 通过。

### 2. 后端服务与接口
- [x] `service/affiliate_team.go`：端口、错误常量、准入、范围、日期校验（1–90 天、北京时间）、前 5 名合并、脱敏。
- [x] `repository/affiliate_team_repo.go`：overview / customers / agents / requests 四类查询（design 第 5 节），排序白名单，搜索转义。
- [x] `AffiliateRepository.CountUplineCustomers` 及 3 个手写桩；`GetAffiliateDetail` 返回 `team_access`。
- [x] `handler/affiliate_team_handler.go`、`routes/user.go`（`/aff/team` 路由组 + Heavy）、wire provider，执行 `make generate`。
- [x] 测试：
  - service 单元测试：准入矩阵、日期边界、前 5 名合并、「历史」判定。
  - repository 集成测试：AC2 场景、AC3 历史二级、AC4 口径（与直接 SQL 汇总对比）、AC5 越权、AC6 响应字段白名单、邮箱前缀搜索无结果。
  - 路由测试：路径注册、Heavy 限流挂载。

**检查点 B**：`gofmt -l`、`go vet`、`make test-unit`、`make test-integration` 通过。跑单测时去掉代理环境变量：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY`。

### 3. 前端
- [x] 类型与 `api/affiliateTeam.ts`。
- [x] `useAffiliateTeamQuery.ts`：白名单解析、同步、下钻时只携带时间范围。
- [x] 组件 `TeamToolbar`、`TeamSummaryStrip`、`TeamTrendChart`、`TeamModelShare`、`TeamCustomerTable`、`TeamAgentTable`、`TeamRequestTable`。
- [x] `AffiliateTeamView.vue`（三个路由名）、路由注册、`AffiliateView.vue` 入口、`affiliate.team.*` 中英文案。
- [x] vitest：
  - query 解析与非法值回退。
  - 三种视角渲染：一级、二级、被降级。
  - Tab 显隐、面包屑生成、模型点击筛选、403/404 状态、入口显隐。

**检查点 C**：在 `frontend/` 下执行 `npx -y pnpm@9 lint:check && npx -y pnpm@9 typecheck && npx -y pnpm@9 test:run && npx -y pnpm@9 check:i18n && npx -y pnpm@9 build`。必须用 pnpm@9，不能用全局 pnpm 12，否则会改写 lockfile。

### 4. 本机性能验证（AC9）
- [x] `research/seed_team_perf.sql`：按 design 第 8 节规模造数，写入本机 Postgres（127.0.0.1:25432）。
- [x] 记录回填耗时、触发器开销；对四个接口在 7 天和 90 天区间做 `EXPLAIN ANALYZE`，并用 curl 测耗时，结果写入 `research/perf.md`。
- [x] 收益查询经合并扫描后达标（research/perf.md），未启用备选方案。

### 5. 本机端到端（AC1–AC8、AC10）
- [x] `sub2api-local`：`./build.sh` → `docker compose up -d`；用 e2e helper 构造一级 / 二级 / 降级 / 历史二级场景，写入消费与结算。
- [x] `e2e/ui/shots.sh` 截三层页面、移动端宽度、空状态，截图放进 `screenshots/`。
- [x] 核对页面数字与 SQL 直查一致。
- [x] 真实链路 e2e（`sub2api-local/e2e/p6_team.py`，已并入 `run_all.sh`）：经网关真实请求消费、定时结算、Playwright 真实点击，并重跑原有 104 条回归；结果与发现见 `research/e2e.md`。
- [x] e2e 发现、经用户确认后修改：任命名额只计任命人还能取消的二级（改挂走、账号已删除的不计），邀请列表里已删除的用户不再显示「设为二级」/「取消」（`research/e2e.md` 第 4、5 条）。

### 6. 收尾
- [x] 更新 spec：
  - `database-guidelines.md`：用户日汇总触发器约定与修复 SQL。
  - `frontend/state-management.md`：团队页 URL query 写法。
- [ ] 提交（由用户确认时机）；部署一号机前先查 `usage_logs` 行数，评估迁移耗时。

## 回滚点
- 步骤 1 完成后单独提交一次：迁移一旦在一号机执行就不能再改。如果在本机发现问题，只能在部署前修改 251；部署后只能追加新迁移。
- 前后端可以独立下线：前端隐藏入口，后端摘掉路由。
