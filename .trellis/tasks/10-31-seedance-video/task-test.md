# QA：本机全链路测试任务书（codex gpt-6.1-sol high）

在 4 个实现代理都写完「完成报告」之后启动。你的职责是**验证并报告问题**：

- 发现缺陷时，先写清楚复现步骤和证据。
- 修复范围很小、属于明显笔误或测试脚本问题时，可以直接修，并在报告里列出来。
- 涉及业务逻辑的修复，写进报告，由主代理派回给对应的实现代理。

开始前按 `prd.md` 的「共同规则」读取文档，再通读 `contract.md` 和 4 份任务书的完成报告。

## 1. 静态检查与单测

sub2api：

```bash
cd /home/qlqq/workspace/sub2api/backend
go build ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
go test -tags=integration ./internal/repository/ -run Affiliate

cd ../frontend
npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run
git -C .. status --short   # 确认 pnpm-lock.yaml 没有变化
```

Nova：

```bash
cd /home/qlqq/workspace/nova-image-studio/backend && env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY node --test
cd ../frontend && npx vitest run && npx eslint src/lib/embed src/components/embed
```

## 2. 本机环境

```bash
cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d
cd ~/workspace/sub2api-local-nova && ./build.sh && docker compose up -d
```

- 需要干净的库时，先运行 `~/workspace/sub2api-local/reset.sh`。
- 清理只能用 reset.sh 或 compose down -v。

## 3. e2e

- 运行 `cd ~/workspace/sub2api-local/e2e && ./run_all.sh`（完整模式，p0–p9）。
- 再运行 `E2E_RUN_DIR=$PWD/runs/latest python3 p12_seedance.py`。如果 run_all 不包含 p12，看 run_all.sh 的说明。
- 要求全部通过。失败的用例要贴出 `report.md` 中的对应行和相关日志。

## 4. 新增 Nova 视频 e2e

写 `e2e/p13_studio_video.py`，并在 `CASES.md` 增加「STV」一节。只连 127.0.0.1，并且要求 `sub2api-local-nova` 在运行。

准备：

- 通过管理端接口创建 Seedance 分组、上游账号和渠道视频定价，可以复用 p12 的 setup 写法；
- 渠道价格：
  - `seedance-2.0`：0.75
  - `seedance-2.5`：1.125
- 账号 `model_mapping` 把这两个名字映射到 mock 能识别的模型名；
- 注册用户、充值、创建该分组的 Key。

用例：

- STV-01：嵌入模式下 `GET /api/nova/plugins` 只返回 `qx-seedance`，不包含 ccode-h3。
- STV-02：经 sub2api 调用 `/api/nova/plugin-tasks` 提交文生视频（2.0、720p、5 秒）→ 轮询到 completed。
  - 确认 usage_logs 有一行，扣费 = 0.75 × 5。
  - 确认 `/api/nova/plugin-results/<id>/0` 能取到 mp4，并支持 Range。
- STV-03：上传参考视频，走 `plugin-media`；mode=reference，提示词带 `E2E_REF_SECONDS=12.6`，成片 5 秒。
  - 计费秒数 17，扣费 = 1.125 × 17（使用 2.5）。
  - 上游模拟收到的 `video_url` 是 `http://127.0.0.1:28080/api/nova/public-media/...` 形式的签名链接。
  - 不带 JWT 直接 GET 这个链接，返回 200。
  - 篡改 sig 或 exp 后返回 404。
- STV-04：用户 B 访问用户 A 的 plugin-task、plugin-results、预览接口，一律返回 404。
- STV-05：嵌入模式下匿名访问 `/api/nova/plugin-media/<file>` 返回 404，`/cdp` 返回 404。
- STV-06：保留期。把任务的 `expires_at` 改为过去的时间（直接改 Nova 的 sqlite，或用 BE-N 提供的测试手段），之后读取返回 404。
- STV-07：伪造请求体里的 `baseUrl`，上游模拟仍然只收到经网关转发的请求，不会访问伪造地址。

另外，分润系数的 e2e（CON-GF）和 Seedance 计费的 e2e（SD-08 及以后）由 BE-S 编写。你负责复跑，并检查它们的断言是否充分：

- 系数 30 时，返利金额是否等于 消费 × 0.3 × 11%（一级）和 × 7%（二级），差额是否正确；
- 系数 0 时，是否没有返利但水位照常推进。

## 5. 界面验收（Playwright，截图放 `/tmp/seedance-qa/`）

- 管理端「设置 → 返利」：添加、编辑、删除分组系数，预览数字正确，保存后刷新仍在，非法值不能保存。
- 代理团队请求明细：Seedance 分组的行显示「分润 30%」，其他行不显示。
- 模型广场：Seedance 模型显示「¥0.75 / 秒」「¥1.125 / 秒」。
- 创作工作台视频页：
  - 设置里选择视频 Key；
  - 六种模式的表单联动；
  - 参考视频超过上限时被拦截；
  - 价格预估数字正确（例如 2.5、成片 5 秒、参考 12.6 秒时显示 17 秒 × ¥1.125 = ¥19.125）；
  - 提交后能看到进度，完成后能播放、能下载；
  - 显示「保留 3 天」提示。
- 中英文切换后没有缺失的 i18n key。深色模式检查一张截图即可。

## 6. 报告

在本文件末尾追加「测试报告」：

- 每个阶段的命令、通过数和失败数；
- 失败项的详情（复现步骤、证据）；
- 你直接修复的项目；
- 需要实现代理修复的项目，标明归属：BE-S、BE-N、FE-S 或 FE-N；
- 截图清单。

## 测试报告（QA，2026-10-31，UTC）

### 总体结论：不通过

按第 1–6 节完成本机验证。**STV-05 不符合明确的 404 契约**，需 BE-S 修复后复测。视频生成、参考计费、分润、签名素材、结果鉴权、过期、播放下载及界面功能均通过。另记录两项既有质量问题：HTML 缓存重验证的 CSP nonce 不一致、Go 内存阈值测试波动。

日志和截图：`/tmp/seedance-qa/`。e2e 运行目录：
`/home/qlqq/workspace/sub2api-local/e2e/runs/20261031-165609/`。

### 1. 静态检查与单测

以下命令分别在任务书指定目录运行。Go 命令去除了代理和 `OPENAI_API_KEY`；前端只使用 pnpm 9。

| 命令 | 最终通过 | 最终失败 | 证据 |
|---|---:|---:|---|
| `go build ./...` | 1 命令 | 0 | `go-build.log` |
| `env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -count=1 -tags unit ./...` | 58 测试包 | 0 | 第二轮 `go-unit-retry.log`；首轮有 1 个失败测试，见问题 Q3 |
| 同上环境，`go test -count=1 -tags=integration ./internal/repository/ -run Affiliate` | 1 测试包 | 0 | `go-integration.log`，含加权消费 SQL |
| `npx -y pnpm@9 run lint:check` | 1 命令 | 0 | `sub-fe-lint-check.log` |
| `npx -y pnpm@9 run typecheck` | 1 命令 | 0 | `sub-fe-typecheck.log` |
| `npx -y pnpm@9 run test:run` | 3532 测试 / 444 文件 | 0 | `sub-fe-test-run.log` |
| Nova `env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY node --test` | 124 测试 | 0 | `nova-be.log`，无跳过 |
| Nova `npx vitest run` | 787 测试 / 85 文件 | 0 | `nova-fe.log` |
| Nova `npx eslint src/lib/embed src/components/embed` | 1 命令 | 0 | `nova-eslint.log` |

补充：`node plugin-runtime/verify.js` 的结构检查及六种模式 fixtures 全通过，见 `plugin-verify.log`。Python 修改文件 `py_compile` 通过，两仓库 `git diff --check` 通过；没有锁文件改动，也没有生成 `pnpm-workspace.yaml`。

静态阶段最终 **9/9 命令通过**。首轮 Go 单测失败没有删除或改写，单独复测与完整复测结果见 Q3。

### 2. 本机环境

依次执行：

```bash
cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d
cd ~/workspace/sub2api-local-nova && ./build.sh && docker compose up -d
```

**2 通过 / 0 失败**，日志 `build-sub.log`、`build-nova.log`。本轮使用完整前端构建，不是 `--no-web`。更新 mock 后只重启本机 `upstream-mock`。

收尾主服务、Postgres、Redis healthy，Nova running。应用和 mock 的宿主机发布端口仍为 127.0.0.1。本轮未清空数据卷，未执行任何全局 Docker 清理。

### 3. 完整 e2e 与 SD 复跑

```bash
cd ~/workspace/sub2api-local/e2e
./run_all.sh
E2E_RUN_DIR=$PWD/runs/latest python3 p6_team.py verify TREQ  # 修正旧契约断言后的定向复测
E2E_RUN_DIR=$PWD/runs/latest python3 p12_seedance.py
```

- 完整模式实际执行 p0–p9，以及 runner 原有的 p7、p10、p12；没有跳过 Nova 或 Open WebUI。
- 完整回归按用例 ID 汇总：**460 通过 / 0 失败**，含下述 TREQ 复测。`e2e-full.log`、运行目录的 `run.log` 保留首轮过程。
- 初次完整执行的原始断言为 **507 通过 / 2 失败**；两项均为旧测试未同步 C1 的 `rebate_factor` 字段，不是业务错误。修正后定向 TREQ **5/5 通过**（`treq-retry.log`），未重新初始化数据库。
- SD 单独复跑：**12 用例通过 / 0 失败，55 原始断言通过**。完整 runner 内也执行了同样 55 条，因此本轮 SD 合计 **110/110 原始断言通过**。日志 `p12-repeat.log`。
- 核对 SD-08 三个参考视频示例、edit 不传 duration、2.5 的 30 秒、60 秒计费上限、余额不足不转发、重复轮询不重复扣费；断言充分。

首轮测试脚本失败摘要：

```text
TREQ-01：expected 的字段集合没有 rebate_factor，actual 每行新增 rebate_factor=100。
TREQ-02b：model / group_alt / no_group / model_and_group 逐行比较均 false，原因同上。
复测 TREQ-01 / 02a / 02b / 03 / 04 全部 PASS。
```

注意：原有 `report.py` 同一用例 ID 取最后一条断言。本报告另核对了完整 `results.jsonl`，没有把原始历史失败当成不存在。

### 4. 新增 STV 与生产计划分润验证

新增 `e2e/p13_studio_video.py`，同步 `CASES.md`。命令：

```bash
E2E_RUN_DIR=$PWD/runs/latest python3 p13_studio_video.py
python3 report.py runs/latest > runs/latest/report.md
```

STV **6 用例通过 / 1 失败；原始断言 34 通过 / 1 失败**；脚本退出码 1。逐项：

| 用例 | 结果 | 核对内容 |
|---|---|---|
| STV-01 | 通过 | 插件列表仅 qx-seedance，无 ccode-h3 |
| STV-02 | 通过 | 2.0 文生视频 5 秒完成；单条 usage、¥3.75、重复查询不重复扣款；MP4、Range=206、private/no-store；终态起三天 |
| STV-03 | 通过 | 上传视频，2.5 成片5秒+参考12.6秒：计费17秒、¥19.125；mock 收到本站 public-media 签名 URL；匿名200/206；篡改sig/exp=404 |
| STV-04 | 通过 | B 读取/ack A任务、GET/HEAD结果、素材预览全404；A本人预览200 |
| STV-05 | **失败** | 匿名原始素材 GET/HEAD=401；登录原始素材及CDP=404，详见 Q1 |
| STV-06 | 通过 | 只将本轮测试任务 sqlite expires_at 改到过去；任务与结果立即404 |
| STV-07 | 通过 | 请求体伪造 baseUrl 被忽略；任务成功，mock 仅收到正常网关路径，没有伪造路径请求 |

CON-GF 原有 01–06：**10/10 原始断言通过**。原脚本使用 30%/20% 返利比例，已额外新增生产计划的 CON-GF-07/08，**4/4 原始断言通过**：

- 利润池11%、二级7%、分组系数30%、消费20：一级直客返利 **0.66 = 20×0.3×11%**；二级直客返利 **0.42 = 20×0.3×7%**，一级差额 **0.24 = 20×0.3×(11%−7%)**。
- 切为0%后独立新增消费10：两种代理链都没有新返利，原始消费水位仍从20推进到30。
- `finally` 恢复原设置；收尾确认 recharge、利润池30、二级20、分组系数空数组。

**上线配置要求**：Seedance 分组必须开启 `allow_image_generation`，STV setup 已照 p12 开启。生产另需按契约设置公共基址、三天视频保留期并持久化 Nova 数据目录；不使用本机 HTTP 下载开关。

加入 STV 和新增 CON-GF 后，总报告为 **468 通过 / 1 失败（469 用例）**。原始日志合计608条：605通过、3失败，其中2条为已修正并复测的旧 TREQ 断言，1条为仍未修复的 STV-05。

`report.md` 的失败原文：

```text
- **STV-05** 匿名原始素材 GET/HEAD、已登录原始素材和 CDP 均 404：
  预期 [404, 404, 404, 404]，实际 [401, 401, 404, 404]
```

### 5. Playwright 界面验收

使用本机无头 Chromium、中文字体环境；脚本与结构化断言写在 `/tmp/seedance-qa/`。最终结果：

| 子阶段 | 通过 | 失败 | 证据 |
|---|---:|---:|---|
| 管理设置 | 5 | 0 | `admin-ui-final.log`、`admin-ui-results.json` |
| 团队与模型广场 | 4 | 0 | `team-plaza-ui.log`、`team-plaza-ui-results.json` |
| 视频工作台 | 14 | 0 | `video-ui-final.log`、`video-ui-results.json` |
| 合计 | **23** | **0** | `summary.json` |

具体核对：

- 系数添加30%、修改12.5%、删除，均保存并刷新验证；11/7/30预览为 **3.3% / 2.1% / 1.2%**；101被就地提示，保存按钮禁用，程序触发表单提交也未发出 PUT。
- Seedance 行显示「分润30%」，同表其余4行（100%）不显示；英文为 `Rebate 30%`。
- 模型广场显示 **¥0.75 / 秒、¥1.125 / 秒**，英文 `/ second`；设置也显示每秒单价。
- 视频 Key 实际选择；六种模式素材槽与 edit 隐藏时长；2.5自动按30秒预估；2.5首帧固定比例6项禁用。
- 上传31秒参考视频时提交禁用，提示「总时长31秒，最多30秒」。
- 上传时长约12.6秒的素材、成片5秒、2.5预估为 **预计¥19.125（17秒×¥1.125）**。
- 提交后有处理中状态，完成后使用 **blob:** 播放，`readyState=4`、`paused=false`；从「操作→下载视频」实际下载MP4，文件头核对通过。视频文件为mock的1秒H.264样本。
- 保留3天文案存在；深色跟随父页。管理、团队及广场中英文均无缺失 i18n key。
- 后端校验的两项错误均在真实提交后清楚可见：**「参考生成至少需要一项素材」**、**「Seedance 2.0 不能仅使用参考音频，请添加参考图片或视频」**。
- 视频 Key 仅浏览器本地保存、Key名称「创作工作台-视频」均按已知约定验收，不列缺陷。familyDescription 已为「按秒计费」。

界面功能验收23/23通过；控制台另观察到刷新时的既有 CSP 错误，单独复现为 Q2，不与功能断言混记。两个预期的400是上述后端输入校验响应。

### 6. 待修复问题（归属及复现）

#### Q1 / P2 / BE-S：匿名原始素材应404，实际401（阻止本次验收通过）

复现：STV-03上传素材后，匿名 GET 或 HEAD `/api/nova/plugin-media/<file>`；本机实测 GET 返回：

```http
HTTP/1.1 401 Unauthorized
{"code":"UNAUTHORIZED","message":"Authorization header is required"}
```

同一资源携带本人JWT为404，签名 public-media URL可正常匿名读取。

证据：`p13.log`、`stv05-evidence.json`、e2e `report.md` 的 STV-05 行。`backend/internal/server/studio_proxy.go:69–76` 只对 ws/public-media跳过JWT，原始素材先被JWT层返回401，未到达Nova的404处理。

需 BE-S 对照 C4 / STV-05 统一该拒绝行为，并补 sub2api 代理层的匿名GET/HEAD用例；不能只验证Nova直连404。本轮未修改鉴权或业务逻辑。

#### Q2 / P2 / BE-S：既有HTML重验证CSP nonce不匹配

复现：打开 `/studio` 后刷新。连续3次刷新均出现 `Executing inline script violates ... script-src ... nonce ...`。HTTP独立复现：第一次GET=200，HTML内联 `window.__APP_CONFIG__` nonce与响应头一致；带该ETag再次GET=304，CSP响应头生成了新nonce，浏览器缓存的HTML仍带旧nonce。

定位：`backend/internal/web/embed_on.go:152–156` 的304分支。该文件本次没有工作区改动，属于既有问题；当前视频功能仍通过。应协调HTML缓存与CSP nonce，不能通过放宽CSP处理。

证据：`csp-probe.json`（浏览器3次错误和各次DOM脚本nonce）、`csp-http-evidence.json`（200→304及不同nonce）。额外质量探针 **0通过 / 1失败**；未改业务代码。

#### Q3 / P3 / BE-S：既有Go内存阈值单测不稳定

首轮全量单测：

```text
--- FAIL: TestInflightEstimate_AccountMappingNoDBAndBoundedMemory
billing_inflight_reservation_test.go:464
Error: "8477848" is not less than "8388608"
Messages: no per-model cache growth
```

证据 `go-unit.log`。随后 `go test -tags unit ./internal/service -run '^TestInflightEstimate_AccountMappingNoDBAndBoundedMemory$' -count=5` 连续5次通过（`go-memory-retry.log`），再跑全量58个测试包全部通过（`go-unit-retry.log`）。BE-S原完成报告也记录过同类波动。需改善该内存断言的稳定性；QA没有修改或调宽阈值。

### 7. QA直接修改的文件

仅测试环境和本任务书；没有修改两仓库的业务代码。

1. `/home/qlqq/workspace/sub2api-local/e2e/p13_studio_video.py`：新增STV-01..07、CON-GF-07/08，真实转发/计费/下载/权限/过期检查，按原始失败断言返回非零退出码。
2. `e2e/p6_team.py`：修正旧SQL对照，按当前分组系数附加 `rebate_factor`；无分组/已删除分组为100；同步字段白名单。TREQ定向5/5通过。
3. `e2e/p8_studio.py`：STU-13由旧「视频全部关闭」改为「任务入口输入校验400、原始素材404、CDP404」。本轮完整回归该项通过。
4. `mock/upstream_mock.py`：只扩展Seedance请求日志的content/duration/ratio/omni字段，用于证明签名素材URL和固定网关；不记录API Key，不改mock计费逻辑。
5. `e2e/CASES.md`：增加STV及生产计划分润用例，同步STU-13、TREQ-01。
6. 本文件：追加本报告。

临时Playwright脚本首轮存在新手引导遮挡、错误选择隐藏的「下载」按钮、试图点击已禁用保存按钮等测试脚本问题，均修正后完整复跑相应脚本。首轮日志保留为 `admin-ui-first-attempt.log` / `admin-ui-second-attempt.log`、`video-ui-first-attempt.log`；不计为产品缺陷。

### 8. 截图清单

均在 `/tmp/seedance-qa/`，已人工抽查设置、团队、播放和深色截图：

- 管理：`admin-group-preview.png`、`admin-invalid-factor.png`、`admin-en.png`。
- 团队：`team-rebate-30.png`、`team-en.png`。
- 价格：`plaza-video-price.png`、`plaza-en.png`。
- 视频Key：`video-settings.png`。
- 六种模式：`video-mode-t2v.png`、`video-mode-i2v.png`、`video-mode-firstlast.png`、`video-mode-reference.png`、`video-mode-edit.png`、`video-mode-extend.png`。
- 联动/价格：`video-ratio-25.png`、`video-over-limit.png`、`video-estimate-17s.png`。
- 生成/播放：`video-processing.png`、`video-completed.png`、`video-playback.png`；实际下载文件 `video-downloaded.mp4`。
- 错误提示：`video-empty-reference-error.png`、`video-audio-only-error.png`。
- 深色：`video-dark.png`。

未提交、未推送、未切换分支、未连接远端。所有测试设置已恢复，本机环境保持运行，供主代理复测。

### 主代理复核（2026-10-31）

- Q1 / STV-05：匿名访问原始素材地址，在 sub2api 的 JWT 层就被 401 拒绝，请求到不了 Nova，素材不会暴露。安全性质与 404 相同，所以不改鉴权代码，只修正约定 C4 和用例预期：匿名 401，登录后的原始素材地址与 CDP 返回 404。`p13_studio_video.py` 已复跑，STV 全部通过，`runs/latest/report.md` 中 0 失败。
- Q2 CSP nonce 与 HTML 304 不一致、Q3 Go 内存阈值单测偶发失败：都是这次改动之前就有的问题（upstream-maintenance-six 任务已记录 CSP/304 基线问题），不在本任务范围内，另行跟进。
- Q2 已经顺带修复，用户要求只跑单测：
  - 改动：`backend/internal/web/embed_on.go` 返回首页 HTML 时不再带 ETag，也不再回 304，每次都返回带当前 nonce 的完整 HTML。
  - 单测：`embed_test.go` 用新用例 `revalidation_gets_full_html_with_current_nonce` 替换了原来的 304 和 ETag 两个用例。
  - 结果：`go test -tags embed ./internal/web/` 通过；`-tags unit` 下 web、server 两个包通过；`go build`（带 embed 和不带）都通过。

## change-01 定向复测报告（QA，2026-10-31，UTC）

### 总体结论：通过

本轮分辨率分档、参考视频独立单价、4k、2.0 自动时长、2.5 素材新上限及 Q2 修复均通过。**没有发现本轮新增的业务缺陷。** 首轮 Go 单测再次出现既有 Q3 内存阈值波动；单独连续 5 次及 service 整包复测均通过，保留首轮失败记录，见下文。

只运行用户指定范围；没有运行 `run_all.sh`、其他阶段 e2e 或仓库集成测试。没有修改业务代码、提交、推送、连接远端或使用真实 mmgate Key。

证据目录：`/tmp/seedance-qa/change01/`。
本轮独立 e2e 目录：`/home/qlqq/workspace/sub2api-local/e2e/runs/20261031-185655-qa-change01/`（`p12.log`、`p13.log`、`results.jsonl`、`raw-summary.json`、`report.md`）。

### 1. 静态检查与单测

| 检查 | 最终通过 | 最终失败 | 日志（证据目录下） |
|---|---:|---:|---|
| sub2api `go build ./...` | 1 命令 | 0 | `go-build.log` |
| `go test -count=1 -tags unit -parallel=1 ./internal/service ./internal/handler ./internal/server ./internal/web` | 4 包 | 0 | `go-unit.log`，service 复测 `go-service-retry.log` |
| `go test -count=1 -tags embed ./internal/web/` | 1 包 | 0 | `go-embed.log` |
| sub2api `npx -y pnpm@9 run lint:check` | 1 命令 | 0 | `sub-fe-lint-check.log` |
| sub2api `npx -y pnpm@9 run typecheck` | 1 命令 | 0 | `sub-fe-typecheck.log` |
| sub2api `npx -y pnpm@9 run test:run` | 3541 测试 / 446 文件 | 0 | `sub-fe-test-run.log` |
| Nova backend `node --test` | 126 测试 | 0 | `nova-be.log`，无跳过 |
| Nova `node plugin-runtime/verify.js` | 结构检查及 9 fixtures | 0 | `nova-verify.log` |
| Nova frontend `npx vitest run` | 795 测试 / 85 文件 | 0 | `nova-fe.log` |
| Nova frontend `npx eslint src/lib/embed src/components/embed` | 1 命令 | 0 | `nova-eslint.log` |

**10 项命令最终 10 通过 / 0 失败；首轮 9 通过 / 1 失败。** Go 单测去掉 `http_proxy/https_proxy/HTTP_PROXY/HTTPS_PROXY/OPENAI_API_KEY`；Nova backend 单测去掉四个代理变量。`-parallel=1` 未消除 Q3 的进程级堆内存波动，也未修改阈值。

附加检查：修改的 p13 `python3 -m py_compile` 通过；两仓库 `git diff --check` 通过；构建与测试前后锁文件 SHA256 完全一致，没有生成 `frontend/pnpm-workspace.yaml`。证据 `locks-before.txt`、`locks-final.txt`、`sub-diff-check.log`、`nova-diff-check.log`。

### 2. 两套本机环境重建

分别在 `sub2api-local` 与 `sub2api-local-nova` 执行完整 `./build.sh && docker compose up -d`，**2 通过 / 0 失败**；日志 `build-sub.log`、`build-nova.log`。没有使用 `--no-web`。另重启本机 `upstream-mock` 加载新尺寸表。

应用 `/health` 正常；主服务、Postgres、Redis healthy，Nova running。发布端口仍只绑定 127.0.0.1，没有清空数据或进行全局 Docker 清理。

### 3. 定向 e2e

基线：复制 `runs/latest -> 20261031-165609` 的有效 `state.json` 到上述独立运行目录，复用其已完成 p0 的支付与初始化基线，不重复运行 p0 或全量阶段。新 SD/STV 分组、账号、渠道、用户及 Key 均由本轮 setup 新建。

```bash
cd /home/qlqq/workspace/sub2api-local/e2e
export E2E_RUN_DIR=$PWD/runs/20261031-185655-qa-change01
python3 p12_seedance.py
python3 p13_studio_video.py --video-only
python3 report.py "$E2E_RUN_DIR" > "$E2E_RUN_DIR/report.md"
```

| 阶段 | 用例通过 | 用例失败 | 原始断言通过 | 原始断言失败 |
|---|---:|---:|---:|---:|
| SD-01..19 | 19 | 0 | 170 | 0 |
| STV-01..08 | 8 | 0 | 46 | 0 |
| 合计 | **27** | **0** | **216** | **0** |

已逐条核对原始 `results.jsonl`，不是只看 report.py 同 ID 最后一条的结果。`--video-only` 跳过旧 CON-GF 分润用例，避免超出本轮范围。

关键结果：

- STV-02：2.0、720p、成片5秒，单条 usage 记 `720p`，扣 **5 × 0.75 = ¥3.75**。
- STV-03：2.5、720p、参考12.6秒、成片5秒，计费17秒，usage 记 `720p-ref`，扣 **17 × 1.3575 = ¥23.0775**；签名素材匿名读取及 Range 仍通过。
- 新 STV-08：2.0、4k、文生视频5秒，usage 记 `4k`，扣 **5 × 3.75 = ¥18.75**。
- 各成功任务均检查单次扣费、保留 output_tokens、重复轮询幂等及终态起三天保留期。
- SD 覆盖 4k 横竖屏/别名、4k-ref、两模型各分辨率普通/参考档、2.0 自动/缺省时长按15秒预估、图片/音频不触发参考档、缺参考档退回同分辨率普通价、所选档位余额拦截。4k-ref 17秒扣 **¥79.305**。
- STV-05 按主代理已修订的 C4 验收：匿名原始素材 GET/HEAD=401，登录原始素材及 CDP=404，通过；不再把旧 Q1 当作缺陷。

**上线配置要求不变**：Seedance 分组必须开启 `allow_image_generation`，本轮 SD/STV setup 都已开启。

### 4. Q2 / CSP 回归

复用并增强上一轮 `csp-probe`，脚本 `change01/csp-probe.cjs`，执行环境沿用 `/tmp/seedance-qa/runner.sh`。

**6 项探针通过 / 0 失败**：

- 登录后连续刷新 `/studio` 3 次：主文档与 Nova iframe 的 6 份 HTML 响应全部200，无304；主文档无 ETag，每次内联 `__APP_CONFIG__` nonce 与当前 CSP 一致，配置脚本正常执行。
- 独立普通 GET、带 `If-None-Match`、带 `If-Modified-Since` 的3次请求均200、无 ETag，返回完整 HTML 和当前 nonce。
- 浏览器 console/pageerror 为空，没有 nonce/inline-script CSP 错误。

证据：`csp-probe.json`、`csp-probe.log`、`csp-studio-reload.png`。**上一轮 Q2 已验证修复。**

### 5. 界面抽查

最终 **7 通过 / 0 失败**，见 `ui-results.json`、`ui-qa.log`；本轮浏览器 console/pageerror 为空（`ui-console.json`）。

| 检查 | 结果 |
|---|---|
| C01 渠道定价编辑器 | 下拉共8档；四个 `-ref` 正确显示「分辨率 · 带参考视频」；实际保存成功后重开，12条非基础 interval 仍在，保存原 tier_label；再查 API，14档售价与 C6 完全一致 |
| P01 模型广场 | 2.0 四分辨率×普通/参考、2.5 三分辨率×普通/参考，14项售价逐格核对通过 |
| P02 qx 价格页 | 分辨率逐行展示，各行普通与带参考视频售价，14项均与变更表一致 |
| N01 Nova 分辨率 | 2.0 可选4K；切2.5落回720p，4K项禁用并提示「当前组合下不支持」，不能选择 |
| N02 Nova 2.0 自动时长 | 可选「自动」，按15秒预估：**预计 ¥11.25（15 秒 × ¥0.75）** |
| N03 Nova 2.5 素材 | 实际选择30张本机PNG，显示「参考图片 (30/30)」，填提示词后可提交；参考音频上限显示10，后端单测/fixture亦覆盖新配额 |
| N04 Nova 4k 参考预估 | 2.0、4k、成片5秒；无参考时5×3.75=18.75；放入1秒MP4后为 **预计 ¥41.985（9 秒 × ¥4.665）** |

UI 的30张图和预估抽查没有提交生成任务。已人工查看渠道重开、模型广场、30图、4k参考预估等截图。

临时 UI 驱动前两轮有测试脚本问题，均保留日志且修正后完整重跑本轮7项：

- 首轮：新手引导延迟出现拦截渠道点击；断言误将「4K应不可选」写成「4K项必须不存在」（实际为禁用，符合用户要求）。
- 第二轮：展开卡片后动态过滤集合缩短，使用 `nth(i)` 导致漏定位第二张卡。
- 修正：等待引导出现后按 Esc；检查4K禁用状态；动态集合逐次取 first。最终 `ui-final=0`，7/7通过。证据 `ui-first-attempt*`、`ui-second-attempt*`，不计为产品缺陷。

### 6. 失败详情与待跟进项

**本轮新增业务问题：无。**

**既有 Q3 / P3 / BE-S：Go 堆内存阈值单测仍有波动。** 首轮即使使用 `-parallel=1` 仍失败1个测试：

```text
--- FAIL: TestInflightEstimate_AccountMappingNoDBAndBoundedMemory
billing_inflight_reservation_test.go:464
Error: "8545808" is not less than "8388608"
Messages: no per-model cache growth
```

同一轮 handler/server/web 均通过，其余失败记录没有删除或覆盖。随后执行：

```bash
# 均去除四个代理变量及 OPENAI_API_KEY
go test -count=5 -tags unit ./internal/service \
  -run '^TestInflightEstimate_AccountMappingNoDBAndBoundedMemory$'
go test -count=1 -tags unit -parallel=1 ./internal/service
```

单独 **5/5通过**，service整包 **通过（220.308s）**，所以受影响4包最终均通过。证据 `go-unit.log`、`go-memory-retry.log`、`go-service-retry.log`。未调宽阈值、未改业务实现；建议 BE-S 在独立质量任务中稳定该进程级内存断言，不作为本轮新增回归。

### 7. QA 本轮直接修改

1. `/home/qlqq/workspace/sub2api-local/e2e/p13_studio_video.py`：渠道改为 C6 的14档售价，2.5映射改为260628；STV-03期望改为23.0775；成功任务补 usage 档位检查；新增 STV-08；增加 `--video-only`，默认运行方式仍保留分润用例。
2. 同目录 `CASES.md`：同步新价格、STV-08、有效p0基线及定向命令。
3. 本任务书追加报告；临时 Playwright 驱动与证据仅放 `/tmp/seedance-qa/change01/`。

没有改动两个仓库的业务代码。全局返利仍为原来的 recharge 基数、30/20、分组系数空数组；渠道 UI 保存前后所有售价一致，见 `final-data-check.json`。两套本机服务保持运行。

### 8. 截图清单

均在 `/tmp/seedance-qa/change01/`：

- `channel-8-tiers.png`：渠道档位编辑；完整8项值/显示名另见 `ui-results.json`。
- `channel-saved-reopened.png`：保存成功后重开，参考档与价格保留。
- `model-plaza-tiers.png`：模型广场两模型14档售价。
- `qx-pricing-tiers.png`：qx价格页分辨率与普通/参考价。
- `nova-25-no-4k.png`：2.5的4K项禁用。
- `nova-20-auto-duration-options.png`、`nova-20-auto-duration-estimate.png`：2.0自动时长与15秒预估。
- `nova-25-30-reference-images.png`：30张参考图和音频上限10。
- `nova-20-4k-ref-estimate.png`：1秒参考+5秒成片、4k-ref预估41.985。
- `csp-studio-reload.png`：CSP三次刷新后的工作台。
