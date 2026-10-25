# 调研：Open WebUI v0.12.3 安全面、通告、部署（2026-10-24）

源码 `~/workspace/open-webui`（v0.12.3，HEAD 91ea4e342）。通告由 `gh api` 只读拉取。

## 结论

- 环境变量基本能关掉服务端代码执行；但 **JWT 存在 `localStorage`，任何 XSS = 会话被盗**。
- **安全通告数量极大**：GitHub 共 166 个（2024: 1，2025: 6，**2026: 159**；critical 1 / high 71 / medium 87 / low 7），成批修在 0.9.0（42）、0.10.0（18）、0.11.0（17）、0.11.1（18）、0.11.4（19）。「只手工回补安全修复」工作量大，需要在设计里认真对待（见 design「安全维护策略」）。
- 影响 0.12.x 的已公开通告只有 GHSA-3vvh-g22g-jq6r（频道 webhook 冒充，0.12.2 修复；关闭频道即不受影响）。上游习惯发版数日后才公开通告；0.12.3 CHANGELOG:26 "various improvements … security" 可能含未公开修复（未确认）。
- 发版：0.11.0 07-27、0.11.1 08-25、0.11.2/3 08-31、0.11.4 09-21、0.12.0 10-10、0.12.1/2 10-14、0.12.3 10-16 —— 约每月一个小版本，补丁数日内连发。

## 危险功能与关闭

- 配置生效：环境变量仅首次写库，之后以库为准（`models/config.py:131-160,248-275`）→ `ENABLE_PERSISTENT_CONFIG=false`（`config.py:3334`）。
- Functions / Tools：`exec(content, module.__dict__)`（`utils/plugin.py:242,291`）= RCE。Functions 仅 admin（`routers/functions.py:110,211,389`）；Tools 需 `workspace.tools(_import)` 权限（默认 False，`routers/tools.py:385-397`）；Pipelines 仅 admin，转发外部服务。frontmatter 依赖自动 `pip install`（`utils/plugin.py:434-454`，`ENABLE_PIP_INSTALL_FRONTMATTER_REQUIREMENTS` 默认 True，`env.py:1200`）。
  - 关闭：`ENABLE_PLUGINS=false`（总开关，覆盖 TOOLS / FUNCTIONS / TOOL_SERVERS，`env.py:1194-1198`）+ `ENABLE_PIP_INSTALL_FRONTMATTER_REQUIREMENTS=false`；`SAFE_MODE=true` 启动停用 Functions（`main.py:406`）。
- 代码执行：`ENABLE_CODE_EXECUTION` / `ENABLE_CODE_INTERPRETER` 默认 True，引擎 pyodide（浏览器内，`sandbox="allow-scripts"` 不透明源，`pyodideSandboxHost.ts:210-215`），micropip 从 `cdn.jsdelivr.net` 拉包；jupyter 引擎为服务端执行。→ 两者 false。
- Terminals：Open WebUI 侧只是代理（`routers/terminals.py`），配置接口仅 admin；生产使用需要 Enterprise License（open-webui/terminals README）。不配置即不启用，`ENABLE_PLUGINS=false` 一并关闭。
- MCP / OpenAPI 工具服务：`ENABLE_TOOL_SERVERS`；用户直连工具 `features.direct_tool_servers` False、`ENABLE_DIRECT_INTEGRATIONS` False；`ENABLE_DIRECT_CONNECTIONS` False（历史 CVE-2025-64496）。
- 联网 / 网页抓取：`ENABLE_WEB_SEARCH` False；但 `/api/v1/retrieval/process/web` 任何登录用户可调、服务端未查 `chat.web_upload`（`routers/retrieval.py:2086-2093`）；有 SSRF 防护（`retrieval/web/utils.py:120-150`），历史多次绕过 → **代码里删除该路由**。
- 文件上传：存 `DATA_DIR/uploads`；默认不限大小 / 扩展名（`config.py:988-1006`）；扩展名仅 `process=True` 检查（`routers/files.py:354-362`），大小写盘后才查（`:394`）；`chat.file_upload` 只在前端检查（`MessageInput.svelte:979`，服务端未见，未确认）→ sub2api 转发层限制请求体；fork 补服务端权限检查。

| 功能 | 默认 | 位置 |
|---|---|---|
| `ENABLE_IMAGE_GENERATION` | 关 | `config.py:1360` |
| `ENABLE_CHANNELS` | 关（权限 `features.channels` True） | `config.py:2069` |
| `ENABLE_NOTES` | 开 | `config.py:2095` |
| Memories | 开 | `config.py:432` |
| Calendar | 开 | `config.py:2077` |
| Automations | 开（权限 False） | `config.py:2079` |
| `ENABLE_EVALUATION_ARENA_MODELS` | 关 | `config.py:2099` |
| `ENABLE_MESSAGE_RATING` | 开 | `config.py:2143` |
| `ENABLE_COMMUNITY_SHARING` | **开，发往 openwebui.com** | `config.py:2141` |
| `chat.share` | 开 | — |
| `public_chats` / `open_chats`（免登录） | 关 | `routers/chats.py:1280-1290` |
| `ENABLE_ADMIN_CHAT_ACCESS` | 开 | `env.py:371` |
| `CORS_ALLOW_ORIGIN` | `*` | `config.py:2179` |

`DEFAULT_USER_PERMISSIONS`（`config.py:1761-2053`）：workspace.* 与 sharing.* 全 False；chat.* 大多 True（含 file_upload、web_upload、share、export、import、stt、tts、call、multiple_models、temporary）；features 中 notes / folders / channels / web_search / image_generation / code_interpreter / memories / calendar 为 True，api_keys / direct_tool_servers / automations / webhooks 为 False。

## 渲染与 XSS

- marked + DOMPurify（`HTMLToken.svelte:18`、`MarkdownInlineTokens.svelte:97,181`）、KaTeX、mermaid。
- `<iframe src>` 特例：用未净化原文取 src，以空 `sandbox` 渲染（`HTMLToken.svelte:72-90`）→ 可嵌外站页面（钓鱼）；CSP `frame-src` 可挡。
- Artifacts / 引用 / HTML 预览 iframe：默认 `allow-scripts allow-downloads allow-forms`，用户设置 `iframeSandboxAllowSameOrigin` 可加 `allow-same-origin`（`InterfaceSettings.svelte:1729-1740`）→ **fork 写死 false**；`IFRAME_CSP` 默认空（`config.py:1759`）。
- `/api/v1/files/{id}/content/html` 只返回管理员上传文件，同源无 CSP（`files.py:870-886`）。
- mermaid `securityLevel:'loose'`，`sanitizeSvg` 放开 `foreignObject`/`use`/`style`/`href`（`utils/index.ts:2160,2250-2290`），未深入验证。
- 默认无 CSP（`utils/security_headers.py:35-58`）。sub2api 转发层为 `/_chat/*` 加严格 CSP（参照 `studioContentSecurityPolicy`）是重要的纵深防御。

## 近期高危（均已在 0.12.3 前修复，可作回归对照）

GHSA-vpq8-f445-hcq7（community stats 消息窃取 token）、GHSA-q46m-r89w-j74p（终端越权）、GHSA-f9xp-mfmq-x6cg / CVE-2026-109290（DOCX 预览 XSS）、GHSA-qpqv-xwg8-cqpj、GHSA-wf9m-46cp-c6h6（`javascript:` 链接）、GHSA-jmc6 / GHSA-3xpf（终端预览 allow-same-origin）、GHSA-8x5v / GHSA-4v28 / GHSA-w2rx（SSRF）、GHSA-74h3-cxq7-vc5q（Socket.IO 跨用户执行）、GHSA-2r4p-jpmg-48f4 / CVE-2026-44551（LDAP 空密码，critical）、ZDI-26-031/032（插件执行，厂商称预期行为）。

## 部署

- Dockerfile：构建 `node:22-alpine3.20`，运行 `python:3.11-slim-bookworm`；参数 `USE_CUDA` / `USE_OLLAMA` / `USE_SLIM` / `USE_PERMISSION_HARDENING` / `USE_EMBEDDING_MODEL` / `UID`/`GID`（默认 0 = root）；非 slim 装 CPU torch 并预置 embedding / whisper / tiktoken；端口 8080，HEALTHCHECK `/health`。
- 体积：**slim 约 175–197 MB 压缩**；main 约 1.54–1.62 GB 压缩。
- slim 限制（CHANGELOG:750-761）：DB 仅 SQLite / PostgreSQL；向量仅 pgvector；embedding 需外部服务；PDF / Word 需外部解析器；无 git。
- `DATABASE_URL` 支持 postgres（`env.py:271-301`）；pgvector 默认复用 `DATABASE_URL`（`config.py:655`）。
- 数据目录 `/app/backend/data`；单 worker 无需 Redis（`WEBSOCKET_MANAGER=redis` 才需，推断）——但无 Redis 时 signout 的 jti 撤销不生效（见 `auth.md`，用 session_stamp 撤销替代）。
- `WEBUI_SECRET_KEY` 必须显式设置（否则生成在非数据卷路径，重建后会话全失效，`start.sh:31-49`）。
- 内存：未确认；slim 估计数百 MB。

## LICENSE

见 `frontend.md` / 仓库 `LICENSE:20-34`。Terminals 限制来自其独立仓库的 Enterprise License；`WEBUI_NAME` 非默认时强制追加 ` (Open WebUI)`（`env.py:957-959`）。

## 最小攻击面建议

- 环境变量：`ENABLE_PERSISTENT_CONFIG=false`、`ENABLE_PLUGINS=false`、`ENABLE_PIP_INSTALL_FRONTMATTER_REQUIREMENTS=false`、`ENABLE_CODE_EXECUTION=false`、`ENABLE_CODE_INTERPRETER=false`、`ENABLE_WEB_SEARCH=false`、`ENABLE_COMMUNITY_SHARING=false`、`ENABLE_CHANNELS=false`、`ENABLE_SIGNUP=false`、`ENABLE_ADMIN_CHAT_ACCESS=false`（待定）、`CORS_ALLOW_ORIGIN` 收紧、CSP / `IFRAME_CSP`、显式 `WEBUI_SECRET_KEY`。
- 代码：删 `/process/web`；iframe 写死无 `allow-same-origin`；服务端补 `chat.file_upload` 检查与大小限制。
- 镜像：slim + Postgres（pgvector 如需 RAG）+ 单 worker + 非 root UID。

来源：<https://github.com/open-webui/open-webui/security/advisories> · <https://github.com/open-webui/open-webui/security/advisories/GHSA-3vvh-g22g-jq6r> · <https://github.com/open-webui/terminals> · <https://docs.openwebui.com/getting-started/quick-start/> · <https://www.zerodayinitiative.com/advisories/ZDI-26-032/> · <https://safeguard.sh/resources/blog/open-webui-september-2026-advisories-0-11-1-oauth-sqlite>
