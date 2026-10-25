# 阶段 1：Open WebUI fork 后端改造报告

日期：2026-10-24（UTC）  
工作区：`/home/qlqq/workspace/open-webui`，分支 `sub2api-custom`，基线 v0.12.3。

## 结论

阶段 1 指定的后端挂接及验证门已实现并通过：**252 个 pytest 用例通过**；真实 uvicorn + 假网关 + 剥前缀/注入身份头的假反代冒烟通过；`S2A_EMBED_MODE=0` 启动通过，`/api/config` 无 `sub2api` 字段且品牌保持原版。

**这不是整项目上线验收。** 默认内置工具白名单与“原生工具模式生图”存在约定冲突；另外发现上游管理员文件/笔记访问特权与 PRD R3 的严格所有者隔离不一致。见「已知问题」及「约定变更提议」，不能把本阶段通过等同于 R3 全角色隔离、生产出网策略或真实网关计费已验收。

未提交、未推送、未操作 Docker；未修改前端文件或 sub2api 代码。本报告是唯一写入 sub2api 工作区的文件。没有访问真实网关、第三方 API 或现有 sub2api-local 环境；不需要补装依赖。

## 逐项完成情况

| 项目 | 实现与验证 |
|---|---|
| 1.1 设置 | `Settings.from_env()` 读取所需 8 个 S2A 变量。网关必须是无路径、凭据、query、fragment 的 http(s) origin；CIDR 与生图上限校验失败即拒绝启动。模式关闭时忽略 S2A 附加参数，不改变原版设置。未实现旧稿 `S2A_ALLOW_ADMIN`，以用户要求及 contract B/D 的 `admin/user` 为准。 |
| 1.2 ASGI 边界 | 基于未经 uvicorn 改写的 TCP peer 校验 CIDR。不可信对端剥全部 `X-Sub2API-*`，非静态 403；可信请求缺少/错误/重复身份头 401。验证合成邮箱与用户 ID 一致、角色、单 IP XFF、Key/平台格式；XFF 写入 `scope.client`。支持 Go 发来的 UTF-8 显示名。仅普通 HTTP 设置 Key、Image-Key、Platform、真实 IP ContextVar；WebSocket **及 engine.io HTTP polling** 不捕获计费 Key；finally reset。计费明文头不继续留在下游 scope headers 中。 |
| 启动脚本 | 嵌入模式强制最后传 `--no-proxy-headers`，拒绝 `--root-path`，大小写布尔值一致。嵌入模式跳过 Playwright 安装、bundled Ollama、HF Space 自动注册流程；模式关闭保留原行为。 |
| 1.3 身份 | HTTP 每请求比较 JWT 用户邮箱与可信头并同步角色；`/api/config` 的独立 token 解析也校验邮箱。关闭自身 API Key 鉴权，首用户不再自动 admin，环境预创建管理员关闭。socket connect / user-join / session 复检校验邮箱；join-note 也校验。嵌入模式将 Socket.IO `always_connect` 关闭，避免鉴权前先发连接成功；频道 socket 入口同时关闭。管理员加用户、密码/邮箱/角色修改、退出重定向由表禁止。 |
| 1.4 出口 | `get_openai_connection`、旧 models GET 分支、`get_headers_and_cookies`、6 个同步/异步 embedding helper 均挂接请求上下文。模型 URL 固定网关 `/v1`，清除 Azure/custom headers/forward cookies 等选项；空 Key 400，正文为 `请选择可用的 Key`；拒绝 direct model；带 XFF；OpenAI 平台使用 Responses，其他使用 CC。所有被挂接的请求不跟随重定向，共享 session 禁止自动 CookieJar。 |
| 1.4a 生图 | 生成/编辑均固定 OpenAI 引擎和网关 URL；生图 Key 优先、空则回退对话 Key；校验 `1 <= n <= S2A_IMAGE_MAX_N`。结果接受 b64 或严格同网关 origin 的 URL，编辑接受 data/本站文件/同网关 origin；外站与跨 origin 重定向不会下载。配置读写 403、verify 404、models 固定上游 OpenAI 列表。**默认原生生图工具的白名单冲突见下。** |
| 1.4a 搜索 | 移植 PoC-C 的三处逻辑：config 开关、权限检查、工具解析后追加 `web_search`。保留内置 function tools 与原生搜索并存，不启用自建搜索/网页抓取。 |
| 1.4c 工具、附件 | 工具类别与环境白名单取交集，note-chat/voice 特例不绕过白名单。slim PDF/DOCX 复用原版加载器，PDF OCR 固定关闭。上传检查 `chat.file_upload`；`process=false` 仍检查扩展名，图片放行；拒绝 `metadata.knowledge_id` 绕过。全文附件编辑、删除不再调用向量库，避免 slim 中删除文件失败。 |
| 1.4b 路由表 | `embed/blocklist.py` 单表覆盖 R9、管理配置、用户管理、OAuth、终端、频道、自动化、分享、评分、记忆、知识库、向量/网页检索、音频、Pipelines、外部 URL 导入等。Workspace 模型/提示词管理仅 admin，普通用户仍能读取 Key 可用模型及共享提示词；函数/工具/工具服务器管理仅 admin。**工具服务器是配置封锁的明确例外**，见提议。 |
| 1.5 模型 | 模型列表以 Key 的 SHA-256 为缓存键、TTL 60 秒，最多 256 个 Key 条目，缓存不存 Key 明文。返回独立副本，`OPENAI_MODELS`/`MODELS` 合并元数据并集，不保存某用户的最后一次视图。启动不预取；chat 入口先刷新当前 Key 元数据，再用并集做原 Model-not-found 检查。预设助手要求当前 Key 有底层模型并保留上游 ACL。未登记的网关基础模型不再被上游“仅管理员可见”规则隐藏。 |
| 1.6 出网入口 | 嵌入模式在 ASGI 层对禁用路由返回 404、不分发；为保证模式关闭时原版一致，没有物理删除上游函数定义。图片 URL 转 base64 只允许 data 与本站文件；拒绝时不再静默把被拒 URL 发给模型。`process/file` 仅留全文提取，显式 collection_name 403；vector factory 防御性禁止初始化。 |
| 1.7 品牌、Cookie | 只有 embed + hide_branding 时清空后端 WebUI 名称、不追加 Open WebUI 后缀。两个 token Cookie 签发点及异常清理路径均使用 `/_chat`；模式关闭仍 `/`。`/api/config.sub2api` 严格按 contract D 输出。 |
| 1.8 测试 | 单元矩阵、真实 HTTP/WebSocket 冒烟、真实挂接函数隔离测试均通过；详见下一节。 |

## 测试命令与结果

工作目录均为 `/home/qlqq/workspace/open-webui`。

```bash
/home/qlqq/workspace/owui-poc/venv/bin/python -m pytest -q backend/open_webui/test/embed
# 252 passed, 2 warnings in 49.28s

PYTHONPATH=backend /home/qlqq/workspace/owui-poc/venv/bin/python \
  backend/open_webui/test/embed/smoke/run.py

/home/qlqq/workspace/owui-poc/venv/bin/python -m compileall -q backend/open_webui
bash -n backend/start.sh
git diff --check -- backend
```

后面三个静态检查退出码均为 0。两条 pytest warning 是环境中 Typer 使用已弃用 Click API，非本次补丁错误。v0.12.3 checkout 未包含可运行的上游 Python `test_*.py` 测试，因此没有虚称跑过上游完整 Python 回归；额外进行了关闭嵌入模式的真实启动回归。

测试覆盖：

- 不可信 TCP peer、头剥离、缺头/错格式/重复头、UTF-8 名称、可信 XFF；WebSocket 及 polling 不持有计费 Key。
- 30 个并发 ASGI 请求的 ContextVar 隔离、异常 reset、父请求结束后后台任务继承。
- JWT 换账号在 auth/config 两条路径均 401；每请求角色升/降级；第一名普通用户不是 admin；浏览器伪造身份被假反代剥除。
- socket 旧账号握手拒绝、user-join 换账号断开，以及实际周期复检函数的错邮箱/缺邮箱断开。
- 每条 blocklist 规则分别验证 user/admin，另外真实后端验证每个禁用/受限入口；admin 的 functions/tools/tool_servers 读取可用。
- Key 哈希缓存、TTL、返回副本、元数据并集、空 Key；共享助手按底层模型过滤、私有助手 ACL；真实 helper 不读取危险配置里的 Key/URL。
- 所有 embedding helper、image fallback、数量边界、外站/重定向拒绝、无 Cookie；上传禁用权限时 403、图片放行、原版 PDF/DOCX 文本提取、编辑/删除附件无向量调用。
- start.sh 布尔大小写、proxy flag、root-path 禁止；非嵌入启动/品牌/config 不回退。

## 冒烟证据

真实拓扑（没有替换 Open WebUI 路由实现）：

```text
HTTP/Socket.IO 驱动 -> 127.0.0.1:28202/_chat/*（假反代）
                    -> 127.0.0.1:28200（真实 Open WebUI / uvicorn）
                    -> 127.0.0.1:28201（假网关）
```

- 两用户不同模型、不同 Key 并发 CC/Responses，对话与标题均使用本人 Key、本人 XFF。
- 测试包含真正的 socket session、异步聊天 task 和初始标题 `create_task`，不只是直接调用 title endpoint。
- 原生 `web_search` 与内置 function tools 同时存在；禁止的 knowledge/search_web/fetch_url/code/timer 等不出现。
- 生图分别验证对话 Key 回退与专用生图 Key，以及 b64、网关图片、外站拒绝、网关 302 拒绝、编辑。
- PDF/DOCX 上传后原版加载器文本入库；PDF 全文确实进入发往网关的消息。普通用户交叉访问聊天/文件被拒；聊天拒绝状态与随机不存在 ID 一致。
- uvicorn 子进程安装仅测试使用的 Python audit hook：非 127.0.0.1 或不在批准端口范围的 DNS/connect，在尝试网络 IO 前失败并记录。成功运行的后端实际目的地集合只有 **127.0.0.1:28201**。
- 最近一次 pytest 内完整冒烟：**23 个网关请求，7 个带原生搜索工具，PASS**；后台标题也计入这 23 次。随后重启验证上传权限关闭，以及非嵌入启动。
- 证据目录：`/tmp/s2a-stage1-pj_rst4f/`，含 `result.json`、`gateway.json`（仅虚构测试 Key）、`embed/server.log`、`embed/network.jsonl`、`upstream/server.log`。pytest 总日志 `/tmp/stage1-final-tests.log`。
- 单独再次执行冒烟脚本也通过：`/tmp/s2a-stage1-3qrbpen0/`，同为 23 次网关请求、7 次原生搜索，仅连接 `127.0.0.1:28201`；独立日志 `/tmp/stage1-standalone-smoke.log`。
- 所有进程由脚本 finally 关闭；不会清理其他部署或 Docker 资源。

## 环境变量（交给阶段 3）

### S2A 变量

| 变量 | 源码默认 / 部署要求 |
|---|---|
| `S2A_EMBED_MODE` | 默认 `0`，专用嵌入镜像设 `1`。 |
| `S2A_GATEWAY_URL` | 模式打开时必填，例如 `http://sub2api:8080`；**不能带结尾 `/` 或 `/v1`**。源码无可用默认值，避免误连其他上游。 |
| `S2A_TRUSTED_PROXY_CIDRS` | 默认空（全部非静态请求拒绝），部署设置真实对话网络 CIDR，支持逗号分隔；不要设 `0.0.0.0/0`。 |
| `S2A_HIDE_BRANDING` | 默认 `0`；仅本部署符合法定许可条件时设 `1`。 |
| `S2A_NATIVE_WEB_SEARCH` | 嵌入模式默认 `true`。 |
| `S2A_BUILTIN_TOOL_CATEGORIES` | 默认严格使用 contract G：`time,user_input,files,chats,notes,tasks,calendar`。原生生图的缺口见下。 |
| `S2A_SLIM_DOC_LOADERS` | 嵌入模式默认 `true`；镜像需有 pypdf/docx2txt。 |
| `S2A_IMAGE_MAX_N` | 默认 `4`，必须正整数。 |

### 部署必须设置

- `USE_SLIM_DOCKER=true`；安装原版 slim 依赖，另加 `pypdf`、`docx2txt`；测试另外用 pytest/pytest-asyncio/httpx，生产不必带。
- `WEBUI_SECRET_KEY=<稳定随机秘密>`，不把本报告/测试的值用于部署。
- `WEBUI_URL=https://<实际域名>/_chat`，`DEFAULT_LOCALE=zh-CN`，`UVICORN_WORKERS=1`；不用 uvicorn root_path。
- `DATA_DIR` 指向独立持久卷；不需要配置向量库。不建议直接复用原版多用户数据库。
- `HTTP_PROXY` / `HTTPS_PROXY` 指向管理员登记域名的出站代理；`NO_PROXY` 包含 sub2api。容器必须放在 internal-only 专用网络且不映射宿主端口。代码不是任意 Python 插件的沙箱。
- 建议设置实际站点的 `CORS_ALLOW_ORIGIN`，不要照抄测试环境的宽泛 CORS。
- 健康检查注意：`/health` 是非静态接口，按 contract D 也需要可信身份。可以用本机 `/static/favicon.png` 静态检查，或由主会话另行约定最小匿名健康接口；不要为健康检查放宽全部 CIDR。

### 嵌入模式中代码强制的原版开关

`embed/settings.py::configure_environment()` 是准确清单，Dockerfile 可显式写出以便审计，但不能依赖管理员 UI 修改这些值：

```text
true:
  OFFLINE_MODE WEBUI_AUTH ENABLE_PASSWORD_AUTH ENABLE_OPENAI_API
  BYPASS_EMBEDDING_AND_RETRIEVAL ENABLE_PLUGINS
  ENABLE_IMAGE_GENERATION ENABLE_IMAGE_EDIT ENABLE_TITLE_GENERATION

false:
  ENABLE_SIGNUP ENABLE_LOGIN_FORM ENABLE_INITIAL_ADMIN_SIGNUP
  ENABLE_PASSWORD_CHANGE_FORM ENABLE_MFA ENABLE_PERSISTENT_CONFIG
  ENABLE_API_KEYS ENABLE_OAUTH ENABLE_LDAP ENABLE_SCIM
  ENABLE_VERSION_UPDATE_CHECK ENABLE_OLLAMA_API ENABLE_DIRECT_CONNECTIONS
  ENABLE_DIRECT_INTEGRATIONS ENABLE_WEB_SEARCH ENABLE_COMMUNITY_SHARING
  ENABLE_USER_WEBHOOKS ENABLE_MEMORIES ENABLE_CODE_EXECUTION ENABLE_CODE_INTERPRETER
  ENABLE_CHANNELS ENABLE_AUTOMATIONS ENABLE_SUBAGENTS ENABLE_ADMIN_CHAT_ACCESS
  ENABLE_ADMIN_EXPORT ENABLE_ADMIN_ANALYTICS ENABLE_EVALUATION_ARENA_MODELS
  ENABLE_MESSAGE_RATING ENABLE_PIP_INSTALL_FRONTMATTER_REQUIREMENTS
  ENABLE_TAGS_GENERATION ENABLE_FOLLOW_UP_GENERATION ENABLE_RETRIEVAL_QUERY_GENERATION
  ENABLE_IMAGE_PROMPT_GENERATION ENABLE_AUTOCOMPLETE_GENERATION
  ENABLE_GOOGLE_DRIVE_INTEGRATION ENABLE_ONEDRIVE_INTEGRATION ENABLE_APPS
  ENABLE_BASE_MODELS_CACHE ENABLE_FORWARD_USER_INFO_HEADERS
  ENABLE_OPENAI_API_PASSTHROUGH ENABLE_OTEL
```

可信头固定为 `WEBUI_AUTH_TRUSTED_EMAIL_HEADER=X-Sub2API-Email`、`...NAME_HEADER=X-Sub2API-Name`、`...ROLE_HEADER=X-Sub2API-User-Role`，`DEFAULT_USER_ROLE=user`。`OPENAI_API_BASE_URL(S)` 固定网关 `/v1`；`OPENAI_API_KEY(S)` 为空；不允许配置里的 Key 兜底。`TERMINAL_SERVER_CONNECTIONS=[]`、`CONTENT_EXTRACTION_ENGINE=''`、`PDF_EXTRACT_IMAGES=false`、`AIOHTTP_CLIENT_ALLOW_REDIRECTS=false`。`OLLAMA_BASE_URL` 设置非默认值以避免上游即使关闭 Ollama 仍执行端口探测。

用户权限强制关闭：workspace 模型/提示词/工具/知识库/skills 及相关导入、chat share/STT/TTS/call、features memories/code_interpreter/channels/direct_tool_servers/api_keys/automations/webhooks。保留上游对文件上传、notes/calendar、image_generation/web_search、导出和参数面板的权限机制。

若未设置，则补这些默认值：

```text
IMAGE_GENERATION_MODEL=gpt-image-1
IMAGE_EDIT_MODEL=gpt-image-1
CHAT_RESPONSE_MAX_TOOL_CALL_ITERATIONS=20
RAG_FILE_MAX_SIZE=25
RAG_ALLOWED_FILE_EXTENSIONS=pdf,docx,txt,md,csv,html,htm,json,xml,yaml,yml,py,js,ts,tsx,jsx,css,go,rs,c,cpp,h,java,sh,sql,log
```

清除 LICENSE_KEY、CUSTOM_NAME、可信组头与环境预建管理员账号。源码默认品牌和原版运行模式不改变。

## 改动文件清单

### 新增逻辑

- `backend/open_webui/embed/__init__.py`
- `backend/open_webui/embed/settings.py`：设置、启动校验、安全环境约束。
- `backend/open_webui/embed/sub2api_auth.py`：纯 ASGI 信任边界、角色同步、socket 身份判定。
- `backend/open_webui/embed/gateway.py`：ContextVar、网关连接/头、origin/direct 检查。
- `backend/open_webui/embed/models.py`：按 Key 哈希的模型缓存。
- `backend/open_webui/embed/images.py`：生图配置、数量、下载/编辑来源约束。
- `backend/open_webui/embed/embeddings.py`：同步/异步 embedding 网关出口。
- `backend/open_webui/embed/blocklist.py`：路由策略单表。

### 上游挂接点（均有 `# sub2api-embed:` 注释）

```text
backend/start.sh
backend/open_webui/env.py
backend/open_webui/main.py
backend/open_webui/models/config.py
backend/open_webui/routers/auths.py
backend/open_webui/routers/files.py
backend/open_webui/routers/images.py
backend/open_webui/routers/openai.py
backend/open_webui/routers/retrieval.py
backend/open_webui/routers/tasks.py
backend/open_webui/socket/main.py
backend/open_webui/utils/auth.py
backend/open_webui/utils/headers.py
backend/open_webui/utils/models.py
backend/open_webui/utils/access_control/__init__.py
backend/open_webui/utils/middleware.py
backend/open_webui/utils/tools.py
backend/open_webui/utils/files.py
backend/open_webui/utils/automations.py
backend/open_webui/utils/session_pool.py
backend/open_webui/retrieval/utils.py
backend/open_webui/retrieval/loaders/main.py
backend/open_webui/retrieval/vector/factory.py
```

### 新增测试

`backend/open_webui/test/embed/`：`conftest.py`、`test_settings.py`、`test_boundary.py`、`test_blocklist.py`、`test_gateway.py`、`test_start.py`、`test_smoke.py`；`smoke/run.py`、`smoke/server.py`、`smoke/hook_checks.py`。

## 已知问题 / 后续验证

1. **原生生图默认白名单冲突**：contract G 没有 `image_generation` 类别。因此按现有约定，REST 生图、编辑及 legacy 生图可用，但默认 native 模式不会向模型提供 `generate_image/edit_image` 内置工具。不能把本次 REST 生图成功说成默认 native 生图体验已完整。代码支持显式把该类别加入 S2A 白名单，需主会话确认约定。
2. **PRD R3 的管理员隔离缺口（发布前处理）**：本次验证了普通用户之间的聊天/文件隔离；上游 `routers/files.py` 多处仍有 `user.role == 'admin'` 直接放行，笔记相关路径也保留上游管理员特权。`ENABLE_ADMIN_CHAT_ACCESS=false` 只覆盖聊天，不等于所有资源严格所有者隔离。这个缺口不能靠隐藏管理页解决，需追加管理员/共享授权的文件、笔记、日历全路径隔离补丁和测试。阶段 1 没有声称完成这项阶段 6 的全资源验收。
3. 真实网关引用映射、搜索支持/计费、余额刷新、前端 CSP/Artifacts、同浏览器账户切换与 parent revoke、生产出站白名单均需后续阶段联调；本次没有访问真实网关。已有 WebSocket 的 sub2api Cookie 撤销仍依赖父页断开/重建，后端不能凭已建立的握手知道远端 Cookie 后来被删除。
4. 插件是可信 Python 代码，管理员可以在进程内读取 ContextVar；应用层不可能替代 internal 网络和白名单代理。工具服务器 OAuth 的浏览器授权路径仍属于被关闭的 OAuth 面，本阶段不声明该流程可用。
5. 扫描 PDF 空文本保持上游行为；非法图片 URL 的拒绝可能发生在网关生成已计费之后。Key 模型缓存最长 60 秒，最终授权仍由每次网关请求的当前 Key 决定。

## 约定变更提议（未修改 contract.md）

- **C1（需要决定）**：contract G 的 `S2A_BUILTIN_TOOL_CATEGORIES` 加 `image_generation`，或明确原生生图工具不受这张类别白名单控制。推荐前者；这样符合一期生图与默认原生函数调用目标，不引入另一套特殊绕过逻辑。
- **C2（需文字澄清）**：design §3.2 的“禁用全部工具服务器配置写入口”与 PRD/§2.4“管理员配置工具服务器”冲突。当前按后者实现：`/api/v1/configs/tool_servers` 及 verify 仅 admin，其余 configs 禁止。另仅 `tool_server.connections` 持久化，避免 `ENABLE_PERSISTENT_CONFIG=false` 使管理员配置重启丢失；没有放开模型/计费连接配置持久化。
- **C3（可选）**：若阶段 3 强制要求 `/health` 匿名健康检查，需在 contract B/D 明确一个精确、只读、无身份信息的例外；当前不自行扩大匿名接口。

不需要变更 contract D 的 header、错误正文、Cookie path 或 `/api/config.sub2api` 字段。

## 补充（R3 管理员隔离、C1）

日期：2026-10-24。以下结论覆盖上文“原生生图默认白名单冲突”“管理员隔离缺口”两项旧状态；旧记录保留以便追溯。

### 完成结论

- **R3.1**：嵌入模式管理员不再凭角色访问其他用户的个人资源。所有者及上游原有的显式共享 ACL 保留；不全局降级 admin 身份，Workspace 预设助手/提示词/函数/工具管理权限保持。
- **C1**：两处源码默认值均新增 `image_generation`；默认值及显式覆盖测试通过。真实原生函数调用流程中模型收到 `generate_image`，假模型返回工具调用，后端执行生图，再将结果回送模型；已断言该次生成使用 **`smoke-image-1`** 而非对话 Key。
- **C2**：维持已采纳实现，增加真实 HTTP 写入/重启回读验证：普通用户写工具服务器配置 403、管理员写入 200，重启后连接仍存在；其他 configs 继续禁止。测试使用禁用状态的本机桩连接，不连接任何外站。
- **C3**：没有新增匿名 `/health` 例外。可信但匿名的 `/health` 返回 401、不可信对端返回 403；匿名静态 `/static/favicon.png` 可作容器探针，真实后端返回 200。

### 实现与本轮文件清单

新增核心模块 **`backend/open_webui/embed/access.py`**：

1. `resource_admin()`：仅非嵌入模式保留上游管理员旁路；支持对象、dict 和 role 字符串。
2. `private_denied()` / `tool_denied()`：个人资源拒绝与不存在的响应一致，不暴露 ID 是否存在。
3. `check_model_attachments()`：管理 Workspace 不等于可引用任意个人文件/笔记，创建/更新/导入/恢复历史版本均验证附件权限；阻断“先挂到自己管理的模型再读取”的绕过。
4. `check_chat_session()`：临时聊天及 HTTP 指定的 Socket.IO session 必须属于当前用户；嵌入模式同时拒绝 `channel:` 请求。
5. `valid_document_id()`：嵌入模式只允许规范的笔记协作房间，不开放无授权的任意 Yjs 文档空间。

本轮修改（前轮文件不重复列举）：

| 文件（均相对 `backend/open_webui/`） | 挂接/用途 |
|---|---|
| `embed/settings.py` | C1 两处默认类别同步 |
| `embed/blocklist.py` | 增加全局 `/api/tasks` 与 `/api/tasks/stop/*` 禁止；保留按聊天授权的任务入口 |
| `main.py` | chat action/outlet、临时任务所有者检查；HTTP session 归属；禁用 channel chat body |
| `models/chats.py` | 修复 internal chat 的 admin 读旁路；聊天附件关联必须可读 |
| `routers/chats.py` | 消息修改/删除/事件、共享辅助函数、聊天删除等个人数据旁路 |
| `routers/files.py` | 列表/搜索/count 按当前用户；ID/content/状态/重命名/更新/删除按所有者/ACL |
| `routers/notes.py` | 搜索、读写、内部聊天、pin、删除、共享更新；撤销共享时 admin 也移出房间 |
| `routers/calendar.py` | 日历/事件统一授权；事件不存在使用事件自身的错误正文；共享管理/删除不再 admin 放行 |
| `routers/folders.py` | 共享树、父目录、内容、读写/共享配置、删除；修复 expanded 未授权分支落入 scoped 更新的存在性区别 |
| `routers/models.py` | 背景/头像资源以及模型引用/历史版本中的个人附件授权 |
| `routers/retrieval.py` | process/file 内容更新按个人文件权限；旧分支同步收紧 |
| `retrieval/utils.py` | 通过 folder/note/chat/file/collection 间接读取上下文的管理员旁路 |
| `utils/access_control/files.py` | 文件夹附件过滤不能因 admin 跳过；原版显式授权的文件共享逻辑保留 |
| `utils/files.py` | 图片文件转 base64 同样按文件所有权/共享 ACL |
| `utils/middleware.py` | 临时聊天文件夹上下文、笔记内部聊天附件重验 |
| `tools/builtin.py` | 文件/笔记/日历及知识辅助函数不因 admin 放行；错误结果也隐藏私有资源存在性 |
| `socket/main.py` | join-note、Yjs join/save/update 不再 admin 放行；协作显示身份绑定已认证用户 |

新增测试/审计文件：`test/embed/test_access.py`、`test/embed/test_admin_audit.py`、`test/embed/audit_admin.py`、`test/embed/smoke/isolation.py`、`test/embed/smoke/isolation_checks.py`。
修改测试文件：`test/embed/test_settings.py`、`test/embed/test_boundary.py`、`test/embed/smoke/run.py`、`test/embed/smoke/hook_checks.py`。
所有上游挂接均含 `# sub2api-embed:`；未修改前端、contract、sub2api 代码；没有执行 git 写操作、提交或推送。

### 授权与回归覆盖

- 两个普通用户 + 一个管理员，真实 HTTP 校验**他人 ID 与随机不存在 ID 的状态码和 JSON 正文均相同**：普通/内部会话、标签操作、文件原始内容/提取内容/状态/重命名/更新/删除、笔记及笔记聊天、日历/事件、文件夹。
- 列表、搜索、导出、文件 count、日历范围/指定日历查询、聚合搜索无越权结果；读取者 role=admin 不会形成全局视图。
- 间接文件/笔记/聊天/文件夹上下文不向假网关泄露私有标记；禁止模型附件引用洗白。
- 管理员 socket 不能 join 私有普通/内部会话、note/`note_` 别名文档；收不到笔记更新/聊天 typing 推送。笔记显式读写授权后可加入、接收、修改，撤销后移出房间且不能再读状态/写快照。
- 笔记附件显式授权只允许读、不授予删文件；日历/文件夹只读共享也不因 admin 升为写权限。
- 全局任务列表/直接 task-ID 停止禁止；按聊天查询/停止和临时 session 身份归属单独检查。
- 非嵌入模式除启动回归外，真实 helper/model 路径测试验证原版 admin 仍可读内部聊天、文件、日历、全局文件列表。
- 静态防回归检查锁定个人资源模块剩余的管理员判断：只允许经审计的功能开关、角色枚举、HTML 作者可信判定和已封锁知识库分支；新增 raw admin 旁路会导致测试失败。

### 环境变量（给阶段 3）

**没有新增变量**。唯一默认值变化：

```text
S2A_BUILTIN_TOOL_CATEGORIES=time,user_input,files,chats,notes,tasks,calendar,image_generation
```

若 Docker/compose 显式写了旧类别字符串，须同步加入 `image_generation`（显式覆盖仍然优先，不会被代码偷偷扩展）。其余阶段 1 环境变量和 `USE_SLIM_DOCKER=true`、`--no-proxy-headers`、无 `--root-path` 不变。
容器探针示例：请求 `http://127.0.0.1:<容器后端端口>/static/favicon.png`，不要改用匿名 `/health`。

### 管理员判断全面审计

审计不仅搜索单/双引号 `role == admin`，还覆盖 `.get('role')`、`getattr(..., 'role')`、`!=`/`in`/`not in`、`is_admin`、`BYPASS_ADMIN_ACCESS_CONTROL`、`ENABLE_ADMIN_CHAT_ACCESS`，并补查 `Depends(get_admin_user)`（由此发现全局 tasks 列表/停止旁路）。

可复核命令：

```bash
rg -n 'admin|is_admin|BYPASS_ADMIN|ENABLE_ADMIN_CHAT_ACCESS|get_admin_user' backend/open_webui --glob '*.py'
PYTHONPATH=backend /home/qlqq/workspace/owui-poc/venv/bin/python \
  -m open_webui.test.embed.audit_admin > /tmp/s2a-admin-inventory.json
```

AST 清单排除测试、历史迁移，得到 **517 个匹配、515 个不同源码位置**；下表对每个位置逐条列出判定。行号对应本轮完成时源码，后续变更可用函数名定位。只保留 Workspace 管理与功能许可，而不是机械删除所有 `admin` 字符串。

额外 grep 的非条件/别名项目：

| 项目 | 判定 |
|---|---|
| `models/models.py` 的 `is_admin` 参数/分支、`routers/models.py` 的 `is_admin=True` | Workspace 模型标签元数据范围，保留；不是个人会话标签 |
| `routers/folders.py` 的 `is_admin` 局部变量及所有使用点 | 定义已改为 `resource_admin(user)`，嵌入模式为 False，后续 has_write/read/子树分支同步收紧 |
| `utils/apps.py` 的 actor.is_admin | App 事件身份元数据；Apps 功能禁用，不是个人资源旁路 |
| `config.py`/`env.py` 的 BYPASS_ADMIN/ENABLE_ADMIN 定义与 imports | 不全局关闭 BYPASS_ADMIN（避免破坏 Workspace）；用户资源逐点 helper；ENABLE_ADMIN_CHAT_ACCESS/EXPORT 强制 false |
| `routers/chats.py` 的 `if not ENABLE_ADMIN_CHAT_ACCESS` 全用户列表、全库导出开关 | 对应跨用户路由额外 blocklist 拦截 |
| `role == 'user'` 的聊天删除/导入/参数权限分支 | 仅功能许可；实际查询/删除均使用当前 user.id，非管理员数据旁路 |
| OAuth 角色映射、首管理员/角色字段 schema、迁移、日志/文案中的 admin | 账户身份配置或历史数据声明；嵌入账号管理/OAuth 关闭，未当作资源权限修改 |
| 无 admin 分支的 `models/files/notes/calendar/folders/tags.py`、`routers/search.py` SQL 查询 | 检查 user_id/显式 grants 过滤；标签变更先通过聊天所有者检查，聚合搜索不扩大 admin 范围 |
| 可信 Python 函数/工具源码与工具服务器 | 仍为合同允许的管理员管理面；见下方信任边界说明，不声称沙箱隔离任意恶意插件代码 |

| 位置（相对 `backend/open_webui/`） | 所在函数 | 判定/处理 |
|---|---|---|
| `embed/access.py:16` | `resource_admin` | 保留：身份格式/角色同步/路由授权或隔离 helper 自身，非个人资源旁路。 |
| `embed/blocklist.py:130` | `route_status` | 保留：身份格式/角色同步/路由授权或隔离 helper 自身，非个人资源旁路。 |
| `embed/sub2api_auth.py:53` | `validate_identity` | 保留：身份格式/角色同步/路由授权或隔离 helper 自身，非个人资源旁路。 |
| `embed/sub2api_auth.py:160` | `sync_user` | 保留：身份格式/角色同步/路由授权或隔离 helper 自身，非个人资源旁路。 |
| `embed/sub2api_auth.py:176` | `socket_identity_matches` | 保留：身份格式/角色同步/路由授权或隔离 helper 自身，非个人资源旁路。 |
| `functions.py:269` | `generate_function_chat_completion` | 保留：Workspace 函数执行 ACL。 |
| `main.py:986` | `get_base_models` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `main.py:996` | `unload_model` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `main.py:1200` | `chat_completion` | 保留：Workspace 模型访问与聊天控制参数的功能权限。 |
| `main.py:1231` | `chat_completion` | 保留：Workspace 模型访问与聊天控制参数的功能权限。 |
| `main.py:1310` | `chat_completion` | 保留：工具服务器功能许可，不授予个人资源访问权；管理员插件和服务器属于可信管理面。 |
| `main.py:1402` | `chat_completion` | 封锁前置：embed 的 channel: chat_id 由 check_chat_session 拒绝，频道分支不可达。 |
| `main.py:1428` | `chat_completion` | 封锁前置：embed 的 channel: chat_id 由 check_chat_session 拒绝，频道分支不可达。 |
| `main.py:2144` | `verify_chat_ownership` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `main.py:2196` | `stop_task_endpoint` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `main.py:2205` | `list_tasks_endpoint` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `main.py:2214` | `list_tasks_by_chat_id_endpoint` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `main.py:2236` | `stop_tasks_by_chat_id_endpoint` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `main.py:2336` | `get_app_config` | 保留：角色枚举与配置/人数展示，非个人资源数据；admin chat access 输出 false。 |
| `main.py:2568` | `get_app_config` | 保留：角色枚举与配置/人数展示，非个人资源数据；admin chat access 输出 false。 |
| `main.py:2572` | `get_app_config` | 保留：角色枚举与配置/人数展示，非个人资源数据；admin chat access 输出 false。 |
| `main.py:2616` | `get_event_catalog` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `main.py:2624` | `get_event_webhooks_api` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `main.py:2629` | `create_event_webhook` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `main.py:2660` | `update_event_webhook` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `main.py:2694` | `delete_event_webhook_api` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `main.py:2753` | `get_current_usage` | 封锁：管理/事件/usage/全局任务入口；任务仅允许按聊天所有权查询/停止。 |
| `models/chats.py:600` | `ChatTable.require_chat_creation_permission` | 保留：创建自己聊天的功能许可。 |
| `models/chats.py:1921` | `ChatTable.get_accessible_chat_by_id` | 保留：合法用户角色枚举；下一层 owner/ACL 决定访问，内部聊天不再为 admin 开口。 |
| `models/chats.py:1928` | `ChatTable.get_accessible_chat_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `models/chats.py:2856` | `ChatTable.insert_chat_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `models/users.py:713` | `UsersTable.get_first_user` | 保留：管理员查找/排序是账户内部逻辑；管理路由已封锁，非用户数据授权。 |
| `models/users.py:898` | `UsersTable.get_super_admin_user` | 保留：管理员查找/排序是账户内部逻辑；管理路由已封锁，非用户数据授权。 |
| `retrieval/utils.py:1355` | `filter_accessible_collections` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `retrieval/utils.py:1429` | `get_sources_from_items` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `retrieval/utils.py:1473` | `get_sources_from_items` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `retrieval/utils.py:1492` | `get_sources_from_items` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `retrieval/utils.py:1554` | `get_sources_from_items` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `retrieval/utils.py:1585` | `get_sources_from_items` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `retrieval/utils.py:1600` | `get_sources_from_items` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `retrieval/utils.py:1623` | `get_sources_from_items` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/analytics.py:63` | `get_model_analytics` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/analytics.py:91` | `get_user_analytics` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/analytics.py:134` | `get_messages` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/analytics.py:168` | `get_summary` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/analytics.py:205` | `get_daily_stats` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/analytics.py:242` | `get_token_usage` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/analytics.py:296` | `get_model_chats` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/analytics.py:349` | `get_model_overview` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/apps.py:56` | `get_apps` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/apps.py:67` | `preview_app_manifest` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/apps.py:106` | `create_new_app` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/apps.py:150` | `get_app_details_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/apps.py:174` | `update_app_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/apps.py:219` | `connect_app_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/apps.py:254` | `uninstall_app_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/audio/__init__.py:317` | `get_audio_config` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/audio/__init__.py:326` | `update_audio_config` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/audio/__init__.py:773` | `speech` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/audio/__init__.py:1463` | `transcription` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/audio/realtime.py:393` | `realtime_call` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/audio/realtime.py:411` | `realtime_call` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/audio/realtime.py:531` | `realtime_call.check_auth` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/auths.py:715` | `signin` | 保留：可信身份头角色枚举；嵌入模式逐请求同步，非首用户自动提权。 |
| `routers/auths.py:1057` | `add_user` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1162` | `get_admin_config` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1231` | `update_admin_config` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1255` | `update_admin_config` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1291` | `get_ldap_server` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1296` | `update_ldap_server` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1322` | `get_ldap_config` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1331` | `update_ldap_config` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1479` | `get_oauth_config` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1484` | `update_oauth_config` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/auths.py:1491` | `_check_api_key_permission` | 封锁：账号管理/LDAP/OAuth/API-Key/管理员配置不可由嵌入接口操作。 |
| `routers/automations.py:52` | `check_automations_permission` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/automations.py:71` | `check_automation_limits` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/automations.py:127` | `check_automation_channel_access` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/calendar.py:47` | `check_calendar_permission` | 保留：日历功能许可；automations 强制关闭；实际日历与事件访问单独授权。 |
| `routers/calendar.py:59` | `_user_has_automations` | 保留：日历功能许可；automations 强制关闭；实际日历与事件访问单独授权。 |
| `routers/calendar.py:69` | `_check_calendar_access` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/calendar.py:422` | `update_calendar` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/calendar.py:463` | `delete_calendar` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/channels.py:168` | `check_channels_access` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:248` | `get_all_channels` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:325` | `create_new_channel` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:456` | `get_channel_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:486` | `get_channel_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:562` | `get_channel_members_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:633` | `update_member_mute_status` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:696` | `add_members_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:751` | `remove_members_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:795` | `update_channel_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:850` | `delete_channel_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:982` | `get_channel_messages` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1049` | `get_pinned_channel_messages` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1390` | `new_message_handler` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1542` | `get_channel_message` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1583` | `get_channel_message_data` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1624` | `pin_channel_message` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1702` | `get_channel_thread_messages` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1772` | `update_message_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1775` | `update_message_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1780` | `update_message_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1844` | `add_reaction_to_message` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1918` | `remove_reaction_by_id_and_user_id_and_name` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:1999` | `delete_message_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:2002` | `delete_message_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:2011` | `delete_message_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:2097` | `get_webhook_profile_image` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:2144` | `get_channel_webhooks` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:2164` | `create_channel_webhook` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:2196` | `update_channel_webhook` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/channels.py:2231` | `delete_channel_webhook` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/chats.py:111` | `get_optional_verified_user` | 保留：已验证角色枚举（user/admin），不授予数据权限。 |
| `routers/chats.py:128` | `can_read_shared_chat` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:279` | `require_chat_import_permission` | 保留：导入功能许可；导入强制当前 user_id、目标文件夹检查写 ACL。 |
| `routers/chats.py:665` | `export_chat_stats` | 封锁：跨用户管理、全库导出、stats、共享发布及配置路由禁用；ID 访问另有收紧。 |
| `routers/chats.py:714` | `export_single_chat_stats` | 封锁：跨用户管理、全库导出、stats、共享发布及配置路由禁用；ID 访问另有收紧。 |
| `routers/chats.py:730` | `export_single_chat_stats` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:793` | `get_user_chat_list_by_user_id` | 封锁：跨用户管理、全库导出、stats、共享发布及配置路由禁用；ID 访问另有收紧。 |
| `routers/chats.py:893` | `get_chat_config` | 封锁：跨用户管理、全库导出、stats、共享发布及配置路由禁用；ID 访问另有收紧。 |
| `routers/chats.py:898` | `set_chat_config` | 封锁：跨用户管理、全库导出、stats、共享发布及配置路由禁用；ID 访问另有收紧。 |
| `routers/chats.py:1116` | `get_all_user_chats_in_db` | 封锁：跨用户管理、全库导出、stats、共享发布及配置路由禁用；ID 访问另有收紧。 |
| `routers/chats.py:1311` | `get_shared_chat_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:1512` | `update_chat_message_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:1578` | `delete_chat_message_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:1626` | `send_chat_message_event_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:1671` | `delete_chat_by_id` | 保留功能豁免：admin 可删自己的聊天；紧接所有者检查，实际删除改走 scoped 方法。 |
| `routers/chats.py:1673` | `delete_chat_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:1699` | `delete_chat_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:1960` | `clone_shared_chat_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:1979` | `clone_shared_chat_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:2075` | `share_chat_by_id` | 封锁：跨用户管理、全库导出、stats、共享发布及配置路由禁用；ID 访问另有收紧。 |
| `routers/chats.py:2161` | `update_shared_chat_access_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/chats.py:2202` | `get_shared_chat_access_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/configs.py:101` | `import_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:121` | `export_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:126` | `get_config_namespace` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:142` | `get_connections_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:150` | `set_connections_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:179` | `register_oauth_client` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:241` | `get_tool_servers_config` | 保留 C2：仅 admin 管理工具服务器；仅连接配置持久化。 |
| `routers/configs.py:249` | `set_tool_servers_config` | 保留 C2：仅 admin 管理工具服务器；仅连接配置持久化。 |
| `routers/configs.py:332` | `get_terminal_servers_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:340` | `set_terminal_servers_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:362` | `verify_terminal_server_connection` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:440` | `put_terminal_server_policy` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:480` | `put_terminal_server_lifecycle` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:520` | `refresh_terminal_server_terminals` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:569` | `verify_tool_servers_config` | 保留 C2：仅 admin 管理工具服务器；仅连接配置持久化。 |
| `routers/configs.py:725` | `get_code_execution_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:731` | `set_code_execution_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:770` | `get_models_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:775` | `set_models_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:804` | `get_subagents_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:812` | `set_subagents_config` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:841` | `set_default_suggestions` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/configs.py:876` | `set_banners` | 封锁：除 tool_servers 精确例外之外 configs 一律 403。 |
| `routers/evaluations.py:226` | `get_leaderboard` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:272` | `get_model_history` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:286` | `get_config` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:304` | `update_config` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:328` | `get_feedback_model_ids` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:333` | `get_all_feedback_ids` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:340` | `delete_all_feedbacks` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:357` | `export_all_feedbacks` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:405` | `get_feedbacks` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:451` | `get_feedback_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:470` | `update_feedback_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/evaluations.py:495` | `delete_feedback_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/files.py:213` | `process_uploaded_file._process_handler` | 封锁：knowledge_id 上传 metadata 在入口拒绝；不会进入知识库自动关联分支。 |
| `routers/files.py:295` | `upload_file` | 保留：上传功能许可；文件所有者固定当前用户，不是跨用户访问。 |
| `routers/files.py:514` | `list_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:548` | `search_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:583` | `count_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:594` | `delete_all_files` | 封锁：files/all 全局删除 403。 |
| `routers/files.py:632` | `get_file_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:659` | `get_file_process_status` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:717` | `get_file_data_content_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:751` | `update_file_data_content_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:839` | `get_file_content_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:902` | `get_html_file_content_by_id` | 保留：判断文件作者是否为可信 HTML 作者；调用者仍须所有者/显式授权。 |
| `routers/files.py:908` | `get_html_file_content_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:950` | `get_file_content_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:1014` | `rename_file_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/files.py:1054` | `delete_file_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:76` | `check_folders_permission` | 保留：folders 或 chat.delete 功能许可；数据所有者/ACL 已单独检查。 |
| `routers/folders.py:113` | `get_folders` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:184` | `create_folder` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:317` | `get_folder_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:322` | `get_folder_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:349` | `update_folder_name_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:357` | `update_folder_name_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:503` | `update_folder_is_expanded_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:557` | `update_folder_access_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:614` | `get_shared_folder_chats` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:682` | `mark_folder_chats_read_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:725` | `delete_folder_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/folders.py:735` | `delete_folder_by_id` | 保留：folders 或 chat.delete 功能许可；数据所有者/ACL 已单独检查。 |
| `routers/functions.py:59` | `get_function_list` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:74` | `get_functions` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:110` | `load_function_from_url` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:169` | `sync_functions` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:211` | `create_new_function` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:276` | `get_function_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:297` | `toggle_function_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:351` | `toggle_global_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:389` | `update_function_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:446` | `delete_function_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:473` | `get_function_valves_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:501` | `get_function_valves_spec_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:532` | `update_function_valves_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:697` | `get_function_history` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:705` | `get_function_history_diff` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:715` | `get_function_history_entry` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:723` | `delete_function_history_entry` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/functions.py:740` | `set_function_production` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/groups.py:57` | `get_groups` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:76` | `create_new_group` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:116` | `get_group_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:156` | `export_group_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:177` | `get_users_in_group` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:199` | `update_group_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:244` | `add_user_to_group` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:286` | `remove_users_from_group` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:327` | `delete_group_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:374` | `preview_group_access` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/groups.py:478` | `inspect_group_members` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/images.py:280` | `get_config` | 封锁/固定：config 与 verify 禁用、models 固定列表；原生图管理特权不暴露用户数据。 |
| `routers/images.py:285` | `update_config` | 封锁/固定：config 与 verify 禁用、models 固定列表；原生图管理特权不暴露用户数据。 |
| `routers/images.py:347` | `verify_connection` | 封锁/固定：config 与 verify 禁用、models 固定列表；原生图管理特权不暴露用户数据。 |
| `routers/images.py:590` | `generate_images` | 保留：自己生图/编辑的功能权限；引擎/地址/Key 已固定，非个人数据读取旁路。 |
| `routers/images.py:828` | `image_generations` | 封锁/固定：config 与 verify 禁用、models 固定列表；原生图管理特权不暴露用户数据。 |
| `routers/images.py:901` | `edit_images` | 保留：自己生图/编辑的功能权限；引擎/地址/Key 已固定，非个人数据读取旁路。 |
| `routers/knowledge.py:186` | `get_knowledge_bases` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:211` | `get_knowledge_bases` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:251` | `search_knowledge_bases` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:276` | `search_knowledge_bases` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:328` | `create_new_knowledge` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:380` | `reindex_knowledge_files` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:472` | `reindex_knowledge_base_metadata_embeddings` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:692` | `get_external_knowledge_connections` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:701` | `create_external_knowledge_connection` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:721` | `get_external_knowledge_connection` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:734` | `update_external_knowledge_connection` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:759` | `delete_external_knowledge_connection` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:787` | `test_external_knowledge_connection` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:859` | `test_external_knowledge_source` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:884` | `test_external_knowledge_retrieval` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:898` | `create_external_knowledge` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:948` | `create_external_knowledge_source` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1014` | `update_external_knowledge_source` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1105` | `get_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1119` | `get_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1172` | `update_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1248` | `update_knowledge_access_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1313` | `get_pending_knowledge_files` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1373` | `get_knowledge_files_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1391` | `get_knowledge_files_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1451` | `add_file_to_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1473` | `add_file_to_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1550` | `update_file_from_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1640` | `remove_file_from_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1674` | `remove_file_from_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1725` | `delete_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1758` | `delete_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1818` | `reset_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:1834` | `reset_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:2031` | `sync_knowledge_cleanup` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:2080` | `add_files_to_knowledge_batch` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:2105` | `add_files_to_knowledge_batch` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:2178` | `export_knowledge_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:2259` | `_verify_knowledge_write_access` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/knowledge.py:2377` | `delete_knowledge_directory` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/memories.py:42` | `check_memories_permission` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/memories.py:447` | `reindex_memories_from_vector_db` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/models.py:140` | `_verify_background_image` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/models.py:176` | `_verify_voice_avatar` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/models.py:197` | `_verify_knowledge_file_access` | 收紧前置：新增 check_model_attachments 先校验个人文件/笔记，原 Workspace 判断随后保留。 |
| `routers/models.py:263` | `get_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:289` | `get_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:311` | `get_all_model_records` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:316` | `get_base_model_tags` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:324` | `get_base_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:339` | `get_model_tags` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:358` | `create_new_model` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:382` | `create_new_model` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:455` | `export_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:466` | `export_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:515` | `import_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:541` | `import_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:584` | `import_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:596` | `import_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:633` | `import_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:768` | `sync_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:809` | `get_model_by_id` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:864` | `authorized_model_history` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:926` | `_verify_version_dependencies.require_resource` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/models.py:941` | `_verify_version_dependencies` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/models.py:983` | `set_model_version` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:1012` | `get_model_profile_image` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:1042` | `get_model_profile_image` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:1118` | `toggle_model_by_id` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:1186` | `update_model_by_id` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:1205` | `update_model_by_id` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:1281` | `update_model_access_by_id` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:1311` | `update_model_access_by_id` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:1360` | `delete_model_by_id` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/models.py:1389` | `delete_all_models` | 保留：Workspace 模型（预设助手）管理/模型 ACL；个人附件另走收紧后的验证器。 |
| `routers/notes.py:50` | `check_notes_access` | 保留：Notes 功能许可；实际 note CRUD/search/socket 另查所有者/ACL。 |
| `routers/notes.py:78` | `leave_note_rooms_for_revoked_users` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:202` | `search_notes` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:277` | `get_note_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:292` | `get_note_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:325` | `get_note_chat_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:409` | `get_note_chats_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:461` | `create_note_chat_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:525` | `update_note_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:605` | `update_note_access_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:658` | `pin_note_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notes.py:701` | `delete_note_by_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/notifications.py:38` | `_check_notifications_access` | 封锁功能：ui.enable_user_webhooks=false；目标 CRUD 原本也固定 user.id。 |
| `routers/ollama.py:245` | `verify_connection` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:280` | `get_config` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:312` | `update_config` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:483` | `get_ollama_tags` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:511` | `get_ollama_loaded_models` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:557` | `get_ollama_versions` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:610` | `unload_model` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:673` | `pull_model` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:711` | `push_model` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:754` | `create_model` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:787` | `copy_model` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:826` | `delete_model` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:1101` | `validate_ollama_backend_idx` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:1534` | `get_openai_models` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:1679` | `download_model` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/ollama.py:1709` | `upload_model` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/openai.py:483` | `get_config` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:495` | `update_config` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:540` | `speech` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:822` | `get_models` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:906` | `get_provider_model_catalog` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:915` | `download_provider_model` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:938` | `get_provider_model_download_status` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:954` | `load_provider_model` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:970` | `unload_provider_model` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:982` | `stream_provider_model_events` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:991` | `delete_provider_model` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/openai.py:1026` | `verify_connection` | 锁定：仅固定网关的精确 models/chat/responses 入口可用；配置、索引 provider、speech 等封锁。 |
| `routers/pipelines.py:205` | `get_pipelines_list` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/pipelines.py:228` | `upload_pipeline` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/pipelines.py:317` | `add_pipeline` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/pipelines.py:367` | `delete_pipeline` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/pipelines.py:412` | `get_pipelines` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/pipelines.py:451` | `get_pipeline_valves` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/pipelines.py:498` | `get_pipeline_valves_spec` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/pipelines.py:539` | `update_pipeline_valves` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/prompts.py:56` | `get_prompts` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:66` | `get_prompt_tags` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:103` | `get_prompt_list` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:127` | `get_prompt_list` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:150` | `create_new_prompt` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:213` | `get_prompt_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:226` | `get_prompt_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:276` | `update_prompt_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:364` | `update_prompt_metadata` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:423` | `set_prompt_version` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:480` | `update_prompt_access_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:537` | `toggle_prompt_active` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:590` | `delete_prompt_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:634` | `get_prompt_history` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:672` | `get_prompt_diff` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:715` | `get_prompt_history_entry` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/prompts.py:758` | `delete_prompt_history_entry` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/retrieval.py:444` | `get_embedding_config` | 封锁：非 process/file 的 retrieval 路由禁用；内置本地联网搜索禁用。 |
| `routers/retrieval.py:513` | `update_embedding_config` | 封锁：非 process/file 的 retrieval 路由禁用；内置本地联网搜索禁用。 |
| `routers/retrieval.py:605` | `get_rag_config` | 封锁：非 process/file 的 retrieval 路由禁用；内置本地联网搜索禁用。 |
| `routers/retrieval.py:939` | `update_rag_config` | 封锁：非 process/file 的 retrieval 路由禁用；内置本地联网搜索禁用。 |
| `routers/retrieval.py:1591` | `process_file` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/retrieval.py:2570` | `process_web_search` | 封锁：非 process/file 的 retrieval 路由禁用；内置本地联网搜索禁用。 |
| `routers/retrieval.py:2896` | `delete_entries_from_collection` | 封锁：非 process/file 的 retrieval 路由禁用；内置本地联网搜索禁用。 |
| `routers/retrieval.py:2955` | `reset_vector_db` | 封锁：非 process/file 的 retrieval 路由禁用；内置本地联网搜索禁用。 |
| `routers/retrieval.py:2969` | `reset_upload_dir` | 封锁：非 process/file 的 retrieval 路由禁用；内置本地联网搜索禁用。 |
| `routers/retrieval.py:3067` | `process_files_batch` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `routers/scim.py:711` | `update_user` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/scim.py:803` | `patch_user` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/scim.py:874` | `delete_user` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/search.py:439` | `search` | 保留：决定启用搜索种类；search_workspace 的 SQL 始终按 user.id/显式 ACL 过滤。 |
| `routers/skills.py:63` | `get_skills` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:105` | `get_skill_list` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:147` | `authorized_skill` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:173` | `export_skills` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:185` | `export_skills` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:226` | `create_new_skill` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:304` | `get_skill_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:317` | `get_skill_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:369` | `update_skill_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:452` | `update_skill_access_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:495` | `toggle_skill_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:561` | `delete_skill_by_id` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:795` | `require_import` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/skills.py:808` | `load_skill_by_url` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/tasks.py:137` | `update_task_config` | 封锁：tasks/config 403；仅保留自己的标题生成。 |
| `routers/tools.py:84` | `get_tools` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:241` | `get_tool_list` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:304` | `load_tool_from_url` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:361` | `export_tools` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:372` | `export_tools` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:393` | `create_new_tools` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:480` | `get_tools_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:491` | `get_tools_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:553` | `_update_tool` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:673` | `update_tool_access_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:729` | `delete_tools_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:778` | `get_tools_valves_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:823` | `get_tools_valves_spec_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:870` | `update_tools_valves_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:931` | `get_tools_user_valves_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:971` | `get_tools_user_valves_spec_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:1013` | `update_tools_user_valves_by_id` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:1052` | `can_change_tool_code` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/tools.py:1063` | `require_tool_history_access` | 保留：Workspace 函数/提示词/工具管理或执行 ACL；不是个人数据管理员旁路。 |
| `routers/users.py:90` | `get_users` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:133` | `get_all_users` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:417` | `get_default_user_permissions` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:432` | `update_default_user_permissions` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:446` | `get_default_user_permissions_defaults` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:500` | `update_user_settings_by_session_user` | 保留：只写当前会话用户 settings；功能/偏好权限，不授予他人资源权限。 |
| `routers/users.py:508` | `update_user_settings_by_session_user` | 保留：只写当前会话用户 settings；功能/偏好权限，不授予他人资源权限。 |
| `routers/users.py:518` | `update_user_settings_by_session_user` | 保留：只写当前会话用户 settings；功能/偏好权限，不授予他人资源权限。 |
| `routers/users.py:528` | `update_user_settings_by_session_user` | 保留：只写当前会话用户 settings；功能/偏好权限，不授予他人资源权限。 |
| `routers/users.py:541` | `update_user_settings_by_session_user` | 保留：只写当前会话用户 settings；功能/偏好权限，不授予他人资源权限。 |
| `routers/users.py:802` | `get_user_by_id` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:844` | `get_user_oauth_sessions_by_id` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:923` | `revoke_user_sessions` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:942` | `update_user_by_id` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:957` | `update_user_by_id` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:1068` | `delete_user_by_id` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:1120` | `get_user_groups_by_id` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/users.py:1134` | `get_user_preview` | 封锁：用户管理路由 403（只保留自己的设置、变量、info 等精确例外）。 |
| `routers/utils.py:31` | `format_code` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `routers/utils.py:75` | `download_db` | 封锁：对应产品路由在 blocklist 禁用；相关功能配置/内置类别亦关闭。 |
| `socket/encryption.py:125` | `Relay.authorize` | 封锁：加密频道依赖 channels.enable，嵌入模式强制 false。 |
| `socket/encryption.py:196` | `Relay.eligible_users` | 封锁：加密频道依赖 channels.enable，嵌入模式强制 false。 |
| `socket/main.py:499` | `enter_room_for_users` | 封锁：频道功能配置为 false，connect/user-join/join-channel 均显式禁止进入频道。 |
| `socket/main.py:626` | `user_join` | 封锁：频道功能配置为 false，connect/user-join/join-channel 均显式禁止进入频道。 |
| `socket/main.py:698` | `join_channel` | 封锁：频道功能配置为 false，connect/user-join/join-channel 均显式禁止进入频道。 |
| `socket/main.py:720` | `join_note` | 保留：笔记功能许可；加入房间与更新另查 note 所有者/ACL。 |
| `socket/main.py:731` | `join_note` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `socket/main.py:928` | `ydoc_document_join` | 保留：笔记功能许可；加入房间与更新另查 note 所有者/ACL。 |
| `socket/main.py:940` | `ydoc_document_join` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `socket/main.py:1022` | `document_save_handler` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `socket/main.py:1108` | `yjs_document_update` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:115` | `_has_read_access_to_file` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:1262` | `view_note` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:1385` | `replace_note_content` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:2235` | `search_knowledge_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:2289` | `search_knowledge_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:2705` | `grep_knowledge_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:2934` | `view_knowledge_file` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:2949` | `view_knowledge_file` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:3084` | `list_knowledge` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:3130` | `list_knowledge` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:3233` | `query_knowledge_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:3258` | `query_knowledge_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:3282` | `query_knowledge_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:3749` | `create_skill` | 封锁：create_skill 属 skills 类别，默认/交集白名单禁用。 |
| `tools/builtin.py:4605` | `create_calendar_event` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:4728` | `update_calendar_event` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/builtin.py:4832` | `delete_calendar_event` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `tools/knowledge_fs.py:366` | `_get_accessible_kb_ids._has_access` | 封锁：knowledge 类别/向量库禁用，不暴露 kb_exec。 |
| `utils/access_control/__init__.py:164` | `has_connection_access` | 保留：Workspace 模型/工具服务器 ACL；未将真实身份整体降级。 |
| `utils/access_control/__init__.py:172` | `has_connection_access` | 保留：Workspace 模型/工具服务器 ACL；未将真实身份整体降级。 |
| `utils/access_control/__init__.py:240` | `filter_allowed_access_grants` | 保留：授权分配能力（已经过资源所有权/写权限检查）；不直接赋予资源访问权。 |
| `utils/access_control/__init__.py:251` | `filter_allowed_access_grants` | 保留：授权分配能力（已经过资源所有权/写权限检查）；不直接赋予资源访问权。 |
| `utils/access_control/__init__.py:333` | `has_base_model_access` | 保留：Workspace 模型/工具服务器 ACL；未将真实身份整体降级。 |
| `utils/access_control/__init__.py:340` | `has_base_model_access` | 保留：Workspace 模型/工具服务器 ACL；未将真实身份整体降级。 |
| `utils/access_control/__init__.py:392` | `check_model_access` | 保留：Workspace 模型/工具服务器 ACL；未将真实身份整体降级。 |
| `utils/access_control/__init__.py:419` | `check_model_access` | 保留：Workspace 模型/工具服务器 ACL；未将真实身份整体降级。 |
| `utils/access_control/files.py:144` | `get_accessible_folder_files` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `utils/actions.py:58` | `chat_action` | 保留：Workspace 模型或函数 action 执行权限；聊天 action 另有所有者检查。 |
| `utils/apps.py:248` | `has_app_access` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/apps.py:266` | `check_app_channel_access` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/apps.py:268` | `check_app_channel_access` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/apps.py:485` | `send_app_invocation` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/apps.py:625` | `authorize_app_execution` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/auth.py:535` | `get_current_user_by_api_key` | 保留：身份/API-Key 开关与 admin dependency 角色验证；不直接放行用户资源。 |
| `utils/auth.py:632` | `get_admin_user` | 保留：身份/API-Key 开关与 admin dependency 角色验证；不直接放行用户资源。 |
| `utils/auth.py:694` | `create_signin_response` | 保留：身份/API-Key 开关与 admin dependency 角色验证；不直接放行用户资源。 |
| `utils/automations.py:269` | `_execute_channel_automation` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/automations.py:379` | `execute_automation` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/automations.py:380` | `execute_automation` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/files.py:229` | `get_image_base64_from_file_id` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `utils/memory.py:450` | `review_memory_after_turn` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/mfa.py:226` | `load_challenge` | 封锁功能：Apps/自动化/记忆/MFA 配置强制关闭，关联路由禁用。 |
| `utils/middleware.py:2251` | `process_chat_payload` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `utils/middleware.py:2356` | `process_chat_payload` | 保留功能许可：搜索/生图/代码/skills 开关；禁用功能不可恢复，note/folder 资源旁路已收紧。 |
| `utils/middleware.py:2365` | `process_chat_payload` | 保留功能许可：搜索/生图/代码/skills 开关；禁用功能不可恢复，note/folder 资源旁路已收紧。 |
| `utils/middleware.py:2376` | `process_chat_payload` | 保留功能许可：搜索/生图/代码/skills 开关；禁用功能不可恢复，note/folder 资源旁路已收紧。 |
| `utils/middleware.py:2382` | `process_chat_payload` | 保留功能许可：搜索/生图/代码/skills 开关；禁用功能不可恢复，note/folder 资源旁路已收紧。 |
| `utils/middleware.py:2436` | `process_chat_payload` | 保留功能许可：搜索/生图/代码/skills 开关；禁用功能不可恢复，note/folder 资源旁路已收紧。 |
| `utils/middleware.py:2486` | `process_chat_payload` | 收紧：`resource_admin` 在嵌入模式恒为 False；保留所有者/显式 ACL。 |
| `utils/middleware.py:4612` | `streaming_chat_response_handler.response_handler` | 保留功能许可：搜索/生图/代码/skills 开关；禁用功能不可恢复，note/folder 资源旁路已收紧。 |
| `utils/models.py:516` | `check_model_access` | 保留：Workspace 模型或函数 action 执行权限；聊天 action 另有所有者检查。 |
| `utils/models.py:578` | `get_filtered_models` | 保留：Workspace 模型或函数 action 执行权限；聊天 action 另有所有者检查。 |
| `utils/models.py:607` | `get_filtered_models` | 保留：Workspace 模型或函数 action 执行权限；聊天 action 另有所有者检查。 |
| `utils/models.py:619` | `get_filtered_models` | 保留：Workspace 模型或函数 action 执行权限；聊天 action 另有所有者检查。 |
| `utils/models.py:625` | `get_filtered_models` | 保留：Workspace 模型或函数 action 执行权限；聊天 action 另有所有者检查。 |
| `utils/plugin.py:488` | `install_tool_and_function_dependencies` | 保留：可信管理员插件的依赖来源判断；自动 pip 安装已关闭。 |
| `utils/tools.py:373` | `get_tools` | 保留：Workspace 工具 ACL 或内置功能许可；类别白名单与每工具资源检查另行执行。 |
| `utils/tools.py:647` | `get_builtin_tools.has_user_permission` | 保留：Workspace 工具 ACL 或内置功能许可；类别白名单与每工具资源检查另行执行。 |
| `utils/tools.py:656` | `get_builtin_tools.has_user_chat_permission` | 保留：Workspace 工具 ACL 或内置功能许可；类别白名单与每工具资源检查另行执行。 |
| `utils/tools.py:866` | `get_builtin_tools` | 保留：Workspace 工具 ACL 或内置功能许可；类别白名单与每工具资源检查另行执行。 |

### 测试命令与结果（最终代码）

```bash
cd /home/qlqq/workspace/open-webui
/home/qlqq/workspace/owui-poc/venv/bin/python -m pytest -q backend/open_webui/test/embed
PYTHONPATH=backend /home/qlqq/workspace/owui-poc/venv/bin/python \
  backend/open_webui/test/embed/smoke/run.py
/home/qlqq/workspace/owui-poc/venv/bin/python -m compileall -q backend/open_webui
bash -n backend/start.sh
git diff --check -- backend
```

- **pytest：301 passed，2 warnings，56.31 秒**。两条 warning 仍为 Typer 调用 Click 弃用接口，不是失败。
- compileall、start.sh 语法与 backend diff whitespace 检查均通过。
- pytest 内已运行真实冒烟，证据：`/tmp/s2a-stage1-2ji5ev2x/`；测试日志：`/tmp/stage1-r3-final-tests.log`。
- 独立冒烟复跑结果及证据目录见下方；所有服务只绑定 127.0.0.1:28200–28202，测试结束已回收服务进程。桩流量与 socket 连接均留在本机；后端 audit hook 记录的所有实际出站仅到假网关 28201，没有外网尝试。

独立运行日志：`/tmp/stage1-r3-final-smoke.log`。证据目录中含 `result.json`、`gateway.json`、`embed/network.jsonl`、后端日志；`upstream/` 保存非嵌入模式启动记录。

```json
{
  "status": "PASS",
  "gateway_requests": 32,
  "native_search_requests": 9,
  "native_generate_image_with_image_key": "PASS",
  "admin_isolation_checks": 191,
  "network_destinations": [
    "127.0.0.1:28201"
  ],
  "upstream_mode_startup": "PASS",
  "admin_tool_config_persistence": "PASS",
  "anonymous_static_health_probe": "PASS",
  "artifacts": "/tmp/s2a-stage1-y9uqxbok"
}
```

### 已知问题与信任边界

1. 上文列出的 **C1 默认原生生图问题和 R3 管理员 HTTP/socket 资源越权缺口已修复并回归**。本轮没有声称验证生产 Redis 多 worker、真实网关或真实账单；当前证据为独立 SQLite + 单进程后端 + 假反代/假网关。后续阶段仍需生产部署组合验收。
2. “管理员不得访问他人资源”在本轮指通过产品提供的普通 API、模型内置工具、附件检索和 socket 通道不能获得管理员旁路。**保留的 Workspace 函数/工具是可信 Python 代码，不是沙箱**：拥有任意插件代码执行或数据库/容器运维权限的人，可以在进程/数据库层绕过应用 ACL。要防这类恶意插件管理员，需要独立执行沙箱/服务与存储隔离，超出本轮接口修补范围；不能把本次通过误解为隔离任意恶意 Python。
3. 显式授予 read/write 的共享仍按上游语义生效；这不是管理员越权。日历 owner-only 管理、文件共享只读、folder 共享只读等不因 admin 自动升级。
4. 前轮其余真实联调、已有 WebSocket 对上游 Cookie 撤销的限制、出站白名单部署要求仍有效；本轮不访问真实服务、不处理前端代码。

### 约定变更提议

**本轮没有新增变更提议，未修改 contract.md。** C1/C2 已依已更新约定落实并验证；C3 依已更新约定使用静态健康检查。新增全局任务入口封锁是落实 R3 管理员隔离，不要求新增对外能力。

## 补充 2

日期：2026-10-24。已先阅读 `reports/stage2.md` 第 7 节及更新后的 contract B.4 / D，按本次前后端联调约定实现；本节覆盖旧报告中“config 换账号返回 401”“functions 列表仅 admin”“groups 全禁”的旧结论。

### 完成项

| 项目 | 实现与验证 |
|---|---|
| B.4 百分号编码 NAME | 中间件先检查 ASCII、严格校验 `%HH` 和 UTF-8，再解码；拒绝非法 UTF-8、残缺/非法转义、解码后的 NUL/CR/LF/DEL，以及旧版原始非 ASCII 名称。HTTP 返回 401；WebSocket ASGI 边界以 1008 关闭、不进入下游。 |
| NAME 交给上游可信头 | 上游 signin 本来会执行一次 `urllib.parse.unquote`。中间件验证解码后重新规范化为 UTF-8 百分号编码的 **ASCII** 头交给该逻辑，绝不将解码的非 ASCII 字符放入 HTTP/WS `scope['headers']`。不使用 `unquote_plus`，保留真正的 `+`；`%252F` 最终姓名为字面量 `%2F`，不会二次变成 `/`。 |
| `/api/config` 换账号 | 新增 config 专用身份同步入口：token 邮箱与可信身份不一致时只将本次 config 的 user 视为 None，返回与未登录请求完全相同的 config。不会同步/修改旧 token 所属账户的角色，不加载该账户的用户专属配置。其他接口仍调用原同步入口并返回 401。 |
| functions 精确只读例外 | `GET /api/v1/functions/` 对已登录普通用户和管理员放行；无 token 仍须上游鉴权。`/list`、`/export` 等子路由及创建/修改权限没有随之放开，URL 导入仍禁用。 |
| groups 精确只读例外 | `GET /api/v1/groups/` 仅管理员放行，普通用户 403；其他 groups 路径/写请求继续 404。规则沿用可选末尾 `/` 的同资源别名匹配，不是前缀开放。 |
| 无 token socket | **保持原实现，不放开匿名连接。** 测试确认 connect 对 None/空 auth/空 token/缺 token 的 auth 在查用户或存 session 前返回 False；真实 Socket.IO 无 token 连接被服务端拒绝，随后携带 token 主动建立的新连接正常。 |

**处理策略说明**：对非法转义、非法 UTF-8、控制字符及原始非 ASCII NAME 选择严格拒绝而非按原值显示，防止畸形编码进入身份链；合法原始 ASCII 符号仍可兼容规范化（例如 `+` 保持加号，不按表单编码变成空格）。仅对 NAME 做协议规范化，非嵌入模式仍直接透传；反代必须按新约定产生 ASCII 头，不能依赖后端 ASGI 中间件修复已进入 uvicorn 协议层的非 ASCII WebSocket 原始握手头。

config 的匿名回退仅用于有效 token 用户与当前可信身份不一致，不扩大为“吞掉所有鉴权错误”：缺失/错误身份头仍由中间件拒绝；其他资源接口的账号不匹配仍 401；无匿名 `/health` 例外。

### 本轮改动文件

源码（均在 `backend/open_webui/`）：

- `embed/sub2api_auth.py`：严格 NAME 解码/规范化、`sync_config_user()`；其他接口身份同步保持严格。
- `embed/blocklist.py`：functions / groups 两个精确 GET 例外。
- `main.py`：`get_app_config()` 的最小挂接，保留 `# sub2api-embed:` 注释。

测试：

- `test/embed/test_boundary.py`：中文、emoji、大小写百分号转义、空格、`+`、字面 `%`、双重转义；非法转义/UTF-8/控制字符/原始非 ASCII；HTTP 与 WebSocket 均覆盖；config 不匹配时不修改旧账户。
- `test/embed/test_blocklist.py`：只读精确路径、末尾斜杠别名、POST/PUT/PATCH/DELETE/HEAD、子路径/相似前缀的边界。
- `test/embed/smoke/hook_checks.py`：无 token socket 的真实 connect 函数回归。
- `test/embed/smoke/run.py`：假反代 NAME 改为 ASCII 百分号编码；中文/emoji/字面 `%2F` 真实 signin 名称断言及 WS 流式聊天；旧普通用户/旧管理员 token 的 Bearer 与 HttpOnly Cookie 两种换号场景，config 匿名回退→重新 signin→已登录 config；其他 API 仍 401；functions/groups 的真实读写权限检查。

未修改 `socket/main.py` 的拒绝逻辑、前端文件、contract 或 sub2api 代码；未执行 git 写操作、提交或推送。没有新增环境变量。

### 测试与冒烟

```bash
cd /home/qlqq/workspace/open-webui
/home/qlqq/workspace/owui-poc/venv/bin/python -m pytest -q backend/open_webui/test/embed
PYTHONPATH=backend /home/qlqq/workspace/owui-poc/venv/bin/python \
  backend/open_webui/test/embed/smoke/run.py
/home/qlqq/workspace/owui-poc/venv/bin/python -m compileall -q backend/open_webui
bash -n backend/start.sh
git diff --check -- backend
```

- **全部 pytest：352 passed，2 warnings，75.79 秒**。warning 仍仅为 Typer/Click 弃用提示；日志 `/tmp/stage1-supp2-tests.log`。
- pytest 内真实冒烟证据：`/tmp/s2a-stage1-q527cnm2/`；独立完整复跑也通过，日志 `/tmp/stage1-supp2-smoke.log`，证据目录见下方 JSON。
- 原有两用户并发对话、标题生成、原生生图及生图 Key、联网搜索 tools、PDF 上传、191 项管理员隔离检查和非嵌入模式启动均继续通过。
- 新增四项冒烟结果全部 PASS：百分号编码姓名的 HTTP/WS、Bearer/Cookie 两种 config 换账号恢复、functions/groups 只读列表、无 token socket 拒绝。
- compileall、start.sh 语法检查和 backend diff whitespace 检查全部通过。仅使用 127.0.0.1:28200–28202；后端实际出站仅假网关 28201；测试服务已退出，端口已释放。

```json
{
  "status": "PASS",
  "gateway_requests": 32,
  "native_search_requests": 9,
  "native_generate_image_with_image_key": "PASS",
  "admin_isolation_checks": 191,
  "network_destinations": [
    "127.0.0.1:28201"
  ],
  "upstream_mode_startup": "PASS",
  "admin_tool_config_persistence": "PASS",
  "anonymous_static_health_probe": "PASS",
  "percent_encoded_name_http_and_websocket": "PASS",
  "config_account_switch_bearer_and_cookie": "PASS",
  "readonly_function_and_admin_group_lists": "PASS",
  "tokenless_socket_rejected": "PASS",
  "artifacts": "/tmp/s2a-stage1-vivroa63"
}
```

### 已知问题 / 约定变更提议

本轮无新增约定变更提议；采用 contract 的最新 B.4 / D。生产反代的 NAME 百分号编码须与 fork 同步上线；旧版原始 UTF-8 头不再作为可兼容输入。真实 sub2api 和浏览器的跨项目联调仍由相应阶段负责，本轮为本机假反代/假网关验证。

前轮可信 Python 插件并非沙箱、生产 Redis 多 worker 尚未验证等边界保持；新增 groups 例外只提供管理员授权选择器的只读列表，不恢复分组管理写入口，也不恢复 admin 对会话/文件/笔记/日历的旁路。

## 补充 3

### 网关模型列表错误透传（2026-10-24）

**已完成**：嵌入模式 `/api/models`（以及 `/api/v1/models` 别名）拉取网关模型列表失败时，保留网关 HTTP 状态码，返回顶层 `detail` 与 `s2a_gateway_error`，不再只给出泛化的模型列表错误。余额不足实测响应：

```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{"detail":"账户余额不足","s2a_gateway_error":{"status":403,"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}}
```

本轮只读核对了 sub2api `backend/internal/server/middleware/api_key_auth.go`、`middleware.go` 中的真实错误格式：认证中间件使用顶层 `code/message`，兼容 OpenAI 的错误使用嵌套 `error.code/type/message`。未修改 sub2api 源码。

| 要求 | 实现与验证 |
| --- | --- |
| 状态及错误体 | 专用 `GatewayModelError` 与异常处理器直接生成顶层 JSON；不把扩展字段嵌套到 `detail`；普通 HTTPException 的原响应保持不变 |
| code 提取 | 优先顶层 `code`，其次 `error.code`，最后 `error.type`；保留原始大小写；缺失或字段类型错误时回退 `GATEWAY_ERROR` |
| 中文提示 | `INSUFFICIENT_BALANCE` →「账户余额不足」；`INVALID_API_KEY`、`API_KEY_DISABLED`、`API_KEY_EXPIRED` 等失效/禁用/过期/撤销类 →「所选 Key 不可用」；限流 code 或 HTTP 429 →「请求过于频繁」；其他 →「网关模型列表不可用」 |
| message | 顶层或嵌套 message 原文按 Python 字符切片截断至 300 字符；中文/emoji 不按 UTF-8 字节截断；非 JSON 错误体回退原响应文本的前 300 字符 |
| 网络失败/超时 | HTTP 502，code=`GATEWAY_UNREACHABLE`；使用固定 message，不输出异常字符串中的 URL、凭据或底层 socket 信息；协程取消正常传播 |
| 缓存 | 只缓存成功结果；失败不写入 Key 缓存及模型元数据并集；强制刷新或过期重新获取前移除当前 Key 的旧条目，失败后下一请求立即重试，不回退旧成功结果 |
| 刷新隔离 | 修复 `refresh=true` 未传入嵌入缓存的问题；只刷新当前 Key，不清理其他 Key 或上游全局缓存；成功缓存仍为 60 秒 |
| 畸形成功响应 | HTTP 200 但 JSON/模型列表结构无效时返回 502、`GATEWAY_INVALID_RESPONSE`，不缓存为成功空列表 |
| 非嵌入模式 | 仍走原版模型链路；专用异常处理器仅在嵌入模式注册；关闭嵌入模式的启动冒烟通过 |

### 本轮改动文件

源码（均在 `backend/open_webui/`）：

- `embed/models.py`：错误解析与中文映射、专用异常/响应处理器、网络错误转换、失败缓存处理、成功结构校验。
- `main.py`：在既有嵌入初始化块注册专用处理器，带 `# sub2api-embed:` 注释。
- `utils/models.py`：嵌入模式将 `refresh` 传至按 Key 缓存，不清空原版全局缓存；最小挂接带注释。

测试：

- 新增 `test/embed/test_model_errors.py`：45 项用例，覆盖两种网关错误形状、code 优先级/type 回退、中文提示、未知/非 JSON/错误类型字段、300 字符截断、网络异常/超时/取消、异常处理器的扁平 JSON、普通异常不变、失败重试及其他 Key 缓存保留。
- `test/embed/smoke/run.py`：假网关按 Key 注入错误，真实经过假反代→uvicorn→网关。7 类错误（余额不足 403、Key 无效 401、过期 403、限流 429、未知错误 503、非 JSON 502、拒绝跟随的重定向 302），逐类验证 `/api/models?refresh=true`、`/api/models`、`/api/v1/models` 的状态及完整 JSON；再验证其他 Key 仍命中缓存、当前 Key 恢复后立即重取且成功结果可缓存。

没有新增环境变量或依赖；没有修改前端、contract、sub2api 代码；未做 git 写操作、提交或推送。

### 测试命令与结果

```bash
cd /home/qlqq/workspace/open-webui
/home/qlqq/workspace/owui-poc/venv/bin/python -m pytest -q backend/open_webui/test/embed
PYTHONPATH=backend /home/qlqq/workspace/owui-poc/venv/bin/python \
  backend/open_webui/test/embed/smoke/run.py
/home/qlqq/workspace/owui-poc/venv/bin/python -m compileall -q backend/open_webui
bash -n backend/start.sh
git diff --check -- backend
```

- **全部 pytest：397 passed，2 warnings，52.57 秒**；相较补充 2 新增 45 项。两个 warning 仍仅为 Typer/Click 弃用提示。日志：`/tmp/stage1-supp3-tests.log`。
- pytest 自带完整冒烟通过，证据：`/tmp/s2a-stage1-oq09m4m3/`。
- 独立完整冒烟再次通过，日志：`/tmp/stage1-supp3-smoke.log`；证据：`/tmp/s2a-stage1-y7ojtncu/`（含 `result.json`、`gateway.json`、`embed/network.jsonl`）。
- 两用户并发对话、标题生成、原生生图工具与专用生图 Key、联网搜索 tools、PDF 上传、191 项管理员隔离、补充 2 身份/config/socket 回归全部保持通过。
- compileall、start.sh 语法检查、backend diff whitespace 检查通过。
- 服务仅绑定 `127.0.0.1:28200–28202`；后端实际出站只有假网关 `127.0.0.1:28201`。测试服务已退出，三个端口已释放。

独立冒烟摘要：

```json
{
  "status": "PASS",
  "gateway_requests": 60,
  "native_search_requests": 9,
  "native_generate_image_with_image_key": "PASS",
  "admin_isolation_checks": 191,
  "network_destinations": ["127.0.0.1:28201"],
  "upstream_mode_startup": "PASS",
  "model_error_status_body_cache_recovery": "PASS",
  "model_error_cases": 7,
  "artifacts": "/tmp/s2a-stage1-y7ojtncu"
}
```

### 已知问题 / 约定变更提议

无新增约定变更提议。成功结果的 60 秒缓存机制未取消：缓存有效期内不带 `refresh=true` 的请求不会重新探测余额，只有实际拉取网关失败时返回本次新增错误体；刷新失败不会继续使用旧成功条目。

本轮没有访问真实网关或真实计费服务，网络故障/超时通过受控单元测试验证，真实 HTTP 链路通过本机假网关验证。生产 Redis 多 worker、可信 Python 插件不是沙箱等既有边界保持不变。
