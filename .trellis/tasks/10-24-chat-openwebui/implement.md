# 实施计划：对话工作台（Open WebUI fork 原生嵌入）

前置阅读：`prd.md` → `design.md` → `research/*.md`。两个仓库：
- fork：`~/workspace/open-webui`（起点 tag v0.12.3；分支 `sub2api-custom`；远端 `qlqqs/open-webui`，需用户同意后创建）
- sub2api：本仓库，分支 `sub2api-custom`

每个阶段结束都有验证门；未通过不进入下一阶段。提交、推送、部署一号机均需另行征得用户同意。

## 阶段 0：PoC（只验证风险最高的几件事，结论写入 `research/poc.md`）

- [x] 0.1 子路径：`paths.base='/_chat'` + codemod 草稿，`npm run build` 后用 uvicorn `--root-path /_chat` + 本地一个剥前缀的反代跑通：首页、新建会话、socket.io 连接、头像图片。
- [x] 0.2 严格 CSP：统计构建产物中的内联脚本，确认能否只用 hash（不带 `'unsafe-inline'`）跑通；记录 Svelte / KaTeX / mermaid 的 `style-src` 需求。
- [x] 0.3 Key 贯穿：ContextVar 在标题 / 标签 / 追问 / 记忆整理（`asyncio.create_task`）与文件上传后台任务里是否可读；记录拿不到的路径。
- [x] 0.4 内置工具与附件：`ENABLE_PLUGINS=false` 下原生函数调用的内置工具（files / chats / notes / tasks / time / ask_user）是否可用；全文模式与 `view_file` / `grep_chat_files` 对 PDF / docx / 文本附件是否按原版工作（slim + `pypdf` / `docx2txt`）。
- [x] 0.5 联网搜索：在 chat 请求里加 `{"type":"web_search"}` 后，经本机网关（upstream-mock 或网关模拟搜索）返回的引用在 Open WebUI 中如何渲染；生图经网关 `/v1/images/generations` 跑通。
- 门：五项结论写清楚，阻断项与用户确认替代方案后再继续。
- 结果（2026-10-24）：见 `research/poc.md`；无阻塞项；待决定点已由用户确认（只补引用、单独生图 Key、只保留标题）。PoC 代码在 `sub2api-poc-a/b/c` 三个分支，正式实现从 v0.12.3 新建 `sub2api-custom` 分支，按需挑用。

## 阶段 1：fork 后端（`backend/open_webui/embed/` 新目录 + 少量挂接）

- [x] 1.1 `embed/settings.py`：读取 `S2A_EMBED_MODE`、`S2A_GATEWAY_URL`（只允许无路径 / 凭据 / 查询串的 http(s) origin，非法拒绝启动）、`S2A_TRUSTED_PROXY_CIDRS`、`S2A_HIDE_BRANDING`、`S2A_ALLOW_ADMIN`。
- [x] 1.2 `embed/sub2api_auth.py`：纯 ASGI 中间件（http + websocket）——不可信对端剥头 + 非静态 403；可信对端写 `scope['client']`、Key 入 ContextVar；ContextVar 只对 http 设置；`start.sh` 改 `--no-proxy-headers`（不用 `--root-path`，见 design §1）。
- [x] 1.3 身份补丁：`utils/auth.py` 邮箱头缺失拒绝 + 每请求按头同步角色（admin / user）；`routers/auths.py` 去掉首用户 admin；socket connect / user-join / 复检校验邮箱头；屏蔽管理员加用户、改邮箱 / 密码 / 角色、signout 重定向。
- [x] 1.4 出口强制 `embed/gateway.py`：`get_openai_connection`、`get_headers_and_cookies`、`openai.py:99-105`、embedding 函数改读 ContextVar；空 Key 报错；拒绝 `model_item.direct`；禁用配置写入口。
- [x] 1.4a 生图强制网关 + ContextVar Key、结果只收 b64 / 网关 origin、锁定 images 配置口、限制 `n`；联网搜索开关 `S2A_NATIVE_WEB_SEARCH`（移植 wt-c）；OpenAI 平台 Key 连接用 `api_type=responses`。
- [x] 1.4c 内置工具类别白名单 `S2A_BUILTIN_TOOL_CATEGORIES`；loader 补丁 `S2A_SLIM_DOC_LOADERS`（移植 wt-b）；上传补 `chat.file_upload` 检查与 `process=false` 扩展名白名单。
- [x] 1.4b `embed/blocklist.py`：「路由前缀 → 全部屏蔽 / 仅 admin」表（PRD R9；管理后台只放行函数 / 工具 / 工具服务器）。
- [x] 1.5 模型列表：`/api/models` 按 Key 调网关 `/v1/models`（按 Key 哈希缓存 60 秒），并入全局元数据；启动不预取；`main.py:1167` 改查元数据并集。
- [x] 1.6 删除出网路由：`retrieval/process/web`、`process/url`、YouTube；函数 / 工具 URL 导入；图片 URL 转 base64 只接受 `data:` 与本站文件。
- [x] 1.7 品牌：`env.py:957-959` 在 `S2A_HIDE_BRANDING=1` 时不追加后缀；`token` cookie path `/_chat`（后端 URL 不加前缀，由前端渲染时补）。
- [x] 1.8 测试：`backend/open_webui/test/embed/` —— 伪造头（不可信对端 / 缺头 / 错格式）、换账号 401、出口只到网关且带用户 Key、空 Key 拒绝、配置写入口 403、删除的路由 404、ContextVar 后台任务继承、记忆 / 知识库 / 向量检索接口关闭、blocklist 每一项（普通用户与管理员分别验证）。桩服务只绑定 127.0.0.1。
- 门：`pytest backend/open_webui/test/embed` 全过；上游已有测试中与改动相关的部分不回退。

## 阶段 2：fork 前端

- [x] 2.1 codemod 脚本 `scripts/embed-codemod.mjs`（可重复执行，以 `sub2api-poc-a` 草稿 R1–R12 为基础）+ `scripts/embed-postbuild.mjs`（内联脚本外置）：`goto` / `href` / `location.href` / `pathname` 判断、硬编码 `/static` `/api` `/audio` 等路径、localStorage / sessionStorage 键加 `owui:` 前缀。手工修 socket.io、`app.html`。
- [x] 2.2 `src/lib/embed/`：`mode.ts`（嵌入开关）、`bridge.ts`（postMessage 协议，见 design §6）、`navigation.ts`、`resolveUrl.ts`、`theme.css`、`errors.ts`（网关错误中文化 + 去充值）。
- [x] 2.3 「模型与密钥」设置分区：调 `/_chat/__sub2api/keys|groups|model-prices`，存 Key ID 到用户设置；全局 fetch 加 `X-Sub2API-Key-Id`；未选 Key 时引导。
- [x] 2.4 隐藏与关闭：UserMenu 各项、设置页 Account / About / Connections / Audio / Personalization（记忆）、Changelog、更新 toast、Banners、Notes / Calendar / Workspace / Playground / 知识库入口、语音输入与通话按钮、联网搜索 / 代码解释器 / 生图开关；`iframeSandboxAllowSameOrigin` 移除；Gravatar / google favicons / OSM 移除。
- [x] 2.5 Artifacts：4 处 srcdoc（Artifacts、FullHeightIframe、FilePreview、CitationModal）改指向 fork 自带的 `/_chat/embed/artifact-frame.html`（contract F），内容经 postMessage 传入；删除 socket `execute` 处理；去掉 Google favicon。
- [x] 2.6 主题与品牌：灰阶 / 主色覆盖、`General.svelte` 内联灰阶、主按钮主色、深浅色由桥驱动；`S2A_HIDE_BRANDING=1` 时 iframe 内不显示任何名称与图标（同创作工作台），版权与 LICENSE 链接放设置页底部小字。
- [x] 2.7 余额：一轮回复结束发 `chat:balance-changed`。
- [x] 2.8 测试：vitest 覆盖 bridge（origin / source 校验、revoke 清理）、resolveUrl、navigation、错误映射、Key 选择逻辑。
- 门：`npm run build`、`npm run check`（类型）、新增 vitest 全过；codemod 二次执行无变化（幂等）。

## 阶段 3：fork 镜像

- [x] 3.1 `docker/sub2api-embed.Dockerfile`：slim、UID 1000、`S2A_EMBED_MODE=1`、`OFFLINE_MODE=true`、design §4 / §5 的全部环境变量默认值；HEALTHCHECK。
- [x] 3.2 `docs/sub2api-embed.md`：双方契约（路径、头、Cookie、postMessage、`__sub2api` 接口、环境变量、品牌开关与许可证说明、变基流程）。
- [x] 3.3 `scripts/rebase-upstream.md`（或脚本）：变基步骤 = 取上游 tag → 重放补丁提交 → 重跑 codemod → 跑测试与 e2e。
- 门：本机 `docker build` 成功；容器在 internal 网络下启动无外网请求报错。

## 阶段 4：sub2api 后端

- [x] 4.1 `config.go`：`chat.webui_url`（`CHAT_WEBUI_URL`）。
- [x] 4.2 `internal/server/chat_proxy.go`：`/api/v1/chat/status|session`；`/_chat/*` 全局中间件（静态直通、Cookie 校验复用 JWT 服务、剥头注入、Key ID → 明文 Key、剥前缀、`Location` 补前缀、WebSocket、请求体上限、CSP 与 `X-Frame-Options: SAMEORIGIN`）；`/_chat/__sub2api/keys|groups|model-prices`，artifact 沙箱页单独 CSP。
- [x] 4.3 `router.go` 一行挂接（与 studio 同处）。
- [x] 4.3a 网关搜索引用（design §7.1）：Anthropic→Responses 与 Responses→Chat Completions 输出 `url_citation` annotations（流式 / 非流式），段间补 `\n\n`；单测含「无 citations 时输出不变」。
- [x] 4.3b `__sub2api/keys` 返回平台与是否支持生图；反代处理 `X-Sub2API-Image-Key-Id`。
- [x] 4.4 单测 `chat_proxy_test.go`：未配置 503；无 Cookie 401；客户端伪造 `X-Sub2API-*` 被剥；他人 Key ID 不注入；禁用 / 过期 Key 不注入；Key 列表无明文；前缀剥离与 `Location` 改写；CSP 只作用于 `/_chat/*`；其他路径安全头不变。
- 门：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...`、`go build ./...` 通过。

## 阶段 5：sub2api 前端

- [x] 5.1 `frontend/src/themes/qx/chat/`：`ChatView.vue`（iframe 无边框、占满内容区）、`chatBridge.ts`（session Cookie 调用、theme / revoke 下发、ready / balance / navigate 处理）、`chatStatus.ts`、locales。
- [x] 5.2 路由 `/chat`、导航入口「对话」（「创作工作台」旁）；退出登录流程调用 `DELETE /api/v1/chat/session`。
- [x] 5.3 vitest：bridge 校验、导航白名单。
- 门：`npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run`；`git status` 无锁文件改动。

## 阶段 6：本机环境与 e2e

- [x] 6.1 `~/workspace/sub2api-local-chat`：`build.sh`、`docker-compose.yml`（Open WebUI + SQLite、白名单出站代理，internal 网络 `sub2api-local-chat`）、`.env`（随机生成，权限 600）；`sub2api-local` 的 compose 接入该网络并设 `CHAT_WEBUI_URL`。
- [x] 6.3 `e2e/p9_chat.py`（CHAT-xx 用例写入 `CASES.md`）：对话扣费与直接调 API 一致；标题等后台任务计在同一 Key；联网搜索与生图计费；跨用户会话 / 文件 / 笔记 / 日历访问拒绝；R9 屏蔽项接口不可用；普通用户不能创建工具 / 添加工具服务器、管理员可以；出站代理白名单外的域名被拒；伪造头无效；Open WebUI 自带登录 / 注册 / API Key 接口不可用；网关 usage IP 为用户 IP；Open WebUI 停机 503 并能恢复；运行期间无外网连接（检查容器网络 / 日志）。
- [x] 6.4 `e2e/ui/chat_ui.sh`（Playwright）：首次进入引导选 Key、对话流式、深浅色切换、换账号隔离、Artifacts 预览脚本无法访问父页存储、余额不足提示与去充值；截图给用户确认。
- [x] 6.5 完整回归：`./run_all.sh`（p0–p9）全过。
- 门：全部通过，截图经用户确认。

- 结果（2026-10-25）：阶段 1–5 由子代理完成（报告 `reports/stage1..5.md`，接口约定 `contract.md`）；阶段 6 本机 p9 CHAT-00..19 全过、`ui/chat_ui.sh` 8 步全过（截图 `~/workspace/owui-poc/chat-ui-final/`，待用户确认）。

## 阶段 7：收尾

- [x] 7.1 更新 `AGENTS.md`（本机环境章节加对话工作台）与 `.trellis/spec`（新的嵌入模式规范，可从 Nova 一节抽象出共用约定）。
- [ ] 7.2 经用户同意：创建 `qlqqs/open-webui`（公开）、推送 fork、CI 构建镜像；提交、推送 sub2api。
- [ ] 7.3 经用户同意并查看一号机资源后：部署 `/root/sub2api-custom-chat`，sub2api `.env` 设 `CHAT_WEBUI_URL`，真机复测。

## 回滚点

- 阶段 4 前 sub2api 无改动；阶段 4–5 的改动在未配置 `CHAT_WEBUI_URL` 时完全不生效。
- 一号机：去掉 `CHAT_WEBUI_URL` 并重启 sub2api 即下线；`docker compose down` 对话工作台项目不影响 sub2api。
