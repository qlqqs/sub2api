# 前端目录结构

本文描述 `frontend/` 当前的 Vue 3 前端边界。判断文件放置位置时，以真实源码、路由配置和邻近测试为准，不以目录中的历史 README 或示例文档为准。

## 运行入口与工程边界

- `frontend/src/main.ts` 是唯一浏览器入口：先设置主题类，再创建 Vue 与 Pinia，读取注入配置，初始化 i18n 和 Router，等待 `router.isReady()` 后挂载。不要在任意页面重复这些全局初始化步骤。
- `frontend/src/App.vue` 是根级横切层，集中放置 `RouterView`、全局 Toast、导航进度、公告和管理员合规弹窗，并管理文档标题、favicon 与根级监听器。
- `frontend/src/router/index.ts` 定义路由、守卫和懒加载页面。路由页面使用 `() => import('@/views/...View.vue')`；新增页面后在这里注册，不要从普通组件自行维护另一套路由表。
- `frontend/vite.config.ts` 将 `@` 映射到 `frontend/src`，开发时代理 API，生产构建输出到 `backend/internal/web/dist`。前端仍是独立源码包，不应直接导入后端源码。
- `frontend/tsconfig.json` 同样声明 `@/* -> ./src/*`，并启用严格 TypeScript、未使用符号检查及 bundler 模块解析。跨目录导入优先使用 `@/`，同一小模块内的兄弟文件可使用相对路径。

## 核心目录

```text
frontend/
├── src/
│   ├── __tests__/             # 全局测试 setup 与跨模块 integration 测试
│   ├── api/                   # HTTP/API 边界；admin/ 保存管理员端 API
│   ├── components/
│   │   ├── common/            # 与业务域无关的基础组件和公共类型
│   │   ├── layout/            # AppLayout、AuthLayout、TablePageLayout 等外壳
│   │   ├── icons/             # 统一 Icon 组件与图标导出
│   │   ├── admin/             # 管理员领域组件，内部继续按 account、usage 等拆分
│   │   ├── account/           # 账号/上游账号领域组件
│   │   ├── auth/              # 登录、OAuth、TOTP 等认证组件
│   │   ├── charts/            # 图表组件
│   │   ├── channels/          # 渠道领域组件
│   │   ├── keys/              # API Key 领域组件
│   │   ├── payment/           # 支付与订阅展示组件及同域辅助模块
│   │   └── user/              # 用户侧组件，内部可按 profile、monitor 等拆分
│   ├── composables/           # 可复用组合式逻辑及其 __tests__
│   ├── constants/             # 跨组件使用的稳定常量
│   ├── i18n/                  # i18n 初始化、英文/中文 locale 与校验测试
│   ├── router/                # 路由、标题解析、setup 重定向及测试
│   ├── stores/                # Pinia 跨页面状态及其 __tests__
│   ├── styles/                # 非全局入口样式，例如 onboarding.css
│   ├── types/                 # 跨领域共享的 TypeScript 类型与全局声明
│   ├── utils/                 # 无 UI 的共享工具及其 __tests__
│   ├── views/                 # 路由级页面
│   │   ├── admin/             # 管理员页面；ops、orders、settings 可继续分区
│   │   ├── auth/              # 登录、注册及各 OAuth 回调页面
│   │   ├── public/            # 无需认证的公共页面
│   │   ├── setup/             # 首次安装向导
│   │   └── user/              # 普通用户页面
│   ├── App.vue                # 根组件
│   ├── main.ts                # 浏览器启动入口
│   └── style.css              # Tailwind 入口和全局组件类
├── package.json               # Vite、vue-tsc、ESLint、Vitest 命令
├── tailwind.config.js         # 主题、dark class、动画和设计 token
├── tsconfig.json              # 应用 TypeScript 配置与 @ alias
├── vite.config.ts             # 开发/构建配置与 @ alias
└── vitest.config.ts           # jsdom、setup、测试匹配与 coverage 配置
```

## 文件放置规则

### 路由页面与页面辅助代码

- 能被 Router 直接导航的容器放在 `src/views/`，并以 `View.vue` 结尾。`src/views/user/ProfileView.vue` 展示了页面职责：读取 store/API 状态、组合 `AppLayout` 和 profile 领域组件，而不是把所有展示细节写在页面里。
- 管理员、用户、认证、公共和 setup 页面分别进入现有分区。更窄的子域沿用现有嵌套，例如 `src/views/admin/ops/`、`src/views/admin/orders/`、`src/views/admin/settings/`。
- 只服务某组页面的纯 TS 逻辑就近放置，例如 `src/views/user/paymentUx.ts`、`src/views/admin/groupsImagePricing.ts`、`src/views/admin/ops/utils/opsFormatters.ts`。不要仅因代码不是 `.vue` 就移入全局 `utils/`。

### 组件

- 跨业务域复用且不认识领域实体的组件放在 `src/components/common/`。代表文件为 `Input.vue`、`DataTable.vue`、`BaseDialog.vue`；公共导出集中在 `src/components/common/index.ts`，但现有源码也大量直接导入具体文件，新增代码应与邻近模块保持一致。
- 页面布局外壳放在 `src/components/layout/`。`AppLayout.vue` 提供应用侧边栏、页头和默认 slot，`TablePageLayout.vue` 提供表格页的布局槽位。
- 认识业务实体或业务动作的组件必须进入相应领域目录。示例：`src/components/admin/usage/UsageTable.vue`、`src/components/user/profile/ProfileInfoCard.vue`、`src/components/payment/PaymentStatusPanel.vue`。
- 一个领域继续增长时，按可识别子功能建立子目录并就近测试；不要把管理员账号、用户 profile 或支付逻辑倒入 `common/`。

### Composable、store、API、工具和类型

- 可复用的响应式/生命周期逻辑放在 `src/composables/use*.ts`，正式名称为 **Composable**。例如 `useTableLoader.ts`、`useAutoRefresh.ts`、`useRoutePrefetch.ts`、`useClipboard.ts`。
- 跨页面、跨路由需要长期保留的状态放 Pinia `src/stores/`；一次组件树内的状态留在组件或 Composable 中。不要把每个 `ref` 都升级成 store。
- 网络端点、请求参数和响应适配放 `src/api/`。页面和 Composable 调用 API 边界，不在模板事件里直接拼 axios/fetch 请求。
- 无响应式状态、无组件生命周期、可纯函数测试的逻辑放 `src/utils/`；业务局部辅助函数优先留在业务目录。
- 只在一个 SFC 使用的 props/局部数据类型写在该 SFC，例如 `Input.vue` 的 `Props`、`BaseDialog.vue` 的 `DialogWidth`。同一小模块共享的类型放兄弟 `types.ts`，例如 `src/components/common/types.ts` 的 `Column`；跨域类型才进入 `src/types/` 或 API 模块导出的类型。

## 测试放置

- 单元和组件测试使用最近的 `__tests__/`，文件名为 `*.spec.ts`。例如：
  - `src/components/common/__tests__/DataTable.spec.ts`
  - `src/components/user/profile/__tests__/ProfileInfoCard.spec.ts`
  - `src/composables/__tests__/useTableLoader.spec.ts`
  - `src/views/user/__tests__/ProfileView.spec.ts`
  - `src/router/__tests__/title.spec.ts`
- 跨路由/跨模块行为放 `src/__tests__/integration/`；全局 jsdom polyfill 和 Vue Test Utils 配置放 `src/__tests__/setup.ts`。
- 测试应跟随被测模块移动。不要新建一个远离实现的顶层 `tests/` 镜像目录。
- `tsconfig.json` 排除了 spec 文件，而 `vitest.config.ts` 使用自身的 Vite/Vue 配置运行 `src/**/*.{test,spec}.*`；测试导入同样可使用 `@/`。

## 命名约定

- Vue SFC 使用 PascalCase：`ProfileInfoCard.vue`、`PaymentQRDialog.vue`。
- 路由页面使用 `*View.vue`；可复用组合式函数使用 `useXxx.ts` 并导出同名 `useXxx`；Pinia 文件使用领域名，例如 `auth.ts`、`subscriptions.ts`。
- 测试名与被测对象对应并以 `.spec.ts` 结尾；特定回归场景可在主名与 `.spec.ts` 之间增加限定词，如 `AccountsView.bulkEdit.spec.ts`。
- 业务辅助 TS 文件使用能表达规则的 camelCase 名称，不使用 `helpers.ts`、`misc.ts` 这类含义过宽的名字。

## 真实例外与应避免的结构

- `src/views/admin/AccountsView.vue`、`UsersView.vue` 和 `src/views/user/KeysView.vue` 等历史页面仍然较大，并包含较多本地状态和处理函数。这是现状，不是新功能继续堆入巨型页面的依据；新增独立 UI 或可测试规则时优先提取到领域组件、局部 TS 模块或 Composable。
- `src/views/auth/README.md`、`VISUAL_GUIDE.md`、`USAGE_EXAMPLES.md` 以及部分组件目录 README 可能落后于源码。规范判断顺序是运行代码、邻近测试、构建配置，最后才是这些说明文档。
- `src/components/common/types.ts` 的 `Column` 仍使用 `any` 表达通用表格行，`DataTable.vue` 也保留了相同历史类型。不要把这种宽类型扩散到新的领域接口；在调用端使用具体 API 类型，并在可控边界增加泛型或收窄。
- 不要使用深层相对路径跨越多个领域（如 `../../../components/...`），不要在 `common/` 放置业务 API 调用，也不要让普通组件承担 `main.ts`/`App.vue` 的全局启动职责。
