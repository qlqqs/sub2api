# 调研：Open WebUI v0.12.3 挂到子路径 `/_chat/`（2026-10-24）

源码：`~/workspace/open-webui`（tag v0.12.3）。行号以该版本为准。

## 结论

上游完全不支持子路径部署（无 `root_path` / base path 设置，CHANGELOG 无相关条目；`WEBUI_URL` 只用于 OAuth 回跳与 opensearch）。必须在 fork 里改。

推荐：**静态资源、API、WebSocket 全部放 `/_chat/` 下，sub2api 反代时剥掉前缀**（一条规则 `/_chat/*` → `owui:8080/*`）。不拆 `/api/chat/`：Open WebUI 已有 `/api/chat/completions` 等路由。

与 sub2api 的冲突：Open WebUI 大量使用 `/api/v1/*`（sub2api `router.go:122` 同样注册），只挪静态资源不够。

估计：后端约 0.5 天；前端机械替换 1.5–2.5 天；回归 1–2 天。

## 前端

- `svelte.config.js:1,14-18` adapter-static，fallback `index.html`，未配 `paths.base`；SvelteKit 2.68.0。
- 生产由 FastAPI 托管前端：`env.py:254` `FRONTEND_BUILD_DIR`，`main.py:3170-3174` 挂 SPA。
- API 常量 `src/lib/constants.ts:9-17`：`WEBUI_BASE_URL = ''`，其余 `WEBUI_API_BASE_URL` 等由它推导；101 个文件、约 360 处 fetch 走常量。
- 未走常量的硬编码：
  - API / WS：`utils/google-drive-picker.ts:7`、`utils/onedrive-file-picker.ts:35`（`/api/config`）；`workspace/common/VersionMenuItem.svelte:53`；`workspace/Models/ModelEditor.svelte:575`；`routes/+layout.svelte:185`（socket `path: '/ws/socket.io'`）、`:1480`（opensearch）。
  - 静态：`src/app.html:9,13,20,23,27,30,38,39,82,83,115,166`；`OnBoarding.svelte:49,58,64`；`admin/Analytics/Dashboard.svelte:663`；`chat/FileNav/SqliteView.svelte:32`；`workers/pyodide.worker.ts:28`、`pyodide/pyodideSandboxHost.ts:20,192`；`workers/kokoro.worker.ts:5`；`NotificationToast.svelte:77`、`routes/+layout.svelte:790,1417`（音频）。
- 前端路由（改动最大）：88 个文件 import `goto`，约 163 处 `goto('/...')`；约 42 处 `href="/..."`；7 处 `location.href = '/'`；约 31 处 `pathname` 判断（如 `Chat.svelte:1938,2747`、`(app)/workspace/+layout.svelte:38-99`、`apis/auths/index.ts:418,428`）。
  - 备选：hash router（`kit.router.type = 'hash'`）可绕开大部分前缀改动，兼容性未确认，需 PoC。
- 坑：`routes/+layout.svelte:180` `io(WEBUI_BASE_URL || undefined, …)`，`WEBUI_BASE_URL='/_chat'` 时会被当成 socket.io **命名空间**；必须改为 `io(window.location.origin, { path: '/_chat/ws/socket.io' })`。

## 后端

- `main.py:541-547` 无 `root_path`；`873` `app.mount('/ws', socket_app)`，`socket/main.py:387-390` `socketio_path='/ws/socket.io'`。
- `876-923` 路由前缀 `/ollama`、`/openai`、`/api/v1/*`；`933-3125` 直接定义 `/api/models`、`/api/chat/*`、`/api/config`、`/oauth/*`、`/manifest.json`、`/health`、`/cache/{path}` 等；`3122` `/static`，`3168` `/pyodide`，`3170` SPA（`SPAStaticFiles` `310-332`）。
- 做法 A（推荐）：反代剥前缀，后端仍按 `/` 服务；uvicorn `--root-path /_chat` 让 `request.base_url` 带前缀（Starlette `get_route_path` 在 path 不以 root_path 开头时原样返回）。
- 做法 B（不推荐）：`Mount('/_chat', app)` 包一层，会破坏按原始路径判断的逻辑：`utils/asgi_middleware.py:115`、`utils/audit.py:188,258-276`、`utils/auth.py:541`（API Key 端点白名单）、engine.io `socketio_path`（未确认）。

## 绝对路径重定向与存库 URL

- 重定向（sub2api `ModifyResponse` 统一给 `Location` 补前缀）：`utils/asgi_middleware.py:201-202`、`routers/models.py:1061,1091`、`utils/oauth.py:1362-1369,2231-2238`、头像 302 `routers/users.py:870`、`routers/channels.py:2106`。
- 存库 / 消息里的根相对 URL：默认头像 `/user.png`（`models/users.py:381,390,757`、`models/auths.py:196`、`utils/oauth.py:1872-1907`）；`/api/v1/users/{id}/profile/image`（`models/users.py:179`、`utils/auth.py:758`）；`/favicon.png`（`config.py:2119`）；`url_path_for` 拼的文件 / 图片 URL 不带 root_path（`utils/files.py:180`、`routers/images.py:559`）；`routers/models.py:697`。
  - 前端约 15 处直接作 `<img src>`；处理：前端 `resolveUrl()` 给以 `/` 开头且非 `/_chat` 的补前缀，后端默认值改带前缀。
  - `utils/validate.py:28-30,60`、`routers/models.py:121` 校验 URL 前缀，需同步。

## PWA / Service Worker

- 无 SW；`routes/+layout.svelte:89-100` 会**注销当前作用域下所有已注册的 SW**（同源下对 sub2api 的影响未确认，sub2api 若无 SW 则无影响）。
- `main.py:2991` manifest `start_url: '/'`、图标路径写死；iframe 内无需 PWA。

## 同源冲突

- Cookie：`routers/auths.py:207-215`、`utils/oauth.py:2295` 设 `token` cookie 无 path（默认 `/`）→ 加 `path='/_chat'`。
- localStorage：两边都用 `theme`（sub2api 12 处；Open WebUI `app.html` 读写 `localStorage.theme`，取值 `system`/`oled-dark`/`her`），还有 `token`、`locale`、`settings` → fork 内统一加命名空间前缀（如 `owui:`）。

## 改动清单（推荐方案）

前端：`paths.base='/_chat'`（fallback 行为待实测）；`WEBUI_BASE_URL` 由 `$app/paths.base` 推导；`app.html` 改 `%sveltekit.assets%`（内联脚本内是否替换待确认）；修 socket；约 20 处硬编码资源；`$lib/navigation.ts` 包 `goto` + `href` / `location.href` / `pathname` 替换；`resolveUrl()`；localStorage 命名空间。

后端：`backend/start.sh:108` 加 `--root-path /_chat`，`WEBUI_URL`；默认 URL 与 `url_path_for` 加前缀（约 10 处）+ `validate.py`；cookie path；manifest。

建议写 codemod / lint 规则防止新增硬编码路径。
