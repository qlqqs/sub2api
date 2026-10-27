# 通用维护补丁首批依赖评估

- 记录日期：2026-10-27（Asia/Shanghai）。
- 当前任务：`.trellis/tasks/10-27-upstream-sync`，状态 `planning`。
- 授权边界：用户只批准创建任务和持久化研究；本文不是实施批准。
- 本次落盘仅限本文件；PRD/design/implement 由主代理维护，台账研究由另一代理维护。
- 范围：纯上游 main 同步的候选基线、custom 选择性通用安全／可靠性／计费修复评估。
- 排除：所有 Terra、模型接入／更新、模型能力和价格族支持；不借本次修复改变模型定价或补齐模型观测功能。

## 1. 基线与证据有效性

以下是此前只读审查的历史观测，不是本次重新查询后的现状保证：

| 对象 | 已观测值 | 含义 |
|---|---|---|
| 上游 GitHub API main HEAD | `ba81c7fd758e`，观测日期 2026-10-27 | 独立于本地 refs 的远端观测；不是持续有效的“最新”声明 |
| 本地 `main` | `8f2cbb53d9d1c40e1865b07ac460571f7d4c48de` | 审查时尚未同步的本地分支 |
| 本地 `upstream/main` | `dba770d56a0ee0ae5bad1b3dc5e11614d323163c` | 陈旧的 remote-tracking ref，不能代替 API HEAD |
| custom HEAD | `98894b242c672937173f5af630eb65e68c565c6d` | 本轮候选补丁适配检查的历史代码基线 |
| 工作树 | 当时观察到 69 项未提交变动 | 并行工作快照，不代表现在仍是 69 项或文件内容未变 |

`ba81c7fd758e`、`8286268047cb`、`c591412c42ab` 在审查时不在本地对象库。后两个候选提交通过 `Wei-Shaw/sub2api` 官方 GitHub commit API 的只读 GET 核对了 SHA、父提交和文件补丁；没有 fetch。**最终同步目标尚未 fetch，也尚未完成本地对象／祖先链核实。** 实施阶段须重新冻结明确目标，不能将这些历史 refs 当作已同步结果。

证据来源：本地 `git show`／`git diff`、代码读取、只读 `git apply --check`，以及已完成的三份 Paseo sidecar：

- 安全：`7e3798e1-c845-48a5-84a6-3f0b56762d61`。
- overload：`bdcae1d0-2e41-4287-ac05-c104f9742d80`。
- Anthropic：`8e7a57b9-ad74-450c-9be1-6e0dfd918a33`。

三位子代理均已归档；本次没有新建子代理或重新研究远端。

## 2. 批次选择与依赖摘要

这些是建议的独立审查／验收单元，不表示应打成一个提交或一起部署。

| 批次 | 源提交 | 依赖与建议 | 历史文本适配结果 |
|---|---|---|---|
| M1 安全与构建一致性 | `cacc80b8620e`、`4c940c3cb706`、`59100d324e9f`、`8286268047cb` | 先 cacc，再对齐 CI 与根 Dockerfile，最后 DOMPurify；版本链作为完整单元验收 | cacc、4c 通过；591 因定制 Node 镜像行造成上下文不匹配；828 已审阅远端补丁，未据此声称完整安全链检查通过 |
| M2 模型限流快照 COW | `38b72d734e0a` | 可独立选入，不需引入 scheduler memo 性能优化 | 通过 |
| M3 Cloudflare 1010 账号处罚修复 | `e0fa4b94f9b9` | 可独立选入，已有基础 403 处理逻辑 | 通过 |
| M4 overload 首输出分类 | `e068c3d02ff8` | 可独立选入，不依赖 c591；作为后续空内容流修复的合理行为基线 | 通过 |
| M5 Anthropic 兼容流断开／异常计费 | `94c7d6b025a4` → `4355236e0023` | 必须成对，独立验收；未发现其他缺失代码前置 | 94c 单独通过；435 单独失败；两提交组合通过 |
| D1 后续候选，暂缓 | `c591412c42ab` | 不整包选入。空内容流和 Retry-After 可拆分复审；身份切换单独决策 | sidecar 对整包检查通过，但不代表适合覆盖并行工作或行为已验证 |

**推荐首批为 M1–M4；M5 单列计费敏感批次；D1 暂缓。** M2/M3/M4 之间没有发现相互依赖；安全版本升级也不是这些小型 Go 逻辑修复的已发现代码前置。

## 3. M1：安全链与构建入口范围

### 改动及顺序

1. `cacc80b8620e`：`go.mod` 从 1.27.2 对齐到 1.27.3；security workflow 断言同步；前端增加 follow-redirects 安全覆盖并把锁定版本从 1.16.0 升到 1.16.1。
2. `4c940c3cb706`：backend CI 两处及 release 一处 Go 版本断言对齐。
3. `59100d324e9f`：根 Dockerfile 的 Go 镜像对齐到 1.27.3。
4. `8286268047cbbca4e7f5fbb63ff0890c1ea37cf5`：DOMPurify 直接依赖、override、锁文件从 3.4.14 对齐到 3.4.17，含 mermaid 的传递依赖。其补丁上下文已经有 follow-redirects override，所以安排在 cacc 之后；这不是 DOMPurify 运行时依赖 follow-redirects。

四提交最小文件集合共 7 个：

- `backend/go.mod`
- `Dockerfile`
- `.github/workflows/security-scan.yml`
- `.github/workflows/backend-ci.yml`
- `.github/workflows/release.yml`
- `frontend/package.json`
- `frontend/pnpm-lock.yaml`

根 Dockerfile 检查失败来自 custom 已固定 `NODE_IMAGE=node:24.22.1-alpine`，而源补丁上下文是 `node:24-alpine`。后续只对齐 Go 镜像，不覆盖 Node pin、`BUILD_TYPE=custom` 等定制行为。

### 尚需明确的工具链／扫描边界

- sidecar 当时观察到宿主机 `go version` 为 1.25.10。后续须核实构建／测试实际选用的 1.27.3 工具链及自动下载条件，不能把宿主机版本等同于模块最终执行版本；本次未安装或下载工具链。
- `backend/Dockerfile:1`、`deploy/Dockerfile:10` 当时也固定 1.27.2，但不在这四个源提交的修改范围。是否要求所有构建入口一致，需显式决定并作为本地适配记录，不能假称上游补丁已经覆盖。
- custom 镜像 workflow 使用根 Dockerfile，Go 设置读取 `backend/go.mod`；保留 `.github/workflows/custom-image.yml` 和 `custom-security.yml` 的现有职责。本批不擅自改变扫描是否阻断镜像发布的策略。
- `.github/audit-exceptions.yml` 当时存在已过期条目；升级依赖不等于独立安全扫描必定通过。不要自动延长例外来掩盖失败。
- pnpm 必须使用 9；依赖解析不得顺手升级无关包或使用全局 pnpm 12 改写锁文件格式。

拟验证（未执行）：冻结安装；前端 lint/typecheck/test；生产依赖审计及例外检查；Go 单测、必要集成测试、构建与 govulncheck；根镜像／CI 版本一致性；项目约定的本机编译与运行验证。

## 4. M2：快照 COW

源：`38b72d734e0aa92ce7547a5ea8618abb7290cf98`。

文件集合：

- `backend/internal/service/model_rate_limit.go`
- `backend/internal/service/model_rate_limit_snapshot_cow_test.go`（新增）

`setAccountModelRateLimitSnapshot` 复制 Extra 顶层与 `model_rate_limits` map，再替换当前 Account 的 Extra，避免原地修改其他副本共享的 map。既有调用方与标准库能力已在代码基线具备。

**依赖方向不能倒置：** 后续 `53002f677` 调度快照常驻缓存优化建立在 COW 后，并给测试增加 `PrepareSharedSnapshot()`；本批不引入该性能优化，也不从最新文件整体覆盖而误带其新增依赖。使用 38b 自身的测试内容。

拟验证：CopyOnWrite、MissingOrInvalidLimits、ConcurrentCopiesOfSharedMaster、FableLimitStaysOnOwnSnapshotCopy 等测试，特别使用 `-race`；兼顾 Cline／图片余额等既有调用方的快照读取行为。这只是共享 map 副本隔离，不承诺多个 goroutine 同时修改同一 `*Account` 字段安全；调用方对象所有权仍是审查边界。

## 5. M3：Cloudflare 1010

源：`e0fa4b94f9b962949da039c09e63968358324e26`。

文件集合：

- `backend/internal/service/ratelimit_service.go`
- `backend/internal/service/ratelimit_service_403_html_test.go`

将 1010 的请求／边缘级拦截豁免前移到通用 403 入口，并识别结构化 JSON 中的 Cloudflare 标志／browser_signature_banned。保留真正凭据或权限错误的账号处罚，以及已有平台专属策略。与 custom 响应模型、citations、图片流没有直接代码重叠。

拟验证：各平台 HTML 1010、JSON 1010、嵌套数值／字符串 code；普通业务 code=1010、仅在 message 提及 1010、畸形 JSON 不被宽泛豁免；Anthropic 真正 403 与 OpenAI 结构化 403 继续处罚，且豁免路径不增加 strike 或临时冷却。

## 6. M4：overload metadata

源：`e068c3d02ff8`。

文件集合：

- `backend/internal/service/openai_gateway_passthrough.go`
- `backend/internal/service/openai_capacity_shed_test.go`
- `backend/internal/service/openai_ws_http_bridge_test.go`

把 `response.queued`、`response.metadata` 加入非语义输出事件集合，避免这些事件解除首输出超时或使后续 overload 丧失安全重试资格。已有首输出判定、容量降载和请求级 failover 基础存在；不需要 c591 的身份或 Retry-After 改动。

拟验证 native Responses／passthrough／WS HTTP bridge：metadata 与 keepalive 后 overload 可以重试；实际文本、有效工具输出、图片 partial、citation／未知事件不得被扩大归类为空输出。独立 Images 流保持独立实现，不顺带重构。

## 7. M5：Anthropic 配对修复与 exactly-once 计费

源提交顺序：`94c7d6b025a4` → `4355236e0023`。两者之间没有其他提交；sidecar 确认 94c 父提交已在 custom 历史中，未发现额外缺失代码前置。

- 94c：客户端断开后停止向下游写，继续读取上游最终 usage。
- 435：在前项之上补齐断流／超时／流内错误收尾、共享读取泵和 handler 带结果错误路径的入账。
- `git diff 94c7d6b025a4^ 4355236e0023 | git apply --check` 当时通过。435 单独检查失败包括缺失 94c 新增测试文件与代码上下文，不应误报为已证实的 custom 冲突。

完整组合文件集合（13 个）：

- `backend/internal/handler/gateway_compat_stream_usage_test.go`
- `backend/internal/handler/gateway_handler_chat_completions.go`
- `backend/internal/handler/gateway_handler_responses.go`
- `backend/internal/service/gateway_forward.go`
- `backend/internal/service/gateway_forward_as_chat_completions.go`
- `backend/internal/service/gateway_forward_as_chat_completions_ping_test.go`
- `backend/internal/service/gateway_forward_as_responses.go`
- `backend/internal/service/gateway_forward_compat_client_disconnect_test.go`
- `backend/internal/service/gateway_forward_compat_stream.go`
- `backend/internal/service/openai_gateway_anthropic_native_pump.go`
- `backend/internal/service/openai_gateway_anthropic_native_pump_test.go`
- `backend/internal/service/openai_gateway_chat_completions_anthropic_native.go`
- `backend/internal/service/openai_gateway_responses_anthropic_native.go`

### 定制兼容及验收要求

- 四个兼容／native 转发 service 文件已有 custom 缓冲响应 citations 保留逻辑，必须保留，不能用上游整文件覆盖。文本适配通过不代表这些状态在错误收尾中仍正确。
- 现有 `chat_gateway_citations_test.go`、`pkg/apicompat/chat_citations*_test.go` 与无引用 golden 测试需回归；新增组合场景应覆盖 citations + 断开／失败帧，引用输出不丢失、不重复，无引用输出不变。
- `client_response_model.go` 及其测试是回归保护面，不是本批修改目标。覆盖已写 header 尚无 body、写失败、context 取消时的模型包装、failover 和终止事件行为；输出模型还原不得改变路由／计费输入。
- 错误路径须恰好入账一次：handler “带结果错误也入账”影响 Gemini／Antigravity 等共享分支，不能只测 Anthropic；防止先记账后再次走正常记账、重复写错误帧或重放已交付输出。
- 覆盖最终 usage 排水、上游 EOF 缺终止事件、显式 error、idle timeout、超长行、正常 terminal、断开时限及上下游连接释放。已失败的流不得补造成功完成事件。
- 图片流没有发现直接文本冲突，但 Responses handler 的图像意图、超时／断开和计费须回归；保留 `openai_images_b64_backfill_test.go` 的流式 URL→b64 行为。

**明确排除既有缺口：** 兼容流 `resultWithUsage` 未填充 `UpstreamResponseModel`／冲突标记是已识别的既有基线缺口，不由本次补齐，不扩展到模型观测或模型定价工作。验证目标是“不引入回归”，不是在本批实现新观测。

## 8. D1：暂缓 c591 的拆分事项

源：`c591412c42ab195dacdede9eb19f9cbfee221144`；父提交为历史本地 `upstream/main` 值 `dba770d56a0ee0ae5bad1b3dc5e11614d323163c`。该父关系不构成必须引入所有中间上游功能的代码依赖。

拆分候选：

1. 空内容 delta/done 的安全首输出判定。
2. overload Retry-After（header/body、秒／毫秒）解析与最多 8 秒延迟。
3. 默认及已配置的 Codex TUI→CLI 出站身份归一化。

本轮三项均不进入首批。前两项可后续另行确认，第三项需独立决策，不能借通用 overload 修复悄然更改管理员已配置身份。

可靠性核心最小源文件集合：

- `backend/internal/service/openai_gateway_passthrough.go`
- `backend/internal/service/openai_gateway_upstream_errors.go`
- `backend/internal/service/openai_capacity_shed_test.go`

若未来批准身份部分，整包其余源文件还包括：

- `backend/internal/pkg/openai/request.go`
- `backend/internal/pkg/openai/request_codex_identity_normalization_test.go`
- `backend/internal/service/openai_codex_identity.go`
- `backend/internal/service/openai_codex_identity_test.go`
- `backend/internal/service/openai_codex_version_sync_service_test.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_oauth_passthrough_test.go`
- `backend/internal/service/openai_setup_token_compat_test.go`
- `backend/internal/service/openai_ws_forwarder_success_test.go`
- `backend/internal/service/setting_gateway_runtime.go`
- `backend/internal/service/settings_view.go`
- `frontend/src/i18n/locales/en/admin/settings.ts`
- `frontend/src/i18n/locales/zh/admin/settings.ts`

审查时后面三个 settings 文件已有并行未提交改动；即使整包 apply-check 通过，也不宜原样落地。已有 transient／SameAccountRetryDelay／容量识别／身份配对函数，不存在已发现的必须先整批引入的新基础函数。

待决：是否接受 Retry-After 对延迟的影响；是否有足够实际证据支持身份策略改变；若拆取空内容判定，需补 WS bridge 与有效工具／图片输出禁止重放测试。这些问题不阻塞 e068 独立选入。

## 9. 检查结论与实施门槛

- 所有 apply-check 是历史工作树上的**只读文本适配检查**，不检查完整编译依赖、运行行为、竞态或计费正确性。
- 本轮审查没有运行测试、构建、依赖安装、安全扫描、容器或部署验证；没有修改产品代码、Git refs，也没有 checkout/stash/fetch/commit/push。
- 本次仅将既有证据持久化到本文件。实施前重新核对并行修改与目标基线，不回退、覆盖或代为提交他人改动。
- 后续验证遵循项目约定：Go 单测清除代理和 OPENAI_API_KEY；前端使用 pnpm 9；先验证隔离本机环境。是否提交、推送、部署仍需用户同意。
- 本文供主代理计划与台账研究引用，不创建或更新独立账本，不把“已审查”“已选入”“已验证”“已部署”混为同一状态。
