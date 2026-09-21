# 后端目录结构

## 分层总览

后端是单个 Go module，入口位于 `backend/cmd/server/`，依赖由 Google Wire 装配。HTTP 请求的常规路径是：

```text
server/routes -> handler -> service-owned interface -> repository -> Ent/PostgreSQL/Redis
```

`backend/.golangci.yml` 的 `depguard` 明确禁止 handler 直接依赖 repository，也禁止绝大多数 service 直接导入 repository、GORM 或 Redis。少数已列出的 ops/Wire 文件是历史例外，不应扩展为新模式。

## 目录职责

| 路径 | 职责 | 代表文件 |
|------|------|----------|
| `cmd/server/` | 进程入口、Wire 装配、启动与清理 | `main.go`, `wire.go`, `wire_gen.go` |
| `ent/schema/` | 手写 Ent schema、字段、索引和 mixin | `user.go`, `group.go`, `mixins/soft_delete.go` |
| `ent/` 其余内容 | Ent 生成代码，不手工编辑 | `user_create.go`, `user_query.go` |
| `internal/config/` | 配置结构、默认值、加载与校验 | `config.go`, `validate_dingtalk.go` |
| `internal/domain/` | 跨服务使用的领域值对象和常量 | `constants.go`, `openai_messages_dispatch.go` |
| `internal/handler/` | Gin 边界：绑定、鉴权上下文、DTO 转换、响应 | `user_handler.go`, `api_key_handler.go` |
| `internal/handler/admin/` | 管理端 HTTP handlers | `user_handler.go`, `channel_handler.go` |
| `internal/handler/dto/` | service model 与 API JSON 之间的 DTO 映射 | `types.go`, `mappers.go` |
| `internal/service/` | 业务规则、用例编排以及依赖端口接口 | `user_service.go`, `proxy_service.go` |
| `internal/repository/` | Ent/SQL/Redis/外部基础设施的具体实现 | `user_repo.go`, `migrations_runner.go` |
| `internal/server/routes/` | 路由分组与 middleware 挂载 | `common.go`, `admin.go`, `user.go` |
| `internal/server/middleware/` | 服务级 HTTP middleware | `request_logger.go` |
| `internal/pkg/` | 可复用、边界清晰的基础包 | `errors/`, `logger/`, `pagination/` |
| `internal/util/` | 较小的技术工具和安全辅助函数 | `logredact/`, `urlvalidator/` |
| `internal/integration/` | 跨模块 E2E 场景 | `e2e_gateway_test.go` |
| `migrations/` | 嵌入二进制并在启动时执行的前向 SQL 迁移 | `migrations.go`, `181_group_duplicate_operation_id.sql` |

## 新功能的放置方式

1. 在 `internal/service/` 定义输入、领域模型、业务错误和最小依赖接口。接口靠近使用者，而不是靠近 repository 实现。
2. 在 `internal/repository/` 实现该接口，并在相应 `wire.go`/`ProviderSet` 中注册构造函数。
3. 在 `internal/handler/` 或 `internal/handler/admin/` 处理协议细节；复用 `handler/dto` 和 `internal/pkg/response`。
4. 在 `internal/server/routes/` 注册端点和正确的认证、限流、中间件链。
5. 测试与被测包同目录；需要真实 PostgreSQL/Redis 的用例使用 integration build tag。

`UserService`/`userRepository` 是完整纵向样例：端口在 `internal/service/user_service.go`，实现位于 `internal/repository/user_repo.go`，HTTP 边界位于 `internal/handler/user_handler.go` 与 `internal/handler/admin/user_handler.go`。`ProxyService`/`proxyRepository` 展示了相同的依赖反转和事务复用方式。

## 命名约定

- Go package 使用简短小写名称；文件通常使用 `snake_case.go`，测试为 `*_test.go`。
- 导出构造函数使用 `New<Type>`；repository 的具体 struct 通常不导出，例如 `userRepository`。
- service 接口按能力命名，例如 `UserRepository`、`APIKeyAuthCacheInvalidator`，避免向业务层暴露具体数据库类型。
- handler 请求/响应 DTO 放在 handler 或 `handler/dto`，不要直接把 Ent entity 作为 API 合约。
- 平台或协议专属的大型流程可以拆文件，但仍留在拥有该行为的包内，例如 `openai_gateway_service.go` 与 `openai_ws_v2/`。

## 不要这样做

- 不要让 handler 直接查询 Ent、SQL 或 Redis。
- 不要从 service 导入 `internal/repository`；新增能力应先定义 service-owned port。
- 不要把业务规则塞进路由注册、Wire provider 或通用 util。
- 不要手改 `backend/ent/` 下的生成文件或 `cmd/server/wire_gen.go`。
- 不要为了一个局部 helper 创建跨层“万能”包；先放在拥有该行为的现有包中。
