# 阶段 2 报告：Open WebUI fork 前端改造（2026-10-24）

仓库 `~/workspace/open-webui`，分支 `sub2api-custom`（= v0.12.3），改动未提交（提交由主会话统一做，拆分方法见 §8）。
协议与实现细节写在 fork 的 `docs/sub2api-embed-frontend.md`。

## 0. 结论

| 完成标准 | 结果 |
|---|---|
| `npx vite build` + postbuild | 通过。index.html 的 3 段内联脚本已外置，剩余内联脚本 0、内联事件 0；postbuild 再跑一次无变化。需要 `NODE_OPTIONS=--max-old-space-size=8192`，默认堆会 OOM |
| `npm run check`（svelte-check）与 v0.12.3 基线比 | 基线 5696 errors / 188 warnings；fork 5696 / 188。按（文件, 消息）比较，新增 0 条 |
| 新增 vitest | 8 个文件 55 条全过（含上游原有 1 个测试文件在内共 59 条） |
| codemod 二次执行无变化 | `npm run embed:check` 退出码 0；在干净的 v0.12.3 树上重跑，结果与工作区的 codemod 部分一致 |
| Playwright 严格 CSP 全流程（§4） | 17 步全部通过。越出 `/_chat` 的请求 0；主文档 CSP 违规 0（含 style）；外站请求只出现在 Artifact 沙箱里，且都被 CSP 拦下 |

联调中发现 3 个跨阶段问题（§9）：
- `X-Sub2API-Name` 含中文时 WebSocket 握手崩溃，流式回复收不到。
- 换账号后旧的 `token` Cookie 让 `/api/config` 返回 401，前端进入错误页。
- 嵌入后端拒绝不带 token 的 socket 连接。

第 3 个已在前端兼容；前 2 个需要改约定。

## 1. 改动清单

### 1.1 codemod（`scripts/embed-codemod.mjs`，生成 286 个文件的改动）

在 PoC 草稿上补齐了手工项，规则表见 `docs/sub2api-embed-frontend.md` §2。

**沿用 PoC 的规则**

| 规则 | 内容 |
|---|---|
| R1 | `svelte.config.js` 设 `paths.base` |
| R2 | `WEBUI_BASE_URL` 改为 `base` |
| R3 | `goto` 改从 `$lib/embed/navigation` 导入（89 处） |
| R4a/b/c | 模板属性补前缀（129 处） |
| R5 | `location.href` 赋值补前缀 |
| R6 | `stripBase($page.url.pathname)`（34 处） |
| R7 | 根相对字符串补前缀（26 处） |
| R11 | `${window.location.origin}/…` 补前缀 |
| R12 | `history.replaceState` / `pushState` 补前缀 |

**新增规则**

| 规则 | 内容 |
|---|---|
| R13 | 存储命名空间：215 个模块加 `import { localStorage, sessionStorage } from '$lib/embed/storage'`，同名导入遮蔽全局，不逐处改键名 |
| P（15 处精确补丁） | 原 PoC 的全部手工项，另加 3 处，见下 |
| R9 | `app.html` 单独处理，见下 |

P 里的原 PoC 手工项：
- socket.io 的 `path`；
- `MessageInput`、`MarkdownInlineTokens`、`auths/index.ts` 的路径判断；
- `safeImageUrl`；
- pyodide / kokoro worker；
- `pyodideSandboxHost`。

P 里新增的 3 处：
- 清 `token` Cookie 时用 `path=${BASE}`；
- i18next 检测器的 `lookupLocalStorage` 改为 `'owui:locale'`；
- `common/Image.svelte` 双重前缀修复（联调发现：生图结果曾请求 `/_chat/_chat/api/v1/files/…`）。

R9 对 `app.html` 的处理：
- 资源路径改用 `%sveltekit.assets%`；
- 删除死代码 `resizeIframe`；
- `localStorage.theme` 改为 `localStorage['owui:theme']`；
- 插入嵌入启动脚本：`?theme=` 预置主题；在 iframe 内或品牌缓存为 1 时，隐藏启动画面 logo。

其他特性：
- 幂等；有 `--dry` / `--check`。
- 锚点失配时打印 `WARN MISSING`、退出码 2，便于变基时发现上游改动。

### 1.2 `scripts/embed-postbuild.mjs`

- 外置内联脚本，并校验剩余内联脚本与事件属性，有残留时退出码非 0。
- 跳过 `build/embed/artifact-frame.html`。
- 幂等。

### 1.3 手写新文件

**`src/lib/embed/`**（全部新增）

| 文件 | 作用 |
|---|---|
| `paths.ts`、`navigation.ts` | 子路径工具；`goto` 包装 |
| `storage.ts` | `owui:` 前缀的 Storage Proxy，`clearEmbedStorage` |
| `mode.ts` | 由 `/api/config.sub2api` 判定嵌入模式，contract D |
| `bridge.ts` | contract E，含 `chat:session-expired` 续期 |
| `fetch.ts` | Key 头；区分 sub2api 的 401 `CHAT_SESSION_INVALID` |
| `keys.ts` | contract C 客户端与 Key 选择逻辑 |
| `errors.ts` | 网关错误中文化 |
| `features.ts` | 隐藏开关表 |
| `theme.ts`、`theme.css` | QxAPI token；深浅色由桥驱动 |
| `avatar.ts` | 中性模型头像 |
| `links.ts` | 去掉 Google favicon / 搜索链接 |
| `artifact.ts`、`ArtifactFrame.svelte` | Artifact 沙箱页的父侧 |
| `app.ts` | 布局入口 |
| `i18n.ts` | 嵌入文案，zh-CN / en-US |
| `KeysSettings.svelte` | 「模型与密钥」 |
| `SidebarSettings.svelte` | 侧栏「设置」按钮 |
| `LicenseFooter.svelte` | 版权与 LICENSE 小字 |
| `GatewayErrorHint.svelte` | 错误提示与动作按钮 |
| `__tests__/` | vitest |

**其他新增文件**

| 文件 | 作用 |
|---|---|
| `static/embed/artifact-frame.html` | Artifact 沙箱页 |
| `static/embed/model-avatar.svg` | 中性模型头像 |
| `static/embed/link.svg` | 引用来源的中性图标 |
| `docs/sub2api-embed-frontend.md` | 前端嵌入说明与 Artifact 协议 |
| `package.json` | 只加了 scripts：`embed:codemod`、`embed:check`、`embed:postbuild`、`build:embed`、`test:embed`。没有新依赖，`package-lock.json` 未动 |

### 1.4 上游组件里的手写挂接

共 25 个文件，每处都有 `sub2api-embed` 注释，可用 `git grep -n sub2api-embed -- src` 列出：

**布局与路由**

| 文件 | 挂接 |
|---|---|
| `routes/+layout.svelte` | theme.css；`installEmbedFetch()`（必须在上游 fetch 覆写前）；`initEmbedApp`；品牌名；revoke 后显示空白页 |
| `routes/(app)/+layout.svelte` | 不请求横幅；Changelog；`embedAfterUserSettings`；被屏蔽路由回 `/` |
| `routes/(app)/admin/+layout.svelte`、`admin/+page.svelte` | 只留函数 / 工具 / 工具服务器 |
| `routes/(app)/workspace/+layout.svelte` | 去掉知识库、技能 |
| `routes/auth/+page.svelte` | 不显示「正在登录 Open WebUI」 |

**侧栏与设置**

| 文件 | 挂接 |
|---|---|
| `layout/Sidebar.svelte` | 两处 UserMenu 换成「设置」按钮；固定显示侧栏项白名单 |
| `chat/SettingsModal.svelte` | 「模型与密钥」分区；分区白名单；底部版权小字 |
| `chat/Settings/General.svelte` | 隐藏主题、语言 |
| `common/InterfaceSettings.svelte` | 隐藏 SameOrigin 开关 |
| `admin/Settings/Integrations.svelte` | 只保留工具服务器 |

**对话界面**

| 文件 | 挂接 |
|---|---|
| `chat/Chat.svelte` | 余额事件；错误 toast 中文化；忽略 `execute` 事件；functions / skills 拉取失败不再中断预设默认值 |
| `chat/MessageInput.svelte` | 麦克风、通话；生图开关按 Key 显示；HEIC 提示 |
| `chat/MessageInput/IntegrationsMenu.svelte` | 「去设置生图 Key」入口 |
| `chat/Messages/ResponseMessage.svelte` | 朗读、点赞 / 点踩 |
| `chat/Messages/Error.svelte` | 网关错误中文化 |
| `chat/ModelSelector/ModelItem.svelte` | 单价 |
| `layout/Navbar/Menu.svelte` | 分享 |
| `Citations.svelte`、`WebSearchResults.svelte` | 不加载 Google favicon、不给搜索链接；LICENSE 注释的 `/favicon.png` 回退保留不动 |
| `common/Valves/MapSelector.svelte` | 不加载 OSM 瓦片 |

**4 处 srcdoc**

`Artifacts.svelte`、`common/FullHeightIframe.svelte`、`FileNav/FilePreview.svelte`、`Citations/CitationModal.svelte`。

## 2. 各项完成情况（implement 2.1–2.8）

### 2.1 codemod 与 postbuild

完成，见 §1。

### 2.2 `src/lib/embed/`

全部完成。约定里的 `resolveUrl.ts` 由 `paths.ts` 的 `withBase` 承担。

### 2.3 模型与密钥

**分区内容**
- 对话 Key 下拉：不可用的 Key 置灰并写原因（已禁用 / 已过期 / 额度已用完 / 分组不可用 / 分组不支持对话）。
- 生图 Key 下拉：只列 `supports_images` 的 Key。
- 一键创建：可选分组，能生图的分组排前面。
- 选择存到 `ui.sub2api`（字段级补丁），随后重拉模型列表与单价。

**请求头**
- 全局 fetch 给 `/_chat/` 下的请求加 `X-Sub2API-Key-Id` / `X-Sub2API-Image-Key-Id`。
- 实测对话、标题任务、生图都带上了正确的 Key：网关桩收到 `sk-test-u1-k11`，生图收到 `k12`。

**引导**
- 首次进入且没有可用 Key 时，自动打开该分区，并给出带「去选择」按钮的 toast；选好 Key 后 toast 关闭。

**单价与生图**
- 模型选择器每行显示 `price_text`。
- 生图开关按 PRD R5.6：生图 Key 可用，或对话 Key 本身支持生图时才显示；否则在扩展菜单里显示「去设置生图 Key」。

### 2.4 隐藏与关闭

全部按清单完成（§1.4）。另外：
- 嵌入模式下 `iframeSandboxAllowSameOrigin` / `terminalPreviewAllowSameOrigin` 强制为 false。
- 被屏蔽功能的路由回到 `/`。
- 管理员的「工作空间」保留在侧栏，底部另有「插件与工具」（进入 `/admin/functions`，页内导航为 函数 / 工具 / 工具服务器）。

### 2.5 Artifacts

4 处 srcdoc 在嵌入模式下改用 `ArtifactFrame`；socket `execute` 在嵌入模式下忽略。协议见 §3。

### 2.6 主题与品牌

- 深浅色由桥驱动，`chat-session` 的 theme / locale 变化即时切换（已按约定 E 补充项实现）。
- `hide_branding` 时：标题为「对话」；侧栏 logo / 名称隐藏；启动画面无 logo；默认模型头像换成中性图标；自动登录页不显示名称。
- 设置页底部保留版权与 LICENSE 小字。

### 2.7 余额

每轮回复结束（成功或出错）发 `chat:balance-changed`，800 ms 去抖。实测父页收到。

### 2.8 vitest

| 文件 | 覆盖 |
|---|---|
| `bridge` | origin / source 校验、非 iframe、revoke、导航白名单、余额去抖、续期共享 / 超时 |
| `storage`、`storage-global` | 前缀、属性访问、枚举与 `clear` 只动本命名空间；revoke 后清理只删 `owui:`，不删 `auth_token` / `s2a_chat_owner` |
| `paths` | `withBase` / `stripBase` / `goto` |
| `errors` | 各类网关错误与动作 |
| `keys` | 置灰原因、生图 Key 回退、请求头只加在同源 `/_chat` 下 |
| `fetch` | sub2api 401 续期重试 / 转 403、Open WebUI 401 透传 |
| `misc` | artifact sandbox、中性头像、隐藏表、嵌入模式判定、文案 |

## 3. Artifact 协议

沙箱页 `static/embed/artifact-frame.html` → 浏览器路径 `/_chat/embed/artifact-frame.html`（contract F 的路径与 CSP 不变）。

**父页的 iframe**
- `sandbox` 为 `allow-scripts` 加原请求里的 forms / downloads / popups；永不含 `allow-same-origin` 和 `allow-top-navigation`。
- 设 `referrerpolicy=no-referrer`。

**消息**

| 方向 | 消息 | targetOrigin |
|---|---|---|
| 沙箱 → 父 | `{protocol:'sub2api-artifact', type:'ready'}` | `'*'` |
| 父 → 沙箱 | `{protocol:'sub2api-artifact', type:'render', html, scripts, autoHeight}` | `'*'` |
| 内容 → 父 | `{type:'iframe:height', height}`（与上游 FullHeightIframe 同名） | `'*'` |

- `scripts: false` 表示原请求不含 `allow-scripts`，沙箱页会追加 `script-src 'none'`。
- 父侧只认 `event.source === iframe.contentWindow`；沙箱只认 `event.source === window.parent` 的第一条 `render`。

**沙箱页的行为**
- 自检：必须是不透明源，且被嵌在 iframe 里，否则拒绝渲染（防止被同源无沙箱嵌入后，用宽松 CSP 执行内容）。
- 渲染：`document.open/write/close`，并在内容 `<head>` 最前插入一段引导脚本，负责上报高度、拦截链接点击。
- 每个内容版本建一个新 iframe，与 srcdoc 变化即重新加载的语义一致。

**实测结果**
- 内联脚本与 `onclick` 可运行（显示 "Artifact JS ran"、"inline handler ran"）。
- 读 Cookie、读 localStorage、访问 `parent.location` 都被拒绝。
- `fetch('/_chat/api/config')` 和外站图片都被 CSP 拦截。
- `window.origin` 为 `"null"`。

## 4. 测试环境与 Playwright 结果

### 4.1 环境

脚本都在 `~/workspace/owui-poc/fe/bin/`，只监听 127.0.0.1：

| 端口 | 服务 | 脚本 |
|---|---|---|
| 28220 | Open WebUI 后端（工作区 fork 后端，阶段 1 进行中的代码） | `run_backend.sh` |
| 28221 | 假 sub2api | `proxy.py` |
| 28222 | 假网关 | `gateway.py` |

假 sub2api 实现的内容：
- 父页 `/chat`；
- `POST/DELETE /api/v1/chat/session`；
- contract B：剥头、注入、两套 CSP、改写 `Location` / `Set-Cookie`、401 `CHAT_SESSION_INVALID`、WebSocket 不注入 Key；
- contract C：四个接口，假数据；
- 测试钩子；
- 其余路径一律 404 并记为 ESCAPE。

假网关：
- 按 Key 返回不同的模型列表；
- 流式回复里含代码、KaTeX、mermaid、HTML artifact；
- 用关键词触发余额不足 / 限流 / 审核等错误；
- 遇到 `generate_image` 工具时发起工具调用；
- 生图返回 b64。

常用命令：
- 启停：`bin/svc.sh start|stop|restart gateway proxy backend`
- 清空重来：`bin/reset.sh`
- 跑流程：`bin/flow.sh <label>`（按 `/home/qlqq/workspace/AGENTS.md` 加载 Playwright 依赖，中文字体用 sub2api-local 的 `fonts.conf`）

结果：
- 汇总：`logs/flow-r8.json`
- 截图：`~/workspace/owui-poc/fe/shots/r8-*.png`
- 测试结束后服务已停止。

### 4.2 最终一轮（r8）17 步全部通过

**新用户与 Key**
- 01 新用户（无 Key）：自动打开「模型与密钥」→ 一键创建 → 自动选中。截图 `r8-01-guide-no-key`、`r8-02-key-created`。
- 02 管理员选 Key：禁用 / 额度用完的 Key 置灰并写原因；分区列表正确；底部有版权与 LICENSE 小字。截图 `r8-03-keys-settings`。
- 03 模型选择器显示单价。截图 `r8-04-model-selector`。

**对话与沙箱**
- 04 对话流式输出：KaTeX 2 处、mermaid 1 个、代码块 2 个，无坏图；模型头像是中性图标。网关收到的 Key 全是 `sk-test-u1-k11`（含标题任务），XFF 为用户 IP。父页收到 `chat:balance-changed`。截图 `r8-05-chat`。
- 05 Artifact 沙箱：结果见 §3。截图 `r8-06-artifact`。
- 06 在 `/_chat/c/<id>` 刷新：200，内容恢复。截图 `r8-07-reload`。

**侧栏、设置、主题**
- 07 侧栏：
  - 没有头像菜单，只有「设置」（管理员另有「插件与工具」）；
  - 侧栏项为 笔记 / 日历 / 工作空间；
  - 点击会话、新对话都正常；
  - 管理后台导航为 函数 / 工具 / 工具服务器，工作空间为 模型 / 提示词 / 工具。
  - 截图 `r8-08-sidebar`、`r8-08b-sidebar-open`、`r8-08c-sidebar-open-dark`、`r8-08d-admin-functions`、`r8-08e-workspace`。
- 08 设置页：
  - 分区为 通用 / 界面 / 键盘 / 模型与密钥 / 数据 / 已归档对话，管理员另有「扩展功能」（工具服务器）；
  - 通用里没有主题和语言；界面里没有 SameOrigin 开关。
  - 截图 `r8-09`、`r8-10`。
- 09 深浅色与语言：父页切换后即时生效（深色页面底色 `#0b0f17`；切到 en-US 后 `html lang` 也变）。截图 `r8-11-dark`。

**错误、续期、生图、文件**
- 10 余额不足：显示中文提示和「去充值」按钮，点击后父页收到 `chat:navigate /purchase`；toast 也是中文。截图 `r8-12-balance-error`。
- 11 s2a_chat 失效：发 `chat:session-expired` → 父页重签（revision 1→2）→ 请求重试成功（200）。
- 12 生图：
  - 对话 Key 是 Claude 分组时，扩展菜单显示「去设置生图 Key」；
  - 设置生图 Key 后出现开关；
  - 生图请求用生图 Key（k12），图片正常显示。
  - 截图 `r8-13-image-hint`、`r8-14-image`。
- 12b 文件：
  - PDF 预览：pdf.js 在严格 CSP 下渲染出 canvas；
  - DOCX 预览：mammoth 正常，显示「Hello DOCX preview 你好」；
  - HEIC：给出中文提示，见 §5。
  - 截图 `r8-14b-preview-pdf`、`r8-14b-preview-docx`。
- 13 导出聊天 PDF（html2canvas-pro）：下载 `chat-桩对话.pdf`，无新增 CSP 违规。

**退出与换账号**
- 14 revoke：显示「会话已结束」；本源存储只剩控制台的 `s2a_chat_owner`。截图 `r8-15-revoked`。
- 15 换到普通用户 bob：看不到上一个用户的会话；`/auths/` 返回的是 u2；没有「工作空间」和「插件与工具」。截图 `r8-16-user2`。

### 4.3 全局统计

| 项 | 结果 |
|---|---|
| 越出 `/_chat` 的请求 | 0 |
| 外站请求 | 只有 Artifact 里探针发出的 `https://example.com/x.png`，被 CSP 拦截 |
| 主文档 CSP 违规 | 0（含 style）。依据是 Chrome 控制台的 CSP 报告：共 9 条，全部来自 Artifact 沙箱里的探针（`connect-src 'none'`、外站图片）；`securitypolicyviolation` 事件采集也是 0 |

剩余的失败请求都是被嵌入后端屏蔽的接口，上游界面仍会调用，前端已容错：
- `/api/v1/terminals`、`/api/v1/skills`、`/api/v1/groups` 返回 404；
- 普通用户的 `/api/v1/functions/` 返回 403；
- 管理员模型选择器的 `/ollama/api/version`、`/openai/config` 返回 404 / 403；
- 首次进入还没选 Key 时，`/api/models` 返回 400。

## 5. 已知问题与实测结论

### 5.1 CSP 下的实测

| 项 | 结论 |
|---|---|
| 导出聊天（html2canvas-pro） | PDF 导出成功，无 eval 违规 |
| docx / pdf 预览 | 都正常（pdf.js 在 `worker-src 'self' blob:` 下工作） |
| HEIC | heic2any 需要 `eval`，严格 CSP 下抛 EvalError。嵌入模式改为直接提示「暂不支持 HEIC 图片，请先转换为 JPG 或 PNG」，不调用 heic2any |
| 生图显示 | 正常（修复双重前缀后，`/_chat/api/v1/files/<id>/content` 可加载） |

### 5.2 其他已知问题

- **上传的 HTML 文件**：`HTMLToken.svelte` 里 `<file type="html">` 的 iframe 用 `src=/api/v1/files/<id>/content/html`（同源 URL，不是 srcdoc）。嵌入模式下已去掉 `allow-same-origin`，是不透明源；但它受默认 CSP 约束，内联脚本不运行。没有改走沙箱页，影响很小。
- **深色模式的代码块与 mermaid**：沿用上游配色（代码块近黑、mermaid 浅色框），和控制台的藏青色略有差异，未调整。
- **品牌**：设置页底部小字里有 Open WebUI 的版权文字，这是 PRD 要求保留的。iframe 的 `<link rel=icon>` 仍指向 favicon（在 iframe 里看不到）。
- **翻译**：zh-CN 上游缺约 9% 的译文，部分新功能仍显示英文。按设计用后端 `ui.i18n` 补，未在本阶段处理。
- **构建内存**：vite build 需要约 8 GB 堆（`NODE_OPTIONS=--max-old-space-size=8192`）。

## 6. 给阶段 3 Dockerfile 的构建步骤

阶段 3 已按此实现，见 `reports/stage3.md`。

```bash
ONNXRUNTIME_NODE_INSTALL_CUDA=skip CYPRESS_INSTALL_BINARY=0 npm ci
npm run embed:check        # codemod 已应用（退出码非 0 则构建失败）
npm run test:embed         # 可选
OWUI_BASE_PATH=/_chat NODE_OPTIONS=--max-old-space-size=8192 npm run build:embed
#   = npm run crypto:verify && vite build && node scripts/embed-postbuild.mjs
# 产物 build/（含 build/embed/artifact-frame.html、_app/immutable/inline/*.js）→ FRONTEND_BUILD_DIR
```

- 不跑 `pyodide:fetch`。
- 可以删除 `build/**/*.map`。

## 7. 约定变更提议

1. **B.4 `X-Sub2API-Name` 必须是 ASCII**：改为 percent-encoded 的 UTF-8。
   - 实测：中文用户名会让 uvicorn 的 WebSocket 握手抛 `UnicodeEncodeError`（`websockets_sansio_impl.handle_connect`），socket.io 连不上，流式回复不显示。
   - fork 后端中间件本来就按 percent-encoded 处理，HTTP 下显示名也正确。
   - 假反代已按此实现。
2. **A / D 换账号的 Cookie 清理**：建议两项都做。
   - 现象：fork 后端对 `/api/config` 也比对 token 用户与身份头，不一致时返回 401。上一个账号的 `token` Cookie（HttpOnly，`Path=/_chat`，前端清不掉）会让下一个账号进入 `/error`。
   - sub2api 侧：`DELETE /api/v1/chat/session` 同时下发 `Set-Cookie: token=; Path=/_chat; Max-Age=0`；父页发现 `s2a_chat_owner` 变化时，先 DELETE 再 POST。假父页已按此实现，换账号测试因此通过。
   - fork 后端侧：`/api/config` 在 token 与身份不一致时按匿名处理。
3. **E iframe src 可带 `&theme=light|dark`**：可选。fork 的 `app.html` 会读这个参数预置主题，避免首次 `chat-session` 到达前先闪一下系统主题。
4. **D socket 握手**（告知，不需要改约定）：嵌入后端拒绝不带 token 的握手，socket.io 被服务端拒绝后不会自动重连。前端已在登录后主动重连（`app.ts`，最多 5 次）。
5. **阶段 1 屏蔽表**：
   - `GET /api/v1/functions/`（精确路径）建议对普通用户放行。上游对已验证用户开放，聊天界面用它显示管理员安装的过滤器开关和动作按钮；现在被限为仅管理员，普通用户用不到这些插件的界面入口。
   - 如果要用「分组授权」共享预设助手，管理员需要 `GET /api/v1/groups/`。现在返回 404，工作空间里的访问控制选择器会报错。
6. **C**：现有字段够用，不需要改。前端由 `supports_chat=false` 自行派生「分组不支持对话」。

## 8. 提交拆分（给主会话）

两份补丁在 `~/workspace/owui-poc/fe/split/`：
- `codemod.patch`：在干净的 v0.12.3 上跑 codemod 的结果，286 个文件；
- `hooks.patch`：手写挂接，25 个文件。

已验证 `git apply --check --cached codemod.patch` 可以应用到 HEAD。只改暂存区、不碰工作区的提交方式：

```bash
cd ~/workspace/open-webui
git apply --cached ~/workspace/owui-poc/fe/split/codemod.patch
git add scripts/embed-codemod.mjs scripts/embed-postbuild.mjs package.json \
        src/lib/embed static/embed docs/sub2api-embed-frontend.md
git commit -m "embed: sub-path + storage namespace codemod, embed modules"   # 提交 1
git add src svelte.config.js
git commit -m "embed: upstream hooks (sub2api-embed)"                          # 提交 2
```

`backend/`、`docker/`、`.github/`、`docs/sub2api-embed.md`、`scripts/rebase-upstream.md` 是其他阶段的改动，不在上面两个提交里。

## 9. 跨阶段发现汇总

| 问题 | 归属 | 本阶段处理 |
|---|---|---|
| 中文 `X-Sub2API-Name` 导致 WebSocket 握手崩溃 | 阶段 4 反代 / 约定 B | 提议 1 |
| 换账号后旧 `token` Cookie 让 `/api/config` 401 | 阶段 1、4、5 | 提议 2 |
| 嵌入后端拒绝匿名 socket | 阶段 1 | 前端已兼容 |
| `functions` 列表被限为仅管理员 | 阶段 1 屏蔽表 | 前端容错；提议 5 |
| 默认模型头像 302 到 Open WebUI logo | — | 前端用 MutationObserver 统一替换（`avatar.ts`），不需要后端改动 |

## 补充（主会话在真实 sub2api-local 联调后的 4 项，2026-10-24）

### 1. 模型列表加载失败时的提示（contract D 新条目）

- **挂接**：`lib/apis/index.ts` 的 `getModels`。嵌入模式下失败时不再抛出，而是返回 `[]`，并把错误写入 `embedModelsError` store；下次加载成功时清空。非嵌入模式仍按上游抛出。
- **映射**：新增 `errors.ts` 的 `mapModelsError`，`errorText` 增加读取 `s2a_gateway_error`。
  - 能识别的网关错误沿用对话错误的映射：余额不足 → 「去充值」（`chat:navigate /purchase`）；Key 无效 / 未选 → 「去设置」，打开「模型与密钥」。
  - 识别不了的显示新的 kind `models`；`detail` 是中文时直接显示原文，否则显示「模型列表加载失败…」。
- **显示**：用同一个 `GatewayErrorHint`（`data-testid="s2a-gateway-error"`、`data-kind`），出现在两处：
  - `chat/MessageInput.svelte`：输入框上方；
  - `chat/ModelSelector/Selector.svelte`：下拉的空状态。嵌入模式下不再显示上游「No models available / Manage Connections」。
- **改动的文案**：`error.settings` 改为「去设置」；新增 `error.models`。
- **实测**（假网关的 `logs/models_fail.json` 钩子让 `/v1/models` 对指定 Key 失败；工作区后端已按 contract D 返回 `s2a_gateway_error`）：
  - 余额不足（403 `INSUFFICIENT_BALANCE`）：输入框上方与下拉里各 1 个提示，kind 为 `balance`；点「去充值」后父页收到 `chat:navigate /purchase`。截图 `r11-17-models-error-balance.png`。
  - Key 无效（401 `INVALID_API_KEY`）：kind 为 `key`；点「去设置」打开「模型与密钥」。截图 `r11-17-models-error-key.png`。
  - 通用 500：kind 为 `models`，显示后端的中文 `detail`「网关模型列表不可用」。截图 `r11-17-models-error-generic.png`。
  - 恢复正常后提示消失。
  - 注意：后端对成功的模型列表按 Key 缓存 60 秒，测试时要在首次加载前就让它失败。

### 2. pageerror 「Object」

- **原因**：模型选择器按钮的 `on:mouseenter`（`Selector.svelte`，另有 4 处同类调用）每次都 `await getModels()`，且没有 catch。`/api/models` 失败（余额为 0、未选 Key）时，`getModels` 会把响应体对象 `{detail…}` 直接 reject，于是在「打开设置 → 一键创建 → 关闭 → 把鼠标移到输入框 / 发消息」这条路径上变成未处理的 rejection，控制台只显示 `Object`。
- **修复**：第 1 项改为嵌入模式下返回 `[]` 后，所有调用点都不再 reject。
- **顺带修掉另一条 pageerror「Not Found」**：`MessageInput/IntegrationsMenu.svelte` 打开扩展菜单时，`getSkills` 拿到被屏蔽接口的 404，改为嵌入模式下当作空列表。
- **结果**：最终一轮 r11 的 pageerror 为 0（r8 时还有 1 条 Not Found）。

### 3. 隐藏品牌时去掉「对话 · v0.12.3」

这一行出在 `chat/Suggestions.svelte`（没有建议时的占位，带 LICENSE 注释）。`hide_branding` 时整行不渲染。实测首页正文里不再有 `vX.Y.Z`（`versionLine: false`）。

### 4. 一键创建的默认分组

- `keys.ts` 的 `sortGroupsForCreate` 改为 `defaultCreateGroup`：先选 `supports_images !== true` 的 anthropic 分组，其次 openai，再其次第一个。
- 下拉列表保持后端返回的顺序，仍可改选。
- vitest 已覆盖。实测新用户默认选中「在分组「Claude」下创建」，创建出的 Key 属于 Claude 分组。

### 复验

| 项 | 结果 |
|---|---|
| `vite build` + postbuild | 通过（3 段内联脚本外置，剩余 0） |
| svelte-check | 5696 errors / 188 warnings，与基线相同，新增 0 条（`logs/check-fork3.txt`）。检查之后只改了输入框上方提示容器的一个 class 字符串 |
| vitest | 10 个文件 62 条全过；新增 `models-error.test.ts`，`keys.test.ts` 改为测默认分组 |
| codemod `--check` | 无变化。干净 v0.12.3 上重跑仍是 286 个文件，与工作区的 codemod 部分一致 |
| Playwright r11 | 18 步全部通过（新增 `16-models-error`）；越出 `/_chat` 的请求 0；主文档 CSP 违规 0；pageerror 0。汇总 `~/workspace/owui-poc/fe/logs/flow-r11.json`，截图 `shots/r11-*.png` |

### 提交拆分更新

`~/workspace/owui-poc/fe/split/` 的两份补丁已重新生成：`codemod.patch` 仍是 286 个文件，`hooks.patch` 变为 28 个文件。新增的 3 个挂接文件是 `lib/apis/index.ts`、`chat/ModelSelector/Selector.svelte`、`chat/Suggestions.svelte`。28 个文件都有 `sub2api-embed` 标记。`git apply --check --cached codemod.patch` 仍然通过，提交命令同 §8。
