# PoC B：Key 传递（0.3）与内置工具 / 附件（0.4）（2026-10-24）

- 代码：worktree `~/workspace/owui-poc/wt-b`，分支 `sub2api-poc-b`（基于 v0.12.3，本地提交 `68554c2`，未推送）。下文行号均为 **v0.12.3 原始行号**，路径省略 `backend/open_webui/`。
- 脚本与原始记录：`~/workspace/owui-poc/b/`
  - `stub_server.py`：OpenAI 兼容桩（127.0.0.1:28183），记录每个请求的路径、`Authorization`、用途推断、`tools` 名单，完整请求体另存；可用 `TOOLCALLS:` 标记脚本化多轮工具调用。
  - `p03_driver.py`（两用户并发主流程）、`p03b_driver.py`（自动化 / 定时器 / 子代理 / socket / 后台任务探针）、`p03c_models.py`（模型列表）、`p04_driver.py`（附件 + 文件工具）、`p04b_flags.py`（开关矩阵、上传限制）、`make_files.py`（手写字节生成 PDF / DOCX / PNG）、`run_owui.sh` / `restart.sh`（启动参数）。
  - 每轮结果：`runA`（0.3 主流程）、`runB`（无请求上下文路径）、`runC`（模型列表）、`run04a/b/c`（0.4），各含 `stub.jsonl`、`stub-bodies.jsonl`、服务端日志。
- 运行环境：uvicorn 只监听 127.0.0.1:28182，`USE_SLIM_DOCKER=true`、`BYPASS_EMBEDDING_AND_RETRIEVAL=true`，其余按任务要求；两名测试用户为 `user` 角色（另建一个首用户 admin `u0`）。所有进程已停止。

## PoC 改动（`git show 68554c2`）

| 文件 | 改动 |
|---|---|
| `embed/gateway.py`（新） | `SUB2API_KEY` ContextVar；纯 ASGI 中间件读 `X-Sub2API-Key`（http + websocket），请求结束 `reset`；`current_key()` 为空时返回 `MISSING-CTX-KEY` 并打调用栈；`probe()` 探针 |
| `main.py:861` 后 | `app.add_middleware(Sub2APIKeyMiddleware)` |
| `routers/openai.py:255-260` | `get_openai_connection` 返回 `current_key()`，去掉 `auth_type/headers/forward_cookies/azure/api_version/prefix_id` |
| `routers/openai.py:99-105` | `send_get_request` 旧分支改用 `current_key()` |
| `utils/headers.py:234` | `get_headers_and_cookies` 末尾强制 `Authorization: Bearer <ctx>`、cookies 置空 |
| `retrieval/loaders/main.py:720-732` | `S2A_SLIM_DOC_LOADERS=true` 时 slim 分支放行 PDF / DOCX（见 0.4.2） |
| 探针（仅 PoC） | `routers/files.py:128`、`utils/middleware.py:3503`、`socket/main.py` heartbeat、`utils/automations.py:353`、`utils/timers.py:231` |

---

## 0.3 Key 传递（ContextVar）

### 结论

- **通过**：所有由用户 HTTP 请求触发的模型调用都拿到了该请求的 Key。两用户并发（runA 共 58 次上游调用，U1/U2 交错到达）：每次都是本人 Key，0 次 CONFIG Key、0 次串号、0 次 `MISSING-CTX-KEY`；上游请求不带 Cookie / `X-*` 头。
- **丢失上下文的只有两类**，都是启动时由 `scheduler_worker_loop`（`main.py:436`，lifespan 里的 `create_task`）驱动、没有请求的路径：**定时执行的自动化**、**定时器（timer 工具）**。runB 中 5 次 `MISSING-CTX-KEY` 全部来自这两处。与设计 §3.2「自动化 / 定时器关闭」一致。
- **新发现 1（需改设计）**：中间件同时处理 websocket 作用域时，socket.io 事件处理器继承的是**握手时**的 Key（探针 `socket.heartbeat key=KEY-USER-1`）。v0.12.3 的 socket 处理器不调模型，所以目前无害；但用户换 Key 后连接不重连，Key 就是旧的。→ 中间件只对 `http` 设置 ContextVar，websocket 一律为空（失败即报错，而不是用旧 Key）。
- **新发现 2（印证设计 §3.3，必须做）**：`/api/models` 按用户的 Key 拉列表（缓存键是 `user.id`，`routers/openai.py:713`；`MODELS_CACHE_TTL` 默认 **1 秒**，`env.py:1043`），但结果写回全局 `app.state.OPENAI_MODELS` / `MODELS`（`:779`）。实测：admin 拉完列表后，全局表只剩 admin Key 的模型，其他 Key 独有的模型立即变成「Model not found」（`main.py:1167`）。另外每次 `/api/models` 基本都会请求一次上游。
- 启动时**不会**预取模型列表：只在 `models.base_models_cache` 开启时预取（`main.py:438`，默认关）。首个 `/api/chat/completions` 在 `MODELS` 为空时用该用户的 Key 拉列表（`main.py:1153-1154`）。

### 各路径实测结果

| 路径 | 是否持有 `request` | 执行方式 | Key | 依据 |
|---|---|---|---|---|
| 主聊天（含 session_id 的异步路径） | 是 | `tasks.create_task` → `asyncio.create_task`（`main.py:1875`、`tasks.py:167`） | 本人 | runA `main` |
| 首轮标题 | 是（`title_ctx['request']`） | `asyncio.create_task(run_initial_title_generation())`（`main.py:1547-1562`） | 本人 | runA `title` |
| 回合后标题 / 标签 / 追问 | 是（`ctx`） | `background_tasks_handler`（`middleware.py:3503`，在聊天 task 内 await，`:4070/:6329`） | 本人 | runA `tags`/`follow_up`；探针 |
| 原生工具多轮循环 | 是 | 同一 task（`middleware.py:6037,6256`） | 本人 | runA `tool_loop` 3 轮 × 2 次 |
| 上下文压缩（回合内） | 是 | `compact_messages_for_request`（`middleware.py:2145`） | 本人 | runA `compaction` |
| 手动压缩 `POST /api/v1/chats/{id}/compact` | 是 | 请求内（`routers/chats.py:1394`） | 本人 | runA |
| `/api/v1/tasks/{title,tags,follow_up}/completions` 直调 | 是 | 请求内 | 本人 | runA |
| 无 session_id 的 API 流式调用 | 是 | StreamingResponse（仍在 ASGI 调用内） | 本人 | run04c 末尾手测 |
| 文件上传后处理 | 是 | Starlette `BackgroundTasks`（`routers/files.py:437`，在响应发送后、仍在 ASGI 调用内执行） | 本人 | 探针 `files.process_uploaded_file key=KEY-USER-1`（BYPASS 下不调上游） |
| 子代理前台 / 后台 `delegate_task` | 是（由源请求构造，`subagents.py:48`） | 前台 await；后台 `create_task`（`subagents.py:675`），完成后回写父聊天（`:276`） | 本人 | runB `[U1] foreground/background subtask`、`[ASYNC SUBAGENT COMPLETE]` |
| 自动化「立即运行」`POST /automations/{id}/run` | 否（`_build_request` 空头，`automations.py:135-160`） | 请求内 `asyncio.create_task`（`routers/automations.py:345`）→ 复制了上下文 | 触发者本人（仅限所有者，`:61-66`） | runB `[U0] automation prompt` 第一次 |
| 自动化定时执行 | 否 | `scheduler_worker_loop`（`main.py:436`）→ `CHAT_COMPLETION_HANDLER`（`automations.py:327,523`） | **MISSING** | runB 末次 `[U0] automation prompt` |
| 定时器 `timer` 工具 | 否（空头假请求，`timers.py:409-424`） | `scheduler_worker_loop` → `timers.py:427`；其 compaction / 回合后任务同样 MISSING | **MISSING** | runB `[U1]/[U2] timer fired` + 2 次 compaction；探针 `timers.execute_due_timer key=NONE`、`background_tasks_handler key=NONE`×3 |
| 工具审批后续跑 `POST /chats/{id}/messages/{mid}/resolve` | 是 | 请求内（`main.py:1930-1945`） | 续跑时那次请求的 Key | 代码 |
| `ask_user` / `__event_call__` 等待浏览器回复 | 是 | 原 task 内等待 socket 回包，后续调用仍在原 task | 本人 | 代码 |
| socket.io 处理器（`socket/main.py`） | 否 | 每个连接在 websocket ASGI 调用内派发 | **握手时的 Key**（见新发现 1）；当前无模型调用 | 探针 |
| 频道 @模型（`routers/channels.py:1373,1499`） | 是 | `BackgroundTasks` | 应为本人（未测，频道关闭） | 代码 |
| 没有 `X-Sub2API-Key` 的请求 | — | — | MISSING（PoC 未拦截） | runC |

### 丢失 ContextVar 的路径与修复建议

| 路径 | 修复 |
|---|---|
| 自动化定时执行（`utils/automations.py:327,523`） | 一期保持 `ENABLE_AUTOMATIONS=false`；同时 `current_key()` 为空时抛错（设计 §3.2 已有），防止误开。二期若要开：自动化记录 Key ID，由 sub2api 提供内部「按 Key ID 取明文」接口，在 `execute_automation` 开头 `SUB2API_KEY.set()`。 |
| 定时器（`utils/timers.py:427`） | `timer` 工具受 `subagents.enable` 控制（`utils/tools.py:765-772`）→ 保持 `ENABLE_SUBAGENTS=false` 即不出现。若以后开子代理，需在 fork 里单独去掉 `timer`（子代理本身能继承 Key）。 |
| socket.io 处理器 | 中间件只处理 `scope['type']=='http'`；sub2api 反代在 WebSocket 升级请求上也不必注入 `X-Sub2API-Key`。 |
| 无 Key 的请求 | `current_key()` 为空时抛 `HTTPException(400, '请选择可用的 Key')`。注意在后台任务里抛错只会进日志（标题 / 追问失败不影响主回复）。 |

---

## 0.4 内置工具与附件

### 结论

- **`ENABLE_PLUGINS` 不影响内置工具**：`ENABLE_PLUGINS=false` 与 `ENABLE_PLUGINS=true` + `ENABLE_PIP_INSTALL_FRONTMATTER_REQUIREMENTS=false` 下，桩收到的 `tools` 完全相同（stub-a 22 个，stub-b 26 个）。代码上 `ENABLE_PLUGINS` 只派生 `ENABLE_TOOLS/FUNCTIONS/TOOL_SERVERS`（`env.py:1194-1198`），`get_builtin_tools`（`utils/tools.py:601`）不读它。内置工具只在请求带 `session_id`、`function_calling != 'legacy'`、模型 `capabilities.builtin_tools != false` 时注入（`utils/middleware.py:2076-2080`）；纯 API 调用（无 session_id）不注入。
- **默认开出的类别比设计多**：time、ask_user、**knowledge（7 个，没有全局开关）**、chats、notes、tasks、**calendar（`ENABLE_CALENDAR` 默认 true）**；子代理 / 自动化 / 记忆 / 搜索 / 生图 / 代码 / 频道按各自开关关闭。设计 §5.1 只想保留 time、ask_user、files、chats、notes、tasks → 需要 fork 补丁（见下）。
- **files 类与「全文放进上下文」二选一**：文件工具只在模型 `capabilities.file_context == false` 时提供（`utils/tools.py:711-716`）；`file_context` 为 true（默认）时文件全文直接注入提示词，不提供文件工具。两种模式实测都正确（见下）。
- **PDF / DOCX**：slim 由 `USE_SLIM_DOCKER=true` 判定（`env.py:45`），slim 分支对 PDF / DOCX 直接 503。加 10 行补丁后 pypdf / docx2txt 正常提取，全文进入提示词，`view_file` / `grep_chat_files` / `list_chat_files` 返回正确内容。
- **上传的服务端检查很弱**：默认没有大小上限、没有扩展名白名单；`process=false` 能绕过白名单；**`chat.file_upload` 权限服务端不检查**。

### 0.4.1 内置工具开关矩阵（`utils/tools.py`，v0.12.3）

「模型 meta」= 模型的 `meta.builtinTools.<类别>`（默认 true，管理员在模型编辑页设置）。「用户权限」对 admin 一律放行。

| 类别（工具） | 行号 | 全局开关 | 用户权限 | 模型条件 / 其他条件 | 本 PoC 默认是否出现 | 需要向量库 / embedding |
|---|---|---|---|---|---|---|
| time（get_current_timestamp, calculate_timestamp） | 694 | 无 | 无 | 模型 meta | 是 | 否 |
| user_input（ask_user） | 697 | 无 | 无 | 模型 meta；运行时需要浏览器 socket 回包 | 是 | 否 |
| files（list_chat_files, query_chat_files, grep_chat_files, view_file） | 711-716 | 无 | `chat.file_upload` | `capabilities.file_upload` 且 **`capabilities.file_context == false`**，且本轮有附件 | 仅 stub-b（file_context=false） | `query_chat_files` 在 BYPASS 下经 `get_sources_from_items` 返回全文，实测可用 |
| knowledge（list/search/query_knowledge_bases, grep/search/query_knowledge_files, view_knowledge_file；`ENABLE_KB_EXEC` 时换一组） | 723-759 | **无** | **无** | 仅模型 meta | 是（7 个） | `query_knowledge_bases/files` → `503: Configure an external embedding engine`；`search_*`/`list_*` 可用但为空 |
| chats（search_chats, view_chat） | 761 | 无 | 无 | 模型 meta | 是 | 否 |
| subagents（delegate_task, timer） | 765-772 | `ENABLE_SUBAGENTS` | 无 | 非内部 / 非 direct / 非临时聊天 | 否 | 否；`timer` 丢 Key |
| memory（8 个） | 775-792 | `ENABLE_MEMORIES` | `features.memories` | `features.memory` + `capabilities.memory` | 否 | 是 |
| web_search（search_web, fetch_url） | 796-803 | `ENABLE_WEB_SEARCH` | `features.web_search` | `features.web_search` + capability | 否 | 出网 |
| image_generation（generate_image / edit_image） | 807-821 | `ENABLE_IMAGE_GENERATION` / `ENABLE_IMAGE_EDIT` | `features.image_generation` | `features.image_generation` + capability | 否（未开） | 否 |
| code_interpreter（execute_code） | 826-832 | `ENABLE_CODE_INTERPRETER` | `features.code_interpreter` | feature + capability | 否 | 否 |
| notes（search/view/write_note, replace_note_content） | 835-838 | `ENABLE_NOTES` | `features.notes` | 模型 meta（笔记内聊天强制开） | 是；`USER_PERMISSIONS_FEATURES_NOTES=false` 后消失（实测） | 否 |
| channels（4 个） | 841-850 | `ENABLE_CHANNELS` | `features.channels` | 模型 meta | 否 | 否 |
| skills（view_skill 等；create_skill） | 854-863 | 无 | `workspace.skills`（create_skill） | 选了技能才有 view/read/grep | 仅 admin 有 create_skill | 否 |
| tasks（create_tasks, update_task） | 866-867 | 无 | 无 | 仅已保存聊天 | 是 | 否 |
| automations（5 个） | 870-876 | `ENABLE_AUTOMATIONS` | `features.automations` | 模型 meta | 否 | 否；定时执行丢 Key |
| calendar（4 个） | 880-883 | `ENABLE_CALENDAR`（**默认 true**） | `features.calendar` | 模型 meta | 是；`ENABLE_CALENDAR=false` 后消失（实测） | 否 |
| notifications（notify） | 885-890 | `ENABLE_USER_WEBHOOKS` | `features.webhooks` | 模型 meta | 否 | 出网 |
| play_animation | 658-690 | 无 | `chat.call` | 语音模式 + 模型配置了手势 | 否 | 否 |

实测（run04c）：模型 meta 设 `builtinTools: {knowledge: false, user_input: false}` + `USER_PERMISSIONS_FEATURES_NOTES=false` + `ENABLE_CALENDAR=false` 后，用户只剩 `get_current_timestamp, calculate_timestamp, search_chats, view_chat, create_tasks, update_task`。

**建议的 fork 补丁**：在 `get_builtin_tools` 开头加环境变量白名单（如 `S2A_BUILTIN_TOOL_CATEGORIES=time,user_input,files,chats,notes,tasks`），用 `is_builtin_tool_enabled` 的结果与白名单取交集。不要依赖管理员逐个模型设置 meta（新增的模型 / 预设助手默认全开，knowledge 没有其他开关）。日历是否保留需在设计里定：§5.1 工具列表没有日历，但 §5.1 用户权限保留了 `..._CALENDAR`。

### 0.4.2 附件

**slim 判定与最小补丁**

- slim 判定：环境变量 `USE_SLIM_DOCKER=true` → `USE_SLIM`（`env.py:45`）。它同时让 `VECTOR_DB` 默认变成 `pgvector`（`config.py:509`）；slim 依赖下必须设为 true，否则 `config.py:515` 导入 chromadb 失败。
- 未改时（run04a）：`text.pdf`、`scanned.pdf`、`doc.docx`、`.bin`、`.zip` 都是上传 200，随后状态 `failed`，错误为 `503: This file type requires an external document extractor in slim...`（`retrieval/loaders/main.py:729-732`）；`.md`、`.csv` 正常。
- 最小改动（已在 wt-b，`retrieval/loaders/main.py`，slim 分支内 `raise` 之前）：

  ```python
  if S2A_SLIM_DOC_LOADERS and file_ext == 'pdf':
      return PDFLoader(file_path, extract_images=False, mode=self.config['rag.pdf_loader_mode'])
  if S2A_SLIM_DOC_LOADERS and (
      file_ext == 'docx'
      or file_content_type == 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
  ):
      return DocxLoader(file_path)
  ```

  外加 `S2A_SLIM_DOC_LOADERS = os.getenv('S2A_SLIM_DOC_LOADERS', 'false').lower() == 'true'`。`extract_images` 必须写死 False：图片 OCR 需要 rapidocr，slim 没有。`PDFLoader` / `DocxLoader` 本身只在调用时导入 pypdf / docx2txt（`retrieval/loaders/pdf.py:22`、`local.py:40`），不需要改其他地方。
- 改后（run04b）：PDF 提取 `PDF-MARKER-7731 hello from a text layer PDF`，DOCX 提取 `DOCX-MARKER-4420 paragraph one\n\n第二段 中文内容`。

**全文模式（stub-a，`file_context` 默认 true）**

桩收到的最后一条 user 消息是 RAG 模板 + 全文 + 附件清单：

```
### Task:
Respond to the user query using the provided context, incorporating inline citations ...
<context>
<source id="1" name="text.pdf" resource-type="file" resource-id="bd9f…">PDF-MARKER-7731 hello from a text layer PDF</source>
<source id="2" name="doc.docx" ...>DOCX-MARKER-4420 paragraph one\n\n第二段 中文内容</source>
<source id="3" name="notes.md" ...># Title\n\nMD-MARKER-9001 markdown body\n</source>
<source id="4" name="table.csv" ...>name: CSV-MARKER-5150\nvalue: 42 name: other\nvalue: 7</source>
</context>
<attached_files> <file type="file" id=… content_type=… name=…/> … </attached_files>
[U1] summarize attachments ECHO
```

- 额外计费调用：有附件且不是全部 `context=='full'` 时，会先调一次「检索查询生成」任务（`utils/middleware.py:1713-1725`，桩里的 `task:Analyze the chat history…`），BYPASS 下这次调用毫无用处，却会扣用户的钱。设 `ENABLE_RETRIEVAL_QUERY_GENERATION=false` 后这次调用消失（run04b 实测）。→ 加入设计的环境变量清单。
- CSV 经 `CSVLoaderWithSummary` 转成「列: 值」文本，不是原始 CSV。

**原生文件工具（stub-b，`file_context=false`，脚本化 4 轮）**

- `list_chat_files` → 4 个文件的 id / 文件名 / content_type / size。
- `view_file`（4 个并行）→ 各自全文（PDF / DOCX / MD / CSV 都正确）。
- `grep_chat_files {"pattern":"MARKER"}` → `text.pdf:1`、`doc.docx:1`、`notes.md:3`、`table.csv:1` 四行命中。
- `query_chat_files` → BYPASS 下返回全文块（可用）；`query_knowledge_bases` → `{"error":"503: Configure an external embedding engine..."}`；`search_knowledge_files` → `[]`。
- 这种模式下附件全文**不进**提示词（只有 `<attached_files>` 清单），完全靠模型自己调工具。

**扫描件 / 图片 / 不允许的类型**

| 文件 | 结果 |
|---|---|
| 无文字层 PDF（只有一张图） | 状态 `completed`，`content` 为空字符串，**没有任何提示**；聊天里等于没附。建议补丁：PDF 提取结果为空时置 `failed` 并提示「扫描件无法识别文字」。 |
| PNG，`process=true` | `failed: File type image/png is not supported for processing`（`routers/files.py:187`） |
| PNG，`process=false`（前端对图片就是这样传的，`MessageInput.svelte:1255`） | 存储成功；聊天时后端把文件 ID 转成 `data:image/png;base64,…` 的 `image_url` 发给模型（`utils/middleware.py:2193` → `utils/files.py:55`）。实测桩收到 `{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBOR…"}}`。注意 `get_image_base64_from_url` 对 `http(s)` URL 会在服务端拉取（`utils/files.py:57-95`，有 SSRF 防护），与设计 §4「只接受 data: 与本站文件」一致，需要补丁限制。 |
| `.bin` / `.zip`（未设白名单） | 上传 200，状态 `failed` 503（同 slim 拒绝） |

### 0.4.3 上传限制与权限（服务端实际执行的）

| 检查 | 位置 | 默认 | 实测 |
|---|---|---|---|
| 扩展名白名单 | `routers/files.py:354-361`，`RAG_ALLOWED_FILE_EXTENSIONS`（`config.py:1005`） | 空 = 全部允许 | 设 `pdf,docx,md,csv` 后 `.bin` → 400；**但只在 `process=true` 时检查**，`process=false` 上传 `.bin` / `.html` 都成功存储（`/content` 以 `attachment` 下载返回，不会内联渲染） |
| 大小上限 | `routers/files.py:394-400`，`RAG_FILE_MAX_SIZE`（MB，`config.py:990`） | 未设 = 无上限 | 设 1 后 2MB `.md` → 413；但检查发生在**整个文件读入内存并写入存储之后**（`Storage.upload_file` 在 `:375`），超限再删 → 反代层必须限制请求体 |
| 数量上限 `RAG_FILE_MAX_COUNT` | 仅前端 | — | 服务端不检查 |
| `chat.file_upload` 权限 | 上传接口不检查（`routers/files.py:280-300` 只有 `get_verified_user`）；只用于前端（`MessageInput.svelte:979`）和文件工具注入（`utils/tools.py:715`） | — | `USER_PERMISSIONS_CHAT_FILE_UPLOAD=false` 时，用户接口确认为 `false`，上传仍 200 `completed`；带附件聊天时文件工具不出现，但全文注入路径（`chat_completion_files_handler`）同样不检查 |

---

## 阻塞项

无硬阻塞。下列为必须在 fork 里做、设计需要补充的点。

## 建议的设计改动

1. **§2.3 / §3.2 中间件**：ContextVar 只对 http 作用域设置，websocket 不设（避免 socket 处理器拿到握手时的旧 Key）；`current_key()` 为空时抛 400（PoC 已验证 MISSING 路径只来自调度器）。
2. **§3.2 后台任务**：保留结论（标题 / 标签 / 追问 / 压缩 / 工具循环 / 子代理 / 文件后台处理都继承 Key，已实测）；补充：定时器受 `ENABLE_SUBAGENTS` 控制，一期保持 `ENABLE_SUBAGENTS=false`；`ENABLE_AUTOMATIONS=false` 时 `/automations/{id}/run` 也会 403（`routers/automations.py:45-51`），不会留下「立即运行」口子。
3. **§3.3 模型列表**：按 Key 哈希缓存、不写全局，已被实测证实为必须（全局表是「最后一次拉取者」的视图，TTL 仅 1 秒）。还需处理 `chat_completion` 的 `model_id not in app.state.MODELS` 检查（`main.py:1167`）：改成查全局元数据并集，否则别的用户刚拉完列表时会误报「Model not found」。
4. **§5.1 内置工具**：
   - 确认 `ENABLE_PLUGINS` 不影响内置工具；
   - 新增 fork 补丁「内置工具类别白名单」，knowledge 类没有全局开关，必须靠它关掉；
   - 明确日历去留（`ENABLE_CALENDAR` 默认开）；
   - files 类与全文注入是按模型 `file_context` 二选一的：建议默认保持 `file_context=true`（全文注入，模型无需会调工具），文件工具作为可选。设计里「保留 files 工具」需改成这个表述；
   - `query_chat_files` 在 BYPASS 下可用，不必单独剔除。
5. **§5.1 附件**：
   - 环境变量加 `USE_SLIM_DOCKER=true`、`ENABLE_RETRIEVAL_QUERY_GENERATION=false`、`RAG_ALLOWED_FILE_EXTENSIONS=pdf,docx,txt,md,csv,…`、`RAG_FILE_MAX_SIZE`；
   - loader 补丁按上文；
   - 空文本 PDF 报错补丁；
   - 上传接口补 `chat.file_upload` 权限检查，且对 `process=false` 也执行扩展名白名单（图片按 `image/*` 放行）；
   - 反代层限制请求体（因为服务端是写完才检查大小）。
6. **计费提示**：每个新对话默认额外产生标题、标签各 1 次调用，每轮追问 1 次（都用用户 Key，走 `TASK_MODEL` 或当前模型）。建议设置 `TASK_MODEL` / `TASK_MODEL_EXTERNAL` 为便宜模型，或在嵌入模式下默认关闭追问；在 PRD 里说明。
