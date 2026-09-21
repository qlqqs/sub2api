# 后端质量规范

## 必须保持的约束

- 使用 Go 1.26 module 语义和仓库已有依赖，不为小功能引入重型框架。
- 所有 Go 改动经过 `gofmt`。`backend/.golangci.yml` 启用 `depguard`、`errcheck`、`gosec`、`govet`、`ineffassign`、`staticcheck` 和 `unused`。
- handler 不依赖 repository/数据库客户端；service 通过自己定义的最小接口依赖基础设施。
- 公共方法接收的 `context.Context` 沿 repository、网络和缓存调用传播。
- 错误增加动作上下文时使用 `%w`；API 错误和日志遵守本目录相应规范。
- 修改 interface 后搜索所有实现、编译期断言、测试 stub 和 mock；`DEV_GUIDE.md` 将漏改 stub 列为已发生过的问题。
- 修改 Ent schema 或 Wire provider 后运行 `make generate`，审查并提交 `ent/`/`wire_gen.go` 的生成差异。

## 测试策略

测试与被测代码同 package/目录，主要使用标准 `testing` 和 `testify/require`。

| 类型 | 方式 | 代表文件 |
|------|------|----------|
| 纯逻辑单元测试 | 表驱动、stub/fake、`//go:build unit`（部分基础包无 tag） | `internal/pkg/errors/errors_test.go`, `internal/service/proxy_test.go` |
| HTTP handler/middleware | Gin test mode + `httptest`，断言状态和 body | `internal/handler/auth_wechat_oauth_test.go`, `internal/middleware/rate_limiter_test.go` |
| Repository 集成 | `//go:build integration`，真实 PostgreSQL/Redis/testcontainers | `internal/repository/user_repo_integration_test.go` |
| 迁移 | sqlmock 验证 runner，集成测试验证最终 schema/数据 | `migrations_runner_notx_test.go`, `migrations_schema_integration_test.go` |
| 跨模块 E2E | `//go:build e2e` | `internal/integration/e2e_user_flow_test.go` |

新增行为至少覆盖成功、输入/权限失败和关键下游失败。并发、幂等、缓存或事务改动还要覆盖重复调用、取消/回滚和竞争边界。优先使用可控时钟、channel 或显式状态，避免依赖脆弱的短时间 `Sleep`。

## 验证命令

从 `backend/` 运行：

```bash
go test ./...
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...
go build ./cmd/server
```

可先用 `go test -run TestName ./internal/<package>` 快速迭代，但交付前扩大到受影响 package；数据库、路由、共享接口或生成代码的改动应运行全量检查。`Makefile` 也提供 `make test-unit`、`make test-integration` 和 `make test-e2e-local`。

## 审查清单

- 层次和依赖方向没有被绕过，接口放在使用者一侧。
- handler 只处理协议，业务不变量在 service，持久化细节在 repository。
- 事务内所有操作使用同一 tx context/client，失败路径可回滚。
- 查询有界且避免明显 N+1；批量路径、分页和空输入行为明确。
- goroutine、ticker、HTTP body、rows、事务和其他资源都有停止/关闭路径。
- 用户输入、动态 URL、SQL 和日志字段经过对应的验证、参数化或脱敏。
- 生成差异与 schema/provider 改动一致，没有手工修改生成文件。
- 测试断言可观察行为，而不是只验证 mock 调用；失败消息足以定位原因。

## 禁止复制的模式

- 忽略关键返回错误、用 `_` 吞掉提交/写入失败，或用 panic 处理普通失败。
- 在新代码中继续扩散 `LegacyPrintf`、裸标准日志或非结构化大 payload。
- 为绕过 lint 在 handler/service 直接引入数据库实现。
- 修改历史迁移，或只改 schema 不更新迁移/生成代码。
- 在测试中依赖共享外部状态、未清理全局 logger/timer，或因本机无 Docker 就宣称集成测试通过。
