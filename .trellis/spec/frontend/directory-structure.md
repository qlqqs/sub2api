# 前端目录结构

## 目录职责

```text
frontend/src/
├── api/            # Axios 客户端和按领域组织的端点函数
│   └── admin/      # 管理端 API
├── assets/         # 打包期静态资源
├── components/     # 可复用 UI 与领域组件
│   ├── common/     # 跨页面基础组件
│   ├── layout/     # 应用与页面布局
│   ├── admin/      # 管理端领域组件
│   ├── account/    # 账号管理组件
│   └── payment/    # 支付流程组件
├── composables/    # 可复用的有状态组合式逻辑
├── constants/      # 领域常量
├── i18n/           # 初始化与中英文 locale 模块
├── router/         # 路由、meta、守卫和标题
├── stores/         # Pinia 全局/跨页面状态
├── styles/         # 补充样式；全局入口为 style.css
├── themes/         # CUSTOM: fork 自有的页面主题（如 themes/qx），替换上游首页/登录外壳
├── types/          # 跨模块 TypeScript 合约
├── utils/          # 纯函数与无 UI 技术辅助函数
└── views/          # 路由级页面，按 admin/auth/public/setup/user 划分
```

`src/main.ts` 负责 Pinia、公开配置、i18n 和 router 的启动顺序；`App.vue` 是根壳。`vite.config.ts` 将构建结果写入后端的 `internal/web/dist`（该目录只在构建后出现），不要手改其中资源。

## 放置规则

- 路由直接加载的页面放 `views/<area>/...View.vue`，并在 `router/index.ts` 使用动态 import。权限、标题和功能门控放 route meta/guard，不只靠页面隐藏。
- 下钻页的路径放在对应菜单项的路径之下（如 `/admin/affiliates/team/:viewerId`）：侧边栏分组按「最长路径前缀」决定高亮哪个子项并保持分组展开（`AppSidebar.vue` 的 `activeChildPath`），不需要为下钻页单独配置菜单状态。
- 跨多个页面复用的视觉/交互组件放 `components/common/`；全局框架放 `components/layout/`；只属于一个业务域的组件放对应子目录。
- 可复用的有状态逻辑放 `composables/`，文件名遵循 `use<Name>.ts`；纯格式化、分类、sanitize 和 key 构造放 `utils/`。
- 所有 HTTP 请求经 `api/client.ts` 创建的 `apiClient`；端点按业务域放 `api/*.ts` 或 `api/admin/*.ts`。组件/页面不直接创建另一个 Axios 实例。
- 跨模块共享的 API/领域类型放 `types/index.ts` 或专题文件（如 `types/payment.ts`）；只在一个组件/API 模块使用的类型留在其附近。
- Pinia 只承载跨页面、跨组件生命周期或需统一缓存/清理的状态；通过 `stores/index.ts` 暴露稳定入口。
- 单元测试放相邻 `__tests__/` 目录并命名 `*.spec.ts`；跨模块用户流程放 `src/__tests__/integration/`。

## Fork 主题（themes/）

替换上游页面外观、又要持续合并上游时，用独立主题而不是改上游组件（参考 `themes/qx/`，即 QxAPI 首页与登录外壳）：

- 主题代码、样式、文案、测试全部放在 `themes/<name>/`；上游文件只改接入点，并加 `CUSTOM` 注释：路由组件、`components/layout/index.ts` 的导出、`tailwind.config.js` 的 primary 变量、`i18n/locales/{zh,en}/index.ts` 的引入（各一两行）。
- 主题入口组件在主题关闭时渲染上游原版组件（开关在 `themes/<name>/config.ts`）。替换上游组件时用 `defineOptions({ name })` 沿用原组件名，按名字 stub 的上游测试无需改动。
- 样式用主题前缀类并限定在根类（如 `.qx`）下；需要让插槽里的上游组件跟随主题色时，在根类上覆盖 `--color-primary-*`（primary 已改为带原青色默认值的 CSS 变量）。
- 主题里的价格、模型数据只取现有公开接口（如模型广场），口径与原页面一致，不另写计价规则；与原组件口径对齐的逻辑写单元测试锁住。
- 全站换肤（控制台、管理后台）走色阶变量，不逐页改模板：`tailwind.config.js` 的 `primary` / `gray` / `dark` / `accent` 色阶和 `glow`、`gradient-primary`、`mesh-gradient` 都读 CSS 变量，默认值即官方原色；主题开启时 `main.ts` 调 `themes/qx/app/install.ts` 给 `<html>` 加 `qx-app`，`app/tokens.css` 覆盖这些变量，`app/app.css` 只改 `style.css` 公共类和布局组件的形状。所有规则限定在 `html.qx-app` 下，关闭主题时与官方原版一致。
- 控制台外壳（侧栏 + 顶栏）：上游 `components/layout/AppLayout.vue` 被各页面直接引用，接入点是它模板里的 `<QxShell v-if>` / `v-else`（CUSTOM），脚本里的新手引导保持不变。`themes/qx/shell/QxSidebar.vue` 自绘分组侧栏，顶栏复用上游 `AppHeader.vue` 只改样式。菜单数据 `shell/useQxNav.ts` 照搬自 `AppSidebar.vue`（功能开关、简洁模式、自定义菜单、引导选择器），`useQxNav.drift.spec.ts` 会检查上游侧栏里的每个路径在主题菜单里都有：合并上游后它失败，就把新菜单项补进 `useQxNav.ts`。
- 需要重做结构的页面（如仪表盘 `themes/qx/dashboard/`）和首页一样在路由里接入主题入口组件，主题关闭时渲染上游原页面。
- 只换样式、不改结构的用户页面（极简线条风格）写在 `themes/qx/pages/`：`QxShell` 在非 `/admin` 路由给 `<html>` 加 `qx-line` 类和 `data-qx-page=<路由名>`（放在 `<html>` 上，传送到 body 的弹窗也能匹配）。`line.css` 是公共类和组件层，限定在 `html.qx-app.qx-line` 下；`pages-console.css` / `pages-account.css` 按 `[data-qx-page="…"]` 覆盖各页面模板里直接写的 Tailwind 阴影、渐变、底色块。合并上游后这些选择器可能失配，失配时页面退回主题原样而不会出错，截图检查即可。
- 嵌入的外部应用（如创作工作台 `themes/qx/studio/`）只放一个同源 iframe，父页经 `postMessage` 传会话（令牌、用户、角色、深浅色），并校验消息的 origin 与 source 窗口；主题跟随用 `composables/documentDark` 的响应式 `isDocumentDark()`，变化时重发会话。iframe 内的配色在对方仓库按控制台 token 覆盖（极简线条、淡色），不在本仓库写样式穿透。
- 合并上游后检查被替换组件（如 `views/HomeView.vue`、`views/user/DashboardView.vue`、`components/layout/AuthLayout.vue`、`AppSidebar.vue`、`AppHeader.vue`）的新改动，需要的话手动搬进主题；同时检查 `tailwind.config.js`、`style.css` 有没有新加写死的颜色（新色阶、新渐变或阴影），需要的话同样改成变量并在 `app/tokens.css` 里给主题值。

## 命名和导入

- Vue 组件和 view 使用 PascalCase 文件名：`BaseDialog.vue`、`UsersView.vue`。
- composable 以 `use` 开头：`useTableLoader.ts`；store 导出 `use<Name>Store`。
- 普通 TS 文件使用当前目录既有风格，主要为 camelCase；测试沿被测模块命名。
- 跨目录导入使用 `@/` alias；同目录内可以使用相对路径。仅类型导入使用 `import type`。
- 常用组件通过局部 `index.ts` barrel 暴露，例如 `components/common/index.ts` 和 `stores/index.ts`；不要创建跨全仓库的无边界 barrel。

## 代表模块

- `views/admin/UsersView.vue` + `components/admin/user/` + `api/admin/users.ts` 展示管理端页面分解。
- `components/common/DataTable.vue` + `composables/useTableLoader.ts` 展示通用表格、分页、请求取消和测试边界。
- `components/payment/` + `stores/payment.ts` + `api/payment.ts` + `types/payment.ts` 展示完整领域模块。
- `router/index.ts` + `router/meta.d.ts` + `router/__tests__/` 展示路由权限和 typed meta。

## 避免

- 不把大段可复用 UI 或请求逻辑长期留在 route view。
- 不在组件中散落硬编码 API base、认证 header、时区或刷新逻辑。
- 不把纯页面临时状态提升成全局 store，也不在 util 中隐藏响应式副作用。
- 不手改 `dist/`、Vite 产物或依赖目录。
