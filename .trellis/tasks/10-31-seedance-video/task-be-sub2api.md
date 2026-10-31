# BE-S：sub2api 后端任务书（codex gpt-6.1-sol high）

工作目录：`/home/qlqq/workspace/sub2api/backend`。另外负责本机环境里的 `~/workspace/sub2api-local/mock/upstream_mock.py`，以及 e2e 脚本 `p5_*.py` 和 `p12_seedance.py`。

开始前按 `prd.md` 的「共同规则」读取文档。接口以 `contract.md` 的 C1、C2、C3 为准。

现状：

- 本地提交 `90bff42b5` 加了 Seedance 透传与按秒计费（未推送）。
- 工作区里有未提交的改动，是把视频计费时长上限改为 30 秒：`video_billing_resolution.go`、`billing_service_test.go`。本任务把上限改为 60。
- 前端的「/ 秒」改动不属于你，不要动。

---

## A. 分组分润系数（C1）

### A1 设置项

- 在 `internal/service/domain_constants.go` 增加 `SettingKeyAffiliateGroupRebateFactors = "affiliate_group_rebate_factors"`。
- 新增类型 `AffiliateGroupRebateFactor{GroupID int64; Factor float64}`。
- 沿用现有 affiliate 设置的完整链路接入，参考 `affiliate_settle_interval_minutes` 的写法：
  - `settings_view.go`、`setting_parse.go`、`setting_update.go`
  - `handler/dto/settings.go`
  - `handler/admin/setting_handler.go`、`setting_handler_update.go`
  - 解析 JSON 时参考 `parseDefaultSubscriptions`。
- 保存时校验：
  - factor ∈ [0,100]；
  - group_id 存在，查分组仓库；
  - group_id 不重复。
  - 不满足时返回 400，错误信息用中文。
  - factor=100 的条目直接去掉；factor 四舍五入保留 2 位。
- GET 时附带 `group_name`。分组已经删除的条目照常返回，`group_name` 为空。
- 设置变更要写进现有的设置审计（如果 affiliate 设置有审计的话），跟其他 affiliate 字段保持一致。

### A2 结算

相关文件：

- `internal/service/affiliate_consumption_settle.go`
- `internal/repository/affiliate_repo_agent_level.go` 中的 `LockPendingConsumption`

做法：

- `AffiliatePendingConsumption` 增加 `WeightedDeltaCost`：窗口内 `SUM(ul.actual_cost * COALESCE(f.factor, 100) / 100)`。
  - 系数通过参数传入 SQL：`$3::jsonb`，再 `LEFT JOIN jsonb_to_recordset($3) AS f(group_id bigint, factor double precision) ON f.group_id = ul.group_id`。
  - `group_id` 为 NULL 时按 100 计。
- 系数由 service 层在每轮结算开始时读取一次，然后传给 repo。
- `ListUsersWithPendingConsumption` 不变。系数为 0 的消费同样要推进水位。
- 封顶规则保持「按真实充值额度」：
  - `eligibleRaw = min(DeltaCost, PaidTotal − SettledConsumption)`
  - `eligibleWeighted = WeightedDeltaCost × eligibleRaw / DeltaCost`（DeltaCost=0 时为 0）
  - 用 `eligibleWeighted` 调用 `distributeRewards`。
  - 水位的 `rebate_settled_consumption` 仍然累加 `eligibleRaw`。
  - `SettleConsumptionRebates` 返回的 `Consumption` 统计 `eligibleRaw`。
  - 如果有别的地方依赖「计入返利的消费」，写明它的口径。
- 先 grep 一遍 consumption 模式下所有「按消费 × 比例」的计算，确认需不需要乘系数，例如团队页的「预计返利」和管理端预览。
  - 需要就一并改。
  - 不需要的，在完成报告里说明原因。
- 充值基数模式不受影响。

### A3 团队请求明细

- `service.AffiliateTeamRequest` 增加 `RebateFactor float64 json:"rebate_factor"`。
- 用户端和管理端查看器两个接口都返回这个字段。
- 取值规则：按行的 `group_id` 查当前设置，查不到时为 100。
- 实现方式：在 service 层读取一次设置后填充，不改 SQL。

### A4 测试

单测（`-tags unit`）：

- 解析与校验：越界、重复、分组不存在、100 被去掉、四舍五入。
- `eligibleConsumption` 的加权与封顶：
  - 部分封顶时按比例折算；
  - 系数为 0 时返利为 0，但水位照常推进；
  - 混合分组。
- 团队请求明细的 `rebate_factor` 填充。

仓库集成测试：

- 在 `go test -tags=integration ./internal/repository/ -run Affiliate` 里补一条：两个分组各有消费、一个分组系数 30，断言 `WeightedDeltaCost`。

e2e：

- 在 `~/workspace/sub2api-local/e2e/p5_*.py`（CON 组）新增用例 CON-GF-xx：
  - 经管理端接口设置系数，直接写 `usage_logs`（两个分组，其中一个系数 30、一个 0）；
  - 等结算完成后核对返利流水金额 = 消费 × 系数 × 比例；
  - 校验非法输入返回 400。
- 同时把用例写进 `e2e/CASES.md`。
- 跑完把系数重置为空，避免影响 p6。

---

## B. Seedance 计费（C2）

### B1 时长上限

- `video_billing_resolution.go`：`VideoBillingMaxDurationSeconds = 60`。
- 改注释，说明 60 = 成片 30 + 参考视频 30。
- 相应修改 `billing_service_test.go` 的期望值。

### B2 建任务时解析

- 修改 `ParseSeedanceRequest`，填充：
  - `info.Resolution`：请求的 `resolution`，缺省 `720p`。
  - `info.DurationSeconds`：
    - 请求的 `duration`；
    - `-1` 或缺省时取模型上限：模型名（大小写不敏感）含 `2-5` 或 `2.5` 时为 30，否则为 15；
    - 同时还要考虑 `omni_reference_task_type=edit` 不传 duration 的情况：按模型上限预估。
  - 新增 `info.HasReferenceVideo`：`content` 中存在 `type=video_url`。
  - 新增 `info.Ratio`：请求的 `ratio`。
- 余额预估：
  - 在 `handler/gateway_inflight_reservation.go` 的 `grokMediaInflightEstimate` 中，Seedance 的视频估算时长 = 成片时长 + （有参考视频时）`ceil(成片 × 2/3)`，结果不超过 60。
  - 确认 `IsGenerationRequest` 包含 `SeedanceEndpointCreate`（已包含）。
- 建任务成功后，handler 会存 `GrokVideoPendingBilling`。
  - 给它加上 `HasReferenceVideo bool json:"has_reference_video,omitempty"`。
  - 存入时带上这个值，同时带上 resolution 和 duration（看 handler 现在怎么存的，跟着补齐）。

### B3 成功时计算计费秒数

- 在 `ForwardSeedance` 的 status 分支，`succeeded` 时读取：
  - `usage.completion_tokens`
  - `resolution`
  - `ratio`
  - `duration`
- 再结合 pending 里的 `HasReferenceVideo` 计算计费秒数。
- 计算放在 `prepareSeedanceCompletionBilling` 里，那里能拿到 pending；或者把 ratio 带到 result 上，放在 service 层算。具体放哪由你决定。
- 规则：
  - 无参考视频：`duration`。
  - 有参考视频：`round((tokens×1024/(w×h) − 1)/24)`，结果小于 `duration` 时取 `duration`，大于 60 时取 60。
  - 尺寸表查不到（未知 ratio 或分辨率）：按 `duration + ceil(duration×2/3)` 计算。这个值等于没有更长参考视频时的计费秒数下限。
- 计费秒数写进 `result.VideoDurationSeconds`，后续计费和 usage_logs 都沿用这个值。

尺寸表（`w×h`）：用方舟官方 Seedance 的标准尺寸，写成 `seedance.go` 里的一个 map：

| ratio | 480p | 720p | 1080p |
|---|---|---|---|
| 16:9 | 864×496 | 1280×720 | 1920×1080 |
| 4:3 | 752×560 | 1112×834 | 1664×1248 |
| 1:1 | 640×640 | 960×960 | 1440×1440 |
| 3:4 | 560×752 | 834×1112 | 1248×1664 |
| 9:16 | 496×864 | 720×1280 | 1080×1920 |
| 21:9 | 992×432 | 1470×630 | 2206×946 |

这张表**尚未用 mmgate 的 `/v1/models` 的 `token_frames` 核对**（七号机还没有 mmgate 账号）。代码里加一行注释，说明来源，并说明用户上线后会用真实任务核对。

### B4 测试

单测（`seedance_test.go`）：

- 无参考视频；
- 有参考视频时的三个文档示例（成片 5 + 参考 3 → 9；成片 5 + 参考 12.6 → 17；成片 10 + 参考 4 → 17），tokens 按 C2 的公式构造；
- 自动时长；
- 未知 ratio 的回退；
- 计费秒数上限 60；
- 解析 resolution 和 duration 的默认值；
- 余额预估。

上游模拟 `~/workspace/sub2api-local/mock/upstream_mock.py`：

- 按 C2 生成 `completion_tokens`、`ratio`。
- 参考视频秒数从提示词 `E2E_REF_SECONDS=` 读取。
- 成功响应的 `content.video_url` 改为 `http://sub2api-local-upstream-mock:18998/_media/sample.mp4`，由 mock 提供这个文件，供 Nova 下载结果（BE-N 会让 Nova 容器接入 `sub2api-local_default` 网络）。
  - 文件要求：浏览器能播放的小 mp4，几十 KB 以内，放在 `mock/` 下。
  - 生成方式：本机有 ffmpeg 就用它生成；没有就用 Python 写一个最小的合法 mp4，或者找现成的小样本。

e2e `p12_seedance.py`：新增 SD-08..：

- 参考视频任务按公式扣费；
- edit 模式不传 duration；
- 2.5 的 30 秒任务；
- 余额不足时建任务被预估拦截（余额小于估算值时返回 402 或 403，以现有行为为准）。

同时更新 `e2e/CASES.md` 的 SD 一节。

---

## C. Studio 反向代理（C3、C4）

文件：`internal/server/studio_proxy.go`（及其测试）。

1. `GET` / `HEAD` `/api/nova/public-media/*`：
   - 不经过 JWT，直接转发给 Nova；
   - 删除客户端带来的 `X-Sub2API-*` 头，不注入身份头；
   - 响应头加 `Cache-Control: private, max-age=3600`。
   - 注意路由注册顺序和 gin 的通配冲突：现在 `/api/nova/*` 是带 JWT 的路由组。
2. studio CSP（`studioContentSecurityPolicy`）增加 `media-src 'self' blob:`。
3. Nova 未配置时（`studio.nova_url` 为空），这条路径和其他 studio 路径一样返回 503 或现有行为。
4. 单测：
   - 不带 JWT 也能转发；
   - 伪造的 `X-Sub2API-User-Id` 被删除；
   - 其他 `/api/nova/*` 仍然需要 JWT；
   - CSP 含 `media-src`。

---

## D. 自检

- `go build ./...`
- 后端全量单测（命令见 `prd.md`）。
- `go test -tags=integration ./internal/repository/ -run Affiliate`
- 本机环境：`cd ~/workspace/sub2api-local && ./build.sh --no-web && docker compose up -d`
  - 如果 FE-S 同时在改前端，用 `--no-web` 避免构建半成品前端。
- 然后运行：
  - `cd e2e && E2E_RUN_DIR=... python3 p12_seedance.py`
  - p5 相关步骤（按 AGENTS.md 的顺序要求；需要时直接跑 `./run_all.sh`）。
- 不要在 e2e 里加远端目标。

## 完成报告

（在此追加）

## 约定变更

- 内部实现调整：团队请求 SQL 当前未返回分组 ID。为在 service 层按当前设置填充 `rebate_factor`，需在现有 SELECT/Scan 中补上分组 ID（不在 SQL 计算系数，不改变公开接口）。已删除分组返回空 ID，按 100% 处理。C1–C5 对外契约不变。

- 本机 SD-12 暴露既有通用在途预留允许「无其他在途时的首笔超额请求」。Seedance 创建请求增加完整预估余额检查，其他接口仍保留原预留语义；C2 对外规则不变。


## 完成报告（BE-S，2026-10-31）

### 实现结果

- A：新增 `affiliate_group_rebate_factors` 设置完整读写、中文校验、两位四舍五入、100% 条目移除、分组名回显与设置审计。PUT 缺字段保留原值。
- 消费结算每轮读取一次系数，SQL 汇总原始与加权消费；封顶后按原始消费比例折算加权基数。零系数同样推进水位，充值基数模式不变。
- 两个团队请求明细接口均按当前系数填充 `rebate_factor`，无分组及已删除分组为 100。
- B：计费上限 60 秒；解析模型默认/自动时长、分辨率、ratio 与参考视频；保存 pending；成功轮询时按 token/尺寸表反推计费秒数，保留原始 output_tokens。补充 Seedance 首笔请求完整预估余额检查。
- C：公开素材 GET/HEAD 免 JWT、不注入身份、剥离伪造身份头、保留 Range 和 Nova 响应头、设置 private 缓存；其他 API 继续鉴权。Studio CSP 增加 `media-src 'self' blob:`。
- Mock 按同一尺寸表生成用量与 ratio，支持 edit/自动时长；提供 2.2KB H.264 MP4（160×90、1 秒、支持 HEAD/Range），Chromium 实播验证通过。

### 消费口径检查

已搜索 service/repository/handler 中消费返利、预计返利及预览计算。实际按消费比例发放统一由 `distributeRewards` 完成，本次改其调用基数；团队页/管理端聚合读取已结算流水，没有另一条独立的预计分润计算需要修改。

- `rebate_settled_consumption`、本轮返回的 `Consumption`：封顶后的原始消费。
- 分润流水 `base_amount`：封顶并按分组系数折算后的加权消费，供流水对账。

### 文件清单

**sub2api 后端（37 个文件）**

- `backend/internal/handler/admin/setting_handler.go`
- `backend/internal/handler/admin/setting_handler_audit.go`
- `backend/internal/handler/admin/setting_handler_group_factors_test.go`
- `backend/internal/handler/admin/setting_handler_update.go`
- `backend/internal/handler/auth_email_oauth_test.go`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/handler/gateway_inflight_reservation.go`
- `backend/internal/handler/gateway_inflight_reservation_test.go`
- `backend/internal/handler/grok_media.go`
- `backend/internal/handler/seedance.go`
- `backend/internal/handler/seedance_test.go`
- `backend/internal/repository/affiliate_repo_agent_level.go`
- `backend/internal/repository/affiliate_repo_agent_level_integration_test.go`
- `backend/internal/repository/affiliate_signup_bonus_integration_test.go`
- `backend/internal/repository/affiliate_team_repo.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/server/studio_proxy.go`
- `backend/internal/server/studio_proxy_test.go`
- `backend/internal/service/affiliate_consumption_settle.go`
- `backend/internal/service/affiliate_group_rebate_factors.go`
- `backend/internal/service/affiliate_group_rebate_factors_test.go`
- `backend/internal/service/affiliate_service.go`
- `backend/internal/service/affiliate_team.go`
- `backend/internal/service/affiliate_team_test.go`
- `backend/internal/service/affiliate_two_tier_test.go`
- `backend/internal/service/billing_inflight_reservation.go`
- `backend/internal/service/billing_service_test.go`
- `backend/internal/service/domain_constants.go`
- `backend/internal/service/grok_media.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/payment_fulfillment_test.go`
- `backend/internal/service/seedance.go`
- `backend/internal/service/seedance_test.go`
- `backend/internal/service/setting_parse.go`
- `backend/internal/service/setting_update.go`
- `backend/internal/service/settings_view.go`
- `backend/internal/service/video_billing_resolution.go`

**本机测试环境**

- `/home/qlqq/workspace/sub2api-local/mock/upstream_mock.py`：参考视频 token、ratio、edit、素材下载模拟。
- `/home/qlqq/workspace/sub2api-local/mock/sample.mp4`：可播放的视频样本。
- `/home/qlqq/workspace/sub2api-local/e2e/p5_consumption.py`：CON-GF-01..06；结束清空系数。
- `/home/qlqq/workspace/sub2api-local/e2e/p12_seedance.py`：SD-08..12，含三个参考视频示例、edit、30/60 秒、余额拦截。
- `/home/qlqq/workspace/sub2api-local/e2e/CASES.md`：同步新增用例。
- 本任务书：记录内部实现调整与完成报告。

### 测试命令与结果

在 `backend/`：

```bash
go build ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags=integration ./internal/repository/ -run Affiliate
```

- `go build ./...`：通过。
- 后端全量单测：通过（service 215.272 秒，其余包通过）（日志 `/tmp/be-s-unit-verified.log`）。
- Affiliate 仓库集成测试：通过，含新增加权 SQL 用例（7.608 秒；日志 `/tmp/be-s-integration-final.log`）。
- `gofmt -l`、`git diff --check -- backend`：通过。
- Python `py_compile`（mock、p5、p12）：通过。

本机环境：

```bash
cd /home/qlqq/workspace/sub2api-local
./build.sh --no-web && docker compose up -d
docker compose restart upstream-mock
cd e2e
./run_all.sh --fast
export E2E_RUN_DIR="$PWD/runs/20261031-155653-be-s-final"
python3 p12_seedance.py
for stage in setup verify1 verify2 verify3; do python3 p5_consumption.py "$stage"; done
python3 report.py "$E2E_RUN_DIR"
```

- 本机后端镜像构建与容器更新通过，应用 healthy；沿用前端产物，没有构建并行修改中的前端。
- 最终 `./run_all.sh --fast`：**264/264 全部通过**，报告 `/home/qlqq/workspace/sub2api-local/e2e/runs/20261031-160220/report.md`，原始日志 `/tmp/be-s-e2e-fast-final.log`。
- fast 首轮 p0–p4、p7、p10 通过，SD-12 暴露首笔预留放行问题。修复后完整重跑 p12 与 p5：**76 条原始断言全部通过**（SD 55、CON 11、CON-GF 10）；按用例 ID 汇总为 29/29。
- 最终报告：`/home/qlqq/workspace/sub2api-local/e2e/runs/20261031-155653-be-s-final/report.md`；详细过程在同目录 `p12.log`、`p5.log`、`results.jsonl`。
- 结束已恢复 `affiliate_rebate_base=recharge`、`affiliate_group_rebate_factors=[]`。
- mock 视频 HEAD 返回 200、Range 返回 206；Playwright Chromium 播放得到 `width=160,height=90,duration=1,readyState=4,error=null`。

**首轮失败及处理（均已复测）**

```text
TestAPIContracts/GET_/api/v1/admin/settings:
+ "affiliate_group_rebate_factors": []
```

新增公开字段后原有两个 API JSON 快照未更新；已补齐，最终全量测试通过。

```text
[FAIL] SD-12: {"status": 200, "forwarded": 1}
```

通用预留会放行首笔超额请求；已为 Seedance 增加完整余额检查，复测为 `{"status":403,"forwarded":0}`。

```text
requests.exceptions.ConnectionError: ('Connection aborted.', ConnectionResetError(104, 'Connection reset by peer'))
```

首次 p5 启动早于重建容器就绪；等待 `/health` 成功后重跑，全部通过。

另一次全量测试出现已有内存断言波动：

```text
TestInflightEstimate_AccountMappingNoDBAndBoundedMemory:
"8441656" is not less than "8388608" (no per-model cache growth)
```

未修改阈值；单独运行 `go test -tags unit ./internal/service -run '^TestInflightEstimate_AccountMappingNoDBAndBoundedMemory$' -count=5` 连续 5 次通过（1.469 秒），随后再次运行完整单测全部通过（`/tmp/be-s-unit-verified.log`）。

### 遗留问题

- BE-S 实现范围无未完成项。
- 方舟尺寸表尚未用 mmgate `token_frames`/真实任务核对，代码已标注；上线后按任务约定复核。
- Nova 新插件、公开签名素材和结果下载的跨仓库联调由 QA 在 BE-N/FE-N 完成后统一验收。
- 没有修改 frontend、Nova 仓库或生产配置；未提交、未推送、未连接远端。


## 完成报告（BE-S，change-01 分档定价，2026-10-31）

- 直接采用变更文档 C6：视频基础价为普通 480p，其他档位放 `intervals[].tier_label/per_request_price`，含 `4k` 和 `<分辨率>-ref`；没有约定变更。
- 已实现 4k/别名识别、参考档计费/余额预估/usage、普通档回退；物理分辨率用于 token 反推后再生成计费档位。2.0 自动时长按 15 秒预估，新增回归覆盖。
- Mock 与后端使用同一张含 4k 的尺寸表；SD 渠道改用真实分档售价，新增 SD-13..19。

### 本次文件清单

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
- `change-01-pricing.md` 的「完成记录 → BE-S」及本任务书。

### 测试命令、结果与遗留

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
