# 后端开发规范

> `backend/` 是 Go 1.26 服务，使用 Gin、Ent、PostgreSQL、Redis、Wire 和 zap。

## 使用方式

修改后端前，先根据改动范围阅读本目录中的专题规范。这里记录的是仓库当前已经采用的模式；旧代码中的例外不自动成为新代码范例。

| 规范 | 内容 |
|------|------|
| [目录结构](./directory-structure.md) | 分层、依赖方向、文件放置和命名 |
| [数据库](./database-guidelines.md) | Ent、SQL、事务、迁移和软删除 |
| [错误处理](./error-handling.md) | 业务错误、错误传播和 HTTP 响应 |
| [日志](./logging-guidelines.md) | zap、请求上下文、字段和脱敏 |
| [质量](./quality-guidelines.md) | 格式化、生成代码、测试和审查 |

## 开发前检查

- 确认改动属于 `handler`、`service`、`repository`、`server` 还是基础包，保持现有依赖方向。
- 修改接口前搜索全部实现、stub 和 mock；服务所需的 repository 接口通常定义在 `internal/service/`。
- 涉及表结构时同时检查 `ent/schema/`、`migrations/` 和相关 repository 集成测试。
- 涉及公开错误或日志时确认不会泄露数据库详情、上游响应、令牌或凭据。

## 完成检查

在 `backend/` 目录按改动范围运行：

```bash
gofmt -w <changed-go-files>
go test ./...
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...
```

修改 Ent schema 或 Wire provider 时先运行 `make generate`，并提交生成结果。集成测试依赖 PostgreSQL、Redis 或 Docker；环境不具备时应明确说明未运行的检查，不能把跳过当作通过。
