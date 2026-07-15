# Composable 规范

Vue 3 组合式逻辑在本项目中正式称为 **Composable**。共享实现位于 `frontend/src/composables/`，文件与导出使用 `useXxx`。数据由 API 模块、页面或 Composable 状态以及 Pinia 共同管理，不套用其他框架的查询层术语。

## 何时创建 Composable

- 当逻辑同时包含响应式状态、派生状态、生命周期，或多个调用方都需要同一组操作时，创建 Composable。代表实现：
  - `src/composables/useTableLoader.ts`：分页、筛选参数、防抖、loading 和请求取消。
  - `src/composables/useAutoRefresh.ts`：持久化刷新间隔、倒计时、暂停与定时器生命周期。
  - `src/composables/useRoutePrefetch.ts`：空闲调度、懒路由加载和待执行任务取消。
  - `src/composables/useClipboard.ts`：Clipboard API、降级复制、通知和短暂 copied 状态。
  - `src/composables/useKeyedDebouncedSearch.ts`：按 key 隔离的防抖、请求取消和过期响应保护。
- 只服务单个组件的少量状态留在组件内；无响应式状态和生命周期的纯函数放 `src/utils/`；跨路由长期共享状态放 `src/stores/`。
- 领域专用 Composable 仍可放在 `src/composables/`，但名字必须表达领域，如 `useAccountOAuth`、`useBatchImageAccess`。不要创建含义模糊的 `useCommon`、`useHelpers`。

## API 形态与类型

- 导出命名函数 `useXxx(options)`，复杂参数使用 options 对象。`useAutoRefresh(options)` 和泛型 `useTableLoader<T, P>(options)` 是现有形态。
- Options、context 和可复用返回类型在同文件导出；纯实现细节保持私有。`UseAutoRefreshOptions` 和 `KeyedDebouncedSearchContext` 是参考。
- 返回 `ref`/`reactive` 状态和命名动作，不返回难以理解的位置数组。动作使用动词，如 `load`、`reload`、`start`、`stop`、`trigger`、`clearAll`。
- 只读状态应显式包装为 `readonly`，例如 `useRoutePrefetch` 的 `prefetchedRoutes`；确实允许调用方协调的状态才返回可写 ref。不要不经说明暴露内部 Map、AbortController 或 timer handle。
- 调用方拥有的依赖通过参数注入。`useTableLoader` 接收 `fetchFn`，`useAutoRefresh` 接收 `onRefresh`/`shouldPause`，`useRoutePrefetch` 接收 Router；这种边界也便于独立测试。

## 数据加载与取消

本项目的数据访问边界是 `src/api/`。Composable 负责编排请求状态，不复制 API URL、鉴权或响应协议。

### 表格加载

`src/composables/useTableLoader.ts` 的约定是：

- `fetchFn(page, pageSize, params, { signal })` 返回类型化的 `BasePaginationResponse<T>`。
- `items`、`loading`、`params` 和 `pagination` 由 Composable 管理；`reload()` 先回到第 1 页，页码和 page size 处理封装为命名动作。
- 每次 `load()` 先 abort 上一个 controller；只允许当前 controller 在 `finally` 中清除 loading，防止旧请求覆盖新状态。
- DOM/Axios 的取消错误被识别后静默结束，其他错误记录并重新抛出，让页面决定 toast 或错误 UI。
- `onUnmounted` abort 当前请求。`src/composables/__tests__/useTableLoader.spec.ts` 覆盖 loading、分页、防抖、取消和错误传播。

该 Composable 目前主要作为共享抽象和单元测试契约存在；部分历史表格页仍维护自己的加载状态。不要只为统一形式而机械迁移大型页面，新增或重构时先确认 API 返回形状与取消语义兼容。

### 页面级加载

并非所有请求都必须封装成通用 Composable。`src/views/user/ChannelStatusView.vue` 在页面内持有 `AbortController`，在新 reload 前取消旧请求，并用 controller 身份与 `signal.aborted` 防止过期响应写入；卸载时再次 abort。这是单页面请求可以采用的本地模式。

- API 函数必须允许向下传递 `AbortSignal`。
- 对取消错误不展示失败 toast；真实错误通过 `extractApiErrorMessage` 等现有边界转换后交给 store/UI。
- 并发请求必须防止旧响应覆盖新响应，不能只在开始新请求时切换 loading。

## 定时器、刷新与生命周期清理

### 自动刷新

`src/composables/useAutoRefresh.ts` 管理一秒 tick、间隔持久化、倒计时和请求中保护：

- `start()` 幂等，已有 timer 时不重复注册；`stop()` 清理并将 handle 复位。
- `tick()` 在禁用、`shouldPause()` 为真或上次 refresh 尚未结束时跳过。
- `setEnabled()` 同时更新 localStorage、倒计时和 timer；组件也可以显式 `start`/`stop`。
- `onBeforeUnmount(stop)` 保证离开组件后不再刷新。
- `src/views/user/ChannelStatusView.vue` 使用 `shouldPause: () => document.hidden || loading.value`，并让页面自己的 AbortController 负责网络取消；刷新节奏和请求所有权保持分离。

### 通用清理规则

- 每个 `setTimeout`/`setInterval`、event listener、Observer、idle callback 和 AbortController 都要明确所有者与清理路径。
- Composable 在组件 setup 中使用且注册生命周期时，使用 `onBeforeUnmount` 或 `onUnmounted` 清理；还要暴露 `stop`、`clearAll` 或 `cancelPendingPrefetch`，供业务主动停止。
- 如果 Composable 也允许在非组件上下文调用，不能无条件依赖组件实例。`useKeyedDebouncedSearch.ts` 通过 `getCurrentInstance()` 决定是否注册 `onUnmounted(clearAll)`，同时总是返回 `clearAll()`。
- `useTableLoader.ts` 当前无条件调用 `onUnmounted`，其单元测试因此 mock Vue 生命周期。这意味着它的正式使用边界是组件 setup；不要在 store、模块顶层或普通脚本中调用。

### 路由预加载

`src/composables/useRoutePrefetch.ts` 从 Router 的懒加载 route record 获取 importer，在 `requestIdleCallback` 中预加载相邻路由，并为不支持该 API 的浏览器使用 timeout fallback。

- 新导航前调用 `cancelPendingPrefetch()`，避免旧空闲任务继续运行。
- 已完成路径记录为只读集合，避免重复工作；`resetPrefetchState()` 同时取消任务并清空状态。
- `src/router/index.ts` 在导航流程中懒初始化该 Composable 并传入 Router。不要维护一份静态组件 import 映射，否则会扩大入口 chunk；邻接表只保存 path。
- 预加载失败不得影响导航；开发模式可记录 debug 信息。

## 防抖与过期响应

- 单一动作可使用 `@vueuse/core` 的 `useDebounceFn`，如 `useTableLoader` 的 `debouncedReload`。
- 多个列表项或规则需要独立搜索时，使用 `useKeyedDebouncedSearch` 的模式：每个 key 分别保存 timer、AbortController 与 version；同 key 新输入取消旧 timer/请求，不同 key 互不干扰。
- 仅 abort 不足以阻止不遵守 signal 的 Promise 回写；还要比较请求版本或 controller 身份。`useKeyedDebouncedSearch.ts` 同时检查 `signal.aborted` 与 version。
- 清除操作必须删除 timer、abort controller 和版本状态，防止 Map 长期增长。
- 不要在 `watch` 或 input handler 中每次创建一个没有取消句柄的新 debounce 函数。

## Clipboard 与浏览器能力降级

`src/composables/useClipboard.ts` 展示了浏览器能力封装：安全上下文优先使用 `navigator.clipboard.writeText`，失败或不支持时使用临时 readonly textarea 和 `execCommand('copy')`，并通过 app store 显示本地化成功/失败消息。

- 浏览器能力检测、降级 DOM 和通知只实现一次，组件调用 `copyToClipboard`；不要在每个按钮复制一份 Clipboard API 判断。
- 临时 DOM 必须在 `finally` 删除。
- 空文本直接返回 false，不触发通知或写入。
- `copied` 在两秒后复位；当前实现未保存和清理该 timeout，这是现有小型生命周期缺口。新增带 timer 的 Composable 不应复制该做法；若组件可能在 timer 完成前卸载，应保存 handle 并清理。

## Composable 测试与 fake timers

- 测试放在 `src/composables/__tests__/useXxx.spec.ts`，使用 Vitest，直接断言返回 ref 和公开动作。
- 涉及 debounce、倒计时或临时状态时使用 fake timers：`beforeEach` 调用 `vi.useFakeTimers()`，`afterEach` 恢复 `vi.useRealTimers()`。代表测试：
  - `useTableLoader.spec.ts` 推进 300ms 并 `runAllTimersAsync()`，验证多次 reload 只执行一次。
  - `useKeyedDebouncedSearch.spec.ts` 推进 delay，验证不同 key 并行、同 key 旧响应被忽略。
  - `useClipboard.spec.ts` 推进 2000ms，验证 `copied` 复位。
- timer 回调启动 Promise 后还需 flush microtasks 或使用 `runAllTimersAsync()`；只推进时间可能在断言时异步工作尚未完成。
- AbortController 测试应让 mock request 监听 `signal.abort`，并验证取消不会作为普通错误传播、最新请求结果生效。
- 浏览器 API 使用可恢复的 stub/mock，并在 `afterEach` 恢复 timer 和全局函数。`useRoutePrefetch.spec.ts` 当前通过 stub idle callback 后等待真实 timeout，属于较慢的历史测试写法；新增或改写 timer 测试优先使用 fake timers，避免真实等待 100ms 或 2100ms。

## 已知例外与反模式

- 不要把所有请求都塞进一个全局万能加载 Composable。领域错误处理、分页协议、轮询和缓存语义不同时，应保留明确边界。
- 不要忽略取消后的过期响应，不要让旧请求的 `finally` 清掉新请求的 loading，不要把 AbortController 暴露给模板。
- 不要在模块导入时启动 interval、注册 window listener 或发请求；副作用由 `useXxx()` 调用和组件生命周期拥有。
- `useAutoRefresh.ts` 当前没有同名邻近单元测试，其行为由真实调用点体现但覆盖不如其他 Composable 完整。修改倒计时、暂停、持久化或清理语义时，应补充 `src/composables/__tests__/useAutoRefresh.spec.ts` 的聚焦 fake-timer 测试。
