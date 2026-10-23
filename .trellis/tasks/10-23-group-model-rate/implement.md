# 实施计划：分组内单个模型倍率

前置：首页改版（`10-23-qx-theme`）已暂停。经用户同意（2026-10-23），它的未提交代码已存为 `git stash`「qx-theme WIP (paused for 10-23-group-model-rate)」，本任务提交后再恢复。

## 步骤

### 1. 数据层
- [x] `migrations/253_group_model_rate_multipliers.sql`、`migrations/group_model_rate_migration_test.go`（design 1.1）。
- [x] `service/group_model_rate.go`：类型、仓储接口、`GroupModelRateService`（快照、匹配、校验、读写、复制）、`resolveBaseRate`（design 2）。
- [x] `repository/group_model_rate_repo.go` + `repository/wire.go`。
- [x] 测试：service unit（匹配、校验、缓存、`resolveBaseRate` 不调用专属解析）；repository integration（CRUD、整组替换、软删除过滤）。

**检查点 A**：`go test -tags unit ./internal/service/ -run GroupModelRate`、`go test -tags=integration ./internal/repository/ -run GroupModelRate`、`go test -tags unit ./migrations/...`。

### 2. 计费接入
- [x] `gateway_usage_billing.go`：调整计费模型与倍率的顺序，`recordUsageMultipliers` 加模型参数；`QuoteUsageCost`；按响应模型重算 + `responseModelBillingAdoptable` 实扣条件（design 3）。
- [x] `openai_gateway_usage.go`：基础倍率、按响应模型重算、Free Fast 重算。
- [x] `batch_image_public.go`、`billing_inflight_reservation.go`。
- [x] wire 注入（`service/wire.go` setter），`go generate` 不需要（不改 provider 集合时）；如需改 provider 则运行 `make generate` 并提交 `wire_gen.go`。
- [x] 测试：design 8 的计费类 unit。

**检查点 B**：`gofmt -l`、`go vet ./...`、`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...`、`go build ./...`。

### 3. 利润控制
- [x] 两个利润门加模型参数与复用条件；处理器层与 WS 每轮的安装点传入模型；`profit_preview.go` 按模型算 D（design 7）。
- [x] 测试：阈值按模型、同组不同模型不复用、利润预览；既有利润控制测试全部通过。

**检查点 C**：同检查点 B，另跑 `go test -tags unit ./internal/service/ -run 'Profit'`、`./internal/handler/...`。

### 4. 管理端接口与模型广场后端
- [x] `handler/admin/group_model_rate_handler.go` + `routes/admin.go`；复制分组调用 `CopyGroup`。
- [x] `ModelPlazaService` 填 `RateMultiplier`；handler 模型 DTO 加 `rate_multiplier`、`user_rate_multiplier`。
- [x] 测试：handler（校验 400、非管理员 403）、模型广场 service / handler（公开倍率、专属倍率只在未命中时给出）。

- [x] 汇率：`service/fx_rate.go`（来源、校验、生效值、后台循环）、`repository/fx_rate_client.go`、`handler/admin/fx_rate_handler.go` + 路由、`setting_public.go` 增加 `usd_cny_rate`、配置项 `fx.source_urls`（design 12、15.4）。
- [x] 测试：来源解析（三种响应格式）、异常值丢弃、全部失败用默认值、关自动更新用默认值、后台接口校验。

**检查点 D**：同检查点 B；`go test -tags=integration ./internal/repository/ -run 'GroupModelRate|Affiliate'`。

### 5. 前端：模型广场
- [x] `api/modelPlaza.ts` 类型；`PlazaModelPricingTable` 按行倍率与比例列；`PlazaGroupSection` 标签与高峰提示；`PlazaFilterBar` / `ModelPlazaContent` 去掉比例筛选；`?group=` 定位；`PlazaNavBar` 登录跳转（design 4.2）。
- [x] 更新 `PlazaModelPricingTable.spec.ts`、`PlazaGroupSection.spec.ts`，新增定位与比例列用例。

**检查点 E1**：`frontend/` 下 `npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run`。

### 6. 前端：用户端去倍率与管理端
- [x] `components/common/PlazaPriceLink.vue`；design 5.2 所列调用处。
- [x] `components/admin/group/GroupModelRatesEditor.vue`、`api/admin` 接口、`GroupsView.vue` 接入与倍率列提示、`SettingsView.vue` 提示。
- [x] `utils/priceDiscount.ts`（design 13）；模型广场「相比官方价」列改用它。
- [x] `UsageTable.vue` 用户端费用明细改 ¥ 与折扣（design 14）。
- [x] `components/admin/settings/FxRateCard.vue` 接入 `SettingsView.vue`；`appStore` 读取 `usd_cny_rate`。
- [x] 文案（中英，key 对齐）；删除不再使用的 key。
- [x] 更新 `AvailableChannelsTable.spec.ts`、`SubscriptionPlanCard.spec.ts`、`PaymentView.spec.ts`、`UsageTable.spec.ts`（用户端）；新增 `PlazaPriceLink`、编辑器、折扣格式化、汇率卡片用例。

**检查点 E2**：`npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run && npx -y pnpm@9 run build`；`git status` 确认 `pnpm-lock.yaml` 无改动。

### 7. 本机端到端
- [x] `~/workspace/sub2api-local/e2e/p7_model_rate.py`、`run_all.sh` 加入 p7、`CASES.md` 加 MRT（模型倍率）与 FX（汇率）两组（design 8）；p7 的 UI 部分复用 `ui/` 下 Playwright 脚本方式。
- [x] `mock/upstream_mock.py` 增加汇率接口；`docker-compose.yml` 加 `FX_SOURCE_URLS` 环境变量指向它（只在本机环境）。
- [x] `./build.sh && docker compose up -d`；`./run_all.sh`（全量 p0–p7）：2026-10-23 运行 `20261023-130102`，291 条全部通过；之后折扣栏上下排列与表头分两行只重跑 p7 界面部分（14 条通过）。
- [x] 截图存 `runs/<时间>/model_rate_ui/`，挑关键几张放进任务目录 `screenshots/` 给用户看。

**检查点 F**：全量 e2e 通过；有失败时回到对应步骤。

### 8. 收尾
- [x] `trellis-check` 全量复查（前后端各一个检查子代理）；`trellis-update-spec`：记录「计费倍率统一走 `resolveBaseRate`」「fork 表 + 进程内快照」「用户端价格展示」约定（backend/quality-guidelines、database-guidelines，frontend/component-guidelines，guides/cross-layer）。
- [x] 向用户报告结果与截图，**经同意后**再提交、推送、部署一号机（用户 2026-10-23 同意）。

## 回滚点

- 步骤 1–4 后端可单独回滚：未配置规则时行为不变。
- 前端改动与后端解耦：前端对缺少模型倍率字段的旧后端有回退。
