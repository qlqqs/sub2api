# 管理员补绑邀请关系 — Technical Design

## Architecture and Boundaries

请求链路沿用现有分层：

```text
SettingsView / Bind Dialog
  -> frontend admin affiliates API
  -> POST /api/v1/admin/affiliates/users/:user_id/inviter
  -> admin.AffiliateHandler
  -> AffiliateService.AdminBindInviter
  -> AffiliateRepository.BindInviterWithMetadata
  -> PostgreSQL transaction
```

业务不变量放在 `AffiliateService`，原子条件更新与计数增量放在 repository。Handler 只解析 path/body、读取认证管理员和映射响应。

## Database Changes

新增前向迁移 `239_affiliate_inviter_binding_metadata.sql`，为 `user_affiliates` 增加：

- `inviter_bound_at TIMESTAMPTZ NULL`
- `inviter_bound_source VARCHAR(32) NULL`
- `inviter_bound_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL`
- `inviter_bind_reason VARCHAR(500) NULL`

迁移回填所有现有 `inviter_id IS NOT NULL` 的记录：

- `inviter_bound_at = created_at`
- `inviter_bound_source = 'registration'`

未绑定用户保持这些字段为空。Repository 的绑定条件改为同时要求 `inviter_id IS NULL AND inviter_bound_at IS NULL`，使“绑定过一次”成为持久不变量。

迁移同时安装向后兼容的 PostgreSQL trigger：旧版本应用若只执行 `inviter_id: NULL -> non-NULL`，数据库自动补写 `inviter_bound_at=NOW()` 和 `inviter_bound_source=registration`；已有 `inviter_bound_at` 永久保留，禁止再次赋值或改为另一个非空邀请人，但允许 `inviter_id -> NULL`，以兼容邀请人删除时外键的 `ON DELETE SET NULL`。

不为该纯 SQL 表新增 Ent schema；继续沿用现有参数化 SQL repository 模式。

## Domain Model and Repository Contract

为绑定操作定义 service-owned 输入模型，例如：

```go
type AffiliateInviterBinding struct {
    Source  string
    BoundBy *int64
    Reason  string
}
```

Repository 原子绑定方法接收绑定元数据。注册绑定写入：

- source=`registration`
- bound_by=`NULL`
- reason=`NULL`
- bound_at=`NOW()`

管理员补绑写入：

- source=`admin`
- bound_by=认证管理员 ID
- reason=校验后的文本
- bound_at=`NOW()`

事务步骤：

1. 确保双方 affiliate profile 存在。
2. 条件更新 invitee 的 `inviter_id` 和绑定元数据。
3. `RowsAffected == 0` 时不修改计数并返回未绑定成功。
4. 更新 inviter 的 `aff_count = aff_count + 1`。
5. 任一步失败回滚。

`AffiliateSummary` 增加绑定时间/来源等服务层字段。邀请列表和返利有效期使用 `COALESCE(inviter_bound_at, created_at)` 作为兼容绑定时间。

## Service Rules

新增 `AdminBindInviter(ctx, inviteeID, inviterID, adminID, reason)`：

- 校验所有 ID 为正数。
- trim reason，长度必须为 1–500。
- 拒绝 `inviteeID == inviterID`，返回稳定的业务错误，例如 `AFFILIATE_SELF_INVITE`。
- 查询/初始化双方 affiliate profile。
- 若 invitee 的 `InviterBoundAt != nil` 或 `InviterID != nil`，返回 `AFFILIATE_ALREADY_BOUND`。
- 调用 repository 原子绑定；竞争失败同样映射为 `AFFILIATE_ALREADY_BOUND`。
- 不调用历史返利补算逻辑。

管理员补绑不检查 affiliate feature switch；返利入账路径仍保留现有总开关判断。

## HTTP Contract

```http
POST /api/v1/admin/affiliates/users/:user_id/inviter
Content-Type: application/json

{
  "inviter_id": 123,
  "reason": "用户注册时漏填邀请码"
}
```

成功数据包含 invitee ID、inviter ID、绑定时间和来源。主要错误：

- `400`：ID/reason 非法或自邀请。
- `404`：任一用户不存在。
- `409 AFFILIATE_ALREADY_BOUND`：已经或曾经绑定。

路由位于现有 admin group，自动继承管理员认证和审计 middleware。在 `auditActionOverrides` 中配置 `admin.affiliate.inviter.bind`；请求 path/body 与认证 actor 提供审计所需信息，必要时通过受限 audit extra 增加双方 ID。

## Frontend Design

在邀请返利设置区域增加“补绑邀请人”按钮，打开独立的 `AdminAffiliateBindInviterDialog.vue`：

- 两个可搜索用户选择器，复用 `affiliatesAPI.lookupUsers`。
- 必填原因 textarea，最多 500 字符。
- 显示不可修改和不追溯历史返利的警告。
- 相同用户时本地阻止提交；后端仍执行权威校验。
- 使用 `BaseDialog` 或项目现有可访问 dialog 基础组件。
- API 模块增加 typed request/response 和 `bindInviter` 方法。
- 中文/英文 locale 同步增加文案。

成功后 emit `success`，父页面显示 toast 并刷新邀请管理相关状态。后端冲突仍通过共享 API error helper 显示。

## Compatibility and Migration

- 原注册绑定流程继续调用同一原子 repository 能力，只是附带 `registration` 元数据。
- 已有 API 字段 `created_at` 在邀请记录中保留名称，但语义改为实际绑定时间。
- 迁移回填保证已有已绑定用户的有效期与展示时间近似保持原行为。
- 未创建绑定元数据的异常旧数据通过 `COALESCE` 保持可读，但新的绑定条件禁止覆盖已经回填的历史关系。

## Operational and Rollback Considerations

- 迁移仅增加 nullable 列并做受限回填，不重写金额或 ledger。
- 应用回滚后新增列可保留，不影响旧代码；旧代码仍读取原字段。数据库 trigger 会为旧代码的邀请人写入补齐注册来源元数据，并继续执行一次性绑定约束。
- 不提供解除接口。误操作需要单独的受审计数据修复流程，不能通过产品 API 绕过一次性规则。

## Upstream Merge Compatibility (2026-10-20)

上游新增同一用户的提现 POST 路由后，补绑路由模板统一使用 `:user_id`，避免 Gin 同级动态参数名冲突。客户端 URL、请求体、响应 `invitee_id` 和审计动作名不变；审计 path params 使用 `user_id`。保留上游提现实现和测试，以及本地补绑实现和测试。迁移按完整文件名识别，保留原有 `239_affiliate_inviter_binding_metadata.sql` 文件及内容，避免已应用环境 checksum 失配。
