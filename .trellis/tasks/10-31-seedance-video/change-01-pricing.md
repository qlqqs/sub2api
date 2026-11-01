# 变更 01：Seedance 按分辨率分档定价，带参考视频单独定价（2026-10-31）

## 背景

用户提供了临时 mmgate Key，主代理已用它拉取 `/v1/models`。快照在 `/tmp/mmg/models.json`，只用于读取，**不要用它发起生成请求**。

结论：mmgate 不是统一价。它按分辨率分档，带参考视频还要另算单价。两个模型的成本价如下（已是本账号折扣价，即原价 × 0.69，单位 ¥/秒）：

| 模型（mmgate ID） | 分辨率 | 普通 | 带参考视频 |
|---|---|---|---|
| doubao-seedance-2-0-260128 | 480p | 0.489279 | 0.547992 |
| | 720p | 0.69 | 0.8349 |
| | 1080p | 1.725 | 2.0769 |
| | 4k | 3.45 | 4.2918 |
| doubao-seedance-2-5-260628 | 480p | 0.684991 | 0.792632 |
| | 720p | 1.035 | 1.2489 |
| | 1080p | 2.553 | 3.0843 |

有声和无声同价（2.5 的 480p 返回的有声价是 0.00001，视为数据异常，忽略）。

**用户决定**：售价 = 原价 × 0.75 = 成本 ÷ 0.69 × 0.75，每一档单独计算，结果如下（¥/秒）：

| 模型 | 480p | 720p | 1080p | 4k |
|---|---|---|---|---|
| seedance-2.0 普通 | 0.5318 | 0.75 | 1.875 | 3.75 |
| seedance-2.0 带参考视频 | 0.5956 | 0.9075 | 2.2575 | 4.665 |
| seedance-2.5 普通 | 0.7446 | 1.125 | 2.775 | — |
| seedance-2.5 带参考视频 | 0.8616 | 1.3575 | 3.3525 | — |

计费规则（mmgate 文档）：

- 只要请求里有参考视频，**整个**计费秒数都按「带参考视频」单价算。
  - 计费秒数 = 成片 + max(floor(参考视频总秒数), ceil(成片 × 2/3))。
- 只有参考图或参考音频、没有参考视频时，按普通单价计算。

### `/v1/models` 里其他与之前约定不同的规格

| 项目 | 2.0 | 2.5 |
|---|---|---|
| 支持的分辨率 | 480p、720p、1080p、**4k** | 480p、720p、1080p |
| 时长 | 4–15 秒，**也支持自动**（auto_duration=true） | 4–30 秒，支持自动 |
| 参考上限（图 / 视频 / 音频 / 参考视频总秒数） | 9 / 3 / 3 / 15 | **30** / 10 / **10** / 30 |
| 默认值 | 720p、5 秒、16:9 | 同左 |

其他说明：

- **返回里没有 `token_frames`**。所以尺寸表仍然无法核对，保持现状：沿用方舟官方尺寸表，4k 尺寸未知，按 C2 的兜底规则处理。
- 生产账号的 `model_mapping`：
  - `seedance-2.0` → `doubao-seedance-2-0-260128`
  - `seedance-2.5` → `doubao-seedance-2-5-260628`

**当前的严重问题（必须修）**：`LookupVideoBillingResolution` 不认识 `4k`，`NormalizeVideoBillingResolutionOrDefault` 会把它折算成 480p。用户用 2.0 生成 4K 视频时只按 480p 扣费，每秒亏约 ¥2.9。

## 约定 C6：计费档位

> 2026-11-01 起渠道改填原价、分组倍率 0.75，见 `change-02-rate.md`。档位格式不变。

- 视频计费分辨率增加 `4k`。可识别的写法为 `4k`、`4K`、`2160p`、`2160`；`uhd` 由 BE-S 视情况决定是否支持。
- 带参考视频的计费档位名为 `<分辨率>-ref`：`480p-ref`、`720p-ref`、`1080p-ref`、`4k-ref`。
  - 渠道定价沿用现有的「视频按秒 + 分档（intervals 的 tier_label）」机制配置：
    - 基础价 `per_request_price` 写 480p 普通价；
    - 其余每一档各配一个 interval，`tier_label` 就是档位名。
  - BE-S 读现有代码后，如果发现有更贴合的现成结构，可以改用，但必须在「完成报告」里写明最终格式。FE-S 的界面以 BE-S 定下的格式为准。
- 计费时如果找不到对应的 `-ref` 档位，按同分辨率的普通档计费。这是渠道配置缺失时的兜底，不是业务分支。
- usage_logs：
  - `video_resolution` 记计费档位：带参考视频时为 `720p-ref` 这样的名字，否则为 `720p`。
  - `video_duration_seconds` 仍记计费秒数。
- 建任务时的余额预估：按请求的分辨率，有参考视频时用 `-ref` 档单价。
- 模型广场、Nova 的价格预估：按所选分辨率显示单价；有参考视频时显示 `-ref` 单价。

## 分工

- **BE-S**：
  - 4k 分辨率的识别；
  - `-ref` 档位的计费、余额预估、usage 记录；
  - 网关的 Seedance 自动时长：2.0 的 -1 或不传，按 15 秒预估；
  - 单测；
  - mock（4k 尺寸先按 3840×2160 / 2160×3840 等比例处理，mock 和后端用同一张表）；
  - e2e：在 SD 组加 4k 和 -ref 计价用例，渠道定价改用上表的分档价格。
- **FE-S**：
  - 渠道定价编辑器的视频分辨率选项加 4k 和 `-ref` 档（看 `PricingEntryCard.vue:387` 一带）；
  - 模型广场和 qx 主题价格页能展示分档价格（看图片分档现在怎么显示，保持一致）；
  - 单测。
- **BE-N**：qx-seedance 插件的变更：
  - 2.0 增加 4k 选项（2.5 不显示）；
  - 2.5 参考图上限改为 30、参考音频上限改为 10；
  - 2.0 也增加「自动时长」；
  - 默认值改为 720p、5 秒（ratio 默认值保持 adaptive）；
  - fixtures、文档、测试同步更新。
- **FE-N**：`estimateVideoCost` 改为：
  - 按分辨率档取单价，有参考视频时用 `-ref` 档；
  - 2.0 的自动时长按 15 秒估算；
  - 文案格式不变；
  - 单测。
  - 广场数据里分档单价的字段名，以 FE-S / BE-S 的结果为准：读 sub2api 模型广场接口里视频分档的返回结构。

所有代理：只跑与本次改动相关的单测，外加各自仓库的 lint / typecheck，不跑全量 e2e。完成后在本文件末尾的「完成记录」下追加自己的小节。最后由 QA 做定向复测。

## 完成记录

### BE-N

已完成 qx-seedance 规格同步：

- `backend/plugins/qx-seedance/ui.schema.json`：2.0 增加 `4k`（通过 `availableWhen.targetModel` 限制为 2.0，2.5 不可选）；2.5 参考图/音频上限改为 30/10，2.0 保持 9/3；两个模型均开放 `duration=-1`；默认值为 `720p`、5 秒、`adaptive`。保留主代理的 `familyDescription: "按秒计费"`。
- `backend/plugins/qx-seedance/manifest.json`：全局图片/音频配额同步提升到 30/10，避免 schema 放行后被 manifest 配额拦截；原 720p 普通档展示兜底价格不变，实际分档价由网关/前端负责。
- 新增 fixtures：`07-2.0-4k-auto`、`08-2.5-reference-limits`、`09-2.0-defaults`，各含 `input.json` 和完整 `expected-request.json`；原六种模式快照保留，共 9 个。
- `backend/test/plugin-runtime.test.js`：更新 2.0 自动时长预期及 fixture 数量，增加 4k 模型可用性、两个模型默认值/自动时长、2.0 与 2.5 图/音频数量边界及超量拒绝测试。
- 同步 `backend/plugins/qx-seedance/README.md` 及 `docs/plugins/{README,testing,ui-schema,LLM}.md`，说明当前分辨率、配额、默认值、自动时长及新增 fixtures。

验证：

- 定向 `node --test test/plugin-runtime.test.js`：24/24 通过。
- 按用户要求另跑 `env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY node --test`：126/126 通过，无失败、无跳过；日志 `/tmp/nova-change-01-tests.log`。
- `node plugin-runtime/verify.js`：结构校验和 9 个 fixtures 全部通过。
- 本轮修改测试/驱动的 ESLint、`git diff --check` 均通过。

未发起真实生成请求，未读取或使用临时 Key，未跑全量 e2e，未修改前端/其他代理范围，未提交、推送或重建本机镜像。无 BE-N 遗留项；QA 可对当前源码做定向复测，运行环境需由整体构建刷新。

### BE-S

**渠道档位格式已确定（供 FE-S / FE-N 开工）**：直接沿用 C6，不新增接口字段。

- `billing_mode: "video"`；`per_request_price` 为 480p 普通单价。
- `intervals` 的每项使用 `tier_label` 和 `per_request_price`，标签为 `720p`、`1080p`、`4k`、`480p-ref`、`720p-ref`、`1080p-ref`、`4k-ref`；`min_tokens: 0`、`max_tokens: null`。也可显式配置普通 `480p` interval。
- 缺失 `-ref` 档时先找同分辨率普通档，再回落基础价；只含参考图/音频不使用 `-ref`。
- `GET /api/v1/model-plaza` 的 `data.groups[].models[].pricing` 保持 `billing_mode`、`per_request_price`、`intervals[].tier_label`、`intervals[].per_request_price`；金额与现有模型广场口径一致。前端依所选分辨率及是否带参考视频匹配档位。
- 4k 别名支持 `4k` / `4K` / `2160p` / `2160` / `uhd`，统一记 `4k`；usage 的参考档统一记 `<规范分辨率>-ref`。

#### 实现结果

- 4k 及其别名不再折算为 480p；渠道分档和分组 `video_model_prices` 均保留 `-ref`，参考档缺失时取同分辨率普通价。
- Seedance 预估按请求物理分辨率选择普通/参考档，并乘预估计费秒数，不再取渠道最高档单价。保留分组覆盖价的既有优先级。
- 完成轮询先用物理分辨率及 token 计算计费秒数，再生成计费档位；`video_resolution` 写规范档位，`output_tokens` 保持上游值。
- 2.0 的 `-1` / 未传时长已按 15 秒预估（带参考视频为 25 秒），2.5 为 30 秒；新增回归测试固定此行为。
- 后端与 mock 采用相同的 4k 近似尺寸：16:9 为 3840×2160，9:16 为 2160×3840，其他比例按 1080p 尺寸两倍；已逐项核对四种分辨率、六种比例的像素表一致。
- SD 渠道改为本变更表的全部人民币分档价，2.5 ID 修正为 `doubao-seedance-2-5-260628`。新增 SD-13..19：4k 横/竖屏及别名、4k-ref、两模型各普通/参考档、2.0 自动时长、图/音频不触发参考档、缺参考档回退、所选档位余额拦截。各任务继续检查异步状态、token 和重复轮询幂等。

#### 本次文件清单

- `backend/internal/handler/gateway_inflight_reservation.go`
- `backend/internal/handler/seedance.go`
- `backend/internal/handler/seedance_test.go`
- `backend/internal/service/billing_inflight_reservation.go`
- `backend/internal/service/billing_inflight_reservation_test.go`
- `backend/internal/service/billing_service.go`
- `backend/internal/service/group.go`
- `backend/internal/service/model_pricing_resolver.go`
- `backend/internal/service/seedance.go`
- `backend/internal/service/seedance_test.go`
- `backend/internal/service/video_billing.go`
- `backend/internal/service/video_billing_resolution.go`
- `backend/internal/service/video_billing_test.go`

- `/home/qlqq/workspace/sub2api-local/mock/upstream_mock.py`
- `/home/qlqq/workspace/sub2api-local/e2e/p12_seedance.py`
- `/home/qlqq/workspace/sub2api-local/e2e/CASES.md`（仅 SD 一节）
- 本变更文档及 `task-be-sub2api.md` 的完成记录。

#### 验证结果

最终代码验证通过：

```bash
cd /home/qlqq/workspace/sub2api/backend
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit -parallel=1 ./internal/service ./internal/handler ./internal/server
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./internal/service -run '^TestInflightEstimate_AccountMappingNoDBAndBoundedMemory$' -count=5
go build ./...
```

- 受影响包单测：service 219.613s、handler 42.706s、server 2.237s，全部通过。日志 `/tmp/be-s-change01-unit-verified.log`。
- 既有内存断言单独连续 5 次通过（1.410s），未修改阈值。日志 `/tmp/be-s-change01-memory-recheck.log`。
- `go build ./...` 通过；日志 `/tmp/be-s-change01-build-final.log`。
- `gofmt -l`、`git diff --check -- backend`、mock/p12 的 `python3 -m py_compile` 通过。
- 后端与 mock 的四种分辨率 × 六种比例像素表逐项比对通过。

```bash
cd /home/qlqq/workspace/sub2api-local
./build.sh --no-web && docker compose up -d
docker compose restart upstream-mock
# 等待 http://127.0.0.1:28080/health 成功后：
cd e2e
E2E_RUN_DIR=/home/qlqq/workspace/sub2api-local/e2e/runs/20261031-181206-be-s-change01-final python3 p12_seedance.py
python3 report.py /home/qlqq/workspace/sub2api-local/e2e/runs/20261031-181206-be-s-change01-final > /home/qlqq/workspace/sub2api-local/e2e/runs/20261031-181206-be-s-change01-final/report.md
```

- 最终后端镜像构建与启动通过，应用 health 正常；沿用前端产物。日志 `/tmp/be-s-change01-local-build-final.log`。
- 最终 SD-01..19：**19/19 用例、170/170 原始断言全部通过**；报告 `/home/qlqq/workspace/sub2api-local/e2e/runs/20261031-181206-be-s-change01-final/report.md`，逐条结果 `results.jsonl`、过程 `p12.log` 均在同目录。
- 其中 4k 普通 5 秒扣 ¥18.75；4k 参考任务计费 17 秒，扣 ¥79.305，usage 记 `4k-ref`；参考档缺失回退普通 720p，usage 仍记 `720p-ref`；余额 7 拦截参考任务、余额 10 放行，2.0 自动时长按 15 秒拦截。

一次并行单测复测出现已有的进程级堆内存断言波动：

```text
TestInflightEstimate_AccountMappingNoDBAndBoundedMemory
"9156248" is not less than "8388608" (no per-model cache growth)
```

其余测试通过；随后单独连续 5 次及上述受影响包串行全包复测通过。失败日志 `/tmp/be-s-change01-unit-final.log`，未放宽断言。

#### 遗留问题与边界

- 无未完成实现项。4k 尺寸为近似值，mmgate `/v1/models` 没有 `token_frames`，真实任务尺寸仍待上线后核对。
- 直接采用 C6，无约定变更；没有修改 `contract.md`、前端、Nova 或 `backend/internal/web/embed_on.go` / `embed_test.go`。
- 本次只运行受影响包单测、构建和 p12，未跑全仓单测或全量 e2e；未提交、推送、连接远端或发起真实生成请求。



### FE-N

只改了 `nova-image-studio/frontend/`，没有提交。

**改动**

- `lib/embed/key-policy.ts`：
  - `perSecondVideoPrice(group, model, resolution, withReference)` 按 BE-S 定的格式取单价，依次查找：
    1. 带参考视频时，`intervals[].tier_label` 为 `<分辨率>-ref` 的档；
    2. 同分辨率的普通档；
    3. 基础价 `per_request_price`。
  - 匹配不区分大小写。不再取第一档兜底。
  - 新增 `normalizeVideoResolution`：`4K` / `2160p` / `2160` / `uhd` 都归为 `4k`。
  - 设置页视频行的单价文案：有分档时显示「¥最低价 起 / 秒，按分辨率与参考视频分档」，没有分档时仍显示「¥X / 秒」。
- `lib/embed/cost-estimate.ts`：`estimateVideoCost` 在有参考视频时，用 `-ref` 档单价乘整段计费秒数；只有参考图或音频时用普通档。文案格式不变，仍为 `预计 ¥X（N 秒 × ¥单价）`。
- 2.0 自动时长：模型上限取 schema 里当前组合可选的最大时长，2.0 为 15、2.5 为 30。现在 2.0 也有 `-1`，按 15 秒估算。这部分代码没改，用真实插件包写了单测确认。
- `lib/plugin-schema.ts`：`coerceFieldValues` 在当前值不可选时，先落回字段默认值，再取第一个可选项。例如 2.0 选 4k 后切到 2.5，分辨率落回 720p（以前是 480p）；从 2.5 的 20 秒切回 2.0，落回 5 秒（以前是 4 秒）。

**联动确认**（BE-N 新的 `ui.schema.json`，单测和界面都核对过）

- 4k 只有 2.0 可选。
- 2.0 和 2.5 都有「自动」时长。
- 参考生成的名额：图 / 视频 / 音频，2.0 为 9 / 3 / 3，2.5 为 30 / 10 / 10；切回 2.0 时超出名额不可提交。
- 默认值为 720p、5 秒、adaptive。

**测试**

- `cd frontend && npx vitest run`：85 个文件、795 个测试全部通过。
  - `embed/__tests__/fixtures.ts` 的 `VIDEO_PLAZA` 改为本变更售价表的 C6 格式。
  - `video-cost-estimate.test.ts` 新增：普通档与基础价、`-ref` 档、4k 各种别名、缺 `-ref` 档或普通档时的回落、只有参考图时用普通档、读真实插件包的 2.0 自动时长 15 秒、2.0 4k 带参考视频。
  - `plugin-schema.test.ts` 新增：4k 联动、2.0 自动时长、新名额、落回默认值。
- `npx eslint src/lib/embed src/components/embed`：通过。

**截图**：`/tmp/seedance-fe-n/change01-2.0-4k-ref-estimate.png`

- 先完整重建 `sub2api-local-nova`（`./build.sh && docker compose up -d`）。
- 当时运行中的 sub2api 已经接受 C6 档位：用管理端接口把我的测试渠道（channel 61，「Seedance 视频 FE-N」分组）改成本表的分档价，模型广场原样返回了 `tier_label`。
- 界面选 2.0、参考生成、4K、5 秒：
  - 不带参考视频时显示 `预计 ¥18.75（5 秒 × ¥3.75）`；
  - 上传 1 秒的参考视频后显示 `预计 ¥41.985（9 秒 × ¥4.665）`，即 5 + max(1, ceil(5×2/3)) = 9 秒，单价取 4k-ref 档；
  - 没有 HTTP 错误。
- 没有提交生成任务。

**遗留**：无。

### FE-S

按 BE-S 定下的格式实现：`per_request_price` 是 480p 普通价，其余各档写在 `intervals[].tier_label`；带参考视频的档位名是 `<分辨率>-ref`。所有改动都在 `frontend/` 内。

**改动**

- `src/constants/channel.ts`
  - 新增 `VIDEO_RESOLUTIONS`（480p / 720p / 1080p / 4k）、`VIDEO_BASE_RESOLUTION`、`VIDEO_REF_TIER_SUFFIX`、`VIDEO_TIER_LABELS`，共 8 个档位。
- `src/utils/videoPricingTiers.ts`（新增）
  - `videoPriceTiers(pricing)` 把广场接口返回的定价按分辨率合并成 `{ resolution, price, refPrice }`：
    - 基础价记为 480p 普通价；显式配置了 480p 档时以该档为准，与后端的查找顺序一致；
    - `-ref` 档并入同一分辨率的 `refPrice`；
    - 按 480p → 4k 排序，不认识的标签排在后面；
    - 没有分档时返回 `[]`。
  - 模型广场和 qx 价格页都用这个函数。
  - FE-N 如果要在 Nova 里复用同样的取价逻辑，可以参考它。
- 渠道定价编辑器
  - `IntervalRow.vue`：视频模式的档位从文本框改成下拉框，包含全部 8 个档位。
    - `-ref` 档显示为「720p · 带参考视频」，保存的仍是 tier_label 原文。
    - 已有的值不在列表里时，保留为一个选项，原样显示。
  - `PricingEntryCard.vue`：视频模式点「添加层级」时，取下一个还没配置的档位，顺序是 720p → 1080p → 4k → 480p-ref → … → 4k-ref。不会自动添加 480p，因为基础价就是 480p。
  - 视频基础价的标签改为「480p 每秒价格（未配置的分辨率也按此价）」。
- 模型广场 `PlazaModelPricingTable.vue`
  - 视频模型有分档时，价格单元格显示成一个小表格：
    - 表头是各分辨率，左上角标出单位「/ 秒」；
    - 第一行「普通」，第二行「带参考视频」，没配置的格子显示「-」；
    - 两个模型都没有带参考视频档时，不显示第二行。
  - 视频单一价格、图片、按次计费的显示都没变。
  - 最初沿用图片分档的价格芯片样式，但在这一列的宽度下 4 档就会折行，难以对照，所以改成了表格。
- qx 价格页 `themes/qx/pricing.ts`、`QxPricing.vue`
  - 视频分档行在「档位」列逐行列出分辨率，价格格每行显示「¥0.75 / 秒 · 带参考视频 ¥0.9075」。
  - 价格格合并了「输入」「输出」两列，避免折行。
  - 分组里没有 token 分档时，档位列的表头显示「分辨率」，否则仍显示「上下文」。
- i18n（中英文）
  - `admin.channels.form.videoTierRef`、`defaultVideoPrice`（改文案）
  - `modelPlaza.table.plainVideo` / `withRefVideo`
  - `qxTheme.pricing.resolution` / `withRefVideo`

**测试**

- 新增：
  - `utils/__tests__/videoPricingTiers.spec.ts`：单一价格、基础价并入 480p、`-ref` 合并与排序、显式 480p、未知标签。
  - `components/admin/channel/__tests__/PricingEntryCard.videoTiers.spec.ts`：添加档位的顺序；全部配满后不再自动填档位。
- 补充：
  - `IntervalRow.spec.ts`：8 个选项的值和显示名，选择后保存原文；旧值保留。
  - `PlazaModelPricingTable.spec.ts`：分档表格逐格核对（含缺档显示「-」）；单一价格不显示分档。
  - `themes/qx/__tests__/pricing.spec.ts`：`videoTiers` 的内容；单一价格仍走 `unitPrice`。
- 结果：
  - `lint:check`、`typecheck` 通过。
  - 相关单测（qx 主题、模型广场、渠道编辑器、videoPricingTiers、i18n）共 41 个文件、334 个用例，全部通过。
  - `pnpm-lock.yaml` 没有变化。

**截图**（`./build.sh && docker compose up -d` 重新构建后截取）

- `/tmp/seedance-fe-s/change01-plaza-group-288.png`：模型广场里的「Seedance 视频 FE-N 161353」分组（BE-S 的分档价）。
  - seedance-2.0：4 个分辨率 × 普通 / 带参考视频；
  - seedance-2.5：3 个分辨率 × 2 行；
  - 价格与本文的售价表一致。
- `/tmp/seedance-fe-s/change01-qx-pricing.png`：qx 首页价格面板的同一分组，每个分辨率一行，同时显示普通价和带参考视频价。
- `~/workspace/sub2api-local/e2e/ui/shots.cjs` 新增 `PLAZA_GROUP=<分组 ID>` 开关，只截该分组的价格表。

**遗留**

- 分组设置里 grok 的 `video_model_prices` 编辑器（`groupsVideoModelPricing.ts`）不在本次范围内，仍只有 480p / 720p / 1080p。
- qx 价格表对非 token 模型沿用「输入 / 输出 / 缓存读取」表头，与图片、按次计费的现状一致，这次没改。
