# 状态管理规范

## 总体模型

前端使用 Vue 3 Composition API 与 Pinia。除只负责集中导出的
`frontend/src/stores/index.ts` 外，`frontend/src/stores/` 下每个包含 `defineStore` 的
具体模块均采用 `defineStore(name, () => { ... })` 的 setup store 形式；状态用 `ref`，
派生值用 `computed`，副作用封装在具名 action 中。代表实现是
`frontend/src/stores/auth.ts` 的 `useAuthStore`、`frontend/src/stores/app.ts` 的
`useAppStore`，以及领域 store `useSubscriptionStore`、`usePaymentStore`。

状态按生命周期与可共享范围放置：

| 状态 | 放置位置 | 真实示例 |
|---|---|---|
| 跨页面身份、全局 UI、公开设置 | Pinia store | `stores/auth.ts`、`stores/app.ts` |
| 跨页面领域缓存或流程状态 | 领域 Pinia store | `stores/subscriptions.ts`、`stores/payment.ts`、`stores/announcements.ts` |
| 单页表单、弹窗、加载和校验 | 组件内 `ref` / `reactive` | `views/auth/LoginView.vue` 的 `formData`、`errors`；`views/user/PaymentView.vue` 的结账流程状态 |
| 纯派生显示值 | `computed`，不要重复存储 | `stores/auth.ts` 的 `isAuthenticated` / `isAdmin`；`stores/app.ts` 的 `backendModeEnabled` |
| 可分享、可恢复的导航状态 | Vue Router query/params | `LoginView.vue` 的 `redirect`；`PaymentView.vue` 的微信支付恢复 query；`LegalDocumentView.vue` 的 `documentId` param |
| 仅浏览器偏好或会话恢复信息 | `localStorage` / `sessionStorage` | `composables/usePersistedPageSize.ts`、`stores/auth.ts`、`router/index.ts` 的 chunk 重载标记 |

不要因为状态“来自 API”就自动放进全局 store。只被一个页面消费的数据应留在页面；需要跨路由复用、轮询、缓存或统一失效时，才提升到领域 store。

## Pinia setup store 约定

- store 名称使用稳定的小写领域名，例如 `defineStore('auth', ...)`、
  `defineStore('subscriptions', ...)`；对外函数使用 `useXxxStore`。
- setup store 内先声明 state，再声明 `computed`，然后声明 actions，最后显式返回公共接口。
  不要返回计时器、请求代际计数器或 in-flight Promise；这些是实现细节。参考
  `stores/subscriptions.ts` 中未返回的 `activePromise`、`pollerInterval` 和
  `requestGeneration`。
- 对外暴露派生状态而非让每个调用方重算。参考 `useAuthStore.isAuthenticated`、
  `useSubscriptionStore.hasActiveSubscriptions` 和 `useAppStore.hasActiveToasts`。
- action 负责成对清理副作用。`stores/auth.ts` 的 `startAutoRefresh` / `stopAutoRefresh`、
  `scheduleTokenRefreshAt` / `stopTokenRefresh`，以及 `stores/subscriptions.ts` 的
  `startPolling` / `stopPolling` 是现有模式。
- store 的公共出口集中在 `frontend/src/stores/index.ts`；新 store 若需要统一入口，按该文件现有导出方式添加，但业务代码也允许直接从具体 store 文件导入。

## 全局身份与应用状态

### 身份状态

`frontend/src/stores/auth.ts` 是身份状态唯一的 Pinia 所有者：`user`、access token、
refresh token、过期时间、运行模式和待完成认证会话均在此协调。必须保持以下存储键与
`frontend/src/api/client.ts` 一致：`auth_token`、`auth_user`、`refresh_token`、
`token_expires_at`。`checkAuth()` 在 `router/index.ts` 首次导航时恢复状态。

- 写入 JSON 后，读取时必须容错。`getPersistedPendingAuthSession()` 对
  `JSON.parse` 使用 `try/catch`，逐字段检查类型，并在无效时删除存储；新增持久化对象应遵循这一模式。
- 密码登录、注册和 2FA 完成由 `useAuthStore` 的 action 在 store 内部调用
  `setAuthFromResponse()`，统一更新 Pinia、`localStorage`、刷新定时器和待处理会话。
  `setAuthFromResponse()` 是私有实现函数，不是页面或回调可调用的公共接口。
- OAuth/SSO 回调走另一条真实流程：先调用
  `persistOAuthTokenContext(...)` 持久化响应中的 refresh token 和过期时间，再调用公共
  `authStore.setToken(accessToken)` 写入 access token 并加载当前用户。代表路径包括
  `views/auth/OAuthCallbackView.vue`、`views/auth/OidcCallbackView.vue`、
  `views/auth/LinuxDoCallbackView.vue`、`views/auth/DingTalkCallbackView.vue` 和
  `views/auth/WechatCallbackView.vue`。回调不得尝试调用私有 `setAuthFromResponse()`，也不要
  在页面复制半套身份写入逻辑。
- 主动刷新由 `useAuthStore` 根据过期时间调度；请求遇到 401 的被动刷新由
  `api/client.ts` 拦截器负责。两条路径共享同一组存储键，不能各自发明 token 状态。
- logout/clear 必须停止 interval/timeout 并清理相关存储，不能只把 `user.value` 设空。

### 应用状态

`frontend/src/stores/app.ts` 管理侧栏、全局 loading 计数、toast、公开设置和版本信息。

- 并发 loading 使用 `loadingCount` 计数，由 `setLoading()` 保证不低于 0；不要用一个
  boolean 让较早完成的请求提前隐藏全局 loading。
- 通用异步包装使用 `withLoading<T>()` 或 `withLoadingAndError<T>()`；后者会显示 toast
  并返回 `null`，调用方必须接受该返回契约，而不能假设失败一定抛出。
- 公开设置同时可来自 `window.__APP_CONFIG__` 和 API，统一由 `applySettings()` 写入 store。
  `fetchPublicSettings()` 的进行中请求优先于缓存和 `force`，所有调用方共享
  `publicSettingsRequest`，并在 `finally` 中以 Promise 身份检查后清理，防止旧请求覆盖新请求。

## 领域缓存、去重与失效

缓存策略按领域明确实现，不存在统一 server-state 库。

- `frontend/src/stores/subscriptions.ts` 是完整范例：60 秒 TTL、`activePromise` 请求去重、
  `requestGeneration` 防止已失效的旧响应回写、五分钟轮询，以及 `clear()` /
  `invalidateCache()`。
- `frontend/src/stores/app.ts` 的公开设置缓存返回 Promise，并保证同一时刻只有一个请求；
  这是多个调用方都必须观察同一结果时的模式。
- `frontend/src/stores/payment.ts` 的配置使用 `configLoaded` / `configLoading` 简单守卫，
  更适合允许重复调用方读取当前值、无需共享同一 Promise 的场景。
- `force` 的语义必须写清。订阅 store 中强制请求会增加 generation，使旧响应不能回写；
  公开设置 store 中已存在的 in-flight 请求即使遇到 `force` 也优先复用。不要在不了解领域并发语义时机械统一两者。
- 发起新请求前设置 loading，在 `finally` 清理；若存在并发请求，还要像
  `subscriptions.ts` / `app.ts` 一样确认当前 Promise 身份，避免旧请求清掉新请求的 loading。

## 局部状态、URL 状态与浏览器持久化

- 标量、开关和单个异步结果使用 `ref`；一组共同提交和校验的表单字段使用
  `reactive`。`LoginView.vue` 同时展示了 `formData`、`errors` 与派生
  `validationToastMessage` 的分工。
- 能从其他响应式值计算出的内容使用 `computed`，不要用 watcher 同步第二份状态。
- URL 是导航意图的状态源。未登录重定向由 `router/index.ts` 写入
  `query: { redirect: to.fullPath }`；OAuth 页面读取并传递该值；`PaymentView.vue` 解析支付恢复 query 后用 `router.replace()` 删除一次性参数。读取 query 时要处理 Vue Router 的
  `string | string[] | null | undefined` 形状，不要无条件断言字符串。
- `localStorage` 仅用于跨刷新偏好或明确的会话恢复，不是服务端数据的长期真相。
  `usePersistedPageSize.ts` 在读写时检查 `window`、捕获存储异常并规范化数值；
  `stores/adminSettings.ts` 也将缓存只用于首屏防闪烁，API 成功后再覆盖。
- 一次性状态优先 `sessionStorage`。`router/index.ts` 的 `chunk_reload_attempted` 防止动态
  import 失败造成无限刷新，API client 的 `auth_expired` 用于一次会话内的登录过期提示。

## 上游账号余额：页面局部状态

> 完整契约见 [backend/upstream-balance.md](../backend/upstream-balance.md)。

余额结果是 **Accounts 页会话局部状态**，不进入 Pinia，不写 DB；离开页面后恢复“未查询”。

### 放置与结构

- 以账号 ID 为键的 `Record`/`Map` 保存展示状态：`idle | loading | available | unsupported | failed`。
- 另备 in-flight `Map<id, Promise>` 做同一账号请求去重；不要把 Promise 暴露到模板。
- 单元格组件（如 `AccountBalanceCell.vue`）只负责展示与触发单行刷新；批量编排留在 `AccountsView.vue`。

### 批量刷新契约

| 规则 | 要求 |
|---|---|
| 范围 | 仅当前分页 `accounts` 中 `type === 'upstream'`（含禁用） |
| 并发 | 固定上限 **4**；禁止无界 `Promise.all` 整表 |
| 去重 | 已有 in-flight 则复用，不双发 |
| 失败 | 逐行独立；保留本页会话内上次成功值 + 显示失败；不中止其余账号 |
| 迟到响应 | 用账号 ID（及必要时列表代际）校验，避免翻页后写入错误行 |
| 汇总 | 完成后 toast：success / failed / unsupported 计数 |

### 配置表单与敏感清空

- `balance_access_token`：密码输入；留空不覆盖（omit）。
- `balance_user_id`：若管理员**故意清空**已配置值，payload 必须带 `balance_user_id: ""`，以触发后端 `MergePreservingSensitiveCreds` 清空；仅 `delete` 字段会保留旧值。
- `upstream_platform_type` 写在 `extra`，默认 `auto`。

## 常见错误

- 不要把上游余额结果放进 Pinia 或账号持久化字段；也不要用无界并发刷全部账号。
- 不要在清空 `balance_user_id` 时只 delete 键；必须发送空字符串才能清除服务端敏感值。
- 不要在组件里复制 token 刷新、用户恢复或公开设置缓存逻辑。
- 不要让两个并发请求无条件在 `finally` 中清空同一个 in-flight/loading 状态。
- 不要把 API 请求失败等同于功能明确关闭；`router/index.ts` 只有在公开设置已成功加载且开关显式为 `false` 时才阻止支付或风控路由。
- 不要信任 `localStorage` JSON 或 URL query 的运行时形状；先解析、缩窄并提供回退。
- 不要把页面内部弹窗、表单错误或临时筛选器提升为 Pinia 状态，除非存在真实的跨页面生命周期需求。
