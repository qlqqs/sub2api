# 组合式函数规范

## 适用边界

本项目使用 Vue composable，而不是 React hook。可复用且含响应式状态、生命周期、浏览器 API 或异步协调的逻辑放在 `src/composables/`，文件名遵循 `use<Name>.ts`。纯输入输出逻辑放 `src/utils/`，跨页面领域状态放 Pinia store。

## 返回形状与类型

- 函数以 `use` 开头，options 使用命名字段，并为泛型/回调声明接口。
- 返回调用方实际需要的 refs/computed 和动作；不要泄露内部 timer、controller 或可被随意破坏的不变量。
- 保留 ref 身份，不随意解包成快照。必要时使用 `Ref<T>` 标明公开响应式契约。
- 依赖 API 操作时接受 typed callback，而不是在通用 composable 中硬编码领域 endpoint。`useTableLoader<T, P>` 接受 `fetchFn` 是参考模式。

## 异步与并发

- 可被新请求替代的加载/搜索使用 `AbortController`，把 `signal` 传到 API 层；取消错误不显示成网络失败。
- 只有 abort 不足以防陈旧结果覆盖时，增加 request key/generation。`useKeyedDebouncedSearch.ts` 同时使用 controller 与 version。
- loading 状态只由当前请求清除；`useTableLoader.ts` 在 `finally` 中比较 controller，避免旧请求结束时关闭新请求的 loading。
- 共享中的请求可以缓存 promise 以去重；这种跨页面缓存更适合 store，参见 `stores/subscriptions.ts`。
- 错误默认向调用方传播，让 view/store 决定 toast 或重试；只静默忽略预期 cancellation。

## 生命周期与持久化

- composable 创建的 interval、timeout、listener、observer 或 in-flight request 必须在 `onUnmounted`/`onBeforeUnmount` 清理。
- `useAutoRefresh.ts` 的 `start` 是幂等的，并在卸载时 `stop`；`useTableLoader.ts` 在卸载时 abort。
- 访问 `localStorage` 时考虑不可用/解析失败，使用稳定且按功能命名的 key；只保存可重建的 UI 偏好，不保存新的敏感信息。
- composable 可能在组件外被调用时，像 `useKeyedDebouncedSearch.ts` 一样先确认存在 component instance，再注册生命周期 hook。

## 测试

- 测试位于 `composables/__tests__/`，文件名遵循 `use<Name>.spec.ts`，直接断言公开 refs 和动作。
- debounce/polling 使用 `vi.useFakeTimers()`，异步结果显式 await；清理后恢复 real timers。
- 请求类 composable 覆盖成功、错误、取消、快速连续请求、卸载清理和陈旧响应。
- 代表测试：`useTableLoader.spec.ts`、`useKeyedDebouncedSearch.spec.ts`、`useClipboard.spec.ts`、`useOpenAIOAuth.spec.ts`。

## 避免

- 在 composable 导入具体 view/component，或把 UI toast 文案硬编码到通用数据加载逻辑。
- 启动重复 interval/listener，却不暴露幂等 stop/clear 路径。
- 把所有 caught error 吞掉，导致调用方误判成功。
- 仅靠 debounce 防止竞态；网络请求仍需取消或 generation 防护。
- 为单个组件的两行 computed 提取没有复用价值的 composable。
