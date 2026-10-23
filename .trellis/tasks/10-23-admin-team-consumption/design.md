# 设计：管理端团队消费页

需求见 `prd.md`，界面见 `ui.md`。代理端的设计见 `../10-22-agent-team-consumption/design.md`，本文只写新增与改动。

## 0. 决策摘要

| 问题 | 决定 | 理由 |
|---|---|---|
| 代理视角怎么实现 | 复用 `AffiliateTeamService` 全部查询，把被查看的代理当作 `viewerID` 传入；只在「受众」上分叉：管理员不脱敏、不受功能开关约束、无团队时返回 404、搜索支持邮箱模糊和 ID、overview 多返回 `subject` | 数字与代理端逐项一致（AC4/AC5）靠「同一段代码」保证，而不是靠两份 SQL 对齐 |
| 汇总页的分润从哪来 | 新增汇总表 `affiliate_agent_reward_daily_rollups`（受益人 × 来源 × 北京时间日），由 `user_affiliate_ledger` 上的语句级触发器维护，迁移 252 | 汇总页要一次聚合**所有**代理在 90 天内的分润。流水条数 = 结算次数 × 活跃客户，结算间隔可配到 5 分钟，流水无保留期；按 2 千活跃、每小时结算估算 90 天约 200 万行，5 分钟结算再乘 12。汇总表的行数只与「有分润的客户 × 天数」有关（约 27 万行 / 90 天） |
| 汇总表为什么要带来源维度 | 代理端「收益」只计**来源仍在团队范围内**的分润（代理端 PRD R3）。汇总页每行要与之相等，需要按来源过滤 | 只按受益人 × 日汇总做不到这一点；带上来源后行数仍可控 |
| 代理端查询是否改用新汇总表 | 本任务不改 | 代理端已按 2 万用户实测达标；改动会扩大回归面。若以后结算间隔调小导致代理端变慢，再切换（第 9 节） |
| 管理端接口放在哪个 handler | 在现有 `handler.AffiliateTeamHandler` 上加管理端方法，路由注册在管理端路由组 | `handler` 包引用了 `handler/admin`，后者不能反向引用参数解析函数；已有先例：管理端路由使用 `h.ChannelMonitorV2` |
| 限流 | 只用管理端路由组已有的 `Global()`，不加 `Heavy()` | 现有管理端统计接口（仪表盘、使用记录）都不加 `Heavy`；管理员频繁下钻时 `Heavy` 的 60 次/分钟会误伤 |
| 前端怎么复用 | 把 `AffiliateTeamView.vue` 的页面主体抽成共享组件，用 provide/inject 注入「受众上下文」（接口、路由、文案）；代理端与管理端各自一个薄视图 | 一份页面逻辑，两种入口；代理端行为不变 |

## 1. 迁移 `252_affiliate_agent_reward_daily_rollups.sql`（事务内）

### 1.1 表

```sql
CREATE TABLE IF NOT EXISTS affiliate_agent_reward_daily_rollups (
    user_id        BIGINT        NOT NULL,           -- 受益代理（ledger.user_id）
    source_user_id BIGINT        NOT NULL DEFAULT 0, -- 来源客户；0 = 来源账号已被物理删除（ledger 里为 NULL）
    bucket_date    DATE          NOT NULL,           -- 入账时间所在的北京时间自然日
    amount         DECIMAL(20,8) NOT NULL DEFAULT 0, -- 与 ledger.amount 同精度
    entries        BIGINT        NOT NULL DEFAULT 0, -- 流水条数；扣减到 0 时删除该行
    PRIMARY KEY (user_id, source_user_id, bucket_date)
);
CREATE INDEX IF NOT EXISTS idx_affiliate_agent_reward_daily_rollups_bucket_date
    ON affiliate_agent_reward_daily_rollups (bucket_date);
```

- 只收 `reward_type IN ('agent_direct', 'agent_diff')`。
- 主键以受益人开头，服务「某代理 × 日期区间」；`bucket_date` 索引服务「全部代理 × 日期区间」。
- 不设保留期：流水永久保存，汇总表体量很小。

### 1.2 触发器

按 `database-guidelines.md`「触发器维护的汇总表」的约定，与 251 同形：

- 一个函数 `apply_affiliate_agent_reward_daily_rollup()`，三个语句级触发器 `user_affiliate_ledger_reward_rollup_{insert,delete,update}`（`REFERENCING NEW/OLD TABLE`）。所有写入都走「先 `GROUP BY` 主键、再 `ORDER BY` 主键」的 upsert，扣减即累加负数，并发批次按相同顺序加锁；扣减后 `entries <= 0` 的行删除。
- 函数体包 `EXCEPTION WHEN OTHERS THEN RAISE WARNING`：汇总出错不能让结算事务失败。
- UPDATE 按键合并「旧行扣减、新行累加」，只写净变化不为 0 的键（不依赖按 `id` 关联）。现有 UPDATE 只有两处：解冻改 `frozen_until`、提现回写快照列（`affiliate_repo.go:377`、`:579`），净变化为 0，不写汇总（集成测试用 `ctid` 验证）。来源用户被物理删除时外键 `ON DELETE SET NULL` 改 `source_user_id`，金额从原来源挪到来源 0。
- 受益人被物理删除时外键 `ON DELETE CASCADE` 删除流水，由 DELETE 触发器扣减。

### 1.3 回填与修复

- `rebuild_affiliate_agent_reward_daily_rollups(from_date DATE, to_date DATE)`：先 `LOCK TABLE user_affiliate_ledger IN SHARE MODE`，再删区间、`INSERT ... SELECT ... ON CONFLICT DO UPDATE` 累加。
  - 加锁的原因：251 的修复函数不加锁，删区间与重建是两条语句、两个快照，若并发批次恰好在两步之间提交一个汇总里还没有的新键，会被重复累加。分润流水的写入是后台结算和少量用户操作，修复期间让它们等待可以接受。
  - 并发写入在修复期间等待到修复事务结束（集成测试覆盖「未提交的新键」场景）。
- 迁移内先建触发器，再 `SELECT rebuild_affiliate_agent_reward_daily_rollups(DATE '1970-01-01', DATE '9999-12-30')` 全量回填。回填耗时与代理分润流水条数成正比；部署前与 251 一起评估（第 8 节）。

## 2. 汇总页查询（新增仓储方法）

记号：`ua` = user_affiliates；渠道客户 = `ua.agent_owner_id IS NOT NULL`；代理 X 的团队 = `agent_owner_id = X OR agent_upline_id = X`（与代理端一致）。

### 2.1 有团队的人（汇总页的行）

```sql
owners AS (
    SELECT user_id FROM user_affiliates WHERE agent_level IN (1, 2)
    UNION SELECT agent_owner_id  FROM user_affiliates WHERE agent_owner_id  IS NOT NULL
    UNION SELECT agent_upline_id FROM user_affiliates WHERE agent_upline_id IS NOT NULL
)
```

等级分组：`COALESCE(ua.agent_level, 3)`，1 一级、2 二级、3 已降级。与代理端准入（`resolveViewer`）同一条件。

### 2.2 `GetPlatformAggregates(rng)`

一次扫描用户日汇总，`GROUPING SETS ((bucket_date), (model), (user_id))`，与代理端 `GetAggregates` 同形，只是范围为「全部用户」并带一个「是否渠道客户」标记：

- 渠道客户的行 → 汇总条消费、活跃客户、趋势消费、模型分布。
- 全部用户的按用户行 → 全站余额消费（占比分母）。
- 渠道客户总数：`COUNT(*) FROM user_affiliates WHERE agent_owner_id IS NOT NULL`（含已删除账号，与代理端客户数口径一致）。
- 分润与分润趋势：分润汇总表按 `bucket_date` 分组，不限来源（时间段内入账的全部代理分润）。
- 等级计数：对 2.1 的集合按等级 `COUNT`。

### 2.3 `ListTeamOwners(rng, query)`

```sql
usage AS (  -- 区间内每个用户的余额消费
    SELECT user_id, SUM(actual_cost) AS cost
    FROM usage_user_daily_rollups WHERE bucket_date BETWEEN $d1 AND $d2 GROUP BY user_id
), membership AS (  -- (代理, 客户)：客户分别算进归属代理和上方一级的团队
    SELECT agent_owner_id AS agent_id, user_id FROM user_affiliates WHERE agent_owner_id IS NOT NULL
    UNION
    SELECT agent_upline_id, user_id FROM user_affiliates WHERE agent_upline_id IS NOT NULL
), per_agent AS (
    SELECT m.agent_id, COUNT(*) AS customers,
           COUNT(*) FILTER (WHERE u.cost > 0) AS active_customers,
           COALESCE(SUM(u.cost), 0) AS team_cost
    FROM membership m LEFT JOIN usage u ON u.user_id = m.user_id GROUP BY m.agent_id
), earn AS (  -- 只计来源仍在其团队内的分润，与代理端「收益」一致
    SELECT r.user_id AS agent_id, SUM(r.amount) AS earnings
    FROM affiliate_agent_reward_daily_rollups r
    JOIN user_affiliates s ON s.user_id = r.source_user_id
    WHERE r.bucket_date BETWEEN $d1 AND $d2
      AND (s.agent_owner_id = r.user_id OR s.agent_upline_id = r.user_id)
    GROUP BY r.user_id
), channel_total AS (
    SELECT COALESCE(SUM(u.cost), 0) AS cost
    FROM user_affiliates ua JOIN usage u ON u.user_id = ua.user_id WHERE ua.agent_owner_id IS NOT NULL
)
SELECT ..., COUNT(*) OVER () AS total
FROM owners o
JOIN user_affiliates ua ON ua.user_id = o.user_id  -- 等级
LEFT JOIN users u ...; LEFT JOIN per_agent ...; LEFT JOIN earn ...;
LEFT JOIN users up ON up.id = ua.agent_owner_id AND ua.agent_level = 2  -- 二级的上级
WHERE <等级筛选> AND <搜索>
ORDER BY <白名单列> <方向> NULLS LAST, o.user_id DESC
LIMIT $n OFFSET $o
```

- 同一客户的归属代理与上方一级正常不会是同一人；仍用 `UNION` 去重，保证与代理端「按客户计数」在异常数据下也一致。
- 搜索（管理端）：`u.username ILIKE %q% OR u.email ILIKE %q%`（转义通配符，排除已删除账号），`q` 为纯数字时另加 `o.user_id = q`（已删除账号也能按 ID 找到）。
- 排序白名单：`team_cost`（默认）、`earnings`、`active_customers`。
- `cost_share = team_cost / channel_total.cost`。

### 2.4 `GetViewerSubject(userID)`

返回被查看代理的展示信息：用户（原文邮箱；不存在的 ID 按已删除）、上级（仅二级，取 `agent_owner_id`，上级账号不存在时按已删除）、累计分润（分润汇总表 `SUM(amount) WHERE user_id = X`，等于代理管理列表的「累计分润」）。

## 3. 服务层

### 3.1 受众

`affiliate_team.go` 内部方法增加受众参数 `affiliateTeamAudience`（`agent` / `admin`）：

| 环节 | 代理端（现状不变） | 管理端 |
|---|---|---|
| 准入 | 功能关闭或无团队 → `AFFILIATE_TEAM_FORBIDDEN`（403） | 不看功能开关；无团队 → `AFFILIATE_TEAM_NOT_FOUND`（404） |
| 用户展示 | `maskEmail` | 原文邮箱；已删除仍只留 ID 与 `deleted` |
| 客户搜索 | 昵称模糊、邮箱完整匹配 | 昵称 / 邮箱模糊、纯数字时按 ID 精确 |
| overview | — | 多返回 `subject`（2.4），各层都带，用于面包屑与元信息行 |

公开方法：现有 `Overview / ListCustomers / ListAgents / ListRequests` 签名不变；新增 `AdminViewerOverview / AdminViewerCustomers / AdminViewerAgents / AdminViewerRequests`（同参数，`viewerID` 为被查看的代理），以及 `AdminOverview(rng)`、`AdminListOwners(rng, query)`。

`AffiliateTeamListQuery` 增加 `Level string`（`""` 全部 / `"1"` / `"2"` / `"downgraded"`）与 `AdminSearch bool`。

### 3.2 新类型（JSON snake_case）

```go
type AffiliateTeamViewerSubject struct {
    User             AffiliateTeamUser  `json:"user"`
    Upline           *AffiliateTeamUser `json:"upline"`            // 只对当前二级代理有值
    LifetimeEarnings float64            `json:"lifetime_earnings"`
}
// 等级与降级标记沿用 overview 已有的 viewer.agent_level / viewer.downgraded，不在 subject 里重复。
// AffiliateTeamOverview 增加 Subject *AffiliateTeamViewerSubject `json:"subject,omitempty"`（代理端为 nil，不出现在响应里）

type AffiliateTeamPlatformOverview struct {
    SettleIntervalMinutes int                         `json:"settle_interval_minutes"`
    Counts                AffiliateTeamOwnerCounts    `json:"counts"`  // all, level1, level2, downgraded
    Summary               AffiliateTeamPlatformSummary `json:"summary"` // cost, platform_cost, earnings, active_customers, customers
    Trend                 []AffiliateTeamTrendPoint   `json:"trend"`   // 区间内每天一条，补 0
    Models                []AffiliateTeamModelShare   `json:"models"`  // 前 5 + 其他
}

type AffiliateTeamOwnerEntry struct {
    User            AffiliateTeamUser  `json:"user"`
    AgentLevel      int                `json:"agent_level"`
    Upline          *AffiliateTeamUser `json:"upline"`
    Customers       int64              `json:"customers"`
    ActiveCustomers int64              `json:"active_customers"`
    TeamCost        float64            `json:"team_cost"`
    Earnings        float64            `json:"earnings"`
    CostShare       float64            `json:"cost_share"`
}
```

趋势补零、前 5 合并沿用 `fillAffiliateTeamTrend`、`topAffiliateTeamModels`。

## 4. 接口

管理端路由组（JWT + 管理员 + `Global()` 限流），`registerAffiliateRoutes` 内：

| 方法与路径 | 说明 |
|---|---|
| `GET /api/v1/admin/affiliates/team/overview?start_date&end_date` | 汇总页：`AffiliateTeamPlatformOverview` |
| `GET /api/v1/admin/affiliates/team/agents?start_date&end_date&level&search&sort_by&sort_order&page&page_size` | 汇总页列表：`Paginated<AffiliateTeamOwnerEntry>` |
| `GET /api/v1/admin/affiliates/team/:viewer_id/overview?…[&agent_id\|&user_id]` | 代理视角，参数与代理端 `overview` 相同 |
| `GET /api/v1/admin/affiliates/team/:viewer_id/customers` | 同代理端 `customers` |
| `GET /api/v1/admin/affiliates/team/:viewer_id/agents` | 同代理端 `agents` |
| `GET /api/v1/admin/affiliates/team/:viewer_id/customers/:user_id/requests` | 同代理端请求明细 |

- 静态段 `overview` / `agents` 与参数段 `:viewer_id` 同层，Gin 1.9 支持（现有 `users/lookup` 与 `users/:user_id` 即如此）。
- 参数解析复用 `affiliate_team_handler.go` 的 `parseOptionalID`、`parseTeamListQuery`；`:viewer_id` 非正整数返回 400。
- 日期、分页、排序、错误码与代理端一致。

## 5. 前端

### 5.1 共享页面

- `components/user/affiliate-team/teamContext.ts`：`AffiliateTeamContext`（`audience`、四个接口函数、路由构造 `root / agent(id) / customer(id)`、未找到时的返回链接）与 `provideAffiliateTeamContext / useAffiliateTeamContext`；未提供时回落代理端上下文。
- `components/user/affiliate-team/TeamConsumptionPage.vue`：由 `AffiliateTeamView.vue` 的页面主体抽出，`view`（team / agent / customer）与 `subjectId` 改为 props，路由、接口、受众相关文案全部取自上下文；管理端时渲染 `subject` 元信息行与「使用记录」链接。
- `views/user/AffiliateTeamView.vue` 变成薄视图：按路由名算出 `view` / `subjectId`，提供代理端上下文。
- 受众相关组件：`TeamCustomerTable`、`TeamAgentTable` 的行链接改由上下文生成；`TeamSummaryStrip`、`TeamTrendChart` 的「收益 / 分润」、`TeamUserCell` 的已删除显示 ID、空态的「去邀请」，按 `audience` 选文案。
- 文案：管理端专用的放在 `admin.affiliates.team.*`（中英）。替换代理端文案用映射表 `ADMIN_AFFILIATE_TEAM_LABELS`（`components/admin/affiliate-team/adminTeamContext.ts`，代理端 key → 管理端 key），共享组件统一调用 `useTeamLabel()` 返回的 `label(key, params)`，避免在组件里写 `if admin`；替换文案可引用上下文参数 `{name}`（被查看代理的名字，由页面注入）。
- 上下文的路由除三层外还有管理端才提供的 `viewer(id)`（上级链接）、`usage(userId, range)`（使用记录）、`summary(range)`（汇总页）；页面按是否提供决定是否渲染对应链接。
- 进度条抽成 `TeamProgressBar.vue`，代理端页面与汇总页共用。

### 5.2 管理端视图与路由

| 路由 | 名称 | 组件 |
|---|---|---|
| `/admin/affiliates/team` | `AdminAffiliateTeam` | `views/admin/affiliates/AdminAffiliateTeamView.vue`（汇总页） |
| `/admin/affiliates/team/:viewerId` | `AdminAffiliateTeamViewer` | `views/admin/affiliates/AdminAffiliateTeamViewerView.vue`（提供管理端上下文 + `TeamConsumptionPage`，以 `viewerId` 为 key：换到另一位代理时整页重建，三层之间沿用同一实例） |
| `/admin/affiliates/team/:viewerId/agents/:id` | `AdminAffiliateTeamViewerAgent` | 同上 |
| `/admin/affiliates/team/:viewerId/customers/:id` | `AdminAffiliateTeamViewerCustomer` | 同上 |

- 四个路由 `requiresAuth`、`requiresAdmin`，`titleKey: nav.affiliateTeam`、`descriptionKey: admin.affiliates.teamDescription`。
- 汇总页 URL 状态：`start, end, level, q, sort, order, page, size`，解析与生成放 `composables/useAdminAffiliateTeamQuery.ts`，日期函数复用 `useAffiliateTeamQuery.ts` 的导出。在汇总页的 `onBeforeRouteLeave` 里把 `from.query` 记在模块级变量（不用 watch：路由切换时旧组件的 watch 可能先拿到新页面的 query），代理视角面包屑首项用它回到原来的筛选（日期取当前页的）。
- 汇总页组件：`TeamToolbar`（无面包屑时只显示右侧日期与刷新）、`TeamSummaryStrip` 新增 `platform` 视图（消费 + 全站占比、分润 + 团队消费占比、活跃客户）、`TeamTrendChart`、`TeamModelShare`、等级标签、`SearchInput`、新表 `AdminTeamOwnerTable.vue`、`Pagination`。
- 接口：`api/admin/affiliateTeam.ts`（汇总两个 + `viewer(id)` 返回与代理端同签名的四个函数）；类型加在 `types/index.ts`。
- 侧边栏：「邀请返利」分组加「团队消费」（`nav.affiliateTeam`），排在代理管理之后。
- 代理管理「⋯」菜单：`agent_level !== 3 || owned_customer_count > 0` 时显示「团队消费」，跳 `AdminAffiliateTeamViewer`。
- 「使用记录」链接：`{ name: 'AdminUsage', query: { user_id, start_date, end_date } }`。

## 6. 测试

| 层 | 内容 |
|---|---|
| 迁移 unit | 252 的关键 SQL 片段（只收两种分润类型、北京时间日界、`EXCEPTION`、`IS DISTINCT FROM` 过滤、回填调用） |
| 触发器 integration | 插入、批量同键、删除到 0、受益人级联删除、来源 `SET NULL` 挪到 0、解冻 UPDATE 不改变汇总、跨北京时间零点、回填与重建结果和流水重算一致、重建与并发插入同时执行 |
| repository integration | `ListTeamOwners`：收录（一级 / 二级 / 已降级 / 已删除，普通用户不收）、等级筛选与计数、搜索（邮箱片段、昵称片段、ID、通配符转义）、排序；每行团队消费 / 活跃 / 客户 / 分润与该代理 `GetAggregates` 的摘要相等（含改挂走的来源被排除）；`GetPlatformAggregates`：去重消费、全站分母、趋势、模型、分润不限来源 |
| service unit | 管理端受众：不脱敏、不看功能开关、无团队 404、`subject` 只在管理端出现、代理端响应不变；`AdminSearch` 与 `Level` 透传；平台总览补零与前 5 合并 |
| handler / routes | 管理端路由注册在管理员组下、`:viewer_id` 校验、未登录 401 与非管理员 403 |
| 前端 vitest | 共享页在两种受众下的文案、路由、元信息行、使用记录链接；汇总页标签计数与筛选、搜索、排序、行点击、面包屑带回 query；侧边栏与「⋯」菜单显隐；代理端原有用例全部保留 |
| e2e（`sub2api-local`） | p6 增加 `TADM`（接口对 SQL、对代理端接口逐项一致、越权与 401/403）与界面用例（汇总 → 代理 → 二级 → 客户 → 使用记录，返回恢复状态，截图）；原有 206 条全部通过 |

## 7. 性能验证

在本机按代理端的造数规模扩展：2 万用户中 10 个一级代理、各 5 个二级，2 千活跃全部是渠道客户；`usage_logs` 90 天；分润流水按每小时结算生成（约 200 万行）。测汇总页两个接口在 7 / 90 天下的 `EXPLAIN ANALYZE` 与 curl 耗时（目标 < 1s），以及迁移 252 的回填耗时、批量插入流水时触发器的额外开销，写入 `research/perf.md`。

## 8. 部署与回滚

- 迁移 251、252 将随本任务一起首次部署到一号机。部署前查 `usage_logs`（余额消费）与 `user_affiliate_ledger`（代理分润）行数，估算两次回填耗时。
- 回滚：迁移只向前；下线时前端隐藏入口、后端摘路由即可。触发器留着无害，确需移除时写新迁移 `DROP TRIGGER`。
- 汇总与流水偏离时：`SELECT rebuild_affiliate_agent_reward_daily_rollups(DATE '起', DATE '止');`

## 9. 以后可选

结算间隔调到很小、大代理的流水很多时，代理端的收益查询（摘要、客户列表、下级代理列表）可以改读分润汇总表，口径不变。
