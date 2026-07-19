# 类型安全规范

## 编译边界

前端使用 TypeScript 5.6 与 Vue SFC 类型检查。`frontend/tsconfig.json` 的关键约束是：

- `strict: true`、`isolatedModules: true`、`noEmit: true`；
- `noUnusedLocals`、`noUnusedParameters`、`noFallthroughCasesInSwitch` 均开启；
- `moduleResolution: "bundler"`，`@/*` 映射到 `src/*`；
- `skipLibCheck: true`，因此第三方声明文件不会做完整检查；
- 全局类型只显式加载 `vite/client`。

生产源文件由 `include: ["src/**/*.ts", "src/**/*.tsx", "src/**/*.vue"]` 覆盖，但
`src/**/__tests__/**`、`*.spec.ts`、`*.test.ts` 被明确排除。也就是说：

> `pnpm --dir frontend run typecheck` 成功，不代表测试文件已通过 TypeScript 类型检查。

Vitest 通过 `frontend/vitest.config.ts` 编译并运行测试，且 `globals: true`；主
`tsconfig.json` 不会检查这些测试。`frontend/tsconfig.node.json` 只 include
`vite.config.ts`，也不覆盖 `vitest.config.ts`。修改测试时必须实际运行对应 Vitest，不可只依赖 typecheck。

## 类型放置

按复用范围放置类型，不把所有接口堆进单一文件：

- `frontend/src/types/index.ts`：跨领域基础类型、用户/认证、通用分页、`ApiResponse<T>`、
  toast 等广泛共享契约。
- `frontend/src/types/payment.ts`：支付领域的大型共享模型，如 `PaymentConfig`、
  `PaymentOrder`、`SubscriptionPlan`。
- `frontend/src/types/global.d.ts`：真正的全局扩展，目前只声明
  `window.__APP_CONFIG__?: PublicSettings`。
- API 文件旁：仅该接口族使用的 request/response 类型。例如
  `frontend/src/api/admin/users.ts` 的管理员身份绑定类型、
  `frontend/src/api/admin/system.ts` 的 `VersionInfo` / `UpdateResult`。
- 功能目录旁：只服务局部页面或组件的类型。例如
  `frontend/src/views/admin/ops/types.ts`、`frontend/src/components/common/types.ts`。
- 框架声明增强放在框架目录，例如 `frontend/src/router/meta.d.ts` 对
  `vue-router` 的 `RouteMeta` module augmentation。

新增类型时先选择最窄的合理位置；只有多个领域都稳定依赖时才提升到 `src/types/index.ts`。
优先 `import type`，现有 store、API 与 router 均以此避免把纯类型带入运行时代码。

## 泛型与 API 契约

- 通用容器保留泛型，例如 `ApiResponse<T = unknown>`、`PaginatedResponse<T>`、
  `BasePaginationResponse<T>`；通用加载逻辑也使用泛型，参考
  `frontend/src/composables/useTableLoader.ts` 与 `utils/usageLoadQueue.ts`。
- 调用 Axios 时提供实际响应类型，如
  `apiClient.get<PaginatedResponse<AdminUser>>()`。`api/client.ts` 已把
  `{ code, message, data }` 解包到 Axios `response.data`，不要再把普通 API 调用声明成
  `ApiResponse<T>` 并重复解包。
- 请求 payload 不要使用无约束 object；为稳定字段定义 interface/type。动态键才使用
  `Record<string, unknown>` 或更窄的值类型，参考 `AdminBindAuthIdentityRequest.metadata`。
- 字面量状态使用 union，而非任意字符串；`types/payment.ts` 的 `OrderStatus`、
  `PaymentType`、`OrderType` 与 `types/index.ts` 的用户角色/状态是现有模式。

## `unknown`、类型缩窄与断言

运行时边界优先从 `unknown` 开始：

- `ApiResponse<T>` 默认参数是 `unknown`；`api/client.ts` 的响应拦截器先检查对象和
  `code` 字段。
- `frontend/src/utils/apiError.ts` 的 `extractApiErrorMessage(err: unknown)` 先检查
  `typeof err === 'object'`，再缩窄到内部 `ApiErrorLike`。
- `frontend/src/api/url.ts` 的 `normalizeAPIBaseURL(value: unknown)` 先规范化环境变量。
- `stores/auth.ts` 中持久化 JSON 的逐字段检查特指
  `getPersistedPendingAuthSession()` 对 `pending_auth_session` 的恢复逻辑。相比之下，
  `checkAuth()` 当前把 `auth_user` 的 `JSON.parse(savedUser)` 结果直接赋给 `user.value`；
  这是现存技术债，不是新代码可照搬的推荐范例。

类型断言用于已经完成运行时检查、框架类型无法表达或兼容后端旧字段的边界。断言应紧贴检查，
并尽量断言到 `Record<string, unknown>` 或小型局部接口。不要用连续 `as unknown as X` 掩盖普通赋值错误；若后端可能返回多个字段形状，应像
`views/admin/orders/AdminOrdersView.vue` 那样把兼容处理集中在边界并尽快转成内部类型。

## `any` 的现实策略

本项目不是“零 any”代码库。`frontend/.eslintrc.cjs` 明确关闭
`@typescript-eslint/no-explicit-any`，现有代码在以下场景使用它：

- 动态 query/params 构造，如 `api/admin/users.ts` 的 `Record<string, any>`；
- 高度动态的表格加载参数，如 `composables/useTableLoader.ts`；
- Vue Router/Vitest mock 或测试参数；
- 部分大型旧页面和错误处理分支。

因此不要写与现状不符的“禁止 any”规则；但新代码仍应优先使用 `unknown` + 缩窄、明确 union、
泛型或具体 interface。只有值确实需要任意属性读写、且缩窄不会增加真实安全性时才使用
`any`，并把范围限制在 API 适配或测试夹具等边界。共享业务模型不应以 `any` 逃避建模。

## 运行时校验边界

当前 `frontend/package.json` 没有 Zod、Yup、Valibot、AJV 等 schema 验证库；TypeScript
interface 不提供运行时校验。现有做法是按风险手工检查与规范化：

- API client 验证响应是对象并检查 `code`；HTTP error data 非对象时回退为空对象。
- `stores/auth.ts` 的 `getPersistedPendingAuthSession()` 对 `pending_auth_session` 逐字段检查，
  并在解析失败或 provider 无效时清除该项。
- 同一文件的 `checkAuth()` 对 `auth_user` 只做 `try/catch`，仍会将
  `JSON.parse(savedUser)` 结果直接赋给 `user.value`；应把它视为待收紧的历史边界，
  而不是持久化 JSON 校验范例。
- `composables/usePersistedPageSize.ts` 使用 `Number.isFinite` 与
  `normalizeTablePageSize()` 校验持久化数值。
- URL query 在页面中检查字符串/数组/空值后再使用。

新增外部输入时沿用局部 guard/normalize 函数；不要仅靠 `as SomeType` 声称已验证，也不要在
未引入依赖和迁移方案的情况下假设项目已有 schema 层。
