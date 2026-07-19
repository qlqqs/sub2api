# 路由规范

## 路由入口与声明

路由集中定义在 `frontend/src/router/index.ts`。该文件使用
`RouteRecordRaw[]`、`createWebHistory(import.meta.env.BASE_URL)` 和统一的全局守卫；页面组件
通过 `() => import('@/views/...')` 懒加载。新增页面路由应保持这一集中式结构，不要在
view/component 中创建第二个 Router 实例或维护另一份权限表。

- 每条页面路由应有稳定的 `path` 与 `name`，组件使用动态 import。现有 OAuth 回调路由、
  用户路由和管理路由均可作为模板。
- 重定向路由只声明 `redirect`；404 使用 `/:pathMatch(.*)*` 并放在路由表末尾。
- route meta 的类型扩展只放在 `frontend/src/router/meta.d.ts`。当前字段包括
  `requiresAuth`、`requiresAdmin`、`requiresPayment`、`requiresRiskControl`、`title`、
  `titleKey`、`descriptionKey`、`breadcrumbs`、`icon` 与 `hideInMenu`。新增 meta 能力时必须先扩展该接口，再在守卫或导航 UI 中消费。
- `requiresAuth` 的默认值是 **true**：守卫使用
  `to.meta.requiresAuth !== false`。登录、注册、回调、公开支付结果等公共路由必须显式写
  `requiresAuth: false`，不能依赖“未填写即公开”的直觉。
- `requiresAdmin` 只在 `true` 时生效。普通登录用户路由虽常显式写 `false`，判断逻辑仍是
  `to.meta.requiresAdmin === true`。
- 页面标题由 `frontend/src/router/title.ts` 的 `resolveRouteDocumentTitle()` 处理，优先结合
  自定义菜单与 `titleKey`。新增可见页面至少提供 `title`；已有对应翻译时同时提供
  `titleKey` / `descriptionKey`。

## 全局守卫顺序

`router.beforeEach` 是权限和功能访问的真实入口。修改时保留其顺序与每次 `next()` 后的
`return`，避免一次导航调用两次回调：

1. 通过 `useNavigationLoadingState().startNavigation()` 开始导航状态。
2. 首次导航调用 `useAuthStore().checkAuth()`，从浏览器存储恢复身份；模块级
   `authInitialized` 防止重复恢复。
3. 组合公开设置与管理员自定义菜单，调用 `resolveRouteDocumentTitle()` 更新标题。
4. `/setup` 通过 `getSetupStatus()` 判断是否应使用
   `resolveCompletedSetupRedirectPath()` 离开安装页；状态查询失败时保持安装页可达。
5. 处理公共路由、已登录用户访问登录/注册、backend mode 公共白名单。
6. 处理未登录跳转，并把 `to.fullPath` 写入登录页 `redirect` query；随后处理管理员角色与
   `useAdminComplianceStore()` 合规确认。
7. 对 `requiresPayment` / `requiresRiskControl`，若公开设置尚未加载，先 await
   `useAppStore().fetchPublicSettings()`。只有“加载成功且开关显式为 false”才拒绝；请求失败是未知状态，不是关闭状态。
8. 最后处理 simple mode 受限路径和 backend mode 的管理员/普通用户访问边界。

backend mode 白名单目前由 `BACKEND_MODE_ALLOWED_PATHS`、
`BACKEND_MODE_CALLBACK_PATHS`、`BACKEND_MODE_PENDING_AUTH_PATHS` 和
`isBackendModePublicRouteAllowed()` 共同定义。新增公开认证回调或 backend mode 可访问页面时，
必须同步审查这些列表，而不只是给 route 写 `requiresAuth: false`。

## 功能访问与导航状态

- 支付和风控访问使用 typed meta，不在页面 mounted 后才重定向。参考 `/purchase` 的
  `requiresPayment` 和管理风控路由的 `requiresRiskControl`。
- simple mode 的限制当前是 `router/index.ts` 中的路径前缀列表；新增同类管理页面时需要
  审查该列表。
- backend mode 允许管理员完整访问，并限制非管理员及匿名用户。认证回调与待完成认证页面
  有单独白名单；修改时测试匿名、普通用户、管理员三种身份。
- 路由 query/params 是运行时输入。页面读取 `route.query` 时应检查字符串/数组/空值，
  一次性 query 消费后使用 `router.replace()` 清理。参考
  `views/user/PaymentView.vue` 的微信支付恢复参数和
  `views/auth/WechatPaymentCallbackView.vue`。
- 需要保留返回目的地时写入 `to.fullPath`，不要只保存 `to.path`，否则会丢失筛选或恢复 query。

## 懒加载、预取与错误恢复

- 路由页面保持动态 import，以便 Vite 分块；不要把 route view 改为顶层静态 import。
- `router.afterEach` 结束导航 loading，并懒初始化
  `frontend/src/composables/useRoutePrefetch.ts`。预取逻辑从 `router.getRoutes()` 取得现有
  component importer，邻接关系由 `PREFETCH_ADJACENCY` 维护；新增常用相邻页面时更新路径，
  不要另建静态 import 映射。
- `router.onError` 识别动态模块/CSS chunk 加载失败，并通过
  `sessionStorage['chunk_reload_attempted']` 的十秒窗口最多自动重载一次。不得移除该保护或改成无条件 reload。
- `scrollBehavior` 对浏览器前进/后退恢复 `savedPosition`，新导航回到顶部；需要特殊滚动行为时应在这里统一处理。

## 路由测试

路由测试使用 Vitest + mocked router/store，测试文件不在主 `tsconfig.json` 的 typecheck 范围内：

- `frontend/src/router/__tests__/feature-access.spec.ts` 导入真实 `router/index.ts` 并捕获注册的守卫，验证首次设置加载、失败时放行、显式关闭时重定向。
- `frontend/src/router/__tests__/wechat-route.spec.ts` 通过 `router.getRoutes()` 验证回调路径、名称和公共 meta。
- `frontend/src/router/__tests__/title.spec.ts` 验证标题解析。
- `frontend/src/router/__tests__/guards.spec.ts` 包含守卫逻辑的测试模型；它与生产守卫存在重复，修改生产分支时必须同步审查，不能仅让模型测试通过。
- `frontend/src/__tests__/integration/navigation.spec.ts` 覆盖实际导航场景。

新增或修改路由时，至少验证：路由已注册、公共/受保护默认值、角色重定向、功能开关未知与
显式关闭两种状态，以及 query 是否保留。可运行：

```bash
pnpm --dir frontend exec vitest run src/router/__tests__
pnpm --dir frontend exec vitest run src/__tests__/integration/navigation.spec.ts
```

这些命令是路由聚焦测试；根 `make test-frontend` 只跑 Makefile 中列出的 critical tests，
当前并不包含完整路由测试集合。

## 禁止模式

- 不要省略公共路由的 `requiresAuth: false`。
- 不要在 view 内复制管理员、simple mode、backend mode 或功能开关授权逻辑。
- 不要把设置加载失败当作功能关闭；只有成功加载后的显式 `false` 才能拒绝。
- 不要在调用 `next()` 后继续执行守卫，也不要同时混用返回式守卫和多次 `next()`。
- 不要新增无法由 `meta.d.ts` 表达的任意 meta 字段，或把页面改为静态 import 破坏路由分块。
