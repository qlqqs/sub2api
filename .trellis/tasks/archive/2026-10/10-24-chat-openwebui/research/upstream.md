# 调研：Open WebUI v0.12.3 上游调用与出网（2026-10-24）

源码 `~/workspace/open-webui`（v0.12.3，91ea4e3），路径省略 `backend/open_webui/`。

## 结论

- 连接 URL / Key 存在 `config` 表（`models/config.py:143-227`），`/openai/config/update` 可写（`routers/openai.py:473-515`）；即使 `ENABLE_PERSISTENT_CONFIG=False`，`upsert` 仍改内存 `Config.DEFAULTS`（`models/config.py:205-208`）→ **必须在代码里强制覆盖，并禁用配置写入口**。
- 默认指向公网：`OPENAI_API_BASE_URL` 默认 `https://api.openai.com/v1`（`config.py:373`），RAG / 图片 / STT / TTS 继承（`config.py:1105,1514,1544,1589,1630`），Realtime 默认 api.openai.com（`config.py:1656`）。
- 没有单一覆盖点：聊天 / 任务 / 模型列表 / embeddings 端点走 `get_openai_connection` + `get_headers_and_cookies`；RAG embedding、图片、音频、外部 reranker 各自拼头。

## 主通道（`routers/openai.py`）

- 模型 → `app.state.OPENAI_MODELS[...].urlIdx` → `get_openai_connection(idx)`（`:255-260`）。
- 请求头 `utils/headers.py:170-235` `get_headers_and_cookies`：`auth_type`（bearer / session / system_oauth / azure_ad）、`forward_cookies`、自定义头模板（在 Authorization 之后 update，可覆盖鉴权）；Azure 额外 `api-key`（`:1525-1529`）。
- 调用点：聊天 `generate_chat_completion :1408-1683`（取连接 `:1479`，拼头 `:1512`，`api_type=responses` 走 `/responses` `:1550-1561`）；`/responses :1812`；`embeddings() :1685`；TTS 代理 `:517`；模型列表 `get_all_models_responses :593` / `send_get_request :87`（**`:99-105` 旧分支不经过 `get_headers_and_cookies`**）；`/verify :985`；模型管理 `:868-983`；透传 `proxy :1925`（默认关，`env.py:868`）。
- **模型列表缓存**：缓存 key 按用户（`:708-713`），但结果写入全局 `app.state.OPENAI_MODELS`（`:779`）与 `app.state.MODELS`；启动用无用户 mock request 预取（`main.py:438-450`）→ 按用户 Key 拉列表会跨用户串数据。可在 `OPENAI_API_CONFIGS` 写静态 `model_ids` 完全不发列表请求（`:617-633`）。
- Anthropic：无独立类型；URL 含 `api.anthropic.com` 或 `provider=='litellm'` 时原生透传 `/messages`（`utils/anthropic.py:35-37,131-135`；`main.py:1972-2030`），否则转 OpenAI 格式（`main.py:2093`）。固定网关 URL 不触发。
- Ollama：`ENABLE_OLLAMA_API=False` 关闭（`/models/download` 会访问 huggingface / github）。

## 统一分发 `utils/chat.py:148-316`

direct（`:195`）/ arena / pipe 函数（`:282`，任意 Python）/ ollama / openai（`:309`）。

调用者：主聊天（`main.py:1146`，在 `create_task` 中）；任务端点 `routers/tasks.py:145-661`（标题、追问、标签、图片提示词、检索查询、自动补全、emoji、MoA；任务模型 `TASK_MODEL` / `TASK_MODEL_EXTERNAL`，`:81-99`、`utils/task.py:17-28`）；后台追问 / 标题 / 标签 `utils/middleware.py:3503,3573,3626,3682`；首次标题 `asyncio.create_task`（`main.py:1562`）；检索查询 `middleware.py:1249,1718`；图片提示词 `:1607`；工具循环 `:6037,6256`；上下文压缩 `utils/context_compaction.py:441`；记忆整理 `utils/memory.py:528-569`（`middleware.py:3718` 回合后触发）；自动化 / 定时器 / 子代理 / 频道 `utils/automations.py:327,523`、`utils/timers.py:427`、`utils/subagents.py:276,496`、`routers/channels.py:1373`。

## 自己拼头的调用

| 功能 | 来源 | 位置 |
|---|---|---|
| RAG embedding | 启动时闭包捕获 `rag.openai.api_base_url/api_key` | `main.py:733-777`；`retrieval/utils.py:921`；引擎为空用本地 SentenceTransformer |
| 外部 reranker | `rag.external_reranker_*` | `retrieval/models/external.py:40-50` |
| 图片生成 / 编辑 / 模型列表 | `IMAGES_OPENAI_API_*`（另有 gemini / comfyui / a1111） | `routers/images.py:627-660,984-990,364` |
| TTS / STT | `audio.tts/stt.openai.*`（另有 elevenlabs / azure / deepgram / 本地 whisper） | `routers/audio/__init__.py:437-457,484,724,895-905` |
| Realtime 语音 | WebSocket 连上游 | `routers/audio/realtime.py:310-335,429` |
| Pipelines 过滤器 | 直接读 `openai.api_keys` | `routers/pipelines.py:57-60,84-89` |

## 用户信息转发头

`ENABLE_FORWARD_USER_INFO_HEADERS`（默认 False，`env.py:997`）：明文 `X-OpenWebUI-User-Name/Id/Email/Role`、`X-OpenWebUI-Chat-Id`（`utils/headers.py:194-197`）；设 `FORWARD_USER_INFO_HEADER_JWT_SECRET` 后改发 `X-OpenWebUI-User-Jwt`（HS256，claims `sub`=user.id、email、name、role、iss=open-webui、iat、exp=+300s，`utils/headers.py:41-52`）。

## 按请求覆盖 URL + Key

- 取 Key：`utils/asgi_middleware.py:126-132`（`_set_token`）处从新头（如 `X-Sub2API-Key`）读入全局 `ContextVar`；不复用 `Authorization` / `x-api-key` / `token` Cookie。
- 出口强制：`get_openai_connection` 永远返回 `(GATEWAY_URL, ctx_key, 清洗后的 config)`（去掉 azure / auth_type / headers / forward_cookies / api_type）；`get_headers_and_cookies` 末尾强制 `Authorization: Bearer ctx_key`、cookies 置空；`openai.py:99-105`、`pipelines.py:57` 同理；RAG embedding / images / audio / realtime / external reranker 改为调用时读 ContextVar。
- 禁用配置写入口：`/openai/config/update`、`/ollama/config/update`、`routers/configs.py` connections / tool_servers、retrieval / images / audio 的 config update。
- 兜底：代码大量临时 `aiohttp.ClientSession(trust_env=True)`，不都走 `utils/session_pool.py:54` → 网络层禁止出网最稳。
- 后台任务：`asyncio.create_task` / `asyncio.to_thread` 复制上下文 → 标题 / 标签 / 追问 / 记忆 / 压缩可继承 ContextVar（推断）；这些代码还持有同一个 `request` 对象（`main.py:1547`）。文件上传后的 embedding 用 Starlette `BackgroundTasks`（`routers/files.py:437`），大概率可继承（未确认）。
- 拿不到 Key：自动化 `scheduler_worker_loop`（`main.py:436`，空 headers 假 request，`utils/automations.py:135-168`）、定时器（`utils/timers.py:409-424`）、子代理异步回传（未确认）→ 关闭 `ENABLE_AUTOMATIONS`（默认 True，`config.py:2079`）、子代理保持关闭、定时器开关待确认。
- Realtime 语音 WebSocket 无法带头 → 一期不开。

## Direct Connections

用户 URL / Key 存入数据库用户设置（`SettingsModal.svelte:449-455`），后端经 Socket.IO 让浏览器去调（`utils/chat.py:49-146`），后端未检查 `direct.enable`（`main.py:2430` 只有前端检查）。**不适合**；保持 `ENABLE_DIRECT_CONNECTIONS=False` 并在后端拒绝 `model_item.direct`。

## 其他出网

| 出站 | 位置 | 处理 |
|---|---|---|
| 联网搜索（约 30 引擎） | `routers/retrieval.py:2187`、`retrieval/web/*` | `ENABLE_WEB_SEARCH=False`，`USER_PERMISSIONS_FEATURES_WEB_SEARCH=False` |
| `#url` `/process/web` `/process/url` YouTube | `routers/retrieval.py:2003,2086`，`retrieval/web/utils.py:1045` | **无开关**，任何用户可调 → 删路由 |
| 内置工具 `fetch_url` / `search_web` | `utils/tools.py:794-802` | 随 web search |
| 网页加载引擎 | `retrieval/web/utils.py` | `WEB_LOADER_ENGINE` 留空（playwright 会在启动时 install，`start.sh:20-25`） |
| 消息里图片 URL 转 base64 | `utils/files.py:55-80`（`middleware.py:1852`） | 无开关 → 代码限制为仅 data: / 同源文件 |
| 图片生成结果 URL 下载 | `routers/images.py:480-510,923-942` | 生图如开放，结果 URL 只允许网关 origin |
| HF 模型下载 | `retrieval/utils.py:1738-1775`、`routers/retrieval.py:148-245`、`routers/audio/__init__.py:236-256,369-376`、`routers/evaluations.py:68-81` | `OFFLINE_MODE=true`（设 `HF_HUB_OFFLINE=1` 并关版本检查，`env.py:1209-1217`）；whisper 找不到模型会回退联网 |
| 版本检查 api.github.com | `main.py:2684-2703` | `ENABLE_VERSION_UPDATE_CHECK=False` |
| License 回传 api.openwebui.com | `utils/auth.py:83-125` | 不设 `LICENSE_KEY` |
| `CUSTOM_NAME` 拉 logo | `config.py:198-215` | 不设 |
| frontmatter pip install | `utils/plugin.py:436-454` | `ENABLE_PIP_INSTALL_FRONTMATTER_REQUIREMENTS=False` |
| 从 URL 导入函数 / 工具 | `routers/functions.py:97-110`、`routers/tools.py:291-304` | 删路由 |
| OpenAPI / MCP 工具服务器、终端 | `utils/tools.py:1355-1398,1626-1645`、`routers/terminals.py:176-191` | `TOOL_SERVER_CONNECTIONS=[]`、`TERMINAL_SERVER_CONNECTIONS=[]`、锁配置口 |
| OAuth / 头像拉取 | `utils/oauth.py:1861-1906,2031` | 不配 provider |
| Webhook | `utils/webhook.py`、`utils/notifications.py:44` | `WEBHOOK_URL` 空，`ENABLE_USER_WEBHOOKS=False`（`config.py:2145`） |
| Gravatar（浏览器加载） | `utils/misc.py:851-862`、`src/lib/utils/index.ts:330` | 前端改 |
| Google Drive / OneDrive | 前端加载 apis.google.com | 默认关 |
| 文档解析引擎 | `retrieval/loaders/*` | 留空（本地解析） |
| 遥测 | chroma 写死关；Dockerfile 设 `SCARF_NO_ANALYTICS` / `DO_NOT_TRACK` / `ANONYMIZED_TELEMETRY=false`；OTEL 默认关 | 保持 |
| 浏览器端外部资源 | `WebSearchResults.svelte:71,111`、`Citations.svelte:188`（google favicons）、`MapSelector.svelte:31`（OSM） | CSP 拦截 / 前端改 |

网络层兜底：Open WebUI 容器只接入 `internal: true` 网络（无默认路由），唯一可达的是 sub2api。

## 网关能力（sub2api）

`backend/internal/server/routes/gateway.go`：`/v1/chat/completions`、`/v1/responses`、`/v1/embeddings`（`:241`）、`/v1/messages`、`/v1/images/*`；**没有** `/v1/audio/*`（语音输入 / 朗读无法经网关）。
