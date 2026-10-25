# PoC-A：子路径 `/_chat/`（0.1）与严格 CSP（0.2）实测（2026-10-24）

- 源码：`~/workspace/open-webui`，分支 `sub2api-poc-a`（基于 tag v0.12.3），改动未提交，保留在工作区。
- 运行环境：venv `~/workspace/owui-poc/venv`（slim 依赖；uvicorn 0.51.0 / starlette 1.10.0 / fastapi 0.136.3 / python-engineio 4.14.0）；Node 22.23.4；Playwright 1.63 无头 Chromium。
- 拓扑：浏览器 → 模拟 sub2api 反代 `127.0.0.1:28181`（只放行 `/_chat/*`，剥前缀，注入 `X-Sub2API-Email/Name`，其余路径 404 并记 `ESCAPE`）→ uvicorn `127.0.0.1:28180` → 假 OpenAI `127.0.0.1:28199`（流式回复内含代码块、KaTeX、mermaid、HTML artifact）。
- 文件：
  - fork 内：`scripts/embed-codemod.mjs`（codemod 草稿）、`scripts/embed-postbuild.mjs`（内联脚本外置）、`src/lib/embed/{paths,navigation}.ts`、`backend/open_webui/embed/root_path.py`
  - PoC 目录 `~/workspace/owui-poc/a/`：
    - `bin/`：`proxy.py`、`stub_openai.py`、`run_backend.sh`、`pw_flow.cjs` / `pw_flow.sh`、`csp_scan.py`
    - `logs/`：`flow-*.json`、`proxy.log`、`csp-scan-build*.json`、`codemod.txt`
    - `shots/`：截图
- 未做：`npm run build` 的 `pyodide:fetch`（会下载 numpy/pandas 等几百 MB），改为 `npm run crypto:verify && npx vite build`，因此代码解释器不可用（嵌入模式本来就关）。没有跑 `svelte-check`。

## 0.1 子路径

### 结论：通过

前端 `paths.base='/_chat'` 加 codemod，后端只需改一处（socket.io 挂载），整条链路可用。最终构建（build3）跑完整流程，结果如下：

| 检查项 | 结果 |
|---|---|
| 可信头自动登录 | ✓ `u1@sub2api.internal`，首个用户成为 admin（设计 §2.3 已计划去掉） |
| 首页渲染 | ✓ |
| 新建对话、流式回复 | ✓ URL 变为 `/_chat/c/<id>` |
| socket.io | ✓ 走 `/_chat/ws/socket.io/?EIO=4&transport=websocket` |
| 头像（用户 / 模型） | ✓ 5 张图片全部加载 |
| 在 `/_chat/c/<id>` 刷新 | ✓ SPA fallback 返回 200，聊天内容恢复 |
| 侧栏点击会话、新建对话链接 | ✓ |
| `/_chat/workspace/models` | ✓ |
| `/_chat/admin/settings` | ✓ 打开设置弹窗 |
| 越出子路径的请求 | 0 次（`escapes: []`，`failed: []`） |

- 截图：`shots/nocsp-0[1-6]-*.png`。
- 不带 `--root-path` 的对照组同样全部通过：`shots/norootpath-*`、`logs/flow-norootpath.json`。

### 关键发现

1. **uvicorn `--root-path` 与设计假设不同（阻塞项，已修）**
   - uvicorn ≥0.26 会把 root_path 前置到 `scope['path']`（`venv/.../uvicorn/protocols/http/h11_impl.py:203`、`websockets/websockets_sansio_impl.py:211`）。所以反代剥掉 `/_chat` 后，应用内看到的仍是 `/_chat/...`。
   - Starlette 路由与 StaticFiles 靠 `get_route_path`（`starlette/_utils.py:137-152`）剥掉前缀，能正常工作。
   - python-engineio 直接拿 `scope['path']` 和 `'/ws/socket.io/'` 比较（`engineio/async_drivers/asgi.py:64-67`），结果 socket.io 全部 404（已实测）。
   - 修法：`main.py:873-876` 挂载时包一层 `EngineIORootPathFix`（`backend/open_webui/embed/root_path.py`），按 `scope['app_root_path']` 剥前缀。修后实测 WebSocket 正常。
   - 曾尝试全局剥前缀的中间件，结果 `/static` 挂载 404（StaticFiles 需要带前缀的 path），已放弃。
   - 加了 `--root-path` 后，其他直接读原始路径的判断看到的都是带前缀的路径，只是不再命中，不会报错：
     - `utils/asgi_middleware.py:115` health 快速通道；
     - `utils/auth.py:541` API Key 端点白名单（我们关闭了 API Key）；
     - `utils/audit.py:188,258-276`（默认关闭）。
   - `--root-path` 的实际收益很小：后端只有 OAuth / SCIM / MFA 用到 `request.base_url`（`utils/oauth.py:574,693,1362,2231`、`routers/scim.py:362-414`、`routers/mfa.py:112`），三者在嵌入模式都关闭。
   - **建议：不用 `--root-path`**，后端按根路径服务，需要绝对 URL 的地方靠 `WEBUI_URL=https://<域名>/_chat`。这样零补丁，对照组已验证。若仍要保留 `--root-path`，必须带上 `EngineIORootPathFix`。
2. **SvelteKit `paths.base` 可用**
   - adapter-static 产物仍输出在 `build/` 根下，只有 `index.html` 一个 fallback 页面，所有 URL 都带 `/_chat`。
   - `app.html` 中的 `%sveltekit.assets%` 在内联脚本里同样会被替换（`relative: false` 时为 `/_chat`）。
   - 后端 `SPAStaticFiles`（`main.py:310-322`）对 `/c/<id>` 等未知路径回退到 `index.html`，刷新正常。
3. **漏改导致的越界（已修）**：第一版构建后，新建对话的 URL 变成 `/c/<id>`（越界），刷新即 404。原因是 `history.replaceState(…, '/c/…')` 不经过 `goto`（`Chat.svelte:4451,4714,2749`、`channel/Channel.svelte:110`），codemod 加规则 R12 后修复。
4. **`page.url.pathname` 带 base**：凡是用 `=== '/'`、`startsWith('/workspace')`、`split('/')[2]` 判断路由的地方都会失效，统一包一层 `stripBase()`（R6）。

### 后端生成的不带前缀 URL（第 5 项）

| 位置 | 内容 | 实测 / 处理 |
|---|---|---|
| `routers/models.py:1061,1091` | 模型头像 302 `Location: /static/favicon.png` | 每次加载模型头像都会触发（本轮 38 次），由反代补 `Location` 前缀后正常 → sub2api 必须改写 `Location` |
| `utils/asgi_middleware.py:201-202` | `/watch?v=`、`?shared=` 307 到 `/?youtube=…` | 实测 `Location=/?youtube=abc` → 同上 |
| `models/users.py` 等默认头像 | `profile_image_url: "/user.png"`（`/api/v1/auths/` 实测） | 前端 `safeImageUrl` 加 `withBase` 与 `src={withBase(…)}` 已兜底，图片正常 |
| `utils/files.py:180`、`routers/images.py:559` | `url_path_for` 生成 `/api/v1/files/<id>/content`，存进消息 | 渲染时由 `withBase` 补前缀（未端到端测生图） |
| `main.py:2994,3015`（工作区行号，上游为 2991,3012） | manifest `start_url: "/"`、`share_target.action: "/"`、图标 `/static/logo.png` | iframe 内用不到，可不改；要改就由 base 生成 |
| `main.py:3034` opensearch | `template="{webui_url}/?q="`，`WEBUI_URL` 未设时为 `/?q=` | 设 `WEBUI_URL` 即可 |
| `routers/auths.py:208` | `Set-Cookie: token=…; Path=/` | 实测确认，与 sub2api 同源冲突 → 改为 `path='/_chat'` |
| 存库 URL | `ModelEditor.svelte:575` 写入 `background_image_url=/api/v1/files/…` | codemod 故意不改（库里存不带前缀的路径），渲染时补 |

- 无 Service Worker 请求。`+layout.svelte:89-100` 注销同源全部 SW 的问题（见 paths.md）本次未涉及。

### codemod 改动清单（给阶段 2）

辅助模块：
- `src/lib/embed/paths.ts`：`BASE`、`withBase()`（只处理以单个 `/` 开头且未带前缀的串，幂等）、`stripBase()`。
- `src/lib/embed/navigation.ts`：`export * from '$app/navigation'`，并覆写 `goto` 自动补前缀。

自动规则（`scripts/embed-codemod.mjs`，首轮输出见 `logs/codemod.txt`；重复执行幂等，第二次只命中新增的 R12）：

| 规则 | 内容 | 处数 / 文件数 |
|---|---|---|
| R1 | `svelte.config.js` 加 `paths: { base: env ?? '/_chat', relative: false }` | 1 / 1 |
| R2 | `constants.ts` `WEBUI_BASE_URL = base`（`$app/paths`）；约 360 处 API fetch 自动跟随 | 1 / 1 |
| R3 | `from '$app/navigation'` → `'$lib/embed/navigation'`（覆盖约 178 处 `goto`，含 `goto(redirectPath)`、`goto(meta.href)` 等动态值） | 89 / 88 |
| R4a | 模板属性 `href/src/poster/action="/…"` → `"{BASE}/…"` | 41 / 17 |
| R4b | `href={expr}` → `href={withBase(expr)}`（菜单 `meta.href`、`action.href`、markdown 链接等动态值） | 31 / 24 |
| R4c | `src={expr}` → `withBase(expr)`（兜底存库的根相对 URL：头像、文件、生成图） | 57 / 33 |
| R5 | `location.href = '/…'` → `withBase(...)` | 7 / 7 |
| R6 | `$page.url.pathname` → `stripBase($page.url.pathname)` | 33 / 13 |
| R7 | 代码里 `'/api…'`、`'/static…'`、`'/audio…'`、`'/assets…'`、`'/favicon.png'`、`'/user.png'`、`'/opensearch.xml'`、`'/manifest.json'` 等字面量 → `withBase()`；排除终端服务器路径 `XTerminal.svelte`、`FileNav/FilePreview.svelte`，以及 workers / pyodide / `safeImageUrl` / ModelEditor | 27 / 21 |
| R9 | `app.html` 中的 `/static/…`、`/manifest.json`（包括内联脚本里的 splash 路径）→ `%sveltekit.assets%/…` | 13 / 1 |
| R11 | `` `${window.location.origin}/…` ``（分享链接）→ 加 `${BASE}` | 2 / 2 |
| R12 | `history.replaceState/pushState(state, '', '/…')` → `withBase()`（不含 `iframe.contentWindow.history`） | 4 / 2 |

自动规则合计：155 个前端文件、约 430 行改动。

手工改动（阶段 2 应并入 codemod 或单独补丁）：

| 位置 | 改法 |
|---|---|
| `routes/+layout.svelte:181,186` | socket：`io(WEBUI_BASE_URL \|\| undefined)` → `io(undefined, { path: withBase('/ws/socket.io') })`；否则 `/_chat` 会被当成命名空间 |
| `chat/MessageInput.svelte:598`、`chat/Chat.svelte:1939` | `location.pathname` / `p.url.pathname` 用 `stripBase` |
| `chat/Messages/Markdown/MarkdownInlineTokens.svelte:50,85-87` | 应用内链接判断用 `stripBase` |
| `apis/auths/index.ts:412-429` | 登出 URL：构造时 `withBase`，判断时 `stripBase` |
| `utils/safeImageUrl.ts:31-40` | 根相对 URL 走 `withBase`；原来已拼了 `WEBUI_BASE_URL`，不能套 R7 |
| `workers/pyodide.worker.ts:28`、`workers/kokoro.worker.ts:5` | worker 里不能用 `$app/paths`，改为从 `self.location.pathname.split('/_app/')[0]` 推导 |
| `pyodide/pyodideSandboxHost.ts:193` | `indexURL` 加 `${BASE}` |
| 后端 `main.py:873-876` + `embed/root_path.py` | 只有保留 `--root-path` 时才需要 |

未改，只记录：
- localStorage 键前缀：993 处引用（214 个文件），其中 `token` 898 处、`theme` 10 处，另有 `locale`、`sidebar`、`settings`、`redirectPath` 等约 25 个键；sessionStorage 43 处（14 个文件）。
  - 建议不要逐处改键名：codemod 把标识符 `localStorage` 换成 `$lib/embed/storage` 导出的 Proxy（读写时自动加 `owui:` 前缀）。
  - `app.html` 内联脚本里的 `localStorage.theme` 单独处理。
- `onedrive` / `google-drive` 的 `fetch('/api/config')` 已被 R7 覆盖（这两个功能会关闭）。

### 阻塞项

无。需要写进设计的约束：
1. uvicorn `--root-path` 的语义（见上文发现 1）：二选一——去掉 `--root-path` 并设 `WEBUI_URL`（推荐），或保留并带 `EngineIORootPathFix`。
2. sub2api 反代必须改写 `Location`（模型头像每次都会 302）。
3. Cookie `token` 的 path 改为 `/_chat`。

### 设计修改建议

- §1 / §2.3 / §8：uvicorn 启动参数去掉 `--root-path /_chat`（保留 `--no-proxy-headers`），加 `WEBUI_URL`；后端补丁清单删除「default 头像等 URL 加前缀、`validate.py` 同步」，改由前端 `withBase`（R4c / `safeImageUrl`）在渲染时补。存库值保持不带前缀，避免 `validate.py` 与将来变基冲突。
- §6：codemod 规则采用上表 R1–R12 + 手工项；`$lib/embed/navigation.ts` 用「`export *` + 覆写 `goto`」，改 import 即可，不需要逐处改调用。localStorage 用 Proxy 替换标识符。
- 本机启动 slim 后端要设 `USE_SLIM_DOCKER=true`（不是 `USE_SLIM`），否则 `config.py:515` 会 import chromadb。
- npm ci 要加 `ONNXRUNTIME_NODE_INSTALL_CUDA=skip CYPRESS_INSTALL_BINARY=0`，否则 onnxruntime-node 的 postinstall 会从 GitHub 拉数百 MB 的 CUDA 库。

## 0.2 严格 CSP

### 结论

- **`script-src` 可以不带 `'unsafe-inline'`**：
  - 用 3 个 sha256 hash 实测通过（build3）。
  - 推荐方案更简单：构建后把内联脚本外置（`scripts/embed-postbuild.mjs`），CSP 就是 `script-src 'self'`，不需要任何 hash，也已实测通过。
  - 两种方式下主文档 CSP 违规都是 0 次；首页、对话、流式回复、KaTeX、mermaid、代码块、刷新、侧栏导航、workspace、admin 设置全部正常。
- **不需要 `'unsafe-eval'`**，已开放功能的路径上没有触发任何 eval 违规。打包产物里确实有 `new Function` / `eval` / WebAssembly，但只在下列关闭或边缘功能中使用：
  - `Chat.svelte:1767` 的 socket `execute` 事件：工具让页面执行任意 JS。在 CSP 下被拦截，这对安全有利。
  - alpinejs：`FullHeightIframe.svelte:101` 把它注入工具富 UI 的 srcdoc。
  - heic2any：上传 HEIC 图片时转换，emscripten 实现，需要 eval，会失效。
  - html2canvas-pro：导出聊天为图片 / PDF，需在阶段 2 实测。
  - mammoth / bluebird：docx 预览。
  - d3-dsv、jszip 的 setImmediate polyfill：只有特定输入才会走到。
  - pdfjs：只做 eval 探测，有回退。
  - WASM 需要 `'wasm-unsafe-eval'`：pyodide（代码解释器，已关）、onnxruntime / kokoro（TTS，已关）、sql.js（`SqliteView`）、@shikijs oniguruma。本次走到的路径都没有触发。
- **`style-src` 必须保留 `'unsafe-inline'`**：去掉后单轮出现 523 条违规，mermaid 图变成黑块、CodeMirror 失去样式（`shots/csp-nostyle-02-chat.png`）。来源包括 Svelte / bits-ui 的内联 style、mermaid SVG 里的 `<style>`、KaTeX、`app.html` 的 `<style>`。
- **Artifacts / HTML 预览在严格 CSP 下失效，符合预期**：
  - srcdoc iframe 继承父文档 CSP。实测预览只显示静态 HTML（"Artifact static text"）：`<script>` 被拦（"Executing inline script violates…"），`onclick` 被拦（"Executing inline event handler violates…"）。
  - 无 CSP 时显示 "Artifact JS ran"，点击后显示 "inline handler ran"（`shots/csp-03-artifact.png` 与 `nocsp-03-artifact.png` 对比）。
  - 同样受影响的 srcdoc：
    - `common/FullHeightIframe.svelte:221`：工具富 UI、HTML 文件、引用文档；
    - `chat/FileNav/FilePreview.svelte:402`；
    - `Citations/CitationModal.svelte:229`；
    - `pyodide/pyodideSandboxHost.ts:194`：代码解释器沙箱，本来就关。
  - 这些都应改走设计 §5.2 的 `/_chat/__sub2api/artifact-frame`。
  - 另：上游 `Artifacts.svelte` 的 `iframeLoadHandler` 会读取 sandbox iframe 的 `contentWindow`，无论有无 CSP 都抛 `SecurityError`（`pageerror`），属上游既有问题。
- **`media-src` 需加 `data:`**：`utils/audio.ts:8` 的 `SILENT_WAV`（`data:audio/wav`）用来解锁 TTS 音频，每轮被拦 15 次。只是控制台噪音，TTS 关闭后无影响；加上 `data:` 风险很低，否则就在 fork 里去掉它。

### 证据

1. **构建产物的内联脚本**（`csp_scan.py`）
   - 只有 `build/index.html` 一个 HTML。
   - 3 段内联 `<script>`，加 1 个外链 `static/loader.js`。
   - 0 个内联事件处理器；8 个 `style=` 属性；2 个 `<style>`。

   | 脚本 | sha256 | 稳定性 |
   |---|---|---|
   | `app.html` 的 `resizeIframe()`（127 B，源码中无人调用，是死代码） | `IwbGmoRaOLT2V3cavVXQBCapN9X3Jy2mX1vd0rYDIHI=` | 稳定 |
   | `app.html` 主题 / splash 启动脚本（3470 B） | `2AgDVK7ebSkeuxN8Aql9O96QfktnXtT/V6Whz6SUQB4=` | 稳定（改 `app.html` 或 base 才变） |
   | SvelteKit 启动脚本（`__sveltekit_<版本>` 全局名 + `import("/_chat/_app/immutable/entry/start.<hash>.js")`） | build1 `R4d0vwjD…`，build3 `zxbRggUm…` | **任何源码改动都会变**：入口 chunk 名、git HEAD 派生的版本名都在其中。同一源码连续两次构建（build3 / build4）`index.html` 字节相同，构建可复现 |

   → 用 hash 方案时，每次构建都要由 fork 输出 hash 给 sub2api（或由 sub2api 在反代 `index.html` 时现算），容易漂移。
2. **CSP 实测**（反代下发，删除上游 CSP / XFO）：

   | 方案 | 截图 | 记录 |
   |---|---|---|
   | hash 方案 | `shots/csp-*` | `logs/flow-csp.json` |
   | 外置方案（`script-src 'self'`） | `shots/csp-selfonly-*` | `logs/flow-csp-selfonly.json` |
   | 去掉 style `'unsafe-inline'` | `shots/csp-nostyle-*` | `logs/flow-csp-nostyle.json` |

   三组的主文档 `securitypolicyviolation` 都由 init script 采集。前两组为 0，第三组为 40（style-src-attr 26 / style-src-elem 14）。

### 设计修改建议（§5.2）

1. fork 构建链加 `scripts/embed-postbuild.mjs`（PoC 版已验证）：把 `index.html` 的全部内联脚本写到 `_app/immutable/inline/<hash>.js`，并替换为 `<script src>`。
   - SvelteKit 启动脚本依赖 `document.currentScript.parentElement`，外链的经典脚本同样可用（已验证）；主题脚本仍在 `<head>` 同步执行，不会出现 FOUC。
   - sub2api 的 CSP 写死 `script-src 'self'`，不需要随构建同步 hash。
   - 顺手删除 `app.html` 中的死代码 `resizeIframe`。
2. 最终 CSP：

   ```
   default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; media-src 'self' blob: data:; worker-src 'self' blob:; frame-src 'self' blob: data:; frame-ancestors 'self'; base-uri 'self'; form-action 'self'
   ```

   不加 `'unsafe-eval'` 与 `'wasm-unsafe-eval'`；将来要开代码解释器 / TTS 时，再给对应 worker 路径单独评估。
3. 所有 srcdoc 用法（Artifacts、FullHeightIframe、FilePreview、CitationModal）在嵌入模式下统一改走 `/_chat/__sub2api/artifact-frame` + postMessage（设计 §5.2 原方案不变，范围从 Artifacts 扩大到这 4 处）。
4. fork 删掉 socket `execute` 事件处理（`Chat.svelte:1761-1775`），或在嵌入模式禁用；它在 CSP 下本来就会失败，显式移除能避免报错。
5. 阶段 2 需要补测：在 CSP 下导出聊天（html2canvas-pro）、docx / pdf 预览、HEIC 上传、生图结果显示。
