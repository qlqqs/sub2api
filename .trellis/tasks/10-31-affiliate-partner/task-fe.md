# FE：前端任务书（claude opus-5.5 high）

- 工作目录：`/home/qlqq/workspace/sub2api/frontend`。
- 开始前按 `implement.md` 第 2 节「共同规则」读文档。
- 接口以 `contract.md` 为准，页面需求以 `prd.md` 第 5 节为准。
- 先按契约写页面和单测。BE 完成后，在本机环境联调并截图。

## 现状速览

**用户端**
- `src/views/user/AffiliateView.vue`（`/affiliate`）
- `src/views/user/AffiliateTeamView.vue`（`/affiliate/team`、`/affiliate/team/agents/:id`、`/affiliate/team/customers/:id`）
- `src/components/user/AffiliateSectionTabs.vue`：邀请 / 团队消费页签，由 `team_access` 控制
- `src/components/user/affiliate-team/*`：`TeamConsumptionPage.vue` 是主体，数据源通过 `teamContext.ts` 注入
- `src/api/affiliateTeam.ts`、`src/api/user.ts`（:185-223）
- `src/composables/useAffiliateTeamQuery.ts`

**管理端**
- `src/views/admin/affiliates/` 下：`AdminAffiliateAgentsView.vue`（代理管理）、`AdminAffiliateTeamViewerView.vue`、`adminTeamContext.ts`（以 viewer 视角复用团队页，可参考）
- `AffiliateUserLookup.vue`（用户选择器）

**财务**
- `src/views/admin/finance/AdminFinanceView.vue`
- `src/components/admin/finance/*`：`FinanceProfitTable`、`FinanceOverviewTab`、`FinanceExpenseDialog`、`FinanceExpensesTab`
- `src/api/admin/finance.ts`：`normalize*` 系列

**路由与导航**
- 路由：`src/router/index.ts`，用户端在 :291 起，管理端在 :725 起
- 默认主题导航：`src/components/layout/AppSidebar.vue`，用户端在 :755，管理端分组在 :832-846
- qx 主题导航：`src/themes/qx/shell/useQxNav.ts`，用户端在 :149，管理端在 :194-205
- 两套主题都要处理。

**i18n**
- `src/i18n/locales/{zh,en}/`，跟随现有分文件方式。
- 档位名称沿用「渠道经理 / 分销代理」，新角色叫「合伙人」（英文 Partner）。

**通用约定**
- 金额都是字符串，用现有财务的 `FinanceMoney` 或同类格式化函数展示，不在前端做加减。
- 文案短、直接，不堆砌说明和警告。

---

## A. 用户端（合伙人）

1. **API**：在 `src/api/` 新建 `affiliatePartner.ts`。
   - 封装 contract §3.2 / §3.3 的全部接口，以及 `GET /user/aff` 新增的 `partner` 字段类型。
   - 写 normalize 函数，参考 `api/admin/finance.ts` 的写法。
2. **邀请页 `AffiliateView.vue`**：当 `partner` 不为空时：
   - 显示合伙人卡片：区域名、分成比例（有下月新比例时显示「下月起 X%」）、设立时间、一句口径说明「按区域利润分成，不拿代理分润」。
   - 隐藏代理比例卡片，以及邀请列表里的「任命 / 取消」按钮。
   - 邀请码和链接照常显示。
3. **页签 `AffiliateSectionTabs.vue`**：合伙人显示三个页签：邀请 / 团队消费 / 利润分成。
   - 团队消费指向区域视角。
4. **利润分成**：新路由 `/affiliate/partner`，明细为 `/affiliate/partner/:month`。
   - 顶部：本月预估卡片（收入、上游成本、分润、专项支出、调整项、利润、预计分成）和累计亏损。
   - 按渠道经理拆分的表格。「合伙人直属团队」一行可以点进 `/affiliate/team`；渠道经理行可以点进区域视角的该渠道经理页。
   - 历次结算单列表：月份、状态（预估 / 待确认 / 已确认 / 已付）、利润、比例、分成、打款日期。
   - 明细页：合计、累计亏损期初 / 期末、各线、成本按分组 / 按模型、调整项（说明和金额）、warnings。
   - **不出现任何账号或供应商信息**（契约本来也不返回）。
5. **区域团队消费**
   - 新路由：
     - `/affiliate/region`：区域汇总
     - `/affiliate/region/customers/:id`：区域内某个客户的请求明细
     - `/affiliate/region/agents/:agentId`：以挂靠的渠道经理为视角
     - `/affiliate/region/agents/:agentId/agents/:id`、`/affiliate/region/agents/:agentId/customers/:id`：该渠道经理名下的分销代理、客户
   - 复用 `TeamConsumptionPage.vue`：
     - 新增 `partnerTeamContext`，写法参考 `adminTeamContext.ts`，数据源映射到 `/user/aff/partner/team/*` 和 `/user/aff/partner/team/agents/:agent_id/*`。
     - 区域汇总页的「下级代理」表换成「渠道经理」表，数据来自 `team/lines`；「直属团队」一行跳到 `/affiliate/team`。
     - 客户表增加「所属渠道经理 / 归属代理」两列，不显示「我的收益」。
   - 面包屑或返回路径：区域 → 渠道经理 → 分销代理 → 客户。
6. **导航**：
   - 两套主题的用户端导航入口仍然只有 `/affiliate`，合伙人页面通过页签进入。
   - 新路由的 meta（title、description）沿用 affiliate。

## B. 管理端

1. **API**：新建 `src/api/admin/affiliatePartners.ts`，封装 contract §1 的全部接口和 §2 的新字段。
2. **合伙人列表页** `/admin/affiliates/partners`
   - 新建 `AdminAffiliatePartnersView.vue`，放在 `views/admin/affiliates/`。
   - 表格列：账号、区域名、比例（含「下月起 X%」）、挂靠数、成员数、本月预估分成、累计亏损、待确认月数。
   - 「设为合伙人」对话框：
     1. 用 `AffiliateUserLookup` 选人，调预览接口，显示当前档位、团队规模，以及当前所属区域（`current_region` 不为空时提示「此人目前在 XX 区域，设立后他本人和他以后的客户归新区域」）。
     2. 填写区域名和比例（默认 40）。
     3. 二次确认，文案「设为合伙人后不能撤销」。
3. **合伙人详情页** `/admin/affiliates/partners/:userId`
   - 页签「概览 / 渠道经理 / 结算单」。
   - **概览**：当月数字、比例。可以编辑区域名和比例，保存后提示「新比例从下月起生效」。
   - **渠道经理**：
     - 列表：账号、挂靠时间、来源（手动 / 升级自动）、团队规模、本月数字。
     - 「挂靠渠道经理」对话框：选人，预览团队规模和当前所属区域，二次确认，文案「挂靠后不能移出或改挂」。
   - **结算单**：
     - 月份列表。
     - 明细抽屉：合计、各线、成本拆分、调整项、warnings。
     - 操作：
       - 草稿可以添加或删除调整项（金额正负、说明必填），可以「确认」（二次确认，确认后金额冻结）。
       - 已确认的可以「标记已付」（日期和备注）。
       - 「导出 CSV」。
     - 按钮的可用状态按契约的状态和顺序规则控制。后端报错时，直接 toast 后端返回的 message。
4. **代理列表 `AdminAffiliateAgentsView.vue`**
   - 增加「所属合伙人」列，以及按合伙人筛选（下拉选项来自合伙人列表）。
   - 合伙人行显示「合伙人」标识，档位编辑和批量设档位里跳过或禁用合伙人。
   - 批量或单个设为渠道经理后，如果响应里 `auto_attached` 不为空，toast 提示「已自动挂到 XX（区域）名下」。
5. **导航**：两套主题的管理端「代理」分组，在「代理管理」之后加入口「合伙人」。
   - 新增 i18n key，例如 `nav.affiliatePartners`。
   - `themes/qx/shell/__tests__/useQxNav.drift.spec.ts` 会检查两套导航是否一致，两边要同时加。

## C. 财务页

1. **支出对话框 `FinanceExpenseDialog.vue`**
   - 增加可选的「区域」下拉，选项来自合伙人列表，可以留空。
   - 支出列表显示区域名。
2. **月度利润表 `FinanceProfitTable.vue`**：月粒度时增加「合伙人分成」「平台净利润」两列。
   - 带 `partner_share_estimated` 的行，在分成旁标「预估」。
   - 日粒度时不显示这两列。
3. **总览卡片**：增加「合伙人分成」「平台净利润」，旁注「按整月计入」（`partner_share_scope`）。
4. `api/admin/finance.ts` 的 normalize 补上新字段。只加字段，不改现有字段的处理。

## D. 测试与验收

- 为新增的 API normalize、`partnerTeamContext`、合伙人列表和详情页的关键交互写 vitest 单测，覆盖：
  - 状态驱动的按钮可用性；
  - 二次确认；
  - 自动挂靠提示。
- 现有单测必须全部通过，尤其是 `AffiliateView`、`SettingsView`、财务相关的 spec。
- 检查命令：

  ```bash
  npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run
  git -C .. status --short     # 确认 pnpm-lock.yaml 没有改动
  ```

- BE 完成后联调：
  1. `cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d`。
  2. 用管理员把一个测试用户设为合伙人，挂一个渠道经理，造少量消费。
  3. 用 `e2e/ui/shots.sh` 给以下页面截图，路径写进完成报告：
     - 合伙人端：邀请页、利润分成、明细、区域团队消费三层
     - 管理端：合伙人列表、详情三个页签、代理列表、财务页
- 截图要注意：桌面宽度和移动宽度各看一次；两套主题下入口都在。

## 约定变更

1. **需要后端补：区域 `GET /user/aff/partner/team/overview` 支持 `user_id`**（FE 2026-10-31）
   - 契约 §3.3 写「查询参数与现有 `/user/aff/team/*` 一致」，现有 overview 用 `user_id` 返回单个客户的 `customer` 主体及该客户的 summary / trend / models / filters。
   - 前端区域客户页 `/affiliate/region/customers/:id` 按此调用 `team/overview?user_id=` + `team/customers/:id/requests`。
   - 当前后端实现（`PartnerService.TeamOverview`）忽略 `user_id`、不填 `customer`：该页会显示整个区域的汇总，面包屑客户名一直是占位条。需要后端按现有代理端语义补上（客户须在区域内，否则 403）。
2. **前端路由（不涉及接口）**：合伙人直属团队不用 `/affiliate/team`，放在 `/affiliate/region/direct`（及 `/direct/agents/:id`、`/direct/customers/:id`）。
   - 数据仍走契约指定的 `/user/aff/team/*`。
   - 原因：`/affiliate/team` 是代理端根页，顶部是两个分区页签、没有回区域的面包屑；放在 region 下，面包屑统一为「区域团队 / 直属团队 / …」，与「区域团队 / 渠道经理 / …」一致。
   - 合伙人直接访问 `/affiliate/team` 仍可用（代理端页面）。

## 完成报告（第一阶段）

### 改动

**API 与类型**
- 新增 `src/api/affiliatePartner.ts`：契约 §3.2 / §3.3 全部用户端接口；`PartnerUserRef`、`PartnerMonthTotals`、`PartnerStatementDetail` 等类型和 normalize（金额保持字符串，缺字段补 "0"/[]）；区域与挂靠渠道经理两套团队页数据源（`partnerRegionTeamAPI`、`partnerAgentTeamAPI(agentId)`）、`listTeamLines`。区域客户的 `owner` / `line_agent` 转成团队页的用户形状。
- 新增 `src/api/admin/affiliatePartners.ts`：契约 §1 全部接口（列表、预览、设立、详情、修改、挂靠预览/挂靠、结算单列表/明细/确认/已付、调整项增删、CSV 导出）及 normalize；挂到 `adminAPI.affiliatePartners`。
- `api/admin/affiliates.ts`：代理列表项 `is_partner`、`partner`；`partner_id` 筛选；设档位接口返回 `auto_attached`。
- `api/admin/finance.ts`：只加字段——summary totals / 月度行的 `partner_share`、`net_profit`、`partner_share_estimated`、`partner_share_scope`；支出 `partner_user_id`、`partner_region_name`。`utils/financeExpense.ts` 表单带 `partner_user_id`。
- `api/user.ts` 的 `getAffiliateDetail` 读出 `partner`；`types/index.ts` 加 `AffiliatePartnerInfo`、`UserAffiliateDetail.partner`、`AffiliateTeamCustomer.owner/line_agent`（可选）。
- `utils/affiliatePartner.ts`：比例格式化 / 百分比输入转换、结算单按钮规则 `partnerStatementActions`（按月份顺序确认、当月不可确认、已确认可付、未确认可调）、调整项金额校验、结算单线的跳转路由。

**用户端**
- `AffiliateView.vue`：`partner` 不为空时显示合伙人卡片（区域名、比例、下月新比例、设立时间、「按区域利润分成，不拿代理分润」），不显示代理比例卡片；任命列随 `appoint=null` 隐藏；邀请码照常。
- `AffiliateSectionTabs.vue`：`partner` 时三个页签，「团队消费」指向 `/affiliate/region`，新增「利润分成」。
- 利润分成 `/affiliate/partner`（`AffiliatePartnerView.vue`）：区域信息、累计亏损、本月预估、按渠道经理拆分（可点进直属团队 / 渠道经理）、历次结算单。明细 `/affiliate/partner/:month`（`AffiliatePartnerStatementView.vue`）：合计、期初/期末亏损、各线、成本按分组/模型、调整项、warnings。共用组件在 `components/user/affiliate-partner/`（管理端对话框也用）。
- 区域团队消费 `/affiliate/region/*`（`AffiliateRegionView.vue` + `PartnerTeamScope.vue` + `partnerTeamContext.ts`），复用 `TeamConsumptionPage`：
  - 上下文新增 `audience: 'partner'`、`region.listLines`、`rootLabel`、`routes.direct`；区域视角下「下级代理」表换成 `TeamLineTable`（渠道经理线 + 直属团队一行），客户表加「所属渠道经理 / 归属代理」两列、去掉收益，汇总条和趋势图不显示收益。
  - 三个视角：区域（`/affiliate/region`、`/customers/:id`）、直属团队（`/direct…`）、挂靠渠道经理（`/agents/:agentId`、`/agents/:agentId/agents/:id`、`/agents/:agentId/customers/:id`）。下钻后面包屑为「区域团队 / 渠道经理 / 分销代理 / 客户」；渠道经理名取自 `team/lines`。
  - 原有代理端、管理端行为不变（`isAdmin` 判断改为看是否有 `routes.summary`，两者等价）。
- 新路由 meta 沿用 affiliate 的 title / description；用户端导航入口不变。

**管理端**
- `/admin/affiliates/partners`（`AdminAffiliatePartnersView.vue`）：列表列为账号、区域名、比例（含下月起）、挂靠数、成员数、本月预估分成、累计亏损、待确认月数；「设为合伙人」对话框 `PartnerCreateDialog`：选人 → 预览（档位、团队、当前所属区域提示、不能设立的原因）→ 区域名、比例（默认 40）→ 二次确认「设为合伙人后不能撤销」。
- `/admin/affiliates/partners/:userId`（`AdminAffiliatePartnerDetailView.vue`，页签记在 `?tab=`）：
  - 概览：本月数字、基本信息；改区域名和比例（比例框预填下月比例，只提交改动字段，改比例提示「新比例从下月起生效」）。
  - 渠道经理：列表（挂靠时间、来源、团队规模、本月数字，账号可点到管理端团队视角）；`PartnerAttachDialog` 选人 → 预览 → 二次确认「挂靠后不能移出或改挂」。
  - 结算单：月份列表，点开 `PartnerStatementDialog`：明细、调整项增删（未确认月份）、确认（二次确认，按规则禁用并说明原因）、标记已付（日期默认今天、备注必填）、导出 CSV。后端报错直接 toast 后端 message。
- 代理列表：「所属合伙人」列、按合伙人筛选（下拉来自合伙人列表）、合伙人行显示「合伙人」标识；编辑对话框里合伙人的档位禁用；批量设档位跳过合伙人并提示；单个 / 批量 / 新增设为渠道经理后按 `auto_attached` toast「#ID 已自动挂到 XX（区域）名下」。
- 两套主题导航：`AppSidebar.vue`、`themes/qx/shell/useQxNav.ts` 在「代理管理」后加「合伙人」（`nav.affiliatePartners`）。

**财务页**
- 支出对话框加可选「区域」下拉（合伙人列表，留空为公共支出；编辑时不在列表里的区域也能显示）；支出列表加「区域」列。
- 月度利润表月粒度加「合伙人分成」「平台净利润」两列，`partner_share_estimated` 的行标「预估」；日粒度不显示。
- 总览卡片加「合伙人分成」（旁注「按整月计入」，有预估时加「预估」）和「平台净利润」。

**i18n**：新文件 `locales/{zh,en}/partner.ts`（`partner.*`）、`locales/{zh,en}/admin/partners.ts`（`admin.partners.*`）；`common.ts` 加 `nav.affiliatePartners`；`admin/finance.ts` 加分成、净利润、区域字段。

### 测试

```
cd frontend
npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run
→ 全部通过：Test Files 456 passed，Tests 3602 passed
git -C .. status --short   → 无 pnpm-lock.yaml / pnpm-workspace.yaml 改动
```

新增 / 扩展的单测：
- `api/__tests__/affiliatePartner.spec.ts`、`api/__tests__/admin.affiliatePartners.spec.ts`、`api/__tests__/admin.finance.spec.ts`（新字段）：normalize 与请求路径、参数。
- `utils/__tests__/affiliatePartner.spec.ts`：按钮可用性规则（顺序确认、当月不可确认、已确认可付、已付无操作）、比例与调整项金额校验。
- `components/admin/affiliate-partner/__tests__/PartnerDialogs.spec.ts`：设立 / 挂靠的预览、当前区域提示、不能操作时禁用、二次确认后才调用、后端错误 toast。
- `components/admin/affiliate-partner/__tests__/PartnerStatementDialog.spec.ts`：草稿 / 后续草稿 / 当月 / 已确认 / 已付的按钮状态，确认二次确认，标记已付，调整项增删。
- `views/admin/affiliates/__tests__/AdminAffiliatePartnersView.spec.ts`：列表、详情改名改比例（只提交改动字段）、渠道经理页签、结算单页签。
- `views/admin/affiliates/__tests__/AdminAffiliateAgentsView.spec.ts`：合伙人标识与列、按合伙人筛选、档位锁定、批量跳过、自动挂靠提示。
- `views/user/__tests__/AffiliateRegionView.spec.ts`：区域视角（三页签、无收益、归属列、渠道经理线、下钻与面包屑、直属团队走代理端接口、403 提示）。
- `views/user/__tests__/AffiliatePartnerView.spec.ts`、`AffiliateView.spec.ts`、`AffiliateSectionTabs.spec.ts`、`components/admin/finance/__tests__/FinanceProfitTable.spec.ts`、`FinanceExpenseDialog.spec.ts`、`router/__tests__/affiliate-partner-route.spec.ts`、`themes/qx/shell/__tests__/useQxNav.spec.ts`（加新菜单项）。

### 遗留与需要配合

- **后端**：区域 `team/overview` 补 `user_id`（见「约定变更」第 1 条），否则区域客户页显示不对。
- 与后端代码核对过的字段（只读）：用户端 / 管理端合伙人 JSON 结构、区域客户的 `owner` / `line_agent`、`team/lines` 返回 `{items}`，与前端一致。
- `useQxNav.drift.spec.ts` 等跑测时有 `ECONNREFUSED 127.0.0.1:3000` 的 stderr（导入阶段的请求），测试通过。这两个文件本次只加了一行菜单配置，没有新的请求；没有对照改动前的基线确认它是否原本就有。代理列表 spec 原先因未 mock 合伙人接口产生的同类 stderr 已补 mock 消除。
- 第二阶段（本机环境联调、截图）未做。QA 可用的选择器：`affiliate-partner-card`、`affiliate-section-tab-partner`、`team-line-table`、`team-line-direct`、`partner-statement-<YYYY-MM>`、`partners-create-open`、`partner-create-*`、`partner-attach-*`、`partner-tab-<overview|agents|statements>`、`partner-statement-confirm|pay|export`、`partner-adjustment-*`、`agents-partner-filter`、`agent-partner-badge`、`finance-col-partner-share`、`finance-expense-partner`。

## 完成报告（第二阶段）

时间：2026-11-01（北京时间）凌晨。本机环境 `~/workspace/sub2api-local`，完整构建三次（`./build.sh && docker compose up -d`，镜像 `c22f82ada-dirty`），最后一次包含下文全部前端修正。

### 与后端约定变更的核对

| 后端变更 | 前端情况 |
|---|---|
| `net_profit` = 原 profit − `partner_share`，无合伙人时分成 "0" | 前端只展示，不运算；无需改动 |
| 比例最多 4 位小数，超出 `PARTNER_INVALID` | 前端输入百分比、最多 2 位小数，换算后最多 4 位小数（`partnerRateFromPercent`），不会超出 |
| 区域成员含合伙人本人；团队页看全部历史；日期参数兼容 | 前端发 `start_date` / `end_date`；区域客户表里合伙人本人一行显示「合伙人直属团队」，归属代理为「-」 |
| 业务错误走标准包，toast `message` | 合伙人相关调用都 toast `message`；代理列表沿用 `admin.affiliates.errors` 命名空间，`PARTNER_LEVEL_FIXED` 等没有翻译键时回落到后端 `message` |
| 补充 1：区域 `team/overview?user_id=` | 已联调：区域客户页显示该客户的主体、汇总、趋势、模型和请求明细；面包屑正常 |

### 联调数据（只在本机）

- 造数脚本：`/tmp/fe-partner-seed/{seed,adjust,extra,final}.py`（用 `e2e/lib.py`；state 在 `/tmp/fe-partner-seed/state`，执行完恢复了返利相关设置）。
- 账号统一为 `e2e-204649-<角色>@e2e.test`，密码为 e2e 用户密码：
  - 合伙人 P **3151**「联调区域」，40%
  - 挂靠渠道经理 X 3152
    - X 的客户 X1 3154
    - X 名下的分销代理 XD 3155，其客户 XD1 3156、XD2 3157
  - 合伙人直属客户 P1 3153
  - 自动挂靠：Y 3158（P 邀请的普通用户，升为渠道经理时自动挂靠）
  - 界面里新设的合伙人：Z 3159「UI 联调区域」35%、Z2「UI 联调区域二」
  - 界面里挂靠的渠道经理：W 3160、W2
- 消费全部经网关真实请求产生：两个分组（联调-稳定 / 联调-高速）、两个模型（claude-sonnet-4-5 / claude-haiku-4-5）。结算出分润后，结算单的成本拆分里有分组和模型两个维度。
- 为了有已结束的月份，只在本机库里把 3151 和 3159 的 `affiliate_partners.created_at` 与首个比例月改到 2026-09-01，用量没动。
- 另外加了 10 月调整项 +20、区域专项支出 ¥30（10-20 起摊 30 天）。
- 截图过程中在界面里做了这些写操作：确认 9 月并标记已付、10 月加调整项 −3.5 后确认、挂靠 W / W2、新设 Z / Z2。

### 本阶段修的前端问题

1. **表头不对齐**：`.table th`（`style.css` 组件类，选择器优先级 0,1,1）压过 `text-right` 工具类，右对齐的表头实际左对齐。合伙人的几张表（各线、成本拆分、结算单列表、调整项、详情页渠道经理表）改用 `!text-right`。
   - 现有财务页的表格同样受影响（历史问题），本次没有改全局样式。
2. **利润分成页卡片之间没有间距**：`FinanceLoadState` 把内容包在一个 div 里，外层的 `space-y-4` 不起作用。改为内层容器加 `space-y-4`；详情页同理。
3. **区域「渠道经理」表首列表头**：「所属渠道经理」改为「渠道经理」。
4. **财务总览卡片**：8 张卡片在 xl 宽度下挤在一行，大额负数会折成两行。改为每行 4 张（`md:grid-cols-4`）。
5. **表格标题**：标题与表头间距加 `pb-3`。
6. **移动端本月分成金额块**：对齐改为 `sm:text-right`（窄屏左对齐）。

每次修改后都重跑了检查：

```
cd frontend && npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run
→ 通过：Test Files 456 passed，Tests 3602 passed（/tmp/fe-p2-check2.log）
git -C .. status --short → 只有 frontend/ 改动，pnpm-lock.yaml / pnpm-workspace.yaml 未变
```

### 截图

截图脚本 `/tmp/fe-partner-ui/{partner,admin,theme}.cjs`，由 `/tmp/fe-partner-ui/run.sh` 加载 Playwright 依赖和中文字体运行，写法同 `e2e/ui/shots.cjs`，放在仓库外。脚本逐层真实点击，每一步都有断言，全部通过。运行中没有浏览器控制台错误。最终截图在 **`/tmp/fe-partner-ui/shots/final/`**：

- **合伙人端** `partner-desktop/`（1440 宽）、`partner-mobile/`（390 宽）：
  - `01-invite`：合伙人卡片、三个页签、无任命列
  - `02-profit-share`
  - `03-statement-2026-10` / `04-statement-2026-11`：明细，含调整项、成本拆分、warnings
  - 区域团队消费逐层点击：`05-region-customers` → `06-region-lines` → `07-manager`（渠道经理老王）→ `08-distributor`（分销代理小周）→ `09-customer-requests`（客户阿杰请求明细，面包屑「区域团队 / 渠道经理老王 / 分销代理小周 / 客户阿杰」）
  - `10-region-customer`：区域视角直接点进客户，用 `overview?user_id=`
  - `11-direct-team`：直属团队
- **管理端** `admin-desktop/`：
  - `01-partners-list`
  - 设为合伙人：`02-create-preview`（含「此人目前在 联调区域…」提示）→ `03-create-confirm`（不能撤销）→ `04-partners-list-after-create`
  - 详情页签：`05-detail-overview` / `06-detail-agents`（含「升级自动」来源）
  - 挂靠：`07-attach-preview` → `08-attach-confirm`（不能移出或改挂）→ `09-detail-agents-after-attach`
  - 结算单：`10-detail-statements`
    - `11-statement-draft-confirmable`：草稿可确认
    - `12-statement-current`：当月，按钮禁用，提示「当月不能确认」
    - `13-statement-confirm-dialog` → `14-statement-confirmed`：已确认，出现「标记已付」，调整项表单消失
    - `15-statement-pay-form`
    - `16-detail-statements-after`
    - `17-statement-draft-blocked`：前一月未确认，按钮禁用，提示「请先确认之前的月份」
  - 代理列表：`18-agents`（「合伙人」标识、所属合伙人列）/ `19-agents-filter-partner`（按「联调区域」筛选）
- **财务** `admin-finance/`（2026-09-01 至 2026-11-01，按月）：
  - `01-finance-overview`：合伙人分成「按整月计入 · 预估」、平台净利润
  - `02-finance-profit`：月度表两列，「预估」标记
  - `03-finance-expenses`：区域列
  - `04-finance-expense-dialog`：区域下拉
  - 另有 `admin-desktop/20–23` 为同一组页面在上一个镜像上的截图，供对照。
- **管理端移动宽度** `admin-mobile/`：合伙人列表（卡片布局）、结算单页签、代理列表。
- **两套主题的入口**：
  - qx 主题（线上默认）：管理端侧栏「邀请返利 › 合伙人」见 `admin-desktop/04-partners-list-after-create`；合伙人三个页签见 `partner-*/01-invite`。
  - 默认主题 `theme-default/`：桌面和移动的管理端侧栏 `*-admin-nav`，合伙人邀请页、区域团队消费、利润分成。
    - qx 主题是编译期开关，所以默认主题用临时开发服务器检查：Vite 只监听 127.0.0.1:3300，代理到 28080；临时配置 `/tmp/fe-default-theme/vite.default.config.mts` 在内存里把 `enabled` 改为 false，不改仓库文件。用完已停止。

### 后端观察（低优先级，未改 backend/）

1. **warnings 总在**：每张结算单（包括没有任何用量的月份）和财务页都带「余额消耗按北京日重放；历史用量或兑换流水被清理后无法完整还原收入」。看起来是财务 Load 的固定提示，信息量低，可以考虑只在真有被清理的数据时返回。
2. **结算单各线列出月后才挂靠的渠道经理**：W、W2 在 11-01 挂靠，却出现在 9 月 / 10 月的结算单里（成员 0，金额 0）。不影响金额；如果希望历史月只列当时已挂靠的线，需要后端按 `attached_at` 过滤。

### 遗留

- 无已知的前端阻塞。截图和造数脚本放在 `/tmp`，QA 写 `e2e/ui/partner_ui.cjs` 时可以参考。
- 本机库里保留了上述联调数据和三个合伙人（3151、3159、Z2）；没有做任何全局清理。

## 补充（TAUI-01 修复）

管理端「邀请返利」分组的菜单顺序改为：代理管理、团队消费、合伙人、邀请记录、返利记录、转账记录。两套主题都已改（`AppSidebar.vue`、`themes/qx/shell/useQxNav.ts`），对应断言也已更新：`useQxNav.spec.ts` 的完整顺序，`AppSidebar.spec.ts` 新增前四项顺序。lint、typecheck、test:run 全部通过（456 files / 3602 tests），锁文件未改动；本机环境未重新构建。
