# PoC-C（0.5）：联网搜索与生图经网关（2026-10-24）

源码：sub2api `backend/internal/`（下文省略前缀）；Open WebUI v0.12.3 worktree `~/workspace/owui-poc/wt-c`（分支 `sub2api-poc-c`，下文路径省略 `backend/open_webui/`，行号为改动后）。
本机环境：sub2api-local（127.0.0.1:28080）+ upstream-mock（扩展见末节）；PoC 账号、分组、Key 均以 `poc-chat-` 开头。
脚本与原始记录：`~/workspace/owui-poc/c/`（`setup.py` 建账号，`gw_test.py` 测网关，`owui_chat.py` 驱动 Open WebUI，`gw/*.json`、`owui/*.json` 是抓到的请求和响应）。

## 结论

| 项 | 结果 |
|---|---|
| 开关 → `tools` 加 `{"type":"web_search"}`，Open WebUI 自己不搜索 | **通过**（3 个文件、+29 行；与原生函数调用的 22 个内置工具正确合并） |
| 网关把 web_search 转给上游 | **通过**：Anthropic 路径转为 `web_search_20250305`，OpenAI 路径原样 `{"type":"web_search"}` |
| 引用 / 来源显示在 Open WebUI | **Chat Completions 路径不通过**：网关在 Anthropic→CC、Responses→CC 两条转换里都把引用丢了，客户端只拿到正文。<br>**OpenAI 分组改走 `/v1/responses`（连接 `api_type=responses`）则已通过**：网关透传 `url_citation`，Open WebUI 存进 `sources`，显示「N 个来源」。 |
| 网关模拟搜索（emulation） | **对本方案不起作用**：只挂在 `/v1/messages` 入口；要求 `tools` 恰好只有一个 web_search；需要 Brave / Tavily Key（服务端出网）；产出的是「搜索结果列表」而不是模型回答 |
| 搜索计费 | **未单独计费**：只按 token 计费；按次搜索费只对 Grok 生效。上游的搜索费（Anthropic `web_search_requests`、OpenAI web_search_call）由平台承担 |
| 生图（`/api/v1/images/generations` 与对话内生图） | **通过**：到达网关 `/v1/images/generations`，使用配置的 Key，按张计费；b64_json 正常入库 |
| 生图返回 URL | 网关原样透传 URL。Open WebUI 服务端会去下载：私网地址被 SSRF 校验拦下，报 400，但该次**已计费**；公网 URL 则是一次服务端出网 → fork 必须只接受 b64 或网关 origin |

**建议**（已验证）：
- 一期按分组平台分流：
  - OpenAI 平台分组的连接设 `api_type=responses`，引用开箱可用。
  - Anthropic 平台分组仍走 Chat Completions，在网关补引用映射（提议 G1）；补上之前只显示正文、没有来源。
- 搜索按次计费（提议 G3）上线前，联网搜索开关默认关闭，或只对有余额的 Key 开放；否则平台要承担上游搜索费。

## 1. 网关侧

### 1.1 请求方向（实测，`gw/*.json` 里有网关发往上游的完整请求体）

- CC→Responses 保留 web_search：`pkg/apicompat/chatcompletions_to_responses.go:530-532`。
- Responses→Anthropic 转为 `{"type":"web_search_20250305","name":"web_search"}`：`pkg/apicompat/responses_to_anthropic_request.go:726-730`。
  - 不带 `max_uses`。
  - 上游一律强制 stream：`service/gateway_forward_as_chat_completions.go:80-82`。
- Anthropic 分组：`/v1/chat/completions` 走 `service/gateway_forward_as_chat_completions.go:29`，链路为 CC→Responses→Anthropic；上游收到 `tools=[web_search_20250305]`。与 function 工具混用时两者都保留（`anth_ws_fn_nonstream`）。
- OpenAI API Key 分组：走 `service/openai_gateway_chat_completions.go:67`，按 `resolveUpstreamProtocolFor` 选 Responses 上游。
  - 上游 `/v1/responses` 收到 `tools=[{"type":"web_search"}]`；混入 function 时为 `[web_search, function…]`。
- 注意：如果 OpenAI 账号走「CC 直转」（探测为不支持 Responses、国产厂商等），web_search 会被丢弃（`pkg/apicompat/chatcompletions_responses_bridge.go:79-81,1165-1166`），没有任何提示。

### 1.2 响应方向：引用在 CC 响应里的形状（实测）

两条路径返回给 `/v1/chat/completions` 客户端的都只有正文，**没有 `annotations`，也没有搜索结果**。

Anthropic 路径（mock 返回：text → server_tool_use → web_search_tool_result → 带 citations 的 text…）：

```
非流式 content: "I'll search for that.According to the search results, Fact A comes from source A and fact B comes from source B."
流式：同样的文本分成多个 delta.content，没有 annotations
```

- 非流式 `pkg/apicompat/anthropic_to_responses_response.go:53-94` 只处理 thinking / text / tool_use；`server_tool_use`、`web_search_tool_result` 和 text 上的 `citations` 全部丢弃。
- 流式 `:312-412`（content_block_start）、`:414-470`（delta）同样没有处理 `citations_delta`。
- `AnthropicContentBlock`（`pkg/apicompat/types.go:56-83`）没有 `Citations` 字段。
- 小问题：搜索前后的两段文本被直接拼接，中间没有分隔（`that.According`）。

OpenAI 路径：

- 正文里模型自己写的 markdown 链接 `([example.com](https://…))` 保留下来，能当普通链接显示。
- `annotations` 丢失：
  - `pkg/apicompat/responses_to_chatcompletions.go:65-67` 直接丢弃 `web_search_call`；
  - `ResponsesEventToChatChunks`（`:151` 起）不处理 `response.output_text.annotation.added`；
  - `ResponsesContentPart`（`types.go:317`）、`ChatMessage`（`:695`）、`ChatDelta`（`:836`）都没有 annotations 字段。
- 对照：同一个 OpenAI Key 直接请求 `/v1/responses`（流式和非流式），`url_citation` 原样透传（mock 的 `output_item.done` / 非流式 `output[].content[].annotations` 都在）。
- Anthropic 分组请求 `/v1/responses` 时同样丢失引用（用的是同一个 Anthropic→Responses 转换器）。

### 1.3 模拟搜索（emulation）

- 判定 `service/gateway_websearch_emulation.go:60-95`，依次检查：
  1. manager 已初始化；
  2. OpenAI 平台只限 API Key 账号；
  3. `tools` **恰好 1 个**且是 web_search（`:98-107`）；
  4. 全局开关打开且至少配置 1 个 provider（`service/websearch_config.go:212-218`）；
  5. 账号 `extra.web_search_emulation=enabled`（`service/account.go:2349`），或分组所属渠道 `features_config.web_search_emulation.<platform>=true`（`service/channel.go:289`）。
- provider 只支持 Brave / Tavily（`service/websearch_config.go:38-41`），需要 API Key，由**网关服务端出网**（可走账号代理）。后台入口：`/api/v1/admin/settings/web-search-emulation`。
- 只有两处调用：`service/gateway_forward.go:125-128`（Anthropic `/v1/messages`）和 `service/openai_gateway_messages.go:48-56`（OpenAI 分组的 `/v1/messages`）。**`/v1/chat/completions` 不经过模拟。**
- 即便改成走 `/v1/messages`，Open WebUI 原生函数调用模式下 `tools` 有 22 个内置工具 + web_search（实测），不满足「只有 1 个」，同样不会触发。
- 模拟的输出是 `buildTextSummary` 拼出的搜索结果列表（`:396-406`），不是模型回答。它是给 Claude Code 的「纯搜索子请求」用的，不适合对话。
- 结论：账号上游不支持原生搜索时，本方案拿不到搜索能力。模拟搜索要改造成「先搜索，再把结果作为上下文交给模型」，属于新功能（提议 G4）。

### 1.4 计费（实测 `usage_logs`）

- Anthropic 带 web_search（mock 返回 `usage.server_tool_use.web_search_requests=1`）：
  - 计费只有 token：claude-sonnet-4-5 in=1000 out=200 → $0.006 = 1000×3/1M + 200×15/1M；
  - `usage_logs` 没有搜索次数列。
- OpenAI 带 web_search：gpt-5.4 同样只按 token（$0.0055）。
- 原因：
  - 按次搜索费 `SearchCount` 只有 Grok 会赋值（`service/openai_gateway_forward.go:1337-1341`、`service/openai_gateway_chat_completions.go:772-779`）；
  - 叠加计费逻辑在 `service/gateway_usage_billing.go:1063-1075`；分组单价 `search_price_per_1k`，默认 $5/1k（`service/billing_service.go:2129`）。
- 模拟搜索：返回 `Usage: ClaudeUsage{}`（`service/gateway_websearch_emulation.go:255,380`），整次请求**零费用**，而 Brave / Tavily 的配额照样消耗。
- 风险：上游真实搜索费约 $10/1k 次。用户开着开关聊天，这部分是平台净亏损。

## 2. Open WebUI 侧（wt-c 改动，`git diff` 共 +29/-1）

- `env.py:999-1001` 新增 `S2A_NATIVE_WEB_SEARCH`（默认 False）。
- `main.py:2448`：`/api/config` 的 `features.enable_web_search = web.search.enable or S2A_NATIVE_WEB_SEARCH`。
  - 前端用到它的地方只有 4 处（`MessageInput.svelte:887`、`Chat.svelte:374,1128,1213`），都只控制开关的显示和启用。
  - 因此**前端不用改**，`ENABLE_WEB_SEARCH` 仍为 false。
  - 用户权限 `features.web_search` 默认 True（`config.py:1936`），模型能力 `web_search` 默认 true。
- `utils/middleware.py:2366-2379`：`features.web_search` 为真、`S2A_NATIVE_WEB_SEARCH` 打开、且有权限时，设 `metadata['s2a_native_web_search']`。
  - 原检索分支 `:2355` 仍以 `web.search.enable` 为条件，因此不会执行。
  - 内置 `search_web` / `fetch_url` 工具同样以 `web.search.enable` 为条件（`utils/tools.py:794-800`），也不会注入。
- `utils/middleware.py:2949-2954`：在服务端工具解析完成之后（原生模式由 `:2932` 生成 `form_data['tools']`，加上 inlet filter 的工具），如果还没有 web_search 就追加 `{'type':'web_search'}`。
  - 原生模式：最终 `tools` = 22 个 function 工具 + web_search（`owui/anth_ws.json`、`oai_ws.json`）。
  - legacy 模式：只有 `[web_search]`。
  - 开关关闭：不带 web_search（`anth_off`）。
  - 工具循环的后续请求用 `{**form_data}`（`:5919`、`:6251`），web_search 会延续下去。
- 下游透传：`routers/openai.py:1330-1348` 转 Responses 格式时，没有 `function` 键的工具原样保留；`features` 在 `middleware.py` 里被 pop，不会发给网关。
- 正式 fork 的补充：在 `routers/retrieval.py` 的配置写入口和 `/process/web` 等路由上做硬禁用（见 upstream.md）。在这之前，`S2A_NATIVE_WEB_SEARCH` 只是「显示开关」，服务端不出网靠的是 `ENABLE_WEB_SEARCH=false`，以及配置写入口被锁。

## 3. 渲染

实测存储（`GET /api/v1/chats/{id}` → `history.messages[助手消息]`）：

| 场景 | content | sources | output |
|---|---|---|---|
| Anthropic 分组，CC，联网 | 纯文本（无 `[n]`） | 无 | 只有 message |
| OpenAI 分组，CC，联网 | 文本，含 markdown 链接 | 无 | 只有 message |
| OpenAI 分组，**`api_type=responses`**，联网 | 文本，含 markdown 链接 | **2 条**（`{source:{name:title,url}, document:[title], metadata:[{source:url,name:title}]}`） | `web_search_call` + message（含 annotations） |
| 桩（模拟网关按 G1 输出 `delta.annotations`），CC 流式 | `…source A [1] … [2].` | **2 条**（同上） | message |
| 同上，CC 非流式（`message.annotations`） | 同上 | **无** | message |

- Open WebUI 已有的解析：
  - CC 流式 `delta.annotations[{type:'url_citation', url_citation:{url,title}}]` → `source` 事件，存入 `sources`（`utils/middleware.py:4968-4995`；要求嵌套的 `url_citation` 键）。
  - Responses 流式 `response.output_item.done` 里 message 的 `annotations`（扁平和嵌套两种都认）→ `source` 事件（`:4836-4865`）。
  - **非流式不解析**（`:4117` 起），但前端默认用流式。
- 前端：
  - `ResponseMessage.svelte:928-933`：`message.sources` 非空时显示 `Citations`（「N 个来源」按钮 + 列表）。
  - 正文里的 `[1]`、`[1,2]` 由 `src/lib/utils/marked/citation-extension.ts` 解析，`MarkdownInlineTokens.svelte:184-189` 有 sourceIds 时渲染成可点击的 `SourceToken`，按 `sources` 顺序对应。
  - 来源 ID 以 http 开头时显示 URL（`Citations.svelte:126-128`）。
  - 弹窗 `CitationModal.svelte` 对 URL 来源只显示链接；iframe 用的是 `srcdoc`，不会加载外站。
  - Responses 的 `web_search_call` 输出项显示为「Web Search / Search: <query>」（`structuredOutput.ts:91-97,278-284`）。
  - 文本里没有 `[n]` 时（Anthropic 正文就没有），来源只出现在消息下方的列表里，不能在正文里定位。
- **最小映射**：Open WebUI 不用改；在网关里输出 OpenAI Chat 标准的 `annotations`（提议 G1）即可。正文里的 `[n]` 标记可选，两种做法：网关在带 citations 的文本块后插入 `[n]`，或者不插（只显示来源列表）。
- **外部资源**（fork 必须改）：
  - `Citations.svelte:188`：`https://www.google.com/s2/favicons?sz=32&domain=…`，每条来源都会让浏览器请求 Google。
  - `ResponseMessage/WebSearchResults.svelte:29`（`google.com/search` 链接）、`:71`、`:111`（favicon）。
  - 改法：把 `src` 换成本地通用图标。`Citations.svelte:191-197` 的 `on:error` → `/favicon.png` 回退带有 LICENSE 注释（「Do not alter, remove… the fallback logo」），**保留这段不动**，只改 `src`。

## 4. 生图

实测（`IMAGE_GENERATION_ENGINE=openai`，`IMAGES_OPENAI_API_BASE_URL=网关/v1`，`IMAGES_OPENAI_API_KEY=poc-chat-oai`，`IMAGE_GENERATION_MODEL=gpt-image-1`，`IMAGE_SIZE=1024x1024`）：

| 入口 | 网关收到 | 结果 |
|---|---|---|
| `POST /api/v1/images/generations`（普通用户） | `/v1/images/generations` `{model:gpt-image-1,prompt,n:1,size:1024x1024}`（gpt-image 不带 `response_format`，`routers/images.py:648-656`、`config.py:1370`） | b64 → 存为 Open WebUI 文件，返回 `/api/v1/files/{id}/content` |
| 对话 + `features.image_generation`，legacy 模式 | 先发 1 次聊天请求生成图片提示词（`ENABLE_IMAGE_PROMPT_GENERATION` 默认 true，单独计费），再发 1 次生图请求 | 消息 `files=[{type:image,url:/api/v1/files/…}]` |
| 对话 + `features.image_generation`，原生模式 | 模型调用内置 `generate_image` 工具 → 同一个 `image_generations()` → 网关 | 同上（`owui/img_native.json`） |
| 网关返回 `url`（mock `POC_URL`） | 网关原样透传 `{"url":"http://127.0.0.1:28998/…"}` | Open WebUI 服务端下载：不是网关 origin → `validate_url` 拦截私网地址（`images.py:480-497`）→ 400；**网关已计费** |

- 计费：`usage_logs` 每张一行，`model=gpt-image-1`、`image_count=1`、`image_size=1K`、$0.134。Key 是 `IMAGES_OPENAI_API_KEY`（`poc-chat-oai`），与聊天 Key 分开配置。
- Anthropic 分组 Key 调生图 → 网关 404 `Images API is not supported for this platform`。用户选的是 Claude 分组 Key 时生图不可用，生图要另选 OpenAI 分组 Key，或者隐藏生图开关（产品决策）。
- URL 处理（`images.py:673-681`）：
  - 和 `IMAGES_OPENAI_API_BASE_URL` 同 origin：带着 `Authorization`（用户 Key）去下载；
  - 不同 origin：经 SSRF 校验后从服务端下载（公网可达 → 出网）。
  - 编辑接口 `load_url_image`（`:923-942`）会下载用户提交的任意 http(s) URL。
  - fork 改法：两处都只接受 `data:` / b64，以及网关 origin 的 URL；其他 URL 直接报错。
- 必须锁定的生图接口：
  - `POST /api/v1/images/config/update`（`:277`，可改引擎 / URL / Key；`set_image_model` `:173` 会 upsert 全局配置）→ 403；
  - `GET /api/v1/images/config`（`:272`，会返回 `IMAGES_OPENAI_API_KEY` 等）→ 去掉 Key 或 403；
  - `POST /api/v1/images/verify`（`:339`，GET 任意 URL）→ 删除；
  - `GET /api/v1/images/models`（`:363`，openai 引擎返回写死的 dall-e / gpt-image 列表，comfyui / a1111 会出网）→ 改为网关模型列表或固定列表；
  - `/edit` 的 URL 下载（同上）；
  - 另外 `n` 没有上限（`CreateImageForm` `:446-452`），建议设上限。

## 需要的 fork 改动（汇总）

1. 联网开关：本 PoC 的 3 处改动（`env.py`、`main.py:2448`、`middleware.py:2366-2379,2949-2954`）。
2. OpenAI 平台分组的连接 `api_type=responses`（由 `get_openai_connection` 按 Key 的分组平台决定）。引用依赖 `middleware.py:4836` 的现有解析，不用改代码。
   - 未验证风险：下一轮对话会把 `web_search_call` 输出项作为 input 回传，真实 OpenAI（store=false）是否接受未验证。
3. 去掉 Google favicon / 搜索链接（`Citations.svelte:188`、`WebSearchResults.svelte:29,71,111`），保留 LICENSE 回退。
4. 生图：引擎固定为 openai，URL 固定为网关，Key 从 ContextVar 读；结果 URL 只接受网关 origin（`images.py:673-679`、`:1051-1057`）；编辑接口不下载外部 URL；锁定上面列出的生图接口；限制 `n`。
5. `CHAT_RESPONSE_MAX_TOOL_CALL_ITERATIONS` 默认 256（`env.py:1095-1106`）。工具循环每一轮都是一次计费请求：PoC 中 mock 误让模型反复调用工具，一条消息产生了 256 次网关请求。建议设为 10–20。

## 网关 / mock 改动

网关改动只是提议，本次未实现：

- **G1 引用映射（Anthropic 分组必需）**：
  - `AnthropicContentBlock` 增加 `Citations`；
  - Anthropic→Responses：非流式把 text.citations（`web_search_result_location`：url / title / cited_text）转为 `output_text.annotations[{type:url_citation,url,title,start_index,end_index}]`（按累计文本算偏移）；流式处理 `citations_delta` → `response.output_text.annotation.added`；
  - `web_search_tool_result` 可转为 `web_search_call`（`action.sources`）；
  - 搜索前后的文本段之间补 `\n\n`。
- **G2 Responses→CC 透传 annotations（OpenAI 分组走 CC 时需要）**：
  - `ResponsesContentPart`、`ChatMessage`、`ChatDelta` 增加 `Annotations`；
  - 非流式 → `message.annotations`；
  - 流式 `response.output_text.annotation.added` → `delta.annotations[{type:'url_citation', url_citation:{url,title,start_index,end_index}}]`（OpenAI Chat 搜索模型的标准形状，Open WebUI 现有代码直接识别，桩已验证）。
- **G3 搜索按次计费**：
  - Anthropic 读取 `usage.server_tool_use.web_search_requests`，OpenAI 统计 `web_search_call` 数量，写入 `SearchCount`（目前限定 Grok：`openai_gateway_forward.go:1340`）；
  - 复用分组 `search_price_per_1k`；
  - 模拟搜索也按次计费。
- **G4（可选）模拟搜索用于对话**：CC 入口、多工具也能用，「搜索结果 → 注入上下文 → 正常请求模型」。依赖 Brave / Tavily Key 和网关出网，一期不建议做。
- **G5** OpenAI 账号走 CC 直转时 web_search 被静默丢弃，建议返回明确错误或标注不支持。

### mock 改动（已实施，本机 `~/workspace/sub2api-local/mock/`）

- 原文件备份为 `upstream_mock.py.bak-poc`。
- `upstream_mock.py` 只加了一行导入和几处分发（+15 行）；逻辑都在新文件 `poc_ext.py` 中。
- 触发条件，满足其一才生效：
  - `/v1/messages` 的 `tools` 含 web_search → Anthropic 原生搜索形态（text、server_tool_use、web_search_tool_result、带 citations 的 text、`usage.server_tool_use.web_search_requests=1`；流式含 `input_json_delta` / `citations_delta`）；
  - 上游 Key 为 `sk-poc-chat-upstream`（只有 PoC 账号使用）→ `/v1/responses`（web_search_call + `url_citation`；`tool_choice=required` 或 `POC_CALL 工具 {参数}` 时返回 function_call）、`/v1/images/generations` 的 `POC_URL` 返回 url、`/_poc_bodies` 记录请求体。
- 回归验证：
  - 原版与新版 mock 在两个临时端口上逐一对比 e2e 会用到的请求（messages 流式 / 非流式、images、responses 探测 404、fx、`_requests`、`_reset`、404），去掉随机 ID 和时间戳后**完全相同**；只有新增的 `/_poc_bodies` 不同。
  - `./run_all.sh --fast`：**146/146 通过**，退出码 0（`e2e/runs/20261024-161704/report.md`）。

## 阻塞 / 风险

- 没有阻塞 PoC 结论的问题。
- 上线前必须确认：一号机实际使用的 Claude / OpenAI 上游账号是否支持原生 `web_search_20250305` / `web_search`。不支持时，上游可能报 400 或忽略工具，而模拟搜索对 CC 入口不生效（见 1.3）。本机只能用 mock 验证。
- 引用：Anthropic 分组没有 G1 就没有来源显示；OpenAI 分组依赖 `api_type=responses`，多轮对话回传 `web_search_call` 未经真实上游验证。
- 计费：G3 之前搜索费由平台承担；返回 URL 的生图失败也会扣费。

## 本机残留

- sub2api-local：
  - 用户 `poc-chat-user@poc.test`（id 1069，余额 100）；
  - 分组 `poc-chat-claude` / `poc-chat-oai`（61/62），账号 `poc-chat-claude` / `poc-chat-openai`（37/38），两个 Key；
  - 对应的 `usage_logs`。
- `~/workspace/owui-poc/c/data`：Open WebUI 数据库，含 `poc-chat-admin` / `poc-chat-user` 两个用户。
- Open WebUI（28184）与桩（28186）进程已停止。
