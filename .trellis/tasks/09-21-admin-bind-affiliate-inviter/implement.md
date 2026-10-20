# 管理员补绑邀请关系 — Implementation Plan

## 1. Database and models

- [x] 新增 `backend/migrations/239_affiliate_inviter_binding_metadata.sql`，增加绑定元数据列并回填历史绑定。
- [x] 更新 migration regression/schema tests，验证列、回填、向后兼容 trigger 和约束行为。
- [x] 扩展 `AffiliateSummary`/相关记录查询，使绑定时间和来源可在 service 使用。

## 2. Repository transaction

- [x] 扩展 affiliate repository 绑定契约，支持 registration/admin 元数据且保持单一事务。
- [x] 将条件更新收紧为“`inviter_id` 和永久绑定标记都为空”。
- [x] 邀请记录查询与用户 invitee 列表改用实际绑定时间。
- [x] Repository 测试覆盖成功、重复/竞争、回滚、计数只增加一次和历史回填兼容。
- [x] 搜索并更新所有 `AffiliateRepository` stubs/mocks。

## 3. Service and API

- [x] 新增管理员补绑 service 方法和稳定错误 reason，校验 ID、自邀请、原因长度、已绑定状态。
- [x] 注册绑定路径写入 `registration` 元数据；管理员路径写入 `admin`、操作者和原因。
- [x] 返利有效期改用绑定时间，确认补绑不触发历史返利。
- [x] 在 `affiliate_handler.go` 增加 typed request/response，并从认证上下文获取管理员 ID。
- [x] 注册 `POST /admin/affiliates/users/:invitee_id/inviter`。
- [x] 将路由映射为审计动作 `admin.affiliate.inviter.bind`，补充审计测试。
- [x] 添加 service、handler、route/API contract 测试。

## 4. Frontend API and UI

- [x] 在 `frontend/src/api/admin/affiliates.ts` 增加 typed `bindInviter` API。
- [x] 新建可访问的补绑 dialog，支持双方用户搜索、原因输入、警告、禁用和 loading/error 状态。
- [x] 在 SettingsView 邀请返利区域接入入口、成功 toast 和数据刷新。
- [x] 增加中英文 i18n key。
- [x] 增加 API 单测和组件/页面交互测试，覆盖成功、自邀请、缺少原因、409 冲突和重复点击。

## 5. Validation

Backend quick checks:

```bash
cd backend
gofmt -w <changed-go-files>
go test ./internal/service ./internal/handler/admin ./internal/repository ./internal/server/middleware
go test -tags=integration ./internal/repository
```

Backend full checks:

```bash
cd backend
go test ./...
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...
go build ./cmd/server
```

Frontend checks:

```bash
cd frontend
pnpm lint:check
pnpm typecheck
pnpm test:run
pnpm build
```

## Risk and Review Gates

- [x] 确认任何失败路径都不会只更新 `inviter_id` 或只增加 `aff_count`。
- [x] 确认并发重复提交不会重复增加邀请人数。
- [x] 确认旧版本仅写 `inviter_id` 时 trigger 自动补齐绑定元数据，永久标记不可清除，并允许 FK `ON DELETE SET NULL`。
- [x] 确认 `inviter_bound_at` 而非 profile `created_at` 驱动补绑后的有效期和邀请时间。
- [x] 确认审计日志不接受客户端伪造管理员 ID，reason 不超过限制且不泄露敏感信息。
- [x] 确认没有增加改绑/解绑入口，也没有执行历史返利补算。


## Validation Results

- Backend unit tests for affected service/handler/audit packages passed.
- Affiliate repository integration tests passed, including concurrency, rollback, legacy-write trigger, FK `ON DELETE SET NULL`, rebinding rejection, and migration reapply coverage.
- Backend `go vet` and server build passed.
- Frontend focused API/dialog/settings/i18n tests passed (51 tests), type-check passed, targeted ESLint passed, and production build passed.
- Full frontend suite has two pre-existing unrelated failures (`ChannelMonitorView.grok.spec.ts`, `GroupsView.codexManifest.spec.ts`).
- Broad backend repository execution can hit pre-existing Aliyun captcha local test-server timeouts; affected focused tests passed.
- `golangci-lint` is unavailable in the local environment.
- A full-repository ESLint invocation segfaulted locally; targeted changed-file ESLint passed.
