# 合伙人 — 技术设计

需求见 `prd.md`，接口见 `contract.md`。新代码按 fork 惯例标注 `// CUSTOM(fork):`，新表用原生 SQL，写法与 `finance_expenses`、`group_model_rate_multipliers` 相同。

## 1. 总体思路

- **合伙人内部就是渠道经理**：合伙人账号的 `user_affiliates.agent_level` 固定为 1 并锁定，另有 `affiliate_partners` 表标记身份。
  - 绑定快照、团队消费页、渠道经理任命分销代理的链路都不用改；合伙人的客户绑定时照常记归属代理为合伙人、档位 1。
  - 发分润时识别合伙人：给合伙人的那份不发。
- **区域归属在读取时推导，不落库**：
  - 输入是「合伙人表 + 挂靠表 + 现有绑定快照」，由纯函数算出每个用户的（合伙人、所在线、计入起点）。
  - 挂靠永久、快照基本不变，所以不需要历史表。
- **利润复用财务引擎**：
  - 收入用 `financeReplays` 的逐用户逐日结果；成本、分润按计入起点精确到时刻另行查询。
  - 区域利润和财务页总利润用同一套口径。
- **结算单只在确认时落库**：
  - 草稿和当月预估每次实时计算，所以不需要定时任务。
  - 确认时把合计和明细快照成 JSON 冻结。

## 2. 数据变更（迁移 `264_affiliate_partners.sql`）

迁移编号取开工时的下一个空号：当前最大是 263，Seedance 改动没有新增迁移。

参照现有快照列的做法，ID 列一律不建外键：用户删除后仍保留 ID，结算按 `users` 判断账号是否存在。

```sql
CREATE TABLE IF NOT EXISTS affiliate_partners (
    user_id     BIGINT PRIMARY KEY,
    region_name VARCHAR(64) NOT NULL,
    created_by  BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),   -- 设立时刻 = 合伙人自己团队的计入起点
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 分成比例按月生效：某月比例 = effective_month <= 该月的最新一行
CREATE TABLE IF NOT EXISTS affiliate_partner_rates (
    partner_user_id BIGINT NOT NULL,
    effective_month DATE NOT NULL,                     -- 北京时间自然月 1 日
    share_rate      NUMERIC(6,4) NOT NULL CHECK (share_rate >= 0 AND share_rate <= 1),
    created_by      BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (partner_user_id, effective_month)
);

-- 挂靠：主键保证一个渠道经理永远只属于一个合伙人；不提供删除接口
CREATE TABLE IF NOT EXISTS affiliate_partner_agents (
    agent_user_id   BIGINT PRIMARY KEY,
    partner_user_id BIGINT NOT NULL,
    attached_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- 该渠道经理团队的计入起点
    attached_by     BIGINT NULL,                        -- 自动升级触发的自动挂靠为 NULL
    source          VARCHAR(16) NOT NULL                -- manual / promote
);
CREATE INDEX IF NOT EXISTS idx_affiliate_partner_agents_partner ON affiliate_partner_agents(partner_user_id);

CREATE TABLE IF NOT EXISTS affiliate_partner_adjustments (
    id              BIGSERIAL PRIMARY KEY,
    partner_user_id BIGINT NOT NULL,
    month           DATE NOT NULL,
    amount          NUMERIC(20,2) NOT NULL CHECK (amount <> 0),
    note            VARCHAR(500) NOT NULL,
    created_by      BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_affiliate_partner_adjustments_month ON affiliate_partner_adjustments(partner_user_id, month);

CREATE TABLE IF NOT EXISTS affiliate_partner_statements (
    id              BIGSERIAL PRIMARY KEY,
    partner_user_id BIGINT NOT NULL,
    month           DATE NOT NULL,
    status          VARCHAR(16) NOT NULL,              -- confirmed / paid
    share_rate      NUMERIC(6,4) NOT NULL,
    consumption     NUMERIC(20,8) NOT NULL,
    revenue         NUMERIC(20,8) NOT NULL,
    account_cost    NUMERIC(20,8) NOT NULL,
    commission      NUMERIC(20,8) NOT NULL,
    region_expense  NUMERIC(20,8) NOT NULL,
    adjustment      NUMERIC(20,8) NOT NULL,
    profit          NUMERIC(20,8) NOT NULL,
    loss_before     NUMERIC(20,8) NOT NULL,
    loss_after      NUMERIC(20,8) NOT NULL,
    share_base      NUMERIC(20,8) NOT NULL,
    share_amount    NUMERIC(20,2) NOT NULL,
    detail          JSONB NOT NULL,                    -- lines / cost_by_group / cost_by_model / adjustments / warnings
    confirmed_by    BIGINT NOT NULL,
    confirmed_at    TIMESTAMPTZ NOT NULL,
    paid_by         BIGINT NULL,
    paid_on         DATE NULL,
    paid_note       VARCHAR(500) NULL,
    UNIQUE (partner_user_id, month)
);

ALTER TABLE finance_expenses ADD COLUMN IF NOT EXISTS partner_user_id BIGINT NULL; -- 区域专项支出
```

- 不新增设置项。
- 回滚时删掉这 5 张表和这一列即可，旧代码不读它们。

## 3. 合伙人与挂靠（写路径）

### 3.1 设为合伙人：`AdminCreatePartner(userID, regionName, rate, adminID)`

在同一事务内完成。

**拒绝条件**
- 用户不存在或已删除。
- 用户已经是合伙人：返回 409 `PARTNER_EXISTS`。
- 用户已挂在某个合伙人名下：返回 409 `PARTNER_AGENT_ATTACHED`。
- 比例不在 0–100% 之间，或区域名为空、超过 64 字：返回 400。

**写入**
- `affiliate_partners`。
- `affiliate_partner_rates`：effective_month 取当月 1 日（北京时间）。
- `user_affiliates`：`agent_level=1`、`agent_level_locked=true`、`agent_level_source='manual'`、`agent_level_updated_by=admin`、`agent_level_updated_at=NOW()`。
  - 如果用户还没有 `user_affiliates` 行，先 `EnsureUserAffiliate`。
- 清掉该用户的代理相关缓存，用现有的 `invalidateAffiliateCaches`。

预览接口返回：
- 当前档位；
- 团队规模：`agent_owner_id = 他` 的人数、`agent_upline_id = 他` 的人数；
- 是否已是合伙人、是否已被挂靠；
- `current_region`：按 §4 当前所属的区域，只作提示。

### 3.2 挂靠：`AdminAttachAgent(partnerID, agentID, adminID)`

**拒绝条件**
- 合伙人不存在：404。
- 目标用户不存在、已删除，或当前不是 1 级：400 `PARTNER_AGENT_NOT_LEVEL1`。
- 目标本身是合伙人：409 `PARTNER_AGENT_IS_PARTNER`。
- 目标已被挂靠：409 `PARTNER_AGENT_ATTACHED`。

写入：`affiliate_partner_agents`，`source='manual'`。

预览接口额外返回目标「当前按快照所属的区域」（可能属于别的合伙人），只作提示，不阻止挂靠。

### 3.3 自动挂靠

把用户升为 1 级有两个入口：
- `SetAgentLevelManual`（管理员，level=1）。
- `PromoteAgentLevelAuto`（自动升级，target=1）。

两处在升级成功后，对每个升为 1 级、既没挂靠也不是合伙人的用户执行：
- 用 §4 的规则 2 解析他**当前**所属的区域。
- 解析到区域，就插入挂靠：`source='promote'`，`attached_by` 取管理员 ID，自动升级时为 NULL，`attached_at=NOW()`。
- 和档位变更放在同一事务里。`SetAgentLevelManual` 现在如果不在事务里，就改成 `WithTx` 包裹。

管理端批量设档位接口的返回里，加上自动挂靠的结果（`auto_attached` 列表），前端据此提示。

### 3.4 保护

- `AdminSetAgentLevel`：目标中有合伙人，整批拒绝，返回 409 `PARTNER_LEVEL_FIXED`。
  - 单个用户的 `PUT /admin/affiliates/users/:user_id` 改档位也走这里，需要确认。
  - 合伙人的专属比例、邀请码照常可改。
  - 自定义专属比例对合伙人没有意义，因为不发分润，但不需要额外拦截。
- `AppointAgentLevel`：任命人是合伙人时返回 403 `PARTNER_CANNOT_APPOINT`，任命和取消都拦。
- 展示层同步：
  - `GetAffiliateDetail` 对合伙人不返回 `Appoint`（`affiliate_detail.go:88`）；
  - 邀请列表的 `canAppoint` 对合伙人为 false（`affiliate_service.go:592`）。
- 自动升级：合伙人已锁定，本来就跳过。
- 没有撤销合伙人、解除挂靠、改挂的接口。

### 3.5 分润与奖励不发给合伙人

- `GetAffiliateNodes` 的 SQL 增加 `EXISTS(SELECT 1 FROM affiliate_partners p WHERE p.user_id = ua.user_id) AS partner`，`AffiliateNode` 增加 `Partner bool`。
- 新增 `nodeRewardable(nodes, id) = nodeActive(nodes, id) && !nodes[id].Partner`，用在三处，替换原来的 `nodeActive`：
  - `planAgentShares` 的最终过滤（`affiliate_distribution.go:101`）。
  - `planInviteShare`（:115）。
  - `ApplyFirstRechargeRewards` 发放邀请人固定奖励之前（`affiliate_first_recharge.go`）。
- **不要改 `Active` 的含义**：绑定快照解析依赖它来认定「有效代理」，合伙人必须仍被认定为有效的 1 级代理。
- 被丢弃的份额不再转给别人，与「受益人已删除」的处理一致。
- 消费结算的水位照常推进。
- 生效时刻是「设立之后的那一轮结算」：一轮结算窗口里如果包含设立前几分钟的消费，这部分也不再发给合伙人。

## 4. 区域归属解析（纯函数）

`ResolvePartnerMembers(partners, attachments, snapshots) map[userID]PartnerMember`

```text
PartnerMember { PartnerID int64; LineAgentID int64 /* 0 = 合伙人自己的团队 */; StartAt time.Time }

anchor(id):  // id 是否能决定区域
  id 是合伙人 P             → (P, line 0,  P.created_at)
  id 已挂靠到 P（attached）→ (P, line id, attached_at)
  否则                      → 无

解析用户 u（snapshot 字段：agent_owner_id、agent_upline_id、agent_owner_bound_at；为空时用 users.created_at）：
  1. anchor(u) 存在                    → 取它（自己就是合伙人或已挂靠渠道经理；StartAt 不再和 bound 取较晚）
  2. 依次检查 owner、upline：第一个 anchor 存在的 → (P, line, max(anchorStart, bound_at))
  3. 否则不属于任何区域
```

- 已删除的用户照常解析，因为历史收入、成本要算进去；列表展示时标记「已删除」。
- 管理员改了某个分销代理的上级（`SetAgentOwner`）后，只影响那个分销代理本人及其之后的新客户，与快照语义一致。
- 团队消费页的区域范围（§8）用同一份成员集合，但不受 StartAt 限制。

## 5. 区域月度利润（纯函数 + 取数）

### 5.1 取数：`PartnerRepository.LoadMonths(ctx, partnerIDs, fromMonth, toMonth)`

在同一个只读 REPEATABLE READ 事务里取以下数据。

1. 合伙人、比例、挂靠、调整项、已确认结算单。
2. 全部 `user_affiliates` 快照：`user_id, agent_level, agent_owner_id, agent_upline_id, agent_owner_bound_at`，加上 `users.created_at / deleted_at / email`。
3. 收入重放所需数据：复用 `FinanceRepository.Load` 的同一批 SQL，包括用户、订单、ledger、事件、固定成本账号 ID、支出。
   - 给 Load 加一个选项跳过 `usage_logs` 成本扫描，因为合伙人成本在下面另查，避免重复扫描。
   - 两边共用同一个事务。
4. 用 §4 解析出成员和起点之后，再查以下三项，起点精确到时刻：
   - **成本与消耗**，按成员 × 北京日 × 分组 × 模型分组：

     ```sql
     WITH m(user_id, start_at) AS (SELECT * FROM unnest($1::bigint[], $2::timestamptz[]))
     SELECT l.user_id, (l.created_at AT TIME ZONE 'Asia/Shanghai')::date, COALESCE(l.group_id,0), COALESCE(g.name,'无分组'),
            COALESCE(NULLIF(BTRIM(l.requested_model),''), l.model),
            SUM(CASE WHEN l.account_id = ANY($fixed) THEN 0 ELSE COALESCE(l.account_stats_cost,l.total_cost)*COALESCE(l.account_rate_multiplier,1) END),
            SUM(CASE WHEN l.billing_type = 0 THEN l.actual_cost ELSE 0 END)
     FROM usage_logs l JOIN m ON m.user_id = l.user_id
     LEFT JOIN groups g ON g.id = l.group_id
     WHERE l.created_at >= GREATEST(m.start_at, $from) AND l.created_at < $to
     GROUP BY 1,2,3,4,5
     ```

     成本公式、模型口径与财务页 breakdown 一致：全部计费类型都计成本，固定成本账号按 0 计。
   - **起点当天的消耗**：`start_at` 落在区间内的成员，在起点所在北京日的 `billing_type=0` `actual_cost`，分别算全天合计和 `>= start_at` 的部分。
   - **分润**：`user_affiliate_ledger` 中 `action='accrue'`，`COALESCE(reward_type,'invite_percent') IN ('agent_direct','agent_diff','invite_percent','invite_fixed')`，`source_user_id` 是成员且 `created_at >= start_at`，并且落在区间内。
5. 区域专项支出：`finance_expenses.partner_user_id = P` 且未删除。按 `FinanceAmortization` 逐日摊销，取落在月内的部分。

`FinanceLedger` 增加 `SourceUserID`，财务页的 Load 也一起读出这一列，`finance_repo.go:99` 的 SQL 要补上。

### 5.2 计算：`ComputePartnerMonth(in) PartnerMonthResult`

逐个成员、逐个北京日计算（日期 ≥ 起点所在日，且在当月内）：

- 收入：`rev = replay.Days[d].RevenuePaid + RevenueCommission`。
  - 起点当天乘以 `after ÷ total`；当天总消耗为 0 时记 0。
  - 起点之后的日子全额计入，包括退款日的负收入。
- 成本、消耗：§5.1 的查询结果本身已经从起点开始。
- 分润：§5.1 的查询结果。
- 汇总到合伙人和所在线（LineAgentID）：
  - 每条线输出 `consumption`、`revenue`、`account_cost`、`commission`、`profit`（线利润 = 收入 − 成本 − 分润，不含专项支出和调整项）、`members`。
  - 线 0 显示为「合伙人直属团队」。
- 成本拆分：按分组、按模型输出 `consumption` 和 `account_cost`，各按成本倒序排列。
- 合计：
  - `profit = revenue − account_cost − commission − region_expense + adjustment`
  - 若 `profit <= 0`：`share_base = 0`，`loss_after = loss_before − profit`。
  - 若 `profit > 0`：`share_base = max(0, profit − loss_before)`，`loss_after = max(0, loss_before − profit)`。
  - `share_amount = round(share_base × rate, 2)`，四舍五入到分。
  - 比例取 `effective_month <= 当月` 的最新一行。
- 精度用 `decimal`，接口输出与财务页一致：字符串，最多 8 位小数，分成金额保留 2 位。

### 5.3 月份序列

- 从设立当月到当前月，逐月计算。
- 已确认的月份直接读快照，并取它的 `loss_after` 作为下一个月的 `loss_before`。
- 从第一个未确认的月份开始，顺序向后计算。
- 一次请求只调用一次 `LoadMonths`，取第一个未确认月份到当月这段区间。

**状态**

| 状态 | 条件 |
|---|---|
| `confirmed` / `paid` | 已落库 |
| `draft` | 已结束但未确认 |
| `current` | 当月 |

**告警（warnings）**
- 区间内有成员的请求走了固定成本账号：「N 条请求走固定成本账号，成本按 0 计」。
- 起点当天的明细已被保留期清理。
- 来自财务 Load 的订阅订单等提示。

## 6. 结算单操作

| 操作 | 规则 |
|---|---|
| 确认 `POST .../statements/:month/confirm` | 事务内 `SELECT ... FOR UPDATE` 锁住 `affiliate_partners` 行。月份必须早于当月；前一个月必须已确认（设立当月除外）；本月不能已确认。计算该月后落库，`detail` 存 lines、cost_by_group、cost_by_model、adjustments、warnings |
| 标记已付 `POST .../statements/:month/pay` | 必须是 confirmed 才能操作。`paid_on` 默认今天（北京时间），`paid_note` 必填，最长 500 字；之后不能改 |
| 调整项 `POST /adjustments`、`DELETE /adjustments/:id` | 所在月份不能已确认。金额不能为 0，两位小数；说明必填 |
| 改比例 `PUT /partners/:id` | 区域名立即生效；比例写入 `effective_month = 下月 1 日` 的行（upsert）。返回当前比例和下月比例 |
| 导出 `GET .../statements/:month/export` | CSV（UTF-8 BOM）：合计、各线、成本拆分、调整项 |

## 7. 财务页联动

- `finance_expenses` 的输入输出增加 `partner_user_id`，输出加 `partner_region_name`。保存时校验合伙人是否存在。
  - 全站汇总里，支出仍然只计一次。
- `Summary`：
  - granularity=month 时，每行加 `partner_share` 和 `net_profit`。`partner_share` 是所有合伙人该月分成之和，已确认的取快照，其余实时计算。
  - 每行再加 `partner_share_estimated`：当月有未确认的结算单时为 true。
  - 无论什么粒度，totals 都加 `partner_share`、`net_profit`，取范围涉及的整月分成之和。
  - 再加 `partner_share_scope: "whole_months"`，前端据此标注「按整月计入」。
  - 没有合伙人时，这几个值全部为 "0"。
- 依赖方向：
  - `FinanceService` 通过接口 `PartnerShareProvider` 拿分成，由 `PartnerService` 实现。
  - `PartnerService` 依赖 `FinanceRepository`，不依赖 `FinanceService`，所以不会成环。

## 8. 团队消费页的区域视角

现有团队页按单个 viewer 划定范围，见 `affiliate_team_repo.go` 的 `scopeSQL`。

**区域范围**
- 设 S = {合伙人} ∪ 挂靠渠道经理。
- 区域成员 = `ua.agent_owner_id ∈ S OR ua.agent_upline_id ∈ S OR ua.user_id ∈ 挂靠渠道经理`。
- 这与 §4 的成员集合一致，只是不受 StartAt 限制。

**区域视角的接口**
- `overview`：汇总、趋势、模型占比，复用现有聚合，只换范围条件。
- `customers`：全区域客户，每行带「所属渠道经理 / 分销代理」。
- `lines`：返回各渠道经理，以及合伙人直属团队这一行。每行带团队消费、客户数、活跃客户数、请求数。
- 返回结构尽量复用 `AffiliateTeamOverview`、`AffiliateTeamCustomerEntry`。
- 「我的收益」列在区域视角下不返回，因为合伙人没有代理分润。

**逐级查看**
- `/user/aff/partner/team/agents/:agent_id/...` 直接调现有代理端的 service 函数：`Overview`、`ListCustomers`、`ListAgents`、`ListRequests`，传入 `viewerID = agent_id`。
- 现有函数自带脱敏和隐私规则。
- 准入：agent 必须挂在当前合伙人名下，否则返回 403。

**合伙人自己的团队**
- 合伙人内部是 1 级，现有 `/user/aff/team/*` 直接可用（viewer = 合伙人）。
- 前端点「合伙人直属团队」这一行时，跳到这组接口。

**准入和限流**
- 与代理团队页相同：受功能开关约束，用重型限流。
- 非合伙人返回 403。

## 9. 其他读接口

**`GET /user/aff`**
- `AffiliateDetail` 增加 `partner`。不是合伙人时为 null，是合伙人时为 `{region_name, share_rate, next_share_rate, next_rate_month, since}`。
- 合伙人的 `team_rate_percent` 等代理比例字段照常返回，由前端决定不显示。

**管理端代理列表 `ListUsers`**
- 每行增加 `is_partner` 和 `partner: {user_id, region_name} | null`，即该用户按 §4 所属的区域。
- 增加查询参数 `partner_id`，按区域筛选。
- 按页内用户计算，页内用户的快照锚点取自合伙人表和挂靠表。

## 10. 性能

- 每次请求重放全部用户的全部历史，与财务页相同，七号机现有数据量可以忽略。
- 合伙人端接口挂重型限流。
- 本期不做缓存。

## 11. 测试要点

**单测**
- §4 解析：各种拓扑、起点取较晚值、自己是锚点。
- §5.2：起点当天按比例折算、退款负收入、亏损链（连续亏损、部分抵扣、完全抵扣）、比例按月生效、四舍五入到分。
- §3.5：`planAgentShares` 遇到合伙人受益人时丢弃份额，分销代理照常拿 r2；邀请奖励和首充固定奖励不发给合伙人。
- §3.4 的拦截。

**集成测试**（testcontainers）
- 迁移。
- LoadMonths 的成本 SQL 只取起点之后的数据；起点当天的消耗。
- 确认时加锁，并发确认同一月只成功一次。
- 自动挂靠和档位变更在同一事务里。

**e2e**：见 `task-test.md`。

## 12. 上线

1. 部署后，在后台把 75 号设为「三科大」、62 号设为「重大」合伙人，比例 40%。
2. 按用户的安排挂靠渠道经理。
3. 用户自己处理 vm2api：移出固定成本列表并设倍率。合伙人只看挂靠之后的数据，历史数据不受影响。
4. 回滚：退回旧镜像即可，旧代码忽略新表和新列。合伙人设立时改过的档位（62 号被改为 1 级并锁定）需要手动改回。
