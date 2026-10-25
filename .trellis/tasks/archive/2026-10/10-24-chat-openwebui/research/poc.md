# 阶段 0 PoC 汇总（2026-10-24）

细节与证据见 `poc-a.md`（0.1、0.2）、`poc-b.md`（0.3、0.4）、`poc-c.md`（0.5）。

PoC 代码（均未推送）：
- `~/workspace/open-webui` 分支 `sub2api-poc-a`：codemod 草稿、postbuild、`src/lib/embed/{paths,navigation}.ts`（未提交）
- `~/workspace/owui-poc/wt-b` 分支 `sub2api-poc-b`：`embed/gateway.py`、出口强制、loader 补丁（本地提交 `68554c2`）
- `~/workspace/owui-poc/wt-c` 分支 `sub2api-poc-c`：原生联网开关（未提交）
- 脚本与日志：`~/workspace/owui-poc/{a,b,c}/`；共用 venv `~/workspace/owui-poc/venv`（uv 创建，slim 依赖 + pypdf + docx2txt）

## 结论

| 项 | 结果 | 要点 |
|---|---|---|
| 0.1 子路径 | 通过 | 越出 `/_chat` 的请求 0 次；**不用 `--root-path`**（uvicorn 0.51 会把它加回 `scope['path']`，导致 socket.io 404），改设 `WEBUI_URL`，后端零补丁 |
| 0.2 严格 CSP | 通过 | 构建后把内联脚本外置 → `script-src 'self'`，不需要 hash、`unsafe-eval`；`style-src` 必须保留 `'unsafe-inline'`；srcdoc 有 4 处要改走独立沙箱页 |
| 0.3 Key 贯穿 | 通过 | 两用户并发，58 次上游调用全部使用本人 Key；丢 Key 的只有调度器驱动的自动化、定时器（均关闭） |
| 0.4 内置工具与附件 | 通过（需补丁） | `ENABLE_PLUGINS` 不影响内置工具；knowledge 类没有开关 → 类别白名单补丁；PDF / DOCX 10 行 loader 补丁可用 |
| 0.5 联网搜索 / 生图 | 部分通过 | 开关、转发、生图计费都通；**网关在 Chat Completions 转换中丢弃引用**；**搜索次数不计费**；Claude 分组 Key 不能生图 |

无阻塞项。需要用户决定的点见末节。

## 已定的技术结论（写入 design / implement）

1. 启动：`uvicorn --no-proxy-headers`，不带 `--root-path`；`WEBUI_URL=https://<域名>/_chat`；`USE_SLIM_DOCKER=true`（否则导入 chromadb 失败）。
2. 前端 codemod：规则 R1–R12（155 个文件，幂等）+ 约 8 处手工；`goto` 靠替换 import 统一处理；localStorage 用 Proxy 统一加 `owui:` 前缀，不逐处改。
3. sub2api 反代必须改写 `Location`（模型头像每次 302 到 `/static/favicon.png`）；`token` Cookie path 改 `/_chat`；存库 URL 不带前缀，前端渲染时补。
4. CSP：`scripts/embed-postbuild.mjs` 外置内联脚本；最终 CSP 见 design §5.2（`media-src` 加 `data:`）；Artifacts、FullHeightIframe、FilePreview、CitationModal 4 处 srcdoc 改走 `/_chat/__sub2api/artifact-frame`；删除 socket `execute` 事件处理。
5. Key：中间件只对 `http` 设置 ContextVar（websocket 不设，避免旧 Key）；Key 为空直接报错；`ENABLE_SUBAGENTS=false`（定时器受它控制）。
6. 模型列表：必须按 Key 缓存、不写全局；`main.py:1167` 的「Model not found」检查改查全局元数据并集。
7. 内置工具：新增 `S2A_BUILTIN_TOOL_CATEGORIES` 白名单（time、user_input、files、chats、notes、tasks、calendar）；附件默认全文注入（`file_context=true`）。
8. 附件与上传：loader 补丁 `S2A_SLIM_DOC_LOADERS`；`ENABLE_RETRIEVAL_QUERY_GENERATION=false`（否则每次带附件多一次无用计费调用）；上传接口补 `chat.file_upload` 检查、`process=false` 也走扩展名白名单（图片放行）；反代层限制请求体（服务端写完才查大小）。扫描版 PDF 按原版行为（内容为空、无提示），PRD R9.5 已写明。
9. 工具循环上限 `CHAT_RESPONSE_MAX_TOOL_CALL_ITERATIONS=20`（默认 256，每轮一次计费请求）。
10. 联网：`S2A_NATIVE_WEB_SEARCH` 让开关显示、在 `tools` 末尾追加 `{"type":"web_search"}`；OpenAI 平台 Key 的连接用 `api_type=responses`（引用开箱可显示）；去掉 Google favicon / 搜索链接（保留 LICENSE 回退）。
11. 生图：结果只接受 b64 或网关 origin（URL 失败也已计费）；锁定 `images/config(/update)`、删除 `images/verify`、`images/models` 改固定列表、编辑接口不下载外部 URL、限制 `n`。

## 待用户决定

1. 联网搜索的网关缺口：引用映射（G1 / G2）与搜索按次计费（G3）。G3 缺口对现有 API 用户同样存在（任何带 web_search 的请求都只收 token 费）。
2. 用户所选 Key 属于 Claude 分组时无法生图（网关 404）。
3. 后台任务费用：每个新对话默认多出标题、标签各 1 次调用，每轮追问建议 1 次，都用用户 Key 计费。
4. 一号机实际上游账号是否支持原生 `web_search`（本机只能用 mock 验证）。

用户决定（2026-10-24）：1 只补引用（G1 / G2），搜索费平台承担；2 单独选一个生图 Key；3 只保留标题；4 部署阶段经同意后用真实请求验证。
