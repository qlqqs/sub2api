# 类型安全规范

## 编译约束

`frontend/tsconfig.json` 开启 `strict`、`isolatedModules`、`noUnusedLocals`、`noUnusedParameters` 和 `noFallthroughCasesInSwitch`，目标为 ES2020。使用 `@/* -> src/*` alias，Vue SFC 通过 `vue-tsc --noEmit` 检查。

`.eslintrc.cjs` 当前为了兼容存量代码关闭了 `no-explicit-any` 和部分规则。这表示 `any` 不会触发 lint，并不表示新代码应默认使用它；能表达边界时优先 `unknown`、泛型、联合类型和 `Record<string, unknown>`。

## 类型组织

- 跨 API、store、view 使用的领域类型放 `src/types/index.ts`；较独立的大领域可拆专题文件，如 `types/payment.ts`。
- 仅一个 API 模块/组件使用的 request/response/props 类型与实现共置；`api/admin/users.ts` 的绑定请求类型是现有样例。
- Vue props/emits 必须在 `<script setup lang="ts">` 中显式类型化；默认值用 `withDefaults`。
- 类型导入使用 `import type`，避免产生运行时依赖。
- 对有限状态使用 string union，如 `role: 'admin' | 'user'`、`sort_order: 'asc' | 'desc'`，而不是任意 string。
- API 字段保持服务端 JSON 的 `snake_case`，不要在各组件内反复做隐式 camelCase 转换。

## API 合约与运行时边界

- `ApiResponse<T>` 和分页结构定义在 `types/index.ts`；`api/client.ts` 成功时解包 `data`，因此端点函数泛型描述解包后的 payload。
- caught error、JSON、storage、`window.__APP_CONFIG__` 和后端响应都是运行时不可信边界。先判断 `typeof`/null/属性，再缩窄；`utils/apiError.ts` 展示了从 `unknown` 提取错误字段的方式。
- 可缺失字段区分 `?`、`null` 和实际默认值；不要用非空断言掩盖异步尚未加载。
- 数值输入先保留文本以处理空值/小数中间态，再显式 parse/范围校验；参见 `AmountInput.vue`。
- 动态内容除了类型检查还需要运行时安全校验：URL 用 `sanitizeUrl`，HTML/SVG 用 DOMPurify。

## 推荐模式

```ts
interface ApiErrorLike {
  status?: number
  reason?: string
  metadata?: Record<string, unknown>
}

function isApiErrorLike(value: unknown): value is ApiErrorLike {
  return typeof value === 'object' && value !== null
}
```

- 泛型列表逻辑复用 `BasePaginationResponse<T>`、`FetchOptions`，参考 `useTableLoader<T, P>`。
- 字典优先 `Record<Key, Value>`；需要固定 key 完整性时不要用无约束 index signature。
- 浏览器 timer 使用 `ReturnType<typeof setTimeout/setInterval>`，兼容 DOM 与测试环境。
- 路由 meta 扩展在 `router/meta.d.ts` 集中声明。

## 避免

- 新代码直接写 `catch (error: any)`，或未经检查强转整个 API 响应。
- 用 `as SomeType` 代替运行时验证来自 storage/网络的数据。
- 用 `!` 解决真实可空状态；应提前返回、提供默认值或建模状态机。
- 在多个文件复制同一 response interface，导致后端字段变化时局部漂移。
- 因 ESLint 允许而扩散 `any`。存量通用组件确有动态 row 时可以局部使用，但公共边界应逐步收窄。
