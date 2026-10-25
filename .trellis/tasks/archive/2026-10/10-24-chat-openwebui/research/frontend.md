# 调研：Open WebUI v0.12.3 前端主题、品牌、布局、存储、postMessage（2026-10-24）

源码：`~/workspace/open-webui`（tag v0.12.3）。

## 1. 主题

- Tailwind v4（`package.json:53`），入口 `src/tailwind.css`（`@config` 引入 `tailwind.config.js`，`darkMode: 'class'`）。
- **灰阶是 CSS 变量**：`src/tailwind.css:21-32` `@theme { --color-gray-50 … --color-gray-950 }`（含 `gray-850`），可在 `:root` / `html.dark` 集中覆盖；默认边框色 `tailwind.css:49` `var(--color-gray-200)`。
- 浅 / 深色共用同一套灰阶（如 `bg-white dark:bg-gray-900`）→ 深色改 800–950，浅色改 50–300。
- 硬编码统计（718 个 .svelte）：`bg-gray-` 2049 / `text-gray-` 5664 / `border-gray-` 657 / `bg-white` 473 / `bg-black` 210 / `gray-850` 383 / 6 位 hex 228（34 文件）/ 强调色 blue/indigo 等约 270。几乎全走 gray 变量。
- 主按钮多为黑白反转（`bg-black … dark:bg-white` 约 25 处）→ 换主色要改这些类；复选框 / 焦点环 `blue-500/600`（`tailwind.css:79,86`、`app.css:442`）→ 覆盖 `--color-blue-500/600`。
- 写死的颜色：`app.css:782-790` body 背景；`app.html:35,59,75` `meta theme-color`；`app.html:211-221` 启动画面。
- 深色模式：
  - `app.html:47-104` 内联脚本读 `localStorage.theme`（`system|light|dark|oled-dark|her`）给 `<html>` 加 class；`oled-dark` 用内联 style 改 gray-800..950。
  - store `stores/index.ts:37`；`+layout.svelte:1272`。
  - `Settings/General.svelte:148-216` `applyTheme`：**深色时用 `style.setProperty` 写 gray-800..950（`:155-160`）**，内联优先级高于样式表 → 要改这几行或加 `!important`；可选全局钩子 `window.applyTheme()`。
  - 现成外部切换：`+layout.svelte:1065-1083` `desktopEventHandler` 的 `theme:update`（只经 electronAPI 注册，`:1221-1223`）。

## 2. 品牌

- `backend/open_webui/env.py:957-959`：`WEBUI_NAME` 非默认时强制追加 ` (Open WebUI)`。经 `/api/config` 下发（`main.py:2392`），manifest / opensearch 也用。
- 前端 `constants.ts:7` `APP_NAME`；`+layout.svelte:1353` 设置 store；31 个文件使用；硬编码 "Open WebUI"：`app.html:128` title、通知标题 `+layout.svelte:645,799,939`；非 i18n 文件共 76 个含该字样。
- 图标：`static/static/` 下 favicon / logo / splash / manifest 图标，启动时从 `STATIC_SOURCE_DIR` 复制到 `STATIC_DIR`（`config.py:117-143`）；遗留 `CUSTOM_NAME` 会从 api.openwebui.com 拉 logo（`config.py:195-226`，必须关闭）。
- About：`Settings/About.svelte`（版本 `:64`、社区链接 `:139-149`、版权与 LICENSE `:164-177`）。
- 各品牌位置有 "LICENSE covers this … Do not alter" 注释，`static/static/BRANDING.md`。代码注释把范围解释为包括 "surrounding UI / metadata"，比 LICENSE 原文更宽。
- LICENSE 第 4 条原文见 `LICENSE:20-34`；第 1 条要求保留版权声明。**我们的处理**：只在满足例外 (i)（30 天内 ≤50 人）时启用品牌替换，做成部署开关；版权声明与 LICENSE 链接保留在不显眼处（如设置页底部小字），不删除。

## 3. 布局

- 根布局 `src/routes/+layout.svelte`（1535 行）：socket、i18n、config、session、Toaster。
- 应用布局 `src/routes/(app)/+layout.svelte`：`SettingsModal`、`ChangelogModal`（`:467-468`）、更新 toast（`:470-480`）、`<Sidebar/>`（`:490`）。
- `layout/Sidebar.svelte`（1788 行）：`isMenuItemVisible`（`:158-190`，notes / workspace / automations / calendar / playground）；logo 与名称 `:1155-1181`；频道 `:1338`；文件夹 `:1371`；`UserMenu` 两处 `:1086`、`:1726`。
- `layout/Sidebar/UserMenu.svelte`：Workspace `:244`、Notes `:290`、Calendar `:336`、Automations `:382`、Playground `:428`、文档链接 `:479-510`、Admin Panel `:537-558`、Settings `:568`、Sign Out `:586-596`。
- `chat/Navbar.svelte`：模型选择、分享、临时聊天、控制面板、横幅（`:283-330`），无账号菜单。
- `chat/SettingsModal.svelte:189-268` 标签页（`account :257`、`about :263`），过滤 `getAvailableSettings`（`:388`）。
- 管理路由守卫 `(app)/admin/+layout.svelte:16`。
- 嵌入开关建议：`stores` 加 `embedMode`；`UserMenu` 加 `!$embedMode`；`isMenuItemVisible` 白名单；`getAvailableSettings` 去掉 account / about；Changelog / 更新 toast 加 `!$embedMode`。保留 Open WebUI 自己的会话侧栏（新建 / 历史 / 文件夹），这是 ChatGPT 体验的核心。

## 4. 加载时弹窗

| 来源 | 条件 | 抑制 |
|---|---|---|
| Changelog | 仅 admin，`showChangelog` 且版本变化（`(app)/+layout.svelte:392-394`） | embed 判断 |
| 版本更新 toast | admin 且 `enable_version_update_check`（`:406-423`） | `ENABLE_VERSION_UPDATE_CHECK=false` / `OFFLINE_MODE=true`（`env.py:1212-1217`） |
| Banners | 管理员配置（`Navbar.svelte:283-330`） | 不配置 |
| Onboarding | 仅 `/auth` 页且首个管理员未创建 | 预建管理员 / embed 不进 `/auth` |
| 断线 / 会话过期 toast | `+layout.svelte:175,207,1014` | 视需要 |

## 5. i18n

- zh-CN 4078 key（en-US 4084），387 个空值 → 实际覆盖约 91%，新功能部分显示英文。
- 检测顺序 `?lang=` → `localStorage.locale` → `DEFAULT_LOCALE`（`config.py:1700`）→ navigator（`i18n/index.ts:90-119`）；运行时 `changeLanguage`（`:127`）；管理员可经 `ui.i18n` 覆盖文案（`config.py:3184`）。

## 6. 存储（多账号隔离）

- localStorage（按 origin，不分用户）：`token`（约 900 处引用）、`theme`、`locale`、`sidebar`、`sidebarWidth`、`showControls`、`redirectPath`、`dismissedUpdateToast`、`dismissedBannerIds`、`version`、各种视图状态。
- sessionStorage：聊天草稿 `chat-input` / `chat-input-<chatId>`（`Chat.svelte:4797,4804`）、OAuth 工具状态。
- 用户设置在服务端（`getUserSettings`）。未发现 IndexedDB（静态 grep）。
- Cookie `token`（httponly，`routers/auths.py:208-214`）。
- 与 sub2api 同源时 `theme`、`token` 等 key 冲突 → fork 内统一加命名空间（见 `paths.md`）；切换账号时清理 token、草稿、`dismissedBannerIds`、`redirectPath` 后 reload。

## 7. postMessage

- 现有监听：`+layout.svelte:1153-1161`（社区来源 `export:stats`）；`Chat.svelte:1800-1880`（`input:prompt` / `input:prompt:submit` / `action:submit`，只信同源或自身 iframe）。前端没有向 `window.parent` 发消息的代码（pyodide 沙箱除外）。
- 新增父页桥位置：`+layout.svelte` onMount，`windowMessageEventHandler` 旁（`:1161`）。校验 `event.origin === location.origin` 且 `event.source === window.parent`；消息：theme（复用 `theme:update`）、locale、session、revoke；加载完成（`:1427` 附近）发 ready。
- iframe 头：`utils/security_headers.py:42-77` 可选配置，默认不设（未确认）；同源部署无需 SameSite=None。
