# 状态管理规范

## 状态分类

| 状态 | 放置位置 | 示例 |
|------|----------|------|
| 单组件交互、表单草稿、modal 开关 | 组件内 `ref`/`reactive` | `AmountInput.vue`, `BaseDialog.vue` |
| 可复用但跟随组件生命周期 | composable | `useTableLoader.ts`, `useAutoRefresh.ts` |
| 身份、全局 UI、跨路由领域缓存 | Pinia setup store | `stores/auth.ts`, `stores/app.ts`, `stores/subscriptions.ts` |
| 可分享/恢复的导航状态 | route params/query/meta | `router/index.ts`, `views/admin/AuditLogView.vue` |
| HTTP 数据传输 | `api/` 返回值；按需进入 store 缓存 | `api/admin/users.ts`, `stores/subscriptions.ts` |
| 用户偏好/会话恢复 | 明确命名的 local/session storage | theme、page size、auth expiry |

## Pinia 模式

- 使用 Composition API store：`defineStore('stable-id', () => { ... })`，state 为 `ref`，派生值为 `computed`，副作用封装为函数。
- store 拥有其缓存、poller、in-flight promise 和持久化键的生命周期，并提供 `clear`/`reset`/`invalidateCache` 等动作。
- 认证状态只由 `useAuthStore` 管理。登录/刷新统一写 token 与 user；失败或登出清理 token、计时器和相关状态。不要在页面维护另一份权威认证状态。
- 全局 toast、loading、公开站点配置由 `useAppStore` 管理。并发 loading 使用计数，而不是简单 boolean 互相覆盖。
- 数据缓存要写清 TTL、force refresh、请求去重和陈旧响应规则。`useSubscriptionStore` 使用 TTL、共享 promise 与 generation counter，是参考实现。

## 服务端状态

- `api/client.ts` 统一附加 token、locale、GET timezone 和 UI 标记，并解包 `{code, message, data}`。端点函数通常返回 `const { data } = await apiClient...; return data`。
- 列表页面不默认放全局 store；局部表格用 `useTableLoader` 管理分页、筛选、abort 和 loading。
- mutation 成功后明确更新本地对象、重新加载或 invalidation，不能期待另一个页面自动同步。
- 对 auth refresh 等全局并发请求进行去重，并确保所有等待者在成功和失败时都被唤醒。
- cancellation 是控制流，不应触发错误 toast；真实 API 错误可以用 `utils/apiError.ts` 按 `reason` 做 i18n 映射。

## URL 与持久状态

- 影响深链接、返回导航或分享结果的筛选/实体标识优先放 route params/query；临时展开状态不必写入 URL。
- localStorage 只放跨刷新需要的状态。读取 JSON 必须容错，版本/缺字段要有默认值。
- token 等既有认证键只能通过 auth/client 流程操作；不要创建未审计的凭据副本。
- 组件/store 卸载或登出时终止 polling、timeout 与在途请求，防止旧会话结果写入新状态。

## 常见错误

- 同一数据在 prop、局部 ref 和 Pinia 中同时作为权威来源。
- 将每个 API 响应都永久塞进 store，却没有 invalidation/clear 策略。
- 并发请求用单个 boolean，旧请求完成后提前清除 loading。
- force refresh 仍被旧响应覆盖，或 clear 后 in-flight promise 回写已清空状态。
- 只依赖前端 route/store 做权限判断；后端仍必须校验认证和角色。
