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
├── types/          # 跨模块 TypeScript 合约
├── utils/          # 纯函数与无 UI 技术辅助函数
└── views/          # 路由级页面，按 admin/auth/public/setup/user 划分
```

`src/main.ts` 负责 Pinia、公开配置、i18n 和 router 的启动顺序；`App.vue` 是根壳。`vite.config.ts` 将构建结果写入后端的 `internal/web/dist`（该目录只在构建后出现），不要手改其中资源。

## 放置规则

- 路由直接加载的页面放 `views/<area>/...View.vue`，并在 `router/index.ts` 使用动态 import。权限、标题和功能门控放 route meta/guard，不只靠页面隐藏。
- 跨多个页面复用的视觉/交互组件放 `components/common/`；全局框架放 `components/layout/`；只属于一个业务域的组件放对应子目录。
- 可复用的有状态逻辑放 `composables/`，文件名遵循 `use<Name>.ts`；纯格式化、分类、sanitize 和 key 构造放 `utils/`。
- 所有 HTTP 请求经 `api/client.ts` 创建的 `apiClient`；端点按业务域放 `api/*.ts` 或 `api/admin/*.ts`。组件/页面不直接创建另一个 Axios 实例。
- 跨模块共享的 API/领域类型放 `types/index.ts` 或专题文件（如 `types/payment.ts`）；只在一个组件/API 模块使用的类型留在其附近。
- Pinia 只承载跨页面、跨组件生命周期或需统一缓存/清理的状态；通过 `stores/index.ts` 暴露稳定入口。
- 单元测试放相邻 `__tests__/` 目录并命名 `*.spec.ts`；跨模块用户流程放 `src/__tests__/integration/`。

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
