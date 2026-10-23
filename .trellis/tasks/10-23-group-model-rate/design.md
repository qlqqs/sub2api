# 设计：分组内单个模型倍率

需求见 `prd.md`。文中路径相对 `backend/internal/` 或 `frontend/src/`，行号为 2026-10-23 的位置。

## 0. 决策摘要

| 问题 | 决定 | 理由 |
|---|---|---|
| 存在哪里 | fork 自有新表 `group_model_rate_multipliers`（迁移 253），原生 SQL 仓储；**不改 `groups` 表和 Ent schema** | fork 从未改过 `ent/`；`schema/group.go`、认证缓存快照、`group_repo`、`api_key_repo` 是上游改动最频繁的文件（4 个月内 25–39 次提交），加字段每次合并都冲突。上游的 `user_group_rate_multipliers` 也是同样做法 |
| 请求路径怎么读 | 进程内整表快照（表很小），30 秒过期 + singleflight；管理端保存后立即清本进程缓存 | 与用户专属倍率缓存同样的时效；每次请求只是一次 map 查找 + 模式匹配；不需要改认证缓存快照、不需要升快照版本 |
| 倍率在哪一步替换 | 新增一个解析函数，只替换「基础倍率」（原来的「专属 ?? 分组」那一项），之后照旧调 `computePeakAwareMultipliers` | 高峰倍率、生图 / 视频独立倍率、渠道分时倍率的叠加规则都在这一项之外，自动保持不变 |
| 如何避免专属倍率缓存被污染 | 命中模型倍率时**不调用** `ResolveUserGroupRateMultiplier`；没命中时按原样调用 | 该缓存会把传入的默认值一起缓存在 `user:group` 键下（`service/user_group_rate_resolver.go:79-84`），传模型倍率进去会污染同一用户的其他模型 |
| 按哪个模型名匹配 | 计费模型（`recordUsageBillingModel` 与 OpenAI 的计费模型），与「分组逐模型定价」`matchGroupModelPricing` 同一口径；名字先 `normalizeChannelPricingModelName` | 同一个模型名决定价格和倍率，管理员只需理解一套规则 |
| 匹配规则 | 精确优先，其次前缀最长的 `*` 通配 | 存储是一行一条规则、无顺序；「最长前缀」结果确定，不依赖录入顺序 |
| 模型广场比例列 | 前端按「本站价 ÷ 官方价」逐行算比例，再按汇率换成折扣（如 `-95%`），不直接显示倍率 | 渠道价卡、分组价卡和官方价不同时，直接显示倍率会误导；按价格算在任何配置下都是真的。¥ 与 $ 不同币种，必须经汇率才能算折扣 |
| 汇率从哪来 | fork 服务定时从公开接口获取（Frankfurter / ECB 为首选，fawazahmed0 currency-api 的 jsDelivr 与 Cloudflare 镜像为备选），存设置表；默认 6.8 兜底 | 三个来源 2026-10-23 本机实测可用、免 key；ECB 是权威参考价；多来源降低单点不可用的影响 |
| 用户端费用明细换成 ¥ | 每项 × (实扣 ÷ 原始费用) | 明细合计严格等于实扣；高峰倍率、Free Fast 等让实扣 ≠ 原始 × 倍率的情况也对得上 |
| 用户端隐藏方式 | 不改共享组件 `GroupBadge` / `GroupOptionItem`，只改用户端调用处不再传倍率与高峰参数；新增 fork 组件 `PlazaPriceLink` | 管理端大量使用这两个组件，保持其行为；共享组件不改也减少上游冲突 |
| 管理端配置 UI | fork 组件 `GroupModelRatesEditor.vue` 嵌入分组新建 / 编辑弹窗；单独接口保存，在分组保存成功后调用 | 与分组倍率放在一起最直观；接口独立使分组主表单与上游保持一致 |

## 1. 数据层

### 1.1 迁移 `253_group_model_rate_multipliers.sql`

```sql
CREATE TABLE IF NOT EXISTS group_model_rate_multipliers (
    group_id        BIGINT        NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    model_pattern   VARCHAR(255)  NOT NULL,   -- 已归一化（小写、claude- 前缀的 . 换成 -），可带末尾 *
    rate_multiplier DECIMAL(10,4) NOT NULL CHECK (rate_multiplier > 0),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, model_pattern)
);
```

- 分组是软删除：已删除分组的规则留在表里无害（分组不会再被使用），硬删除时级联删除。
- 迁移号 253：runner 按文件名排序和记录，与上游将来的同号文件不冲突；加 `migrations/group_model_rate_migration_test.go`（unit，校验关键 SQL）。

### 1.2 仓储 `repository/group_model_rate_repo.go`

- 接口在 `service/group_model_rate.go`：`ListAll(ctx)`、`ListByGroup(ctx, groupID)`、`ReplaceForGroup(ctx, groupID, rules)`（事务内先删后插）、`CopyGroup(ctx, fromID, toID)`。
- 原生 SQL，参数化；`ListAll` 用 `JOIN groups ... WHERE groups.deleted_at IS NULL` 过滤已删分组。
- `repository/wire.go` 注册（一行）。

## 2. 倍率解析 `service/group_model_rate.go`

```go
type GroupModelRateRule struct { Pattern string; Rate float64 }

// GroupModelRateService：整表快照缓存 + 管理端读写 + 校验。
func (s *GroupModelRateService) Match(ctx context.Context, groupID int64, model string) (float64, bool)
func (s *GroupModelRateService) ListForGroup / ReplaceForGroup / CopyGroup ...

// resolveBaseRate 是所有计费路径共用的「基础倍率」：
//   命中模型倍率 → 模型倍率（不查专属倍率，避免污染缓存）
//   否则 → resolveUser(ctx, userID, groupID, group.RateMultiplier)（原逻辑）
func resolveBaseRate(ctx, rates *GroupModelRateService, group *Group, userID int64, model string,
    resolveUser func(ctx context.Context, userID, groupID int64, def float64) float64) float64
```

- 快照：`map[groupID][]rule`，精确规则放 map、通配规则按前缀长度降序；30 秒过期，singleflight 加载；加载失败时沿用旧快照并记 warn，从未加载成功则视为无规则（退回原行为）。
- `ReplaceForGroup` 校验：模型名去空格、归一化后非空；`*` 只允许在末尾且前面至少一个字符；同组不重复；倍率 > 0 且 ≤ 10000（与 `DECIMAL(10,4)` 相容）；单组最多 200 条。保存后清空本进程快照。
- 注入：两个网关服务、批量生图服务、预扣冻结服务、利润预览、模型广场服务通过 wire provider 用 setter 注入（参照 `service/wire.go:279-281`），不改构造函数签名；未注入时 `Match` 返回未命中，行为与现在相同（老测试不受影响）。

## 3. 计费接入点

| 路径 | 位置 | 改法 |
|---|---|---|
| Anthropic / Gemini / Antigravity 网关 | `service/gateway_usage_billing.go:791-792` `recordUsageMultipliers` | 先算 `billingModel` 再算倍率；`recordUsageMultipliers` 增加 `model` 参数，内部改用 `resolveBaseRate` |
| 同上：Gemini 缓存固定费报价 | `gateway_usage_billing.go:982-994` `QuoteUsageCost` | 同上（它复用 `recordUsageMultipliers`，调整顺序） |
| 同上：按响应模型重新计费 | `gateway_usage_billing.go:808-822` | 用响应模型重新解析倍率再算；`responseModelBillingAdoptable` 增加「实扣金额不高于基线」条件（原有的 TotalCost 条件保留） |
| OpenAI 兼容网关（含 WS、Grok 媒体 / 音频、Embeddings、alpha search） | `service/openai_gateway_usage.go:198-226` | 先确定计费模型，再用 `resolveBaseRate` 得到 `baseMultiplier`；按响应模型重算（`:286`）与 Free Fast 重算（`:305-317`）同上处理 |
| 批量生图 | `service/batch_image_public.go:1016-1031` `resolvePricingSnapshot` | 未开启生图独立倍率时，用 `resolveBaseRate(req.Model)` 代替「专属 ?? 分组」 |
| 预扣冻结 | `service/billing_inflight_reservation.go:491-503, 646-650` | 倍率按候选模型逐个算；「免费分组直接跳过」的判断改为同时没有任何 > 0 的模型倍率时才跳过 |
| 实时语音会话 | `service/openai_live.go:844` | 现在固定为 1、不计费，不改 |

- 生图 / 视频：`resolveImageRateMultiplier` / `resolveVideoRateMultiplier`（`service/image_billing_multiplier.go`）在开启独立倍率时直接返回独立倍率、不看基础倍率，因此自然满足「独立倍率优先」；未开启时它们用基础倍率，自然跟随模型倍率。
- `usage_logs.rate_multiplier` 写入的就是上面算出的倍率，无需额外改动。
- 每个接入点改动都在上游文件里，尽量控制在几行；新逻辑都在 fork 文件中。

## 4. 模型广场

### 4.1 后端
- `PlazaModel` 增加 `RateMultiplier float64`：对所有访客相同的公开倍率 = 模型倍率 ?? 分组倍率；`ModelPlazaService.ListGroups` 填充。
- `handler/model_plaza_handler.go`：模型 DTO 增加
  - `rate_multiplier`：公开倍率；
  - `user_rate_multiplier`（可空）：登录用户在该分组有专属倍率、且该模型**没有**单独倍率时给出。
- 分组级的 `rate_multiplier`、`user_rate_multiplier` 字段保留（排序与旧前端兼容），前端不再显示。

### 4.2 前端 `components/modelPlaza/`
- `api/modelPlaza.ts`：`PlazaModel` 增加两个可选字段。
- `PlazaModelPricingTable.vue`：
  - 每行倍率 `rowRate(m) = m.user_rate_multiplier ?? m.rate_multiplier ?? 原分组生效倍率`（兼容旧后端）；`paidPerMillion`、`periodRate` 改为按行；生图 / 视频独立倍率逻辑不变。
  - 「相比官方价」列：`ratio = 本站价 ÷ 官方价`，取首档输入、输出（有阶梯时取最低档），两者都有时相对差 ≤ 1% 才显示，只有其一时用其一，都没有或没有官方价显示「—」；`discount = 1 − ratio ÷ 汇率`，格式化见第 13 节。专属倍率生效的行显示「划线公开折扣 + 专属折扣」。分时时段行用时段价格计算。悬停显示「¥x = $1，按 $1 = ¥y 折算」。
- `PlazaGroupSection.vue`：`GroupBadge` 用 `:show-rate="false"`；高峰提示改为时间段 + 「价格上浮 / 下调 x%」。
- `PlazaFilterBar.vue`、`ModelPlazaContent.vue`：去掉比例筛选（`rates`、`rate`、`selectedRate` 及相关计算），分组排序保留。
- `?group=<id>`：分组 `<section>` 加 `id="plaza-group-<id>"` 和顶部滚动留白（避开吸顶导航）；`ModelPlazaContent` 数据加载完 `nextTick` 后 `scrollIntoView`；找不到则忽略。`PlazaNavBar.vue:34` 登录跳转改为 `route.fullPath`。
- 文案：`modelPlaza.anonymousHint` 改写；删去不再使用的比例筛选文案。

## 5. 用户端不再显示分组倍率

### 5.1 新组件 `components/common/PlazaPriceLink.vue`（fork 自有）
- props：`groupId?: number`，`label?: string`。
- 可见条件与首页入口一致：`isFeatureFlagEnabled(FeatureFlags.modelPlaza) && (已登录 || !model_plaza_require_auth)`。
- 链接：已登录 `{ path: '/model-plaza', query: { embedded: '1', group } }`；未登录不带 `embedded`。

### 5.2 改动清单

| 位置 | 改法 |
|---|---|
| `views/user/KeysView.vue:165-176`（列表） | `GroupBadge` 不传倍率、专属倍率、高峰参数；徽章所在按钮外加 `PlazaPriceLink` |
| `KeysView.vue:551-578`（表单选择） | 同上；选择框下方加链接（按 `formData.group_id`） |
| `KeysView.vue:1174-1189`（切换分组弹出列表） | `GroupOptionItem` 不传倍率与高峰参数；不加链接（选项本身是按钮） |
| `components/channels/AvailableChannelsTable.vue:98-147, 236-287` | 去掉倍率参数与高峰标签，徽章旁加链接；`views/user/AvailableChannelsView.vue` 不再请求用户专属倍率 |
| `views/user/PaymentView.vue:129-147` | 「各分组价格」卡改为一个「查看各分组模型价格」入口；`loadGroupPrices` 删去 |
| `PaymentView.vue:176-190, 263-264` | 去掉「价格比例」「高峰价格」，加链接 |
| `components/payment/SubscriptionPlanCard.vue:48-55` | 同上 |
| `views/user/SubscriptionsView.vue:53-56` | 同上 |
| `components/admin/usage/UsageTable.vue:417-516` | 用户端（`showAccountBilling=false` 时）费用明细改 ¥、「价格比例」改「折扣」，见第 14 节；管理端不变 |
| `views/user/UsageView.vue:668,687`（CSV） | **保留**（单条请求实际采用的倍率） |

- 注意：只去掉 `rate-multiplier` 而保留高峰参数时，两个共享组件会按基准 1 算出错误的高峰比例，所以高峰参数必须一起去掉。
- 管理端调用处一律不动。

## 6. 管理端

### 6.1 接口（`handler/admin/group_model_rate_handler.go`，fork 自有）
- `GET /admin/groups/model-rates`：全部分组的规则（列表页提示用）。
- `GET /admin/groups/:id/model-rates`、`PUT /admin/groups/:id/model-rates`（整组替换，body `{ rules: [{ model, rate_multiplier }] }`）。
- `routes/admin.go` 注册一行；校验错误按 `error-handling.md` 返回 400。

### 6.2 复制分组
- `service/admin_group_duplicate.go` 复制成功后调用 `CopyGroup(from, to)`。放在 `CreateFromSource` 的事务里需要改上游仓储签名，改为事务后调用：失败时记 error 日志，不回滚已复制的分组，复制接口的响应不变（管理员可在编辑弹窗里看到并补上规则）。

### 6.3 前端
- `components/admin/group/GroupModelRatesEditor.vue`：行编辑（`ModelTagInput` 选模型、倍率输入、删除），`defineExpose({ save(groupId) })`；校验与后端一致。
- `views/admin/GroupsView.vue`：新建 / 编辑弹窗「分组倍率」下方各插一处组件；分组创建 / 更新成功后调用 `save(id)`，失败时提示「分组已保存，模型单独倍率保存失败：…」；倍率列有规则时显示「N 个模型单独倍率」并悬停列出。
- `views/admin/SettingsView.vue:7339-7349`：模型广场开关下加提示文案。

## 7. 利润控制

- 阈值 `D × (1 − margin − buffer)` 中的 D 改为 `resolveBaseRate(计费模型) × 高峰倍率`。
- 网关：`service/gateway_profit_control.go:13-54` 增加 `model` 参数，安装点 `gateway_scheduling.go:67, 120` 手里有 `requestedModel`（组合分组已换成上游模型）。
- OpenAI：`openai_profit_control.go:217-270` 同样增加 `model`；处理器层安装点（`openai_gateway_handler.go:664, 1297, 2792`、`openai_chat_completions.go:174`、`openai_embeddings.go:134`、`openai_alpha_search.go:136`）和 WS 每轮重装（`:3051`）传入已知的模型；调度器里的兜底安装点已有 `requestedModel`。
- 门的复用条件从「同一分组」改为「同一分组且同一模型」（`gateway_profit_control.go:17`、`openai_profit_control.go:199-202`）。
- 利润预览 `service/profit_preview.go:105-117`：按模型分别算 D。
- 未开启利润控制的分组不受影响。

## 8. 测试

- **unit（service）**：匹配（精确 / 最长前缀 / 归一化 / 大小写）；`resolveBaseRate`（命中时不调用专属倍率解析；同一用户先 A 后 B 不串值）；网关与 OpenAI 计费（模型倍率 × 高峰；生图独立倍率开 / 关；附加费同倍率）；按响应模型重算的实扣比较；预扣冻结逐候选；批量生图；利润门阈值与复用；利润预览；校验；复制。
- **integration（repository）**：仓储增删改查、整组替换的事务、软删除分组过滤；迁移 SQL。
- **前端 vitest**：比例列（一致 / 不一致 / 无官方价 / 专属 / 分时）、按行 ¥ 价、分组标签不显示倍率、高峰文案、筛选栏、`?group=` 定位、`PlazaPriceLink` 可见条件、PaymentView / SubscriptionPlanCard / AvailableChannelsTable 改动、管理端编辑器。按第 5 节更新受影响的既有用例。
- **本机 e2e 新阶段 `p7_model_rate.py`**（用例组 MRT，写入 `CASES.md`）：
  - 建分组（倍率 1.0）+ 单独倍率 `e2e-model-x = 2.0`，用户专属倍率 0.5；经网关真实请求（upstream-mock）核对 `usage_logs.rate_multiplier`、`actual_cost`；删专属倍率后再核对；同一用户交替请求两个模型核对无串值；
  - 模型广场接口字段；
  - Playwright 截图：模型广场、API 密钥页、可用渠道、充值页、订阅页；断言用户端页面除模型广场比例列外不出现「= $1」；`?group=` 定位；
  - 管理端：PUT 校验（通配位置、重复、≤0）、复制分组。
  - 汇率：`upstream_mock.py` 增加汇率接口（可切换为正常值 / 异常值 / 失败），`docker-compose.yml` 用环境变量 `FX_SOURCE_URLS` 指向它；核对自动更新、异常值丢弃、全部失败时用默认值、后台「立即更新」。
  - 用户端使用记录悬停：明细为 ¥ 且合计等于实扣、折扣正确；管理端仍为 $。
- **unit 补充**：汇率来源解析、校验、生效值选择、后台接口；前端折扣格式化、费用明细换算。

## 9. 上游合并影响

改动的上游文件（都只加少量调用）：`gateway_usage_billing.go`、`openai_gateway_usage.go`、`batch_image_public.go`、`billing_inflight_reservation.go`、`gateway_profit_control.go`、`openai_profit_control.go`、利润门安装点所在的处理器与调度文件、`profit_preview.go`、`model_plaza_service.go`、`model_plaza_handler.go`、`admin_group_duplicate.go`、`service/wire.go`、`repository/wire.go`、`routes/admin.go`、`setting_public.go`（汇率字段）；前端用户端调用处与模型广场组件、`UsageTable.vue`（用户端分支）、`GroupsView.vue`、`SettingsView.vue`。fork 新代码加 `CUSTOM:` 注释标明。

## 10. 上线与回滚

- 迁移 253 只新增表，可重复执行。
- 上线后没有任何规则时，行为与现在完全相同（除了用户端不再显示分组倍率）。
- 回滚：后台清空规则即恢复原计费；代码回滚后新表闲置无害。

## 11. 已知限制

- 渠道把请求模型映射成别的模型并按映射后的模型计费时，模型广场按展示的模型名匹配规则，计费按映射后的模型名匹配，两者可能不一致；与上游「分组逐模型定价」的现有行为相同。管理员用通配（如 `claude-opus-*`）可覆盖两者。
- `/v1/sub2api/billing` 不含模型单独倍率（`prd.md`「不做」）。
- 多实例部署时，其他实例最多 30 秒后生效。
- 一号机汇率接口可达：2026-10-23 经用户同意只读核对，`sub2api-custom` 容器内直连三个来源都返回正常（Frankfurter 6.6743）。将来访问不了时显示默认 6.8 的折扣，后台会提示「未更新成功」。

## 12. 美元汇率

### 12.1 服务 `service/fx_rate.go`（fork 自有）
- `FxRateService`：`Effective(ctx) FxRateView`、`RefreshNow(ctx)`、`Start()/Stop()`（后台循环：启动后 1 分钟、之后每 6 小时；带随机抖动）。
- 来源（按顺序，第一个成功即用；每个 10 秒超时）：
  1. `https://api.frankfurter.dev/v1/latest?base=USD&symbols=CNY` → `rates.CNY`（ECB 参考价，工作日更新）；
  2. `https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.min.json` → `usd.cny`；
  3. `https://latest.currency-api.pages.dev/v1/currencies/usd.min.json` → `usd.cny`。
  来源列表可在配置项 `fx.source_urls` 覆盖（见 15.4）（本机 e2e 指向 upstream-mock），不在后台界面暴露。
- 校验：5 ≤ 值 ≤ 9；已有上次值时 `|新 − 旧| / 旧 ≤ 10%`；否则丢弃、记 warn（含来源与值），继续试下一个来源。
- 防止卡死：只因「变化超过 10%」被丢弃的值，若本轮有两个不同来源的值彼此相差 ≤ 1%，视为真实变动，采用首个来源的值（否则旧值偏离真实汇率超过 10% 后会永远更新不了）。
- HTTP 客户端：与 `repository/pricing_service.go:39` 相同方式，用 `update.proxy_url` 与 `security.proxy_fallback.allow_direct_on_error`；放在 repository 层 `repository/fx_rate_client.go`，服务只依赖接口。
- 多实例：每个实例各自拉取、各自写设置；值相同或相近，无需加锁。

### 12.2 存储（设置表，key 定义在 fork 文件 `service/fx_rate.go`）
| key | 含义 |
|---|---|
| `fx_usd_cny_auto_enabled` | 自动更新，默认 `true` |
| `fx_usd_cny_default` | 默认汇率，默认 `6.8` |
| `fx_usd_cny_fetched` / `fx_usd_cny_fetched_at` / `fx_usd_cny_source` | 最近一次成功获取的值、时间、来源 |

- 生效值：自动更新开启且 `fx_usd_cny_fetched` 有值 → 取它；否则取默认值。

### 12.3 接口
- 公开设置增加 `usd_cny_rate`（生效值），`service/setting_public.go` 加一个字段与一处读取（上游文件，几行）；前端 `appStore` 读取，缺省时按 6.8。
- 管理端：`GET /admin/settings/fx-rate`（开关、默认值、生效值、最近获取的值 / 时间 / 来源、是否超过 3 天）、`PUT /admin/settings/fx-rate`（开关、默认值，默认值校验 5–9）、`POST /admin/settings/fx-rate/refresh`（立即更新，返回结果或失败原因）。handler 放 fork 文件 `handler/admin/fx_rate_handler.go`，`routes/admin.go` 一行。
- 前端：`components/admin/settings/FxRateCard.vue`（fork），在 `SettingsView.vue` 模型广场卡片附近插入一处。

## 13. 折扣格式（`utils/priceDiscount.ts`，fork 自有，模型广场与使用记录共用）

- 输入：本站 ¥ 与基准 $（或直接给比例）、汇率。`d = 1 − ratio ÷ fx`。
- `|d| < 0.5%` →「≈ 官方价」（使用记录里为「≈ 原价」）；`d > 0` → `-{round(d×100)}%`；`d < 0` → `+{round(-d×100)}%`。
- 比例或汇率无效（≤ 0、NaN）→「—」。

## 14. 用户端使用记录费用明细

- 只在 `UsageTable.vue` 的 `showAccountBilling === false`（用户端）分支生效，管理端模板不变；实现为 fork 小函数 `toBalanceUnits(usd, row)` 和少量模板条件，尽量不重排上游模板。
- `k = actual_cost / total_cost`（`total_cost > 0`），否则 `k = rate_multiplier ?? 1`；所有 `$` 金额与单价 × k 后以 `¥` 显示，精度沿用原来的位数。
- 「原始费用 $」保留；「价格比例」行改为「折扣」，`ratio = k`，按第 13 节格式化，悬停 / 次要文字显示「¥k = $1」。
- 更新 `UsageTable.spec.ts` 中用户端相关断言，管理端断言保持。

## 15. 前后端接口约定（实现时以此为准）

路径前缀 `/api/v1`，响应沿用现有 `{ code, message, data }` 信封；下面只写 `data`。

### 15.1 模型广场
- 模型 DTO（`modelPlazaModel`）新增：
  - `rate_multiplier: number`：该模型的公开基础倍率 = 模型单独倍率 ?? 分组倍率（不含高峰、分时）。新后端始终返回。
  - `user_rate_multiplier?: number`：登录用户在该分组有专属倍率，且该模型**没有**单独倍率时返回专属倍率；否则省略。
- 前端每行生效倍率 = `user_rate_multiplier ?? rate_multiplier ?? (分组 user_rate_multiplier ?? 分组 rate_multiplier)`；高峰与分时照旧在前端乘。

### 15.2 公开设置
- `GET /settings/public` 新增 `usd_cny_rate: number`（生效汇率）。前端缺省或 ≤ 0 时按 6.8。

### 15.3 分组模型倍率（管理员）
- 类型 `GroupModelRateRule = { model: string; rate_multiplier: number }`。
- `GET /admin/groups/model-rates` → `Record<string, GroupModelRateRule[]>`，键为分组 id，只含有规则的未删除分组。
- `GET /admin/groups/:id/model-rates` → `{ rules: GroupModelRateRule[] }`，按 `model` 排序。
- `PUT /admin/groups/:id/model-rates`，body `{ rules: GroupModelRateRule[] }`（空数组 = 清空）→ `{ rules }`（归一化后的模型名）。校验失败 400，`message` 为可读原因；分组不存在 404。

### 15.4 汇率（管理员）
- `FxRateView`：
  ```
  {
    auto_enabled: boolean,
    default_rate: number,
    effective_rate: number,
    effective_from: "fetched" | "default",
    fetched_rate: number | null,
    fetched_at: string | null,      // RFC3339
    source: string,                 // 最近一次成功的来源名，没有为 ""
    stale: boolean,                 // 开启自动更新且（从未成功或 fetched_at 超过 3 天）
    last_error: string,             // 本进程最近一次失败原因，没有为 ""
    last_attempt_at: string | null  // 本进程最近一次尝试时间
  }
  ```
- `GET /admin/settings/fx-rate` → `FxRateView`。
- `PUT /admin/settings/fx-rate`，body `{ auto_enabled: boolean, default_rate: number }` → `FxRateView`；`default_rate` 不在 5–9 返回 400。
- `POST /admin/settings/fx-rate/refresh` → `{ ok: boolean, error: string, rate: FxRateView }`；全部来源失败时 `ok=false`、HTTP 仍为 200。
- 来源配置：配置项 `fx.source_urls`（字符串列表，非空时替换内置三个来源），环境变量 `FX_SOURCE_URLS` 用逗号或空白分隔；响应格式按内容识别（`rates.CNY` 或 `usd.cny`）；来源名取 URL 的 host。
