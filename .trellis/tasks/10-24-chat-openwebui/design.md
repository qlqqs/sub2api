# 设计：对话工作台（Open WebUI fork 原生嵌入）

依据：`research/paths.md`、`auth.md`、`upstream.md`、`frontend.md`、`security-deploy.md`（均基于 v0.12.3 源码核实）。
参照：创作工作台 `backend/internal/server/studio_proxy.go` 与 Nova 契约 `docs/sub2api-embed.md`。

## 1. 总体结构

```
浏览器 ──同源── sub2api（1Panel 反代整域名）
                 ├─ /chat              控制台 Vue 页面（QxShell 内，iframe 指向 /_chat/）
                 ├─ /api/v1/chat/*     sub2api 自有：会话 Cookie 签发 / 清除、开关状态
                 ├─ /_chat/__sub2api/* sub2api 自有：给 iframe 用的 Key 列表、单价（Cookie 鉴权，不返回明文）
                 └─ /_chat/*           反代到 Open WebUI（剥掉 /_chat 前缀；含 WebSocket）
                                          │  内部网络 sub2api-*-chat（internal: true，无外网）
                                     Open WebUI（fork，slim 镜像，uvicorn --no-proxy-headers，按根路径服务）
                                          │  所有模型请求 → http://sub2api:8080/v1/*（用户所选 Key）
```

- Open WebUI 前后端都放在 `/_chat` 下（SvelteKit `paths.base`），sub2api 一条反代规则。不拆 `/api/chat`：Open WebUI 已有同名路由。
- 后端**不用** `--root-path`（PoC：uvicorn 0.51 会把 root_path 加回 `scope['path']`，engine.io 按原始路径匹配 → socket.io 404）。sub2api 剥掉 `/_chat` 后按根路径转发；需要绝对 URL 的地方用 `WEBUI_URL=https://<域名>/_chat`（`--root-path` 只对 OAuth / SCIM / MFA 有用，均关闭）。
- `/_chat/__sub2api/` 是 sub2api 保留前缀，Open WebUI 不使用。
- 各方接口的最终约定见 `contract.md`（与本文冲突时以它为准）。

## 2. 身份与会话

### 2.1 sub2api 会话 Cookie

Open WebUI 自己占用 `Authorization` 与 `token` Cookie，且 `<img>`、WebSocket 握手带不了自定义头，所以 sub2api 身份用独立 Cookie 传递：

- 父页（`/chat`）加载 iframe 前调用 `POST /api/v1/chat/session`（常规 Bearer JWT 鉴权）→ sub2api 回 `Set-Cookie: s2a_chat=<token>; Path=/_chat; HttpOnly; Secure; SameSite=Strict`。
  - token 内容：sub2api 现有 access token（复用 `jwtAuth` 的校验与封禁 / 版本检查）。父页在 token 刷新后重新调用。
  - iframe 页面拿不到 sub2api JWT（HttpOnly，且不经 postMessage 传递）——比 Nova 更收紧。
- 退出登录 / 切换账号：控制台退出流程调用 `DELETE /api/v1/chat/session` 清 Cookie；iframe 若打开，同时收到 `revoke` 消息。
- `/_chat/*` 中静态资源（`_app/`、`static/`、页面 HTML）不需要 Cookie；其余（API、WebSocket、文件 / 头像）没有有效 Cookie → 401，不转发。

### 2.2 注入给 Open WebUI 的头

sub2api 先删除客户端自带的全部 `X-Sub2API-*`，再注入：

| 头 | 值 |
|---|---|
| `X-Sub2API-User-Id` | 用户 ID |
| `X-Sub2API-Email` | `u<用户ID>@sub2api.internal`（合成，永不变化） |
| `X-Sub2API-Name` | 用户名 / 邮箱前缀（显示用） |
| `X-Sub2API-User-Role` | sub2api 管理员 `admin`，其他 `user`（见 2.4） |
| `X-Sub2API-Key` | 当前选中 Key 的明文（见 §3；未选则不注入） |
| `X-Forwarded-For` / `X-Real-IP` | 用户真实 IP（同 Nova 的解析规则） |

### 2.3 Open WebUI 侧（方案 B：复用内置可信头）

- 启动：uvicorn `--no-proxy-headers`（默认 `forwarded_allow_ips='*'` 会让任意来源伪造 XFF）；新增 ASGI 中间件 `embed/sub2api_auth.py`：
  - TCP 对端不在 `S2A_TRUSTED_PROXY_CIDRS` → 删除所有 `X-Sub2API-*`，非静态请求直接 403。
  - 可信对端：校验格式，把 `X-Forwarded-For` 写回 `scope['client']`，把 `X-Sub2API-Key` 放入 `ContextVar`。
  - ContextVar **只对 `http` 作用域设置**；websocket 不设（PoC：socket.io 处理器会一直持有握手时的 Key，换 Key 后仍是旧的）。sub2api 在 WebSocket 升级请求上也不注入 Key。
- 环境变量：`WEBUI_AUTH_TRUSTED_EMAIL_HEADER=X-Sub2API-Email`、`WEBUI_AUTH_TRUSTED_NAME_HEADER=X-Sub2API-Name`、`WEBUI_AUTH_TRUSTED_ROLE_HEADER=X-Sub2API-User-Role`、`DEFAULT_USER_ROLE=user`、`ENABLE_SIGNUP=false`、`ENABLE_LOGIN_FORM=false`、`ENABLE_INITIAL_ADMIN_SIGNUP=false`、`ENABLE_PASSWORD_CHANGE_FORM=false`、`ENABLE_PERSISTENT_CONFIG=false`、`ENABLE_API_KEYS=false`、`ENABLE_OAUTH=false`、`ENABLE_LDAP=false`、`ENABLE_SCIM=false`、显式 `WEBUI_SECRET_KEY`。（不能设 `ENABLE_PASSWORD_AUTH=false`，会连可信头一起拒绝。）
- 补丁：
  - `utils/auth.py:458-464`：邮箱头**缺失也拒绝**；每请求按角色头同步角色。
  - `routers/auths.py:824-827`：去掉「首个用户自动成为管理员」。
  - socket.io `connect` / `user-join` / 30 秒复检（`socket/main.py:408-430,551-606`）：校验 token 用户邮箱 == 握手请求的 `X-Sub2API-Email`。
  - 屏蔽：管理员添加用户（直接返回 JWT）、改邮箱 / 密码 / 角色接口、`signout` 的重定向（改为通知父页）。
- 流程：iframe 打开 → Open WebUI 前端见 `auth_trusted_header` 自动 signin → 拿到自己的 JWT（localStorage）。换账号后旧 JWT 与新邮箱头不一致 → 401 → `clearExpiredSession` → 自动重新 signin。

### 2.4 角色

- sub2api 管理员 → Open WebUI `admin`，普通用户 → `user`；每次请求按 `X-Sub2API-User-Role` 同步（补丁见 2.3）。
- 管理员在 Open WebUI 里只能用：Workspace（预设助手 = 自定义模型、提示词库）、管理后台的「函数 / 工具 / 工具服务器」三页。其余管理页与接口由 `embed/blocklist.py` 屏蔽（用户、设置、连接、模型、评测、统计、Playground、各类配置写入口）。
- `ENABLE_ADMIN_CHAT_ACCESS=false`：管理员看不到他人聊天。
- 普通用户：`USER_PERMISSIONS_WORKSPACE_*` 全部 false（原版默认），Workspace 入口隐藏；能在模型下拉与 `/` 菜单里使用管理员共享的预设助手与提示词（访问控制用原版的公开 / 分组授权）。

## 3. 计费：每用户 Key

### 3.1 选 Key（Key ID 存在 Open WebUI 用户设置里，明文只在请求链路上）

- Open WebUI 设置页新增「模型与密钥」分区（嵌入模式下替换 Connections）：
  - 数据来自 `GET /_chat/__sub2api/keys`（Cookie 鉴权）：只返回 ID、名称、状态、分组、平台、额度 / 过期，**不返回明文**。
  - 一键创建：`POST /_chat/__sub2api/keys`（分组列表 `GET /_chat/__sub2api/groups`）。
  - 选中的 Key ID 存入 Open WebUI 用户设置（服务端按用户隔离）。
- 生图 Key（PRD R5.6）：同一分区多一个可选下拉，只列平台支持生图的 Key；Key ID 同样存用户设置。
- 前端在全局 `window.fetch` 覆写处（`src/routes/+layout.svelte:1147`）给发往 `/_chat/` 的请求加 `X-Sub2API-Key-Id`，设了生图 Key 时再加 `X-Sub2API-Image-Key-Id`。
- sub2api 反代：读取 `X-Sub2API-Key-Id` → 校验属于当前用户且可用 → 注入 `X-Sub2API-Key`（明文），删除客户端带来的 `X-Sub2API-Key`。生图 Key 同理：`X-Sub2API-Image-Key-Id` → `X-Sub2API-Image-Key`（另校验分组平台支持生图）；未设置时不注入，Open WebUI 回退到对话 Key（仅当对话 Key 支持生图，由 `__sub2api/keys` 返回的平台字段判断）。Key 不属于本人 / 不可用 → 不注入（Open WebUI 返回「请选择可用的 Key」）。
- 浏览器全程看不到 Key 明文。

### 3.2 Open WebUI 出口强制（`embed/gateway.py`）

- `routers/openai.py:255` `get_openai_connection`：永远返回 `(S2A_GATEWAY_URL + '/v1', ctx_key, 清洗后的 config)`，去掉 azure / auth_type / headers / forward_cookies / prefix_id 等。
- `utils/headers.py:170` `get_headers_and_cookies` 末尾强制 `Authorization: Bearer ctx_key`、cookies 置空、加 `X-Forwarded-For`；`openai.py:99-105` 旧分支同改。
- 开放的其他调用（RAG embedding 等）改为调用时读 `ContextVar`，不用启动时的闭包。
- 禁用全部连接 / 工具服务器 / 检索 / 图片 / 音频配置写入口（返回 403）。
- `ctx_key` 为空时直接报错（400「请选择可用的 Key」），不回退到任何配置里的 Key。后台任务里报错只进日志，不影响主回复。
- 生图（`routers/images.py`）：引擎固定 `openai`，URL 固定网关，Key 读生图 Key 的 `ContextVar`（为空时用对话 Key）；生图开关的显示由前端按两把 Key 的平台决定；结果只接受 `b64_json` 或网关 origin 的 URL（`images.py:673-681`、`:1051-1057`；PoC：URL 下载失败时网关已计费）；编辑接口 `load_url_image`（`:923-942`）不下载外部 URL；锁定 `GET/POST images/config(/update)`（会返回 / 改写 Key）、删除 `images/verify`、`images/models` 返回固定列表；`n` 设上限。
- 联网搜索：前端「联网搜索」开关在嵌入模式下不走原版检索流程，而是让后端在发往网关的 chat 请求 `tools` 里加入 `{"type":"web_search"}`（网关已支持转换：`apicompat/chatcompletions_to_responses.go:530`、`responses_to_anthropic_request.go:726`，以及 `service/gateway_websearch_emulation.go`）。原版 `ENABLE_WEB_SEARCH` 保持 false，服务端不出网。实现（PoC 已验证，wt-c）：新增 `S2A_NATIVE_WEB_SEARCH`，`/api/config` 的 `features.enable_web_search` 取 `web.search.enable or S2A_NATIVE_WEB_SEARCH`（前端不用改）；`middleware.py` 在服务端工具解析完成后追加 `{'type':'web_search'}`（与原生函数调用的工具合并，工具循环后续轮次延续）。
- 引用显示（PoC）：Open WebUI 能解析流式 `delta.annotations[url_citation]` 与 Responses 的 `annotations`，存入 `sources` 并显示「N 个来源」。OpenAI 平台 Key 的连接用 `api_type=responses`，引用开箱可用；Anthropic 平台走 Chat Completions，网关目前丢弃 citations → 本任务补网关（§7.1）。搜索次数不按次计费，保持现状（PRD R10.4）。
- 后台任务（PoC 实测）：标题、标签、追问、上下文压缩、工具循环、子代理、文件上传后处理都继承 `ContextVar`。丢 Key 的只有启动时调度器驱动的自动化定时执行与定时器 → `ENABLE_AUTOMATIONS=false`、`ENABLE_SUBAGENTS=false`（`timer` 工具受它控制；自动化二期）。
- 费用相关：`ENABLE_TITLE_GENERATION=true`、`ENABLE_TAGS_GENERATION=false`、`ENABLE_FOLLOW_UP_GENERATION=false`（PRD R10.5）；`ENABLE_RETRIEVAL_QUERY_GENERATION=false`（全文模式下这次调用无用却计费）；`CHAT_RESPONSE_MAX_TOOL_CALL_ITERATIONS=20`（默认 256，每轮一次计费请求）。
- 管理员插件：运行在同一进程，与对话请求共享 `ContextVar`，因此能读到用户 Key —— 只装可信插件（PRD 约束）。

### 3.3 模型列表

- 全局 `app.state.MODELS` 只作为元数据并集（所有条目都指向唯一的网关连接），不会造成越权：调用时用的是当前用户的 Key，网关按 Key 的分组判定可用性。
- `/api/models`：按 `ctx_key` 调网关 `GET /v1/models`（按 Key 哈希缓存 60 秒，**不写回全局缓存的用户视图**），只返回该 Key 可用的模型，并把新出现的模型 ID 并入全局元数据。
- 启动时不预取（无用户 Key；原版只在 `base_models_cache` 开启时预取）。
- PoC 实测原版问题：缓存键为 `user.id`、TTL 1 秒，但结果写回全局 `OPENAI_MODELS` / `MODELS`，最后一个拉取者决定所有人的视图。`main.py:1167` 的「Model not found」检查必须改查全局元数据并集。
- 单价：iframe 调 `GET /_chat/__sub2api/model-prices?key_id=`（取模型广场在该分组下的价格），显示在模型选择器里，格式同模型广场，不出现「倍率」。

### 3.4 未采用：网关识别签名身份

Open WebUI 的 `FORWARD_USER_INFO_HEADER_JWT_SECRET` 可让网关凭签名身份 + 内部 Key 计费，但要改网关鉴权（安全最敏感处），且仍需记录用户选的 Key。3.1 方案只动反代层，与 Nova 一致。

## 4. 出网控制

- 网络层：Open WebUI 只接入 `internal: true` 的专用网络 `sub2api-*-chat`，没有外网路由；sub2api 同时接入该网络。
- 出站代理（管理员插件 / MCP 用）：同一 compose 里加一个只做 CONNECT 白名单的代理容器（如 tinyproxy / squid，`internal` 网络 + 外网网络各接一条），域名白名单在 `.env` / 配置文件里由管理员登记；Open WebUI 设 `HTTPS_PROXY` / `HTTP_PROXY` 指向它，`NO_PROXY` 包含 sub2api。代码大量 `aiohttp(trust_env=True)`，会自动读取代理变量。白名单为空 = 完全不出网。
- 配置层：`OFFLINE_MODE=true`、`ENABLE_VERSION_UPDATE_CHECK=false`、`ENABLE_OLLAMA_API=false`、`ENABLE_DIRECT_CONNECTIONS=false`、`ENABLE_WEB_SEARCH=false`（原版自建搜索）、`ENABLE_COMMUNITY_SHARING=false`、`ENABLE_IMAGE_GENERATION=true`（经网关）、`TERMINAL_SERVER_CONNECTIONS=[]`、`ENABLE_USER_WEBHOOKS=false`、不设 `LICENSE_KEY` / `CUSTOM_NAME` / OAuth。
- 代码层：删除 `/api/v1/retrieval/process/web`、`/process/url`、YouTube 路由；函数 / 工具「从 URL 导入」路由；消息内图片 URL 转 base64 只接受 `data:` 与本站文件；后端拒绝 `model_item.direct`。
- 浏览器侧：由 sub2api 的 CSP 拦截（见 §5）；前端移除 Gravatar、google favicons（`Citations.svelte:188`、`WebSearchResults.svelte:29,71,111`；`Citations.svelte:191-197` 的 `/favicon.png` 回退带 LICENSE 注释，保留）、OSM 瓦片。
- 消息内图片 URL 转 base64（`utils/files.py:55-95`）：只接受 `data:` 与本站文件 ID。

## 5. 安全

### 5.1 功能面

- 开放：Artifacts（见 §5.2）；`BYPASS_EMBEDDING_AND_RETRIEVAL=true`（文件附件全文放入上下文，不调 embedding、不用向量库）。附件按原版行为处理（PRD R9.5）。slim 对 PDF / docx 直接返回 503（`retrieval/loaders/main.py:720-732`）；fork 镜像加装原版完整镜像本来就有的 `pypdf`、`docx2txt`，嵌入模式下对这两种扩展名放行到原版的 `PDFLoader` / `DocxLoader`，其余非文本类型仍拒绝。附件默认全文注入（模型 `capabilities.file_context=true`）；该值为 false 时改为提供 files 类工具（`list_chat_files` / `view_file` / `grep_chat_files` / `query_chat_files`，BYPASS 下均可用），两者按模型二选一。
- 内置工具（PoC：`ENABLE_PLUGINS` 不影响内置工具）：fork 在 `get_builtin_tools`（`utils/tools.py:601`）加类别白名单 `S2A_BUILTIN_TOOL_CATEGORIES=time,user_input,files,chats,notes,tasks,calendar`，与原版判定取交集。knowledge 类（7 个工具）没有任何全局 / 权限开关、在 BYPASS 下报 503，只能靠白名单关掉。loader 补丁：`S2A_SLIM_DOC_LOADERS=true` 时 slim 分支放行 `PDFLoader(extract_images=False)` / `DocxLoader`（`retrieval/loaders/main.py:720-732`，约 10 行）。
- 屏蔽（PRD R9）：除环境变量关闭外，路由层返回 404 / 403（memories、knowledge、retrieval 的向量 / 网页接口、audio、pipelines、terminals、channels、automations、evaluations、chats 分享、auths 的 signup / 密码 / api_key、管理配置接口；tools / functions / 工具服务器的写接口仅 admin），前端移除对应入口（含斜杠命令与快捷键）。实现为 `embed/blocklist.py` 一张「路由前缀 → 全部屏蔽 / 仅 admin」表 + 前端 `embed/features.ts` 一张开关表，变基时只需核对这两张表。
- 关闭（网关无对应能力）：`ENABLE_MEMORIES=false`、知识库入口、`USER_PERMISSIONS_FEATURES_MEMORIES=false`。
- 插件与 MCP：`ENABLE_PLUGINS=true`，但创建 / 导入 / 配置只允许 admin（Functions 原版即仅 admin；Tools 靠 `USER_PERMISSIONS_WORKSPACE_TOOLS(_IMPORT)=false`；工具服务器配置原版仅 admin；`USER_PERMISSIONS_FEATURES_DIRECT_TOOL_SERVERS=false`、`ENABLE_DIRECT_INTEGRATIONS=false`）；`ENABLE_PIP_INSTALL_FRONTMATTER_REQUIREMENTS=false`（依赖只能预装进镜像）；删除「从 URL 导入」路由；Pipelines 不配置。
- 关闭：`ENABLE_CODE_EXECUTION=false`、`ENABLE_CODE_INTERPRETER=false`、`ENABLE_CHANNELS=false`、`ENABLE_AUTOMATIONS=false`、`ENABLE_ADMIN_CHAT_ACCESS=false`、`ENABLE_EVALUATION_ARENA_MODELS=false`、`ENABLE_MESSAGE_RATING=false`、`TERMINAL_SERVER_CONNECTIONS=[]`。
- 用户权限：`USER_PERMISSIONS_CHAT_SHARE=false`、`USER_PERMISSIONS_FEATURES_CODE_INTERPRETER=false`、`..._CHANNELS=false`、`..._DIRECT_TOOL_SERVERS=false`、`..._API_KEYS=false`、`..._MEMORIES=false`、`..._AUTOMATIONS=false`、`USER_PERMISSIONS_CHAT_STT/TTS/CALL=false`（网关无 `/v1/audio`）；保留 `..._IMAGE_GENERATION`、`..._WEB_SEARCH`（指原生搜索开关）、`..._NOTES`、`..._CALENDAR`、`CHAT_EXPORT`、`CHAT_CONTROLS`。
- iframe 沙箱写死不带 `allow-same-origin`（`InterfaceSettings.svelte:1729-1740` 的开关移除）。
- 上传（PoC：原版服务端几乎不限制）：sub2api 反代层限制请求体（如 20MB；原版写完文件才查 `RAG_FILE_MAX_SIZE`）；fork 补上传接口的 `chat.file_upload` 权限检查；`process=false` 也执行扩展名白名单（`image/*` 放行）；设 `RAG_ALLOWED_FILE_EXTENSIONS`、`RAG_FILE_MAX_SIZE`。

### 5.2 CSP（sub2api 在 `/_chat/*` 响应上设置，覆盖上游）

```
default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline';
img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; media-src 'self' blob: data:;
worker-src 'self' blob:; frame-src 'self' blob: data:; frame-ancestors 'self'; base-uri 'self'; form-action 'self'
```

- Artifacts / HTML 预览：上游用 `srcdoc` iframe，`srcdoc` 会继承父文档 CSP，严格 CSP 下模型生成的内联脚本跑不了。改为 iframe 指向 fork 自带的独立页面 `/_chat/embed/artifact-frame.html`（sub2api 对该精确路径下发单独的 CSP，见 `contract.md` F / B.6；`sandbox="allow-scripts"`，不带 `allow-same-origin` → 不透明源），该页面单独下发允许内联脚本、但 `connect-src 'none'`、`img-src data: blob:` 的 CSP，内容经 postMessage 传入后渲染。这样预览里的脚本既读不到本站存储，也发不出网络请求。
- srcdoc 共 4 处（`Artifacts.svelte`、`common/FullHeightIframe.svelte:221`、`chat/FileNav/FilePreview.svelte:402`、`Citations/CitationModal.svelte:229`）都改走 artifact-frame（协议见 fork `docs/sub2api-embed-frontend.md`）。
- `script-src 'self'`（PoC 已验证，不带 `'unsafe-inline'` / `'unsafe-eval'` / hash）：fork 构建后运行 `scripts/embed-postbuild.mjs`，把 `index.html` 的 3 段内联脚本写到 `_app/immutable/inline/<hash>.js` 并替换为 `<script src>`（SvelteKit 启动脚本的 hash 随每次源码改动变化，不适合由 sub2api 同步 hash）；删除 `app.html` 死代码 `resizeIframe`。
- `style-src` 必须保留 `'unsafe-inline'`（去掉后 mermaid、CodeMirror 失去样式）。
- 删除 socket `execute` 事件处理（`Chat.svelte:1761-1775`，让页面执行任意 JS，CSP 下本来也会失败）。
- 阶段 2 补测 CSP 下：导出聊天（html2canvas-pro）、docx / pdf 预览、HEIC 上传（需要 eval，会失效）、生图结果显示。
- `connect-src 'self'` + `img-src` 不含 `https:`：即使有注入，也无法把数据发到外站。

### 5.3 安全维护策略（2026-10-24 用户选定 S2）

上游 2026 年已发布 159 个安全通告（高危 71），按小版本成批修复。

- 方案 S1「永不跟上游」：每月过一遍通告，只移植影响已开放功能的修复。风险：通告在发版数日后才公开，补丁需在冻结的代码上手工改写，越往后越难。
- 方案 S2「补丁集 + 定期变基」（推荐）：fork 的改动保持为一组集中、可重放的提交（嵌入中间件、出口强制、前端桥 / 主题、路径前缀 codemod），每 1–2 个月或出现影响已开放功能的高危通告时，变基到上游最新版本，跑完整 e2e 后发布。路径前缀改动用 codemod 脚本生成，变基时重跑脚本而不是手工解冲突。

## 6. 前端（fork）

- 路径（PoC 已验证）：`svelte.config.js` `paths: { base: '/_chat', relative: false }`；`WEBUI_BASE_URL = base`；socket.io 改 `io(undefined, { path: withBase('/ws/socket.io') })`；`$lib/embed/navigation.ts` = `export * from '$app/navigation'` + 覆写 `goto`（codemod 只换 import）；`$lib/embed/paths.ts` 的 `withBase()` / `stripBase()`；`history.replaceState/pushState` 也要补前缀。codemod 规则 R1–R12 + 约 8 处手工，清单见 `research/poc-a.md`。
- 后端不改 URL：存库与 API 返回的根相对 URL（默认头像 `/user.png`、`/api/v1/files/<id>/content` 等）保持原样，前端渲染时由 `withBase` 补前缀（避免 `validate.py` 与变基冲突）；后端 302 的 `Location` 由 sub2api 改写（§7）；`token` Cookie path 改为 `/_chat`（`routers/auths.py:208`）。
- 存储：localStorage / sessionStorage 键加 `owui:` 前缀（与 sub2api 的 `theme`、`token` 冲突）。共 993 + 43 处引用，codemod 把标识符换成 `$lib/embed/storage` 导出的 Proxy，不逐处改键名；`app.html` 内联脚本里的 `localStorage.theme` 单独处理。
- 父页桥 `$lib/embed/bridge.ts`（`+layout.svelte` onMount）：校验 `origin === location.origin` 且 `source === window.parent`。

  ```ts
  // 父 → iframe
  { type: 'sub2api:chat-session', revision: number, theme: 'light' | 'dark', locale: 'zh-CN' | 'en-US' }
  { type: 'sub2api:chat-revoke' }
  // iframe → 父
  { type: 'chat:ready' }
  { type: 'chat:balance-changed' }               // 一轮回复结束
  { type: 'chat:navigate', path: string }        // 白名单：/purchase、/keys
  ```

- 主题：新增 `src/lib/embed/theme.css` 覆盖 `--color-gray-*`、`--color-blue-500/600`、body 背景，深色单独覆盖 800–950；改 `Settings/General.svelte:155-160` 的内联灰阶写入；主按钮黑白反转类在嵌入模式改用主色；`applyTheme` 由桥驱动，设置页隐藏主题选项。
- 隐藏：侧栏底部头像菜单换成「设置」按钮（管理员额外有「Workspace」「插件与工具」两项）；设置页 Account / About / Connections / Audio / Personalization（记忆）/ 主题 / 语言；Changelog、更新 toast、Banners、新手引导；频道分区；输入框的麦克风 / 通话 / 代码解释器开关；回复下的朗读 / 点赞点踩 / 分享。保留会话侧栏、笔记、日历、联网搜索与生图开关、对话参数面板。
- 品牌：默认（`S2A_HIDE_BRANDING=0`）保持 Open WebUI 品牌；嵌入模式 + `S2A_HIDE_BRANDING=1` 时与创作工作台一致——iframe 内不显示任何名称与图标（侧栏顶部 logo / 名称、启动画面、关于页、通知标题里的 "Open WebUI"、`env.py:957-959` 的后缀都去掉；模型默认头像换成中性图标），标题只在控制台菜单栏显示「对话」。版权声明与 LICENSE 链接保留在设置页底部小字。用户超过 50 人时设回 0。
- 语言：iframe src 带 `?lang=zh-CN`，`DEFAULT_LOCALE=zh-CN`；缺失译文经 `ui.i18n` 补齐常用项。
- 错误：网关错误（余额不足 / 限流 / Key 失效 / 审核）映射为中文提示，余额不足带「去充值」（`chat:navigate`）。

## 7. sub2api 侧

新文件 `backend/internal/server/chat_proxy.go`（参照 `studio_proxy.go`）：

- 配置 `chat.webui_url`（环境变量 `CHAT_WEBUI_URL`，如 `http://chat-webui:8080`）、`chat.trusted_cidr` 无（由 Open WebUI 校验）。
- `GET /api/v1/chat/status` → `{enabled}`；`POST/DELETE /api/v1/chat/session`。
- `/_chat/*`：全局中间件（在 SPA 中间件前）。静态资源直通；其余校验 `s2a_chat` Cookie（复用 JWT 校验服务）→ 注入头 → 反代（剥 `/_chat`，根相对 `Location` 补前缀（模型头像每次 302 到 `/static/favicon.png`），WebSocket 升级（不注入 Key），请求体上限）。
- `/_chat/__sub2api/keys|groups|model-prices`：Cookie 鉴权，复用现有 Key / 分组 / 模型广场服务，Key 列表不含明文。
- 响应头：删除上游 `X-Frame-Options` / CSP，设 `X-Frame-Options: SAMEORIGIN` 与 §5.2 CSP。
- `/_chat/__sub2api/keys` 返回每个 Key 的分组平台与「是否支持生图」，供前端过滤生图 Key 与决定生图开关。

### 7.1 网关：Claude 分组的搜索引用（PoC 提议 G1，`research/poc-c.md`）

- `pkg/apicompat/types.go` 的 `AnthropicContentBlock` 加 `Citations`；Anthropic→Responses 非流式（`anthropic_to_responses_response.go:53-94`）把 text 的 `web_search_result_location` citations 转为 `output_text.annotations[{type:url_citation,url,title,start_index,end_index}]`；流式（`:312-470`）处理 `citations_delta` → `response.output_text.annotation.added`。
- Responses→Chat Completions（G2）：`ResponsesContentPart` / `ChatMessage` / `ChatDelta` 加 `Annotations`，流式输出 `delta.annotations[{type:'url_citation', url_citation:{url,title,start_index,end_index}}]`、非流式 `message.annotations`（Open WebUI 现有解析可直接识别，PoC 已用桩验证）。
- 搜索前后的文本段之间补 `\n\n`。
- 只增加字段，不改计费；现有客户端多收到 `annotations` 字段（OpenAI 标准形状）。单测覆盖流式 / 非流式、无 citations 时输出不变。

### 7.2 前端

- 前端：`/chat` 页面 `frontend/src/themes/qx/chat/ChatView.vue` + `chatBridge.ts`（参照 `studio/`），导航入口「对话」，退出登录流程调用 `DELETE /api/v1/chat/session`。

## 8. 部署

- 镜像：fork 内 `docker/sub2api-embed.Dockerfile`，基于上游 Dockerfile 的 slim 构建（构建参数 `USE_SLIM=true`，运行时 `USE_SLIM_DOCKER=true`，非 root UID 1000），前端以 `/_chat` base 构建，`npm ci` 带 `ONNXRUNTIME_NODE_INSTALL_CUDA=skip CYPRESS_INSTALL_BINARY=0`；构建后运行 `embed-postbuild.mjs`。
- 出站代理：见 §4（`sub2api-*-chat-egress` 容器，白名单为空时等于不出网）。
- 数据库：一期 SQLite（数据卷）。不开记忆与 RAG，slim 的向量库客户端是惰性创建的（`retrieval/vector/factory.py:100`），不需要 pgvector。以后网关有 embedding 模型、要开记忆 / 文件问答时，再加 pgvector Postgres。单 uvicorn worker，不需要 Redis（signout 的 jti 撤销不依赖它——我们靠 sub2api Cookie 与 session_stamp 撤销）。
- 本机：`~/workspace/sub2api-local-chat`（compose 项目 `sub2api-local-chat`，容器 `sub2api-local-chat`，卷 `sub2api-local-chat_data`，网络 `sub2api-local-chat`（internal），不映射端口）。e2e 新阶段 `p9_chat.py`。
- 一号机：`/root/sub2api-custom-chat`，部署前另行征得同意并查看资源。
- fork 仓库与镜像：公开（`qlqqs/open-webui`，`ghcr.io/qlqqs/open-webui:custom-latest`）。源码与镜像默认 `S2A_HIDE_BRANDING=0`（Open WebUI 品牌），只在我们的部署 `.env` 设 `S2A_HIDE_BRANDING=1`。

## 9. 回滚

- sub2api 不配 `CHAT_WEBUI_URL` → 入口隐藏、`/_chat` 返回 503，其余功能不受影响。
- Open WebUI 数据在独立卷，删除 compose 项目不影响 sub2api。
