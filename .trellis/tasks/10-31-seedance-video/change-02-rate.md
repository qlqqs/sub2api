# 变更 02：Seedance 渠道填原价，分组倍率 0.75（2026-11-01）

## 决定

用户决定：渠道定价填 mmgate **原价**，「Seedance 视频」分组用**分组倍率 0.75** 折算成售价。

- 原价 = 成本 ÷ 0.69；售价 = 原价 × 0.75，与 change-01 的售价表一致，只是不再四舍五入到 4 位小数。
- 用分组的普通倍率 `rate_multiplier`，**不开「视频独立倍率」**，分组不开高峰倍率。
- 档位格式不变（C6）：`per_request_price` 为 480p 普通价，其余各档写在 `intervals[].tier_label`。

替代 prd「分组倍率 1、单价直接填人民币售价」和 change-01 渠道填售价的写法。

## 渠道原价（¥/秒）

| 模型 | 480p | 720p | 1080p | 4k |
|---|---|---|---|---|
| seedance-2.0 普通 | 0.7091 | 1 | 2.5 | 5 |
| seedance-2.0 带参考视频 | 0.794191 | 1.21 | 3.01 | 6.22 |
| seedance-2.5 普通 | 0.992741 | 1.5 | 3.7 | — |
| seedance-2.5 带参考视频 | 1.148742 | 1.81 | 4.47 | — |

由成本价 ÷ 0.69 计算，例如 0.489279 ÷ 0.69 = 0.7091，0.684991 ÷ 0.69 ≈ 0.992741。

## 已核对的现状（FE-S）

四条路径都乘分组生效倍率（用户专属倍率 ?? 分组模型倍率 ?? 分组倍率）：

- 实际扣费：`openAIUsageMultipliers` → `videoMultiplier = resolveVideoRateMultiplier(apiKey, base)`，`base` 不含高峰因子。
- 模型广场、qx 价格页：渠道价 × 生效倍率。前端不用改。
- Nova 预估：`key-policy.ts` 的 `perSecondVideoPrice` 乘 `requestRate`。Nova 不用改。
- 建任务余额预检：`billing_inflight_reservation.go` 的 `InflightEstimateVideo` 用 `math.Max(textRate, imageRate)`。
  - 这里含高峰因子，不读视频独立倍率，和实际扣费口径不一致。
  - 按本决定（普通倍率、不开高峰）两者结果相同；但开了视频独立倍率或高峰的分组会出现预检偏高，例如分组倍率 1、视频独立 0.75 时，预检按原价估算。

usage_logs：`total_cost` 为原价 × 计费秒数，`actual_cost` 为售价，`rate_multiplier` 为 0.75。分组分润系数按实扣消费计算，不受影响。

## 分工

### BE-S

1. 预检口径对齐实际扣费：`InflightEstimateVideo` 的倍率改为与 `openAIUsageMultipliers` 相同的视频倍率（基础倍率走用户专属 / 分组模型倍率 / 分组倍率，再经 `resolveVideoRateMultiplier`，不叠加高峰因子）。补单测：
   - 分组倍率 0.75 → 预检 = 原价 × 0.75 × 秒数；
   - 视频独立倍率开启时取独立倍率；
   - 高峰时段不影响视频预检。
2. e2e `p12_seedance.py`：
   - SD 分组 `rate_multiplier` 改为 0.75，渠道 `PRICES` 改为上表原价；
   - 现有扣费断言按 `actual_cost = 原价 × 0.75 × 计费秒数` 计算（金额与原售价断言在 4 位小数内一致，比较时按精确值算，不要硬编码旧的 4 位小数售价），并核对 `total_cost = 原价 × 计费秒数`、`rate_multiplier = 0.75`；
   - SD-12 / SD-19 的余额边界按 0.75 倍率重算；加一条：余额够售价、不够原价时可以建任务。
   - 其他 e2e（p5 等）不涉及 Seedance 分组倍率，不用改。
3. 在本文件末尾「完成记录」下追加 BE-S 小节。

### QA

BE-S 完成后定向复测：p12 全部 SD 用例；模型广场 Seedance 分组显示售价（原价 × 0.75）；Nova 预估金额与实扣一致。

### 生产配置（主代理部署时执行）

- 「Seedance 视频」分组：倍率 0.75，视频独立倍率关闭，高峰倍率关闭。
- 渠道「视频」：按上表填原价。

## 补充：视频单价显示 4 位小数（2026-11-01）

原价 × 0.75 后部分档位小数很长（如 0.7091 × 0.75 = 0.531825）。用户决定：视频每秒单价在模型广场、qx 价格页、Nova 里**最多显示 4 位小数**（四舍五入，¥0.5318、¥0.5956、¥0.7446），与 change-01 的售价表一致。扣费仍按精确值。

- FE-S：`formatScaled` 增加 `maxFractionDigits` 参数、新增 `VIDEO_PRICE_DECIMALS = 4`；模型广场和 qx 价格页的视频单价（分档与单一价）传 4。图片、按次、token 价格不变。
- FE-N：Nova 的视频每秒单价（预估文案里的「× ¥单价」、设置页视频行单价）最多 4 位小数；预估总额也四舍五入到 4 位。

## 完成记录

### BE-S（2026-11-01）

#### 实现

- `InflightEstimateVideo` 改用与 OpenAI 实际扣费一致的视频倍率：基础倍率按分组模型倍率 → 用户专属倍率 → 分组倍率解析，再通过 `resolveVideoRateMultiplier` 应用视频独立倍率；不叠加高峰倍率。
- 新增单测覆盖：分组倍率 0.75、视频独立倍率覆盖普通倍率、分组模型倍率覆盖分组倍率、高峰时段不影响视频预估。
- `p12_seedance.py` 改为分组倍率 0.75、渠道填写 mmgate 原价；每项用量同时核对 `total_cost = 原价 × 计费秒数`、`actual_cost = total_cost × 0.75` 和 `rate_multiplier = 0.75`。
- SD-12/SD-19 按 0.75 倍率重算余额边界，新增余额 4（售价 3.75、原价 5）可建 720p/5 秒任务的用例。`CASES.md` 同步原价及倍率口径。

#### 文件清单

- `backend/internal/service/billing_inflight_reservation.go`
- `backend/internal/service/group_model_rate_billing.go`
- `backend/internal/service/billing_inflight_reservation_test.go`
- `/home/qlqq/workspace/sub2api-local/e2e/p12_seedance.py`
- `/home/qlqq/workspace/sub2api-local/e2e/CASES.md`（SD 小节）

未修改 mock；未触碰 frontend、Nova、`embed_on.go` 或 `embed_test.go`。

#### 测试命令和结果

```bash
cd /home/qlqq/workspace/sub2api/backend
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./internal/service ./internal/handler ./internal/server
go build ./...

cd /home/qlqq/workspace/sub2api-local
./build.sh --no-web && docker compose up -d
docker compose restart upstream-mock
cd e2e
E2E_RUN_DIR=/home/qlqq/workspace/sub2api-local/e2e/runs/20261101-071414-be-s-change02-final python3 p12_seedance.py
python3 report.py /home/qlqq/workspace/sub2api-local/e2e/runs/20261101-071414-be-s-change02-final
```

- 受影响包单测：service 217.493s、handler 43.064s、server 通过。
- `go build ./...` 通过；本机后端镜像构建、启动及 health 检查通过。
- p12：19/19 用例、175/175 原始断言通过。报告：`/home/qlqq/workspace/sub2api-local/e2e/runs/20261101-071414-be-s-change02-final/report.md`。
- p12 已核对示例：720p/5 秒 `total_cost=5`、`actual_cost=3.75`、`rate_multiplier=0.75`；2.5/1080p/6 秒 `total_cost=22.2`、`actual_cost=16.65`；4 余额可建售价 3.75 但低于原价 5 的任务；余额 33 低于售价预估 33.75 的 SD-12 被拦截。

#### 遗留问题

- 无未完成项。未跑全量单测或全量 e2e，未提交、推送或连接远端。

### QA（2026-11-01）

#### 总体结论：通过

本机 p12 的 Seedance 全部用例、模型广场折后分档价，以及 Nova 普通档/参考视频档的预估与实扣核对通过；无业务问题。本轮只使用本机 upstream-mock，未请求真实上游；未提交、推送或连接远端，也未做 Docker 全局清理。

#### 1. 本机重建与 p12

执行：

```bash
cd /home/qlqq/workspace/sub2api-local
./build.sh && docker compose up -d
docker compose restart upstream-mock
# 等待 http://127.0.0.1:28080/health 返回 ok
cd e2e
E2E_RUN_DIR=runs/20261101-qa-change02-072449 python3 p12_seedance.py
python3 report.py runs/20261101-qa-change02-072449
```

重建、启动、upstream-mock 重启及 health 均通过，日志 `/tmp/seedance-qa/change02/build.log`。p12 **SD-01..19：19/19 用例通过，175/175 原始断言通过**；日志及报告：

- `/home/qlqq/workspace/sub2api-local/e2e/runs/20261101-qa-change02-072449/p12.log`
- `/home/qlqq/workspace/sub2api-local/e2e/runs/20261101-qa-change02-072449/report.md`

数据核对覆盖每个成功任务的 `total_cost = 渠道原价 × 计费秒数`、`actual_cost = total_cost × 0.75`、`rate_multiplier = 0.75`。例如 2.0/720p/5秒为 `5 / 3.75 / 0.75`，2.5/1080p/6秒为 `22.2 / 16.65 / 0.75`。

余额预检边界通过：余额4（低于原价5，但高于售价3.75）允许 2.0/720p/5秒任务；余额33（低于售价预估33.75）拒绝 2.5/720p/30秒任务且未转发上游。`SD-19` 另核对指定档位及参考/自动时长下的余额边界。

#### 2. 模型广场折后价格

按指定方式执行：

```bash
PLAZA_GROUP=24 /home/qlqq/workspace/sub2api-local/e2e/ui/shots.sh \
  e2e-224011-sda072449@e2e.test /tmp/seedance-qa/change02/plaza /model-plaza
```

退出码0，截图 `/tmp/seedance-qa/change02/plaza-plaza-group-24.png`。人工核对两行「普通 / 带参考视频」及所有分辨率：

- 2.0 普通：0.531825 / 0.75 / 1.875 / 3.75；参考：0.59564325 / 0.9075 / 2.2575 / 4.665。
- 2.5 普通：0.74455575 / 1.125 / 2.775；参考：0.8615565 / 1.3575 / 3.3525。

均为原价乘分组倍率0.75，未按渠道原价直接展示。

#### 3. Nova 预估与实际扣费

`sub2api-local-nova` 已在运行，保持其现有容器；未改 Nova 业务代码。使用 p12 创建的用户、Key（ID 85）和分组（ID 24）。为让 Nova 的 `seedance-2.0/2.5` 插件模型名映射到 p12 mock 上游，在该隔离测试分组内补充了 `model_mapping` 测试账号及对应公开模型别名的原价定价；普通倍率仍0.75、视频独立倍率关闭、高峰倍率关闭。该测试配置未改业务代码。

Nova 界面普通档与参考档均提交至本机 mock 并完成：

| 档位 | Nova 预估 | usage_logs 核对 | 结果 |
|---|---|---|---|
| 普通 2.0/720p/5秒 | `预计 ¥3.75（5 秒 × ¥0.75）` | `total_cost=5`、`actual_cost=3.75`、`rate_multiplier=0.75`、档位 `720p` | 一致 |
| 参考 2.5/720p/成片5秒+参考1秒 | `预计 ¥12.2175（9 秒 × ¥1.3575）` | `total_cost=16.29`、`actual_cost=12.2175`、`rate_multiplier=0.75`、档位 `720p-ref` | 一致 |

SQL 核对证据 `/tmp/seedance-qa/change02/actual-cost-check.json`；UI 日志 `/tmp/seedance-qa/change02/nova-qa-retry.log`。截图：

- `/tmp/seedance-qa/change02/nova-ordinary-estimate.png`
- `/tmp/seedance-qa/change02/nova-reference-estimate.png`

参考视频使用本机1秒 `sample.mp4`，提示词中的 mock 参考秒数也设为1，因此账单计费秒数为 `5 + max(floor(1), ceil(5×2/3)) = 9`，用于对齐 UI 估算与 mock 实扣。首次探针沿用12.6秒提示词，但上传的是1秒样本，发现估算秒数与预期测试数据不匹配；未提交该次参考任务，改为与样本时长一致后重跑通过。这是测试数据对齐，不是产品缺陷。

#### 汇总与证据

- 构建：1套（sub2api）通过；Nova 已运行，无需重建。
- p12：19/19 用例、175/175 断言通过。
- 模型广场：普通/参考价及所有分辨率核对通过。
- Nova：3/3 检查通过（Key、普通档预估/实扣、参考档预估/实扣）。
- 问题：无。
- sub2api `pnpm-lock.yaml` SHA256 前后相同；未触碰业务代码。测试状态与日志保存在 `/tmp/seedance-qa/change02/`。

### FE-N

只改了 `nova-image-studio/frontend/`，没有提交，没有重建，没有截图。

**改动**

- `src/lib/embed/key-policy.ts`：
  - `formatScaled` 增加可选参数 `maxFractionDigits`。写法与 sub2api `utils/pricing.ts` 一致：先四舍五入到该位数，再走原来的 `toPrecision(10)` 和补零逻辑。
  - 新增常量 `VIDEO_PRICE_DECIMALS = 4`。
  - 设置页视频行的单价最多显示 4 位小数：「¥最低价 起 / 秒」，以及走到分档列表和单一价的情况。图片、token、按次价格的显示不变。
- `src/lib/embed/cost-estimate.ts`：`estimateVideoCost` 的单价和总额都用精确值计算，显示时各自四舍五入到 4 位小数，不用舍入后的单价去乘。例如 `预计 ¥2.6591（5 秒 × ¥0.5318）`。

**测试**

- 新增 fixture `VIDEO_PLAZA_LIST_PRICE`：用本文件的渠道原价，分组倍率 0.75。
- `video-cost-estimate.test.ts` 新增 4 条：
  - `formatScaled` 的舍入和补零；
  - 2.0 / 480p / 5 秒普通档 → `预计 ¥2.6591（5 秒 × ¥0.5318）`；
  - 2.0 / 480p / 5 秒带 3 秒参考视频 → `预计 ¥5.3608（9 秒 × ¥0.5956）`。总额是精确值 5.36078925 舍入的结果，不是 9 × 0.5956 = 5.3604；
  - 设置页：2.0 显示「¥0.5318 起」，2.5 显示「¥0.7446 起」，无分档时显示「¥0.5318 / 秒」。
- 已有用例（倍率 1、4 位小数售价，以及图片预估）没有改动，结果不变。
- `npx vitest run src/lib/embed src/lib/__tests__/plugin-schema.test.ts`：15 个文件、141 个测试全部通过。
- `npx eslint src/lib/embed src/components/embed`：通过。

### FE-S（2026-11-01，视频单价 4 位小数）

- `frontend/src/utils/pricing.ts`：`formatScaled` 增加 `maxFractionDigits`，新增 `VIDEO_PRICE_DECIMALS = 4`。
- `PlazaModelPricingTable.vue` 的 `paidRequestPrice`、`themes/qx/pricing.ts` 的 `paidUnit`：视频模型传 4 位，分档和单一价都适用；图片、按次、token 不变。可用渠道页显示的是渠道原价配置值，未改。
- 测试：`pricing.formatScaled.spec.ts` 加舍入用例；模型广场与 qx 的视频分档用例改为原价 × 0.75，期望值即 change-01 售价表。相关单测 21 个文件、202 个用例通过；`lint:check`、`typecheck` 通过；`pnpm-lock.yaml` 无变化。
- 重新构建本机环境后截图 `/tmp/seedance-fe-s/change02-plaza-group-24.png`（QA 的 p12 分组）：2.0 普通 ¥0.5318 / 0.75 / 1.875 / 3.75，参考 ¥0.5956 / 0.9075 / 2.2575 / 4.665；2.5 普通 ¥0.7446 / 1.125 / 2.775，参考 ¥0.8616 / 1.3575 / 3.3525。
