# 上游账号余额查询规范

> CUSTOM: 上游账号余额查询的可执行契约。来源：`07-15-upstream-channel-balance` 实现与评审结论。
> 跨层契约；修改 API/DTO/错误分类/凭证键时必须同步前后端与本规范。

相关源码：

| 层 | 路径 |
|---|---|
| Route | `backend/internal/server/routes/admin.go` — `POST /:id/upstream-balance` |
| Handler | `backend/internal/handler/admin/account_upstream_balance.go` |
| Service | `backend/internal/service/upstream_balance.go` |
| HTTP client | `backend/internal/repository/upstream_balance_http.go` |
| 凭证脱敏 | `backend/internal/service/account_credentials_redact.go`、`handler/dto/credentials_redact.go` |
| 前端 API | `frontend/src/api/admin/accounts.ts#queryUpstreamBalance` |
| 前端状态/UI | `frontend/src/views/admin/AccountsView.vue`、`components/account/AccountBalanceCell.vue` |

---

## Scenario: 管理员按需查询上游账号余额

### 1. Scope / Trigger

- **Trigger**：新增/变更管理端余额查询 API、上游协议适配、敏感凭证键、前端单行/当前页批量刷新。
- **查询目标**：`AccountTypeUpstream` 账号（不是本地 `Channel`）。
- **只读边界**（硬约束）：
  - 不写账号状态、健康信息、调度缓存；
  - 不持久化余额到 DB；
  - 不调用凭证刷新、暂停、启用路径；
  - 不进入正常代理/gateway 请求头构造。
- **权限**：仅管理员路由组；非管理员不得进入 Handler。
- **Fork**：优先新增 service/adapter/组件文件；上游文件仅窄挂点并加 `CUSTOM:` 标记。见 [二次开发约束](../guides/secondary-development.md)。

### 2. Signatures

#### HTTP API

```text
POST /admin/accounts/:id/upstream-balance
```

- 使用 `POST` 表达“触发一次实时上游 I/O”，避免缓存。
- 路径参数 `:id` 为正整数账号 ID。
- 无请求 body（配置来自账号 `Credentials` / `Extra`）。
- 成功：管理 API envelope `code: 0`，`data` 为 `UpstreamBalanceResult`。
- 失败：`response.ErrorFrom` → 统一 envelope；**绝不**把上游原始 body 写入 `message`。

#### Service

```go
// 只依赖 GetByID，不注入完整 AccountRepository。
// 全量 AccountRepository 在 Wire 层满足此接口即可。
type UpstreamBalanceAccountLookup interface {
    GetByID(ctx context.Context, id int64) (*Account, error)
}

func NewUpstreamBalanceService(
    accountLookup UpstreamBalanceAccountLookup,
    httpClient UpstreamBalanceHTTPClient,
) *UpstreamBalanceService

func (service *UpstreamBalanceService) QueryAccountBalance(
    ctx context.Context,
    accountID int64,
) (*UpstreamBalanceResult, error)
```

- **依赖面**：service 只读 `GetByID`；测试用 GetByID stub，禁止为满足完整 `AccountRepository` 再铺一堆 panic 写方法。
- **只读保证**：构造函数参数类型本身不含写方法，降低误写状态/健康/调度的风险。
- **Wire**：`NewUpstreamBalanceService(accountRepository, httpClient)` 合法，因 repository 实现了 `GetByID`。

#### HTTP transport

```go
type UpstreamBalanceHTTPClient interface {
    Do(request *http.Request, proxyURL string) (*http.Response, error)
}
```

- 实现：`repository.NewUpstreamBalanceHTTPClient` — **禁止跟随重定向**（`CheckRedirect` → `http.ErrUseLastResponse`）。
- 超时：service 层 `context.WithTimeout(ctx, 10s)`；transport dial/TLS/header 超时 10s。
- Body 上限：`1 << 20`（1 MiB）；超限/空 → `UPSTREAM_BALANCE_RESPONSE_INVALID`。

#### 配置键（账号 JSON，无新表）

| 键 | 位置 | 敏感 | 说明 |
|---|---|---|---|
| `upstream_platform_type` | `Extra` | 否 | `auto`（默认/缺省）、`sub2api`、`new_api` |
| `balance_access_token` | `Credentials` | **是** | 仅余额查询；优先于推理 `api_key` |
| `balance_user_id` | `Credentials` | **是** | New API `New-Api-User` 头；可显式清空 |
| `base_url` | `Credentials` | 否 | 既有上游地址 |
| `api_key` | `Credentials` | 是（既有） | 推理 Key；余额查询在无专用 token 时回退使用 |

### 3. Contracts

#### 响应 DTO：`UpstreamBalanceResult`

| 字段 | JSON | 类型 | 约束 |
|---|---|---|---|
| Status | `status` | string | `available` \| `unsupported` |
| PlatformType | `platform_type` | string | `sub2api` \| `new_api` \| `unknown`（或明确平台上的 unsupported） |
| Scope | `scope` | string | `user` \| `api_key` \| `unknown` |
| Remaining | `remaining` | string | 金额/配额十进制字符串；**禁止 float64** |
| Used | `used` | `*string` | 可选 |
| Total | `total` | `*string` | 可选 |
| Unit | `unit` | string | 如 `USD`；缺省解析后可回填 |
| APIKeyRate | `api_key_rate` | `*string` | 仅当**同一次**余额响应含可靠当前 Key 倍率时设置；否则 omit |
| QueriedAt | `queried_at` | time RFC3339 | UTC |

```json
{
  "status": "available",
  "platform_type": "new_api",
  "scope": "user",
  "remaining": "98.25",
  "used": "1.75",
  "total": "100.00",
  "unit": "USD",
  "queried_at": "2026-07-16T12:00:00Z"
}
```

`unsupported` 示例（业务成功 envelope，非 5xx）：

```json
{
  "status": "unsupported",
  "platform_type": "sub2api",
  "scope": "unknown",
  "queried_at": "2026-07-16T12:00:00Z"
}
```

#### 平台选择

| `upstream_platform_type` | 行为 |
|---|---|
| `sub2api` / `new_api` | **只**调用对应协议；不探测另一平台 |
| `auto` 或空 | 同源固定路径有限探测：先 sub2api，**仅** `errUpstreamBalanceProtocolMismatch` 时再试 new_api |
| 其他值 | `UPSTREAM_BALANCE_PLATFORM_INVALID` |

探测/网络规则：

1. 仅基于账号 `base_url` 的同一 origin 拼接**固定路径**（保留部署子路径）。
2. 每个平台最多一次请求；不按域名猜测。
3. **仅协议不匹配**才继续（auto）；401/403、超时、DNS/TLS/网络错误立即返回。
4. 禁止跨 origin 重定向；client 不跟随 redirect，认证头不得转发到其他域名。
5. 无自动重试。

固定路径：

| 平台 | 路径规则 | 认证 |
|---|---|---|
| sub2api | `{base}/v1/usage`（base 已以 `/v1` 结尾则追加 `/usage`） | `Authorization: Bearer <token\|api_key>` |
| New API | 去掉末尾 `/v1`/`/api` 后拼 `/api/user/self` | `Authorization: Bearer <token\|api_key>` + `New-Api-User: <user_id>` |

凭证优先级：`balance_access_token` → 回退 `api_key`。New API 还要求非空 `balance_user_id`（缺一则 credential-required，**永不**返回零余额）。

#### 敏感凭证

- `balance_access_token`、`balance_user_id` 必须列入 `SensitiveCredentialKeys`。
- 列表/详情 DTO：原文剥离，仅暴露 `has_balance_access_token` / `has_balance_user_id`。
- 日志、错误、客户端 message：**禁止** token/API Key/Authorization/原始上游 body。
- 正常代理路径**不得**读取这两个键。
- `MergePreservingSensitiveCreds`：
  - 请求 omit 键 → 保留库内原值；
  - 请求显式 `""` → **清空**该敏感键；
  - 前端清除 `balance_user_id` 时必须发送空字符串，不能只 delete 字段（否则无法清空）。

#### New API quota

- `quota` / `used_quota` 按 `500000` units = 1 USD 换算为十进制字符串。
- `scope=user`；`remaining`/`used`/`total=remaining+used`；`unit=USD`。

#### sub2api mode

| mode | scope | 字段 |
|---|---|---|
| `quota_limited` | `api_key` | `quota.remaining/used/limit` |
| `unrestricted` | `user` | `remaining` 或 `balance` |
| 其他/缺失 | — | protocol mismatch |

### 4. Validation & Error Matrix

| 条件 | 结果 |
|---|---|
| 非管理员 | 路由/中间件拒绝（不得进 Handler） |
| `id` 非法 | `INVALID_ACCOUNT_ID` |
| service 未注入 | `UPSTREAM_BALANCE_UNAVAILABLE` (503) |
| 账号不存在 | `ACCOUNT_NOT_FOUND` (404) |
| 非 `upstream` 类型 | `UPSTREAM_BALANCE_ACCOUNT_TYPE_UNSUPPORTED` (400) |
| 缺/非法 `base_url` | `UPSTREAM_BALANCE_BASE_URL_REQUIRED` / `_INVALID` |
| 非法 platform 枚举 | `UPSTREAM_BALANCE_PLATFORM_INVALID` |
| 缺凭证（sub2api 无 token/key；New API 无 token/key 或 user id） | `UPSTREAM_BALANCE_CREDENTIAL_REQUIRED`；**禁止** `remaining:"0"` |
| HTTP 401/403 | `UPSTREAM_BALANCE_CREDENTIAL_REQUIRED` |
| sub2api `isValid:false` | credential error（**不是** unsupported / invalid） |
| New API 缺 `success` 字段 / 非 JSON envelope | **protocol mismatch** |
| New API `success:false` | credential error（**不是** unsupported） |
| New API `data` null/缺失/畸形 | invalid response（`UPSTREAM_BALANCE_RESPONSE_INVALID` 或带 cause 的 bad gateway） |
| 明确平台 + protocol mismatch | **业务结果** `status=unsupported`（HTTP 200 + envelope success），不是 500 |
| auto 两平台均 protocol mismatch | `status=unsupported`, `platform_type=unknown` |
| HTTP 404/405（路径） | protocol mismatch |
| 其他非 2xx | `UPSTREAM_BALANCE_RESPONSE_ERROR` |
| 超时 | `UPSTREAM_BALANCE_TIMEOUT` (504) |
| 取消 | `UPSTREAM_BALANCE_CANCELED` |
| 网络错误 | `UPSTREAM_BALANCE_NETWORK_ERROR` |
| 空/超大/非 JSON body（读限制后） | `UPSTREAM_BALANCE_RESPONSE_INVALID` |
| 禁用账号 | **允许查询**；不得改状态 |

**分类原则**：

- `unsupported` = 协议/路径不匹配，可展示业务结果。
- credential = 识别到平台语义后的鉴权/权限失败。
- invalid/network/timeout = 基础设施或畸形载荷；cause 仅留错误链，不回客户端。

### 5. Good / Base / Bad Cases

- **Good**：明确 `new_api` + 专用 token/user id → `available` + `scope=user` + 字符串金额；账号 `status`/health 不变；DB 无余额写入。
- **Good**：明确 `sub2api` + `mode=quota_limited` → `scope=api_key`；可选 `api_key_rate` 仅当响应含可靠值。
- **Base**：`auto`，sub2api 404 → 试 New API；New API 成功 → `platform_type=new_api`。
- **Base**：禁用 upstream 账号仍可查询。
- **Bad**：缺凭证返回 `remaining:"0"` 或 `available`。
- **Bad**：`success:false` 映射为 `unsupported`。
- **Bad**：明确平台 protocol mismatch 返回 500 或把 sentinel `err.Error()` 给客户端。
- **Bad**：跟随跨域 redirect 并带上 `Authorization`。
- **Bad**：余额查询路径写 repository 状态/健康/调度。
- **Bad**：`balance_access_token` 进入 gateway 代理头。

### 6. Tests Required

后端（`upstream_balance_test.go` 及 handler/redact 测试）：

| 断言点 | 要求 |
|---|---|
| 解析 | sub2api unrestricted/quota_limited、零余额字符串、New API quota 换算、可选 rate |
| 协议选择 | 明确类型不二次探测；auto 仅 mismatch 继续；401/网络不继续 |
| 错误分类 | missing `success`→mismatch；`success:false`→credential；`isValid:false`→credential；null data→invalid |
| unsupported | 明确平台 mismatch → 200 + `status=unsupported`，非 5xx |
| 凭证 | 缺凭证 → `UPSTREAM_BALANCE_CREDENTIAL_REQUIRED`，无零余额 |
| Transport | 超时/取消分类；body 超限；不跟随 redirect |
| 脱敏 | 列表/详情无 token 原文，仅 `has_*`；日志字段无密钥 |
| 副作用 | service 测试无 repository 写；禁用账号可查 |
| 权限 | 管理员成功；非管理员拒绝 |

前端：

| 断言点 | 要求 |
|---|---|
| API | `POST .../upstream-balance`，类型为解包后 `UpstreamBalanceResult` |
| 单元格 | idle/loading/available/unsupported/failed；scope 标签；失败保留上次成功 |
| 批量 | 仅当前页 `type===upstream`；并发 4；in-flight 去重；部分失败不中止；汇总 success/failed/unsupported |
| 表单 | 敏感不回显；清空 `balance_user_id` 发 `""` |

### 7. Wrong vs Correct

#### Wrong

```go
// 明确平台协议不匹配时当作内部错误
if errors.Is(err, errUpstreamBalanceProtocolMismatch) {
    return nil, err // 客户端可能看到 500 或 sentinel 文本
}
// success:false 当成不支持
if !envelope.Success {
    return unsupportedResult(), nil
}
// 缺凭证伪装零余额
if token == "" {
    return &UpstreamBalanceResult{Status: "available", Remaining: "0"}, nil
}
```

```ts
// 清空用户 ID 时 omit 字段 → MergePreserving 会保留旧值
delete credentials.balance_user_id
// 无界并发刷整表
await Promise.all(allAccounts.map(a => queryUpstreamBalance(a.id)))
```

#### Correct

```go
if errors.Is(queryErr, errUpstreamBalanceProtocolMismatch) {
    return service.unsupportedResultWithPlatform(UpstreamBalancePlatformNewAPI), nil
}
if envelope.Success == nil {
    return nil, errUpstreamBalanceProtocolMismatch
}
if !*envelope.Success {
    return nil, upstreamBalanceCredentialError()
}
if credential == "" || configuration.userID == "" {
    return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_CREDENTIAL_REQUIRED", "...")
}
```

```ts
// 显式清空
credentials.balance_user_id = ''
// 当前页 + 并发上限 4 + in-flight Map 去重
await runWithConcurrency(upstreamOnPage, 4, refreshOne)
```

---

## 设计决策摘要

| 决策 | 选择 | 原因 |
|---|---|---|
| 查询对象 | Account 非 Channel | 上游连接配置在账号上 |
| 批量 API | 不提供后端批量 | “当前页”由前端分页决定 |
| 金额类型 | 十进制 string | 避免 float64 精度问题 |
| 专用凭证 | Credentials 敏感键 | 与代理隔离；脱敏复用现有管道 |
| unsupported | 业务 data 非 5xx | 可展示，且支持 auto 探测语义 |
| 无 DB 余额字段 | 页面内存状态 | 回滚无需 migration |

## 禁止事项

- 为倍率额外请求管理接口或用分组倍率推断 Key 倍率。
- 返回空的 `group_rates` 占位。
- 在客户端错误中嵌入上游原始 body / SQL / token。
- 在余额路径更新账号状态或写余额历史。
