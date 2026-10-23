# 设计：代理团队消费页

需求见 `prd.md`，界面见 `ui.md`。本文只写技术方案。

## 0. 决策摘要

| 问题 | 决定 | 理由 |
|---|---|---|
| 多客户聚合的数据源 | 新增 `usage_user_daily_rollups`（用户 × 北京时间日 × 模型 × 分组），由 `usage_logs` 上的**语句级触发器增量维护** | 实时、精确，没有水位，也没有"今天实时查明细"的拼接；不需要定时任务 |
| 为什么不照抄分组日汇总（222/223） | 不照抄 | 222 用"水位 + 从水位起全量重建"：重建期间持有状态行 `FOR UPDATE`，所有 usage 写入都要等它；每次保留期清理后会把约 90 天全部重建。按用户 × 模型 × 分组的数据量下，这两点代价太大 |
| 日界 | 固定 `Asia/Shanghai` | PRD 明确按北京时间自然日；固定时区不受会话 TimeZone 影响，也就不需要时区状态表和改时区后的重建逻辑 |
| 触发器出错怎么办 | 触发器内用 `EXCEPTION` 兜住错误，只写 `WARNING` | usage 明细是计费记录，优先级高于统计；修复方法见第 9 节 |
| 收益按时间范围汇总 | 给 ledger 新增覆盖索引 `(user_id, created_at) INCLUDE (source_user_id, reward_type, amount)` | 现有索引没有 `created_at`；大代理 90 天约 100 万行流水，走仅索引扫描 |
| 新数据访问层 | 新建独立的 `AffiliateTeamRepository`；`AffiliateRepository` 只加一个计数方法 | 少改 3 个手写桩，职责也更清楚 |

## 1. 迁移 `251_usage_user_daily_rollups.sql`（事务内）

### 1.1 表

```sql
CREATE TABLE IF NOT EXISTS usage_user_daily_rollups (
    user_id               BIGINT        NOT NULL,
    bucket_date           DATE          NOT NULL,           -- 北京时间自然日
    model                 VARCHAR(100)  NOT NULL,           -- COALESCE(NULLIF(BTRIM(requested_model), ''), model)
    group_id              BIGINT        NOT NULL DEFAULT 0, -- 0 = 无分组
    requests              BIGINT        NOT NULL DEFAULT 0,
    input_tokens          BIGINT        NOT NULL DEFAULT 0,
    output_tokens         BIGINT        NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT        NOT NULL DEFAULT 0,
    cache_read_tokens     BIGINT        NOT NULL DEFAULT 0,
    image_count           BIGINT        NOT NULL DEFAULT 0,
    video_count           BIGINT        NOT NULL DEFAULT 0,
    actual_cost           DECIMAL(20,10) NOT NULL DEFAULT 0,
    last_used_at          TIMESTAMPTZ   NOT NULL,
    PRIMARY KEY (user_id, bucket_date, model, group_id)
);
CREATE INDEX IF NOT EXISTS idx_usage_user_daily_rollups_bucket_date ON usage_user_daily_rollups (bucket_date);
```

- 只收 `billing_type = 0`（余额消费）。
- 模型取"用户请求的模型"，口径与用户使用记录页一致（`usage_log_repo_trend.go:737` 的表达式）。
- 主键以 `user_id` 开头，用于"范围内客户 × 日期区间"的连接；`bucket_date` 单列索引只给保留期清理用。
- 体量：2 千活跃用户 × 约 3 个模型 × 90 天 ≈ 54 万行。

### 1.2 触发器（语句级，作用在 `usage_logs` 父表上）

| 触发器 | 动作 |
|---|---|
| `usage_logs_user_rollup_ins`：`AFTER INSERT REFERENCING NEW TABLE AS new_rows FOR EACH STATEMENT` | 把 `new_rows` 中 `billing_type = 0` 的行按主键 `GROUP BY ... ORDER BY 主键` 后 `INSERT ... ON CONFLICT DO UPDATE`：各项累加，`last_used_at = GREATEST(...)` |
| `usage_logs_user_rollup_del`：`AFTER DELETE REFERENCING OLD TABLE AS old_rows` | 按主键聚合后从汇总中扣减；扣减后 `requests <= 0` 的汇总行直接删除 |
| `usage_logs_user_rollup_upd`：`AFTER UPDATE REFERENCING OLD TABLE AS old_rows NEW TABLE AS new_rows` | 先扣旧、再加新 |

要点：

- 日期统一写成 `(created_at AT TIME ZONE 'Asia/Shanghai')::date`。
- `GROUP BY` 保证一条语句内同一个键只出现一次，避免 `ON CONFLICT ... cannot affect row a second time`。`ORDER BY` 让并发批次按相同顺序锁行，避免死锁。
- 函数体包在 `BEGIN ... EXCEPTION WHEN OTHERS THEN RAISE WARNING ...; END`，汇总出错时不影响 usage 写入。
- UPDATE 触发器不能带列清单，因为带转换表的触发器不允许指定列。运行时代码不会 UPDATE `usage_logs`，只有删除分组时外键 `ON DELETE SET NULL` 会改 `group_id`；分组用的是软删除，这条路径基本不会触发。触发器在这里只是为了让数据保持一致。
- 分区表：语句级触发器加转换表在分区父表上可用（PG 18；222 的集成测试已经覆盖过分区变体）。直接写子分区不会触发，应用不会这样写。
- 触发器名用 `usage_logs_user_rollup_*`，按字母序排在 222 的 `usage_logs_group_rollup_*` 之后执行，和现有锁顺序一致。
- 不处理 TRUNCATE，与 222 相同。

### 1.3 回填

在同一迁移、同一事务中：先 `CREATE TRIGGER`，再调用迁移里定义的 `rebuild_usage_user_daily_rollups(from_date, to_date)` 重算全部日期。该函数也是运维修复入口：同一事务内先删区间，再 `INSERT ... SELECT ... ON CONFLICT DO UPDATE` 累加重建。

- `CREATE TRIGGER` 拿到的锁会挡住并发写入，所以回填和触发器之间没有空档。迁移在 `InitEnt` 启动阶段、开始对外服务之前执行（`internal/repository/ent.go:139`）。
- 迁移用 `IF NOT EXISTS` / `CREATE OR REPLACE` / `DROP TRIGGER IF EXISTS`，重复执行结果一致。
- 回填耗时和 `usage_logs` 行数成正比。默认只保留 90 天（`dashboard_aggregation.retention.usage_logs_days`）。部署一号机前先查行数，并在本机用同量级数据测迁移耗时。

### 1.4 保留期

在 `CleanupUsageLogs`（`internal/repository/dashboard_aggregation_repo.go:233`）末尾追加：

```sql
DELETE FROM usage_user_daily_rollups WHERE bucket_date < (cutoff AT TIME ZONE 'Asia/Shanghai')::date
```

- 批量删除明细时，DELETE 触发器已经同步扣减了汇总。
- 整月删分区（`dropUsageLogsPartitions`）不触发触发器，由这条语句收尾。
- 管理员发起的用量清理任务（`usage_cleanup_repo.go`）走批量 DELETE，由触发器处理。

## 2. 迁移 `251_affiliate_ledger_user_created_index_notx.sql`

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliate_ledger_user_created
    ON user_affiliate_ledger (user_id, created_at) INCLUDE (source_user_id, reward_type, amount);
```

## 3. 范围与准入（全部按归属快照）

| 名称 | 条件（`ua` = user_affiliates，`$me` = 当前用户） |
|---|---|
| 我的客户 | `ua.agent_owner_id = $me` |
| 下级团队成员 | `ua.agent_upline_id = $me` |
| 团队 | `ua.agent_owner_id = $me OR ua.agent_upline_id = $me` |
| 下级代理集合 | `{ua.user_id : ua.agent_level = 2 AND ua.agent_owner_id = $me}` ∪ `{DISTINCT ua.agent_owner_id : ua.agent_upline_id = $me}` |
| 二级代理 X 的团队 | `ua.agent_owner_id = $x AND ua.agent_upline_id = $me` |
| X 是否为「历史」 | `NOT (X.agent_level = 2 AND X.agent_owner_id = $me)` |

- **准入**：`affiliate_enabled`，并且满足以下任一条件：`agent_level IN (1,2)`、存在 owner 客户、存在 upline 客户。都不满足时返回 `AFFILIATE_TEAM_FORBIDDEN`（403）。
- **越权**：`agent_id` 不在下级代理集合中，或 `user_id` 不在团队中，返回 `AFFILIATE_TEAM_NOT_FOUND`（404），不区分"不存在"和"无权"。
- **收益**：`user_affiliate_ledger.user_id = $me AND reward_type IN ('agent_direct','agent_diff')`。来源天然落在团队范围内，所以团队总额不需要再关联范围；看下级团队或单个客户时，再按 `source_user_id` 关联。
- **`/user/aff` 新增 `team_access`**：计算方式为 `isAgent || owned_customer_count > 0 || upline_customer_count > 0`。为此 `AffiliateRepository` 新增 `CountUplineCustomers`，3 个手写桩同步补上（`service/affiliate_two_tier_test.go:28`、`service/payment_fulfillment_test.go:98`、`handler/auth_email_oauth_test.go:366`）。

## 4. 接口契约

- 全部挂在 `GET /api/v1/user/aff/team/*`，路由组 `team.Use(panelRateLimiter.Heavy())`。
- 公共参数：`start_date`、`end_date`（`YYYY-MM-DD`，按北京时间解析，结束日包含在内），跨度 1–90 天，否则返回 `AFFILIATE_TEAM_INVALID_RANGE`（400）。
- 用户展示对象 `TeamUser`：`{ user_id, nickname, email, deleted }`。`email` 已脱敏，`nickname` 为空串时由前端显示 `-`，已删除时 `nickname`、`email` 都为空。
- 金额统一用 `float64`；时间用 RFC3339。

### 4.1 `GET /user/aff/team/overview`

参数：日期，另可选 `agent_id` 或 `user_id`（二者互斥）。

```jsonc
{
  "viewer":   { "agent_level": 1, "downgraded": false, "settle_interval_minutes": 60 },
  "counts":   { "direct_customers": 1234, "sub_agents": 5 } | null,           // 仅团队视角
  "agent":    { "user": TeamUser, "historical": false } | null,                // 仅二级团队视角
  "customer": { "user": TeamUser, "agent_level": 3, "relation": "owner" | "upline",
                "owner_agent": TeamUser | null, "is_sub_agent": false,
                "bound_at": "..." | null, "lifetime_earnings": 88.0 } | null,  // 仅客户视角
  "summary":  { "cost": 0, "direct_cost": 0, "sub_cost": 0, "earnings": 0,
                "active_customers": 0, "customers": 0, "requests": 0, "tokens": 0 },
  "trend":    [ { "date": "2026-10-16", "cost": 0, "earnings": 0 } ],  // 区间内每天一条，无数据补 0
  "models":   [ { "model": "claude-sonnet-4-5", "cost": 0, "share": 0.62 } ], // 前 5 名 + 其他（model 为空串）
  "filters":  { "models": ["..."], "groups": [ { "id": 1, "name": "默认" } ] } | null // 仅客户视角
}
```

- `downgraded` 表示当前不是代理、但有快照客户，前端据此显示降级提示。
- `counts.sub_agents > 0` 时显示「下级代理」Tab，`summary.sub_cost` 才有意义。
- `is_sub_agent` 表示该客户属于我的下级代理集合，前端据此把徽标做成可点击。
- 三种视角各用一个可空字段（`counts` / `agent` / `customer`），前端类型无需按 `kind` 判别。

### 4.2 `GET /user/aff/team/customers`

- 参数：日期、可选 `agent_id`、`search`、`sort_by ∈ {cost, requests, earnings, last_used_at}`（默认 `cost`）、`sort_order ∈ {asc, desc}`（默认 `desc`）、`page`、`page_size`（默认 20，最大 100）。
- 返回 `response.Paginated`，每项为：`{ user: TeamUser, agent_level, cost, requests, earnings, last_used_at|null, cost_share }`。
- `cost_share` 是该客户占当前范围总消费的比例，前端用它画消费单元格的底条。

### 4.3 `GET /user/aff/team/agents`

- 参数：日期、`sort_by ∈ {team_cost, earnings, active_customers}`（默认 `team_cost`）、`sort_order`、`page`、`page_size`。
- 每项为：`{ user: TeamUser, historical, customers, active_customers, team_cost, earnings, cost_share }`。

### 4.4 `GET /user/aff/team/customers/:user_id/requests`

- 参数：日期、可选 `model`、`group_id`（0 表示无分组）、`page`、`page_size`（默认 20，可选 20/50/100）。
- 每项为：`{ created_at, model, group_name, billing_mode, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, image_count, video_count, cost }`。
- 窄 DTO 只含这些字段，**不复用** `dto.UsageLog`。

## 5. 查询方案

所有多客户查询都是同一个形状，下面以客户列表为例：

```sql
WITH scope AS (SELECT ua.user_id, ua.agent_level FROM user_affiliates ua WHERE <范围>),
usage AS (
  SELECT r.user_id,
         SUM(r.actual_cost) FILTER (WHERE r.bucket_date BETWEEN $s AND $e) AS cost,
         SUM(r.requests)    FILTER (WHERE r.bucket_date BETWEEN $s AND $e) AS requests,
         MAX(r.last_used_at) AS last_used_at                              -- 不受区间限制
  FROM usage_user_daily_rollups r JOIN scope USING (user_id)
  GROUP BY r.user_id),
earn AS (
  SELECT l.source_user_id AS user_id, SUM(l.amount) AS earnings
  FROM user_affiliate_ledger l
  WHERE l.user_id = $me AND l.reward_type IN ('agent_direct','agent_diff')
    AND l.created_at >= $sTs AND l.created_at < $eTs
  GROUP BY l.source_user_id)
SELECT ..., COUNT(*) OVER () AS total
FROM scope s
LEFT JOIN users u ON u.id = s.user_id
LEFT JOIN usage ON usage.user_id = s.user_id
LEFT JOIN earn  ON earn.user_id  = s.user_id
WHERE <搜索>
ORDER BY <白名单列> <方向> NULLS LAST, s.user_id DESC
LIMIT $n OFFSET $o
```

- **搜索**：`u.username ILIKE '%' || 转义后的q || '%' OR LOWER(u.email) = LOWER(q)`。LIKE 通配符要转义；邮箱只做等值匹配。
- **已删除用户**：`deleted = (u.id IS NULL OR u.deleted_at IS NOT NULL)`，邮箱在 Go 里经 `maskEmail` 脱敏后返回。
- **下级代理列表**：成员按 `agent_owner_id` 分组；`earn` 只取 `agent_diff`，再按成员关联到代理；集合里没有任何成员的当前二级代理也要列出，数值为 0。
- **摘要、趋势、模型分布（`GetAggregates`）**：汇总表只扫一遍，用 `GROUPING SETS ((bucket_date), (model), (user_id))` 同时得出按天、按模型、按客户的结果，摘要在 Go 里由按客户的行合计；流水只扫一遍，按 `(l.created_at AT TIME ZONE 'Asia/Shanghai')::date` 分天，同时得出总收益与趋势。区间内缺失的日期在 Go 里补 0；模型取前 5 名，其余合并为「其他」。实测见 `research/perf.md`（拆成多次扫描时 90 天团队总览为 1.82 s，合并后 0.66 s）。
- **请求明细**：`usage_logs WHERE user_id = $c AND billing_type = 0 AND created_at >= $sTs AND created_at < $eTs [AND 有效模型表达式 = $m] [AND COALESCE(group_id,0) = $g] ORDER BY created_at DESC, id DESC`，走 `idx_usage_logs_user_created`；`LEFT JOIN groups` 取名称（已软删除的分组也照样显示名称）；总数用 `COUNT(*)`。
- **筛选项**：从汇总表取该客户区间内出现过的模型和分组。

## 6. 后端结构

| 文件 | 内容 |
|---|---|
| `migrations/251_usage_user_daily_rollups.sql`、`251_affiliate_ledger_user_created_index_notx.sql` | 第 1、2 节 |
| `migrations/usage_user_rollup_migration_test.go`（unit） | 校验关键 SQL 片段，参照 `group_usage_rollup_migration_test.go` |
| `internal/service/affiliate_team.go` | `AffiliateTeamService`、端口 `AffiliateTeamRepository`、错误常量、范围与日期校验、前 5 名合并、脱敏 |
| `internal/repository/affiliate_team_repo.go` | 第 5 节查询 |
| `internal/repository/dashboard_aggregation_repo.go` | 保留期追加删除旧汇总 |
| `internal/handler/affiliate_team_handler.go` | 参数解析，`response.Success` / `Paginated` / `ErrorFrom` |
| `internal/server/routes/user.go` | `/aff/team` 路由组加 Heavy；参数名统一用 `:user_id` |
| `internal/service/affiliate_detail.go`、`affiliate_service.go`、`repository/affiliate_repo.go` | `team_access`、`CountUplineCustomers` |
| 各层 `wire.go` | 新的 provider，用 `make generate` 生成 `wire_gen.go` |

## 7. 前端结构

| 文件 | 内容 |
|---|---|
| `src/api/affiliateTeam.ts`、`src/types/index.ts` | 四个接口及其类型（snake_case） |
| `src/composables/useAffiliateTeamQuery.ts` | URL query 白名单解析与同步（`start, end, tab, sort, order, page, size, q, model, group`），参照 `ChannelStatusV2View.vue:670` 的写法 |
| `src/views/user/AffiliateTeamView.vue` | 一个视图承载三个路由名（`AffiliateTeam` / `AffiliateTeamAgent` / `AffiliateTeamCustomer`），共用同一套骨架 |
| `src/components/user/affiliate-team/` | 见下 |
| `src/router/index.ts` | 三个路由，`requiresAuth`，懒加载 |
| `src/views/user/AffiliateView.vue` | 根据 `team_access` 显示入口（代理卡片右上角，或「成为代理」卡片内） |
| `src/i18n/locales/{zh,en}/dashboard.ts` | `affiliate.team.*` |

`components/user/affiliate-team/` 下的组件：

- `TeamToolbar`：面包屑、`DateRangePicker`、刷新按钮。
- `TeamSummaryStrip`：一张卡里横排指标。
- `TeamTrendChart`：vue-chartjs `Bar`，叠加一个 `type: 'line'` 的收益数据集，单 ¥ 轴。
- `TeamModelShare`：排名列表；`clickable` 属性只在客户视角开启。
- `TeamCustomerTable` / `TeamAgentTable` / `TeamRequestTable`：都用 `DataTable` 的 `serverSideSort`、`clickableRows`、`stickyFirstColumn`；URL 中的排序变化时用 `:key` 重新挂载。

其他约定：

- 金额一律用 `formatBalance`，Token 用 `formatCacheTokens` / `formatTokensK`，相对时间用 `formatRelativeTime`，禁止手写 `¥`。
- 403 或 404 时显示「客户不存在或无权查看」+ 返回链接。后端是唯一的权限判断，前端不另加路由守卫（与 `/affiliate` 现状一致）。

## 8. 性能验证

本机 `sub2api-local` 造数，脚本放在 `research/seed_team_perf.sql`：

- 2 万用户。最大的一级代理 B 有 5000 个直属客户和 5 个二级代理，每个二级代理 300 个客户。
- 2 千活跃用户，B 的团队里有 500 个活跃。
- `usage_logs`：90 天、约 540 万行（每个活跃用户每天 30 条），汇总表由触发器生成。
- ledger：B 约 100 万行（每个活跃客户每小时结算一次）。

测量方式：对四个接口分别用 90 天和 7 天区间做 `EXPLAIN (ANALYZE, BUFFERS)`，并用 curl 测耗时，目标 < 1s（AC9）。顺带记录迁移回填耗时，以及批量插入明细时触发器带来的额外开销。

如果收益查询不达标，备选方案是再加一张按受益人 × 来源 × 日的收益日汇总表，同样用触发器维护。只有实测不达标才加。

## 9. 风险与回滚

| 风险 | 处理 |
|---|---|
| 触发器异常导致汇总与明细偏离 | 错误被兜住，PG 日志里有 `WARNING`。修复：`SELECT rebuild_usage_user_daily_rollups(DATE '起', DATE '止');`。与并发写入同时执行时结果仍正确，集成测试 `TestUsageUserRollupRebuildWaitsForConcurrentInsert` 已验证 |
| 热点汇总行的锁竞争 | 一个批次一次 upsert，持锁只有毫秒级；按主键排序，避免死锁；只更新非索引列，走 HOT 更新 |
| 一号机迁移耗时 | 部署前查 `usage_logs` 行数，并在本机测同量级 |
| 二级代理改挂、降级后的展示 | 全部按快照处理；「历史」标记见第 3 节 |
| 回滚 | 迁移只向前：前端隐藏入口、后端摘掉路由即可下线功能。触发器留着无害；确实要移除时再写新迁移 `DROP TRIGGER` |
