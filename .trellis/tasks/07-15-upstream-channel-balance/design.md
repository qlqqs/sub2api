# 上游账号余额查询技术设计

## 1. 设计目标

为管理员提供 `AccountTypeUpstream` 账号的按需余额查询能力，并保持以下边界：

- 查询目标是账号，不是本地 `Channel`。
- 查询只读且不持久化余额，不改变账号状态、健康信息或调度状态。
- 支持 sub2api、New API 和受控自动识别。
- 余额查询专用凭证与正常代理凭证隔离。
- 前端支持单行刷新及当前分页受限并发刷新。
- 响应模型允许未来加入当前推理 API Key 的单一倍率，但本次不为倍率额外请求。

## 2. 现有边界与落点

### 2.1 账号配置

`backend/internal/service/account.go` 的 `Account` 已通过 `Credentials` 和 `Extra` 承载不同账号类型的扩展配置。`AccountTypeUpstream` 已持有正常代理所需的 `base_url` 和 `api_key`，因此不为本功能新增账号表或余额字段。

建议配置键：

- `Extra["upstream_platform_type"]`：非敏感枚举，值为 `auto`、`sub2api`、`new_api`。
- `Credentials["balance_access_token"]`：可选，仅用于账号余额查询。
- `Credentials["balance_user_id"]`：可选，New API 管理接口需要时使用。

新增敏感键必须进入现有 DTO 脱敏和日志脱敏覆盖范围。账号列表、详情和普通错误响应不得返回 `balance_access_token` 原文。普通代理请求构造不得读取这些余额查询专用键。

### 2.2 管理 API

在现有管理员账号路由组增加单账号动作：

```text
POST /admin/accounts/:id/upstream-balance
```

使用 `POST` 表达“触发一次实时上游 I/O”，并避免浏览器或中间代理缓存。前端当前页批量刷新通过受限并发调用单账号接口完成，不新增后端批量接口，从而让“当前页”范围始终由当前页面数据决定。

路由继续位于 `backend/internal/server/routes/admin.go` 的受保护管理员组中。非管理员不会进入 Handler。

### 2.3 服务与适配器

新增聚焦的上游余额查询服务，避免把协议解析塞入现有巨型账号 Handler 或正常网关服务：

```text
AccountHandler.QueryUpstreamBalance
  -> UpstreamBalanceService.QueryAccountBalance
     -> AdminService/GetAccount 读取账号及凭证
     -> UpstreamBalanceAdapter 查询并解析
```

建议核心契约：

```go
type UpstreamBalanceScope string

const (
    UpstreamBalanceScopeUser    UpstreamBalanceScope = "user"
    UpstreamBalanceScopeAPIKey  UpstreamBalanceScope = "api_key"
    UpstreamBalanceScopeUnknown UpstreamBalanceScope = "unknown"
)

type UpstreamBalanceResult struct {
    Status       string
    PlatformType string
    Scope        UpstreamBalanceScope
    Remaining    string
    Used         *string
    Total        *string
    Unit         string
    APIKeyRate   *string
    QueriedAt    time.Time
}
```

金额和倍率优先使用十进制字符串承载，避免 `float64` 在金额、配额单位换算和未来倍率展示中产生精度歧义。`APIKeyRate` 仅在同一余额响应明确返回当前 Key 倍率时设置；否则省略。

`status` 至少区分：

- `available`：成功得到余额。
- `unsupported`：平台或响应协议不受支持，作为可展示的业务结果返回。

鉴权失败、缺少查询凭证、超时、网络失败和非法响应通过稳定应用错误返回，不把它们伪装成 `unsupported` 或零余额。

## 3. 平台协议策略

### 3.1 明确平台

配置为 `sub2api` 或 `new_api` 时，只调用对应适配器，不探测其他平台。这样可以避免额外认证请求并提升错误可诊断性。

### 3.2 自动识别

配置为 `auto` 或旧账号未配置类型时，执行有限、固定顺序的兼容探测：

1. 仅基于账号配置的同一 origin 拼接适配器固定路径。
2. 每个平台最多一次请求；不根据域名猜测。
3. 仅当响应明确表示路径或协议不匹配时尝试下一个适配器。
4. 401/403、超时、DNS/TLS 和其他网络错误立即返回，不继续携带同一凭证探测其他协议。
5. 禁止跨 origin 重定向；任何重定向均不得把认证头转发到其他域名。

### 3.3 New API

参考 `cc-switch` 已验证模板：

```text
GET {base_url}/api/user/self
Authorization: Bearer <balance_access_token>
New-Api-User: <balance_user_id>
```

响应中的 `quota`、`used_quota` 按平台协议的 quota 单位换算并返回明确单位与 `scope=user`。若缺少管理凭证，可尝试平台已验证的推理 Key 兼容查询；若服务端明确拒绝权限，则返回“需要配置余额查询凭证”的稳定错误，不显示零余额。

### 3.4 sub2api

sub2api 适配器复用本项目公开的用户资料/余额协议及相应管理访问令牌。实现前以当前路由和响应 DTO 固定测试夹具，确认 base path、认证方式、余额字段和单位；不得仅因仓库名称相同就复用 New API 的 `/api/user/self` 结构。

如果仅有推理 API Key 且 sub2api 不提供 Key 级余额接口，则返回需要专用查询凭证，而不是尝试内部管理员接口或推断余额。

## 4. URL、HTTP 与安全

- 复用项目现有账号代理和 HTTP transport 构造方式；余额查询不绕过账号代理配置。
- 使用独立短超时，建议 10 秒，并响应请求上下文取消。
- 固定路径追加必须保留部署子路径，避免简单字符串拼接造成重复 `/api` 或丢失 base path。
- 限制响应体大小后再解码 JSON，拒绝 HTML、空响应和不匹配结构。
- 不自动重试；管理员可以显式刷新。
- 不记录请求头、凭证、完整 URL query 或原始响应体。
- 上游错误只转成预定义、可理解的错误类别；内部 cause 保留在错误链，不返回客户端。
- 查询不得调用账号状态更新、暂停、健康检查、调度缓存更新或凭证刷新路径。

## 5. 前后端响应契约

成功响应示例：

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

同一响应能可靠得到当前 Key 倍率时，可增加：

```json
{ "api_key_rate": "0.8" }
```

不支持示例：

```json
{
  "status": "unsupported",
  "platform_type": "unknown",
  "scope": "unknown",
  "queried_at": "2026-07-16T12:00:00Z"
}
```

可空字段使用 `omitempty`/可选 TypeScript 属性；不得返回空的 `group_rates` 占位字段。

## 6. 前端交互与状态

### 6.1 表格

在 `frontend/src/views/admin/AccountsView.vue` 增加可切换的“账号余额”列。建议将单元格抽成自定义组件，例如 `AccountBalanceCell.vue`，只负责展示：

- 未查询
- 查询中
- 用户余额 / Key 额度 / 可用额度
- 可选 API Key 倍率
- 查询时间
- 不支持
- 需要查询凭证或其他失败
- 单行刷新按钮

非 `AccountTypeUpstream` 账号显示“不适用”且不提供刷新按钮。

### 6.2 页面局部状态

余额结果是页面会话局部状态，使用以账号 ID 为键的 Map/Record，不进入 Pinia，也不写数据库：

```text
idle | loading | available | unsupported | failed
```

失败刷新可以保留同一页面会话内的最近成功值，但必须同时显示失败标识和最近成功查询时间。

### 6.3 当前页批量刷新

页面顶部增加“刷新当前页账号余额”按钮：

- 从当前 `accounts` 数组筛选 `type === "upstream"` 的账号，包括禁用账号。
- 使用固定并发上限，建议 4；不使用无界 `Promise.all`。
- 复用单行查询函数及同一账号 in-flight 去重状态。
- 页面切换或账号列表变化时，不把旧请求结果写入错误行；用账号 ID 校验结果归属。
- 完成后汇总 `success`、`failed`、`unsupported` 并显示顶部通知。
- 部分失败不清除成功结果，不中止剩余账号。

## 7. 配置 UI

在 `AccountTypeUpstream` 创建和编辑表单增加：

- 上游平台类型：自动识别（默认）/ sub2api / New API。
- 余额查询访问令牌：密码输入，可选，留空时不覆盖已有值。
- 余额查询用户 ID：可选。

字段说明必须明确：这些凭证仅用于余额查询，不参与模型代理请求。编辑表单遵循现有敏感凭证“已配置但不回显原文”的交互方式。

## 8. 兼容性与迁移

- 不新增数据库余额字段或余额历史表。
- 平台类型和专用查询凭证写入现有 JSON 配置，旧账号缺省等价于 `auto` 且无专用凭证。
- 账号导出/导入沿用现有管理员显式备份语义；新配置随 `Credentials`/`Extra` 传递，但普通账号 DTO 必须脱敏。
- 修改上游原有文件时添加 `CUSTOM:` 标记；优先新增 service、适配器、组件和测试文件，现有路由、Handler、DI、表格及表单仅做窄挂点修改。

## 9. 测试策略

### 后端

- 适配器解析：sub2api、New API、数值/字符串、零余额、缺字段、非法 JSON、HTML、超大响应。
- 协议选择：明确类型不探测；自动识别有限探测；401/403 和网络错误不继续探测。
- 安全：跨域重定向不携带凭证，错误和日志不含 token/API Key。
- 服务：账号不存在、非 upstream、缺 base URL、缺凭证、禁用账号仍允许、无任何 repository 写入。
- Handler/路由：管理员成功，非管理员拒绝，统一 envelope 和错误映射正确。

### 前端

- 单元格各状态、口径标签、时间和可选倍率。
- 单行刷新仅更新对应账号，重复点击去重。
- 当前页筛选、并发限制、部分失败和汇总通知。
- 翻页不查询其他页，非 upstream 不查询。
- 创建/编辑配置字段及敏感值不回显。

## 10. 回滚

本功能不持久化余额且不新增数据库结构。回滚时移除路由和前端入口即可；已保存在账号 JSON 中的新增配置键会被旧版本忽略。若需要清理，可后续提供显式配置清理，不在回滚过程中批量改写账号凭证。
