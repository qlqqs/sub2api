# API 访问规范

## 分层与入口

`frontend/src/api/client.ts` 是统一 HTTP 边界，业务 API 不应自行创建 Axios 实例。
模块按领域放在 `frontend/src/api/*.ts`，管理员接口放在 `frontend/src/api/admin/*.ts`：

- `frontend/src/api/index.ts` 汇总认证、用户、支付、公告等公共入口，并导出 `apiClient`。
- `frontend/src/api/admin/index.ts` 将各管理模块组成 `adminAPI`，同时保留 named export 与
  default export；新增管理员领域时要同步该 barrel。
- `frontend/src/api/payment.ts` 展示“对象方法返回 Axios response”的风格；
  `frontend/src/api/admin/users.ts`、`admin/system.ts` 展示函数内部 `const { data } = ...`
  后直接返回业务数据的风格。扩展现有模块时保持该模块既有返回契约，不要让调用方多解包或少解包一层。
- URL 组合统一使用 `frontend/src/api/url.ts` 的 `getAPIBaseURL()`、`buildApiUrl()`、
  `buildGatewayUrl()`；它们会规范化相对 base、斜杠与 gateway origin。不要在页面手拼
  `'/api/v1'`。

API 函数只做传输契约、轻量适配与参数构造；跨页面缓存/去重属于 store，用户提示属于
view/composable 或 `utils/apiError.ts`。

## Axios client 配置

`apiClient` 通过 `axios.create()` 固定：

- `baseURL: getAPIBaseURL()`；默认 base 为 `/api/v1`；
- `withCredentials: true`，支持跨域 cookie/OAuth 绑定流程；
- 30 秒 timeout；
- 默认 JSON content type。

请求拦截器负责所有请求的横切信息：

1. 从 `localStorage['auth_token']` 附加 `Authorization: Bearer ...`。
2. 用 `getLocale()` 写入 `Accept-Language`。
3. 对 GET 请求在 `config.params.timezone` 写入浏览器时区，失败回退 `UTC`。
4. 通过 `frontend/src/api/adminUIRequest.ts` 的 `shouldMarkAdminUIRequest()` /
   `shouldMarkUserUIRequest()` 写入 `X-Admin-UI-Request`、`X-User-UI-Request`。

新增 API 模块不得重复实现这些 headers，也不要用裸 Axios 绕过 locale、timezone、UI 标记与认证逻辑。唯一明确例外是 `client.ts` 内部的 token refresh：它必须绕过自身拦截器以避免递归。

## 响应解包与模块返回值

后端标准成功 envelope 为 `{ code, message, data }`。响应拦截器在 `code === 0` 时把
`response.data` 替换为 envelope 的 `data`；没有 `code` 字段的响应保持原样。因此 API 模块应：

- 调用 `apiClient.get<T>()` / `post<T>()` 时，泛型 `T` 表示**解包后的业务数据**；
- 返回业务数据的函数使用 `const { data } = await apiClient...; return data`，参考
  `api/admin/users.ts#list` 与 `api/admin/system.ts#checkUpdates`；
- 返回 Axios response 的对象方法让调用方读取一次 `.data`，参考
  `api/payment.ts` 与 `stores/payment.ts`；
- 不在每个模块重复检查 `code === 0`，也不要把普通调用写成 `ApiResponse<T>` 后二次解包。

`ApiResponse<T = unknown>` 定义在 `frontend/src/types/index.ts`，主要描述线上 envelope；
`client.ts` 内绕过 `apiClient` 的 refresh 请求必须自己按该类型读取原始响应。

## Token refresh 与 401 并发

身份主动刷新在 `frontend/src/stores/auth.ts`；HTTP 401 的被动恢复由
`frontend/src/api/client.ts` 完成。修改任一处时必须保持存储键和时序一致。

- client 使用模块级 `isRefreshing` 与 `refreshSubscribers`，确保并发 401 只发一个
  `/auth/refresh`。等待请求在新 token 到达后重放。
- 每个原请求通过 `_retry` 标记最多重试一次；登录、注册、refresh endpoint 不触发 refresh，防止循环。
- refresh 使用裸 `axios.post` 且显式 30 秒 timeout；成功后更新 `auth_token`、
  `refresh_token`、`token_expires_at`，通知队列并重放原请求。
- 失败时必须以空 token 释放所有订阅者，重置 `isRefreshing`，清理 auth 存储，设置
  `sessionStorage['auth_expired']`，并在非登录页跳转 `/login`。不能让等待 Promise 永久挂起。
- 无 refresh token 的 401 同样清理身份；仅当请求确实携带 token 且不是 auth endpoint 时记录过期提示。

取消请求是例外：`ERR_CANCELED` / `axios.isCancel(error)` 必须原样 reject，不能被改写成网络错误。

## 错误形状与调用方处理

API client reject 的通常是普通结构化对象，不保证是 `Error` 或 `AxiosError`。当前字段可能包括：

```text
{ status, code, reason, error, message, metadata }
```

- 业务 envelope 的 `code !== 0` 会生成上述形状。
- HTTP error 从经过对象检查的 `response.data` 提取字段；非对象/HTML body 不可直接读属性。
- 网络错误规范化为 `{ status: 0, message: 'Network error. Please check your connection.' }`。
- ops 关闭的 404 会缓存功能状态、广播 `ops-monitoring-disabled`，必要时离开 ops 页面。
- 管理员合规 423 会广播 `admin-compliance-required`，保留 token，并把 metadata 传给
  `useAdminComplianceStore()`。

catch 参数优先保持 `unknown`，显示消息或读取 code 时使用
`frontend/src/utils/apiError.ts` 的 `extractApiErrorMessage()`、
`extractApiErrorCode()`、`extractI18nErrorMessage()`；不要假设 `error.response.data` 永远存在。
需要识别特殊错误时，先检查对象/字段或缩窄到最小局部接口。

## API 类型、参数与取消

- 请求/响应类型放在共享 `src/types` 或所属 API 文件，规则见 `type-safety.md`。
- Axios 泛型必须与解包后的响应一致；分页使用 `PaginatedResponse<T>` 或
  `BasePaginationResponse<T>`，按后端实际字段选择，不能混用。
- 复杂查询先构造 typed params，再传 `{ params }`。动态属性筛选可像
  `api/admin/users.ts#list` 一样集中在 API 层转换为 `attr[id]`，不要让 view 拼协议字段。
- 可取消列表请求接受 `AbortSignal`，现有 `FetchOptions` 位于 `types/index.ts`；
  `api/admin/users.ts#list` 把 `signal` 直接交给 Axios。调用方取消后应识别取消错误而非显示失败 toast。
- 管理/用户 UI timing header 的路径 allowlist 在 `api/adminUIRequest.ts`；新增需要 timing 的接口要同步审查该文件及后端 allowlist，而不是在单个调用处硬编码 header。

## API 测试与变更检查

`frontend/src/api/__tests__/client.spec.ts` 是拦截器契约的主要测试，覆盖 base URL、认证头、
timezone、withCredentials、Admin/User UI 标记、成功解包、结构化错误、合规事件、401 清理、
网络错误和取消。领域 API 测试位于同目录，例如 `payment.spec.ts`、
`admin.users.spec.ts`、`admin.system.rollback.spec.ts`。

聚焦运行：

```bash
pnpm --dir frontend exec vitest run src/api/__tests__/client.spec.ts
pnpm --dir frontend exec vitest run src/api/__tests__
```

这些测试被 `frontend/tsconfig.json` 排除，不属于 `pnpm --dir frontend run typecheck`；必须实际运行 Vitest。根 `make test-frontend` 也不包含完整 API 测试集合。

## 禁止模式

- 不要在 view/composable 新建 Axios client、重复 token refresh，或手工附加横切 headers。
- 不要把 `ApiResponse<T>` 与拦截器解包后的 `T` 混为一谈。
- 不要捕获后只依赖 `error.message` / `error.response`，也不要吞掉取消语义。
- 不要让 refresh 失败路径遗漏队列释放、状态复位或存储清理。
- 不要为新增 admin 模块忘记更新 `api/admin/index.ts` 的 `adminAPI` 与导出。
