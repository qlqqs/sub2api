# 后端目录与依赖结构

## 适用范围

后端是单 Go module：`backend/go.mod` 的 module path 为
`github.com/Wei-Shaw/sub2api`。代码按运行时层和技术职责组织，而不是按每个业务功能建立
独立 module。新增功能通常会横跨 route、handler、service、repository 和 Ent schema/SQL
migration；不要创建虚构的 `src/` 层级。

## 目录地图

```text
backend/
├── cmd/server/                 # 主服务进程入口、Wire injector、进程生命周期
├── internal/
│   ├── server/                 # Gin/HTTP server 组装
│   │   ├── routes/             # URL、HTTP method、route group 与 middleware 链
│   │   └── middleware/         # JWT、API key、admin auth、日志、恢复、安全头等
│   ├── handler/                # HTTP 输入输出适配；admin/ 放管理端 handler，dto/ 放响应 DTO
│   ├── service/                # 业务模型、端口接口、业务编排、后台 worker
│   ├── repository/             # PostgreSQL/Ent、原生 SQL、Redis、外部 HTTP 等基础设施适配
│   ├── config/                 # 配置读取、默认值、校验和 Wire provider
│   ├── domain/                 # 跨层使用的状态、角色等领域常量
│   ├── payment/                # 支付 provider 适配及其 Wire provider
│   ├── pkg/                    # 可复用技术组件，不承载某个 handler 的业务流程
│   ├── setup/                  # 首次安装的 CLI/HTTP setup 流程
│   └── web/                    # 嵌入前端及静态页面 middleware
├── ent/
│   ├── schema/                 # 手写 Ent schema 与 mixin
│   └── ...                     # Ent 生成代码；根目录大部分文件不可手改
├── migrations/                 # 嵌入二进制并在启动时执行的 PostgreSQL SQL migrations
├── .golangci.yml               # 依赖边界、静态检查和 gofmt 规则
└── Makefile                    # build、generate、unit/integration/e2e 测试入口
```

参考入口：

- `backend/cmd/server/main.go`：`main`、`runSetupServer`、`runMainServer` 负责 flags、setup
  模式、启动、信号和优雅关闭，不放业务规则。
- `backend/cmd/server/wire.go`：`initializeApplication` 聚合各层 `ProviderSet`，
  `provideCleanup` 负责 worker 和基础设施的关闭顺序。
- `backend/internal/server/http.go`：`ProvideRouter` 和 `ProvideHTTPServer` 构造 Gin 与
  `http.Server`。
- `backend/internal/server/router.go`：`SetupRouter` 安装全局 middleware，`registerRoutes`
  把具体模块交给 `internal/server/routes`。

## 请求链与各层职责

典型调用链是：

```text
cmd/server -> internal/server/routes -> internal/handler -> internal/service
                                                     -> service 端口接口
internal/repository -> 实现 service 端口接口 -> Ent / database/sql / Redis / upstream HTTP
```

### Route 层

- URL、method、route group、鉴权 middleware 顺序放在 `backend/internal/server/routes/`。
  例如 `routes.RegisterAuthRoutes`、`routes.RegisterGatewayRoutes` 由
  `server.registerRoutes` 调用。
- route 文件可以做与路由选择直接相关的平台 gate，但不应复制 service 业务流程。
  `backend/internal/server/routes/gateway.go` 是多套兼容 URL 和平台 gate 的真实例子；
  `backend/internal/server/routes/gateway_test.go` 验证路径实际注册。
- 健康检查等无 service 依赖的小端点可直接写在 route 注册函数中；现实例外见
  `backend/internal/server/routes/common.go` 的 `RegisterCommonRoutes`。

### Handler 层

- Handler 负责 `gin.Context`、binding、header/cookie、HTTP 状态和 DTO 转换，然后调用
  service。`backend/internal/handler/auth_handler.go` 的 `AuthHandler.Register` 展示
  `ShouldBindJSON -> service -> response.ErrorFrom/Success` 的常见形状。
- 用户侧 handler 位于 `backend/internal/handler/`；管理端位于
  `backend/internal/handler/admin/`；传输 DTO 集中在 `backend/internal/handler/dto/` 或与
  小型 handler 相邻。
- 生产 handler 不直接 import `internal/repository`、GORM 或 Redis；这是
  `backend/.golangci.yml` 的 `handler-no-repository` 规则。某些 OAuth/身份流程仍通过
  service 暴露的 Ent client 执行紧耦合事务，例如 `auth_handler.go` 的 `entClient` 使用链；
  这是现状例外，不代表新 handler 应绕开 service 端口。

### Service 层

- 业务模型、错误、repository/cache/upstream 端口接口和业务编排主要位于
  `backend/internal/service/`。例如 `AuthService` 依赖同包的 `UserRepository` 等接口，
  而不是具体 `repository.userRepository`。
- 一个较大的功能会拆成同前缀的多个文件，而不是强制一个 service 一个文件；支付、OAuth、
  gateway 等均采用这种方式。后台 worker 的 `Start`/`Stop` 也在 service 层，并由
  `backend/internal/service/wire.go` provider 启动、由 `cmd/server/wire.go:provideCleanup`
  停止。
- `backend/.golangci.yml` 的 `service-no-repository` 禁止大多数 service import
  `internal/repository`、GORM 和 Redis。明确排除的是 `ops_aggregation_service.go`、
  `ops_alert_evaluator_service.go`、`ops_cleanup_service.go`、`ops_metrics_collector.go`、
  `ops_scheduled_report_service.go` 和 `service/wire.go`；这些历史/运维例外不得扩散到普通
  service。
- Service 并非完全与数据库技术隔离：部分跨 repository 原子操作直接持有 `*ent.Client`
  或 `*sql.DB`。例如 `SubscriptionService.withSubscriptionUpdateTx` 和
  `adminServiceImpl.AdminUpdateAPIKeyGroup` 使用 Ent transaction，运维服务也有原生 SQL。
  修改这些路径时保持其事务协议，不要把“service 不 import repository”误写成“service
  永远不能依赖 Ent/database/sql”。

### Repository 层

- `backend/internal/repository/` 包含数据库 repository、Redis cache、分布式锁、外部 HTTP
  client、备份适配和基础设施初始化；文件按职责使用 `_repo.go`、`_cache.go` 或具体适配器
  名称。
- 该层反向 import `internal/service` 是项目既有端口模式：构造函数返回 service 接口，实体
  在 Ent model 与 service model 之间转换，业务错误也定义在 service/pkg errors 边界。
  例如 `repository.NewUserRepository` 返回 `service.UserRepository`，
  `repository.ProviderSet` 将实现交给 Wire。
- PostgreSQL、Redis 和 Ent 初始化也在此层：`repository.InitEnt`、`ProvideSQLDB`、
  `ProvideRedis` 位于 `ent.go`/`wire.go`。数据库细节见 `database-guidelines.md`。

### Server 组装层的现实例外

`internal/server` 不只是纯 handler dispatcher。`ProvideRouter`/`SetupRouter` 还直接接收
`SettingService`、`APIKeyService`、`OpsService` 和 `*redis.Client`，用于启动时配置、动态
websearch、route middleware 和 rate limit 组装（见 `backend/internal/server/http.go` 与
`router.go`）。新增普通业务逻辑仍应下沉到 service；不要据此把数据库查询写进 server。

## Wire 与生成代码

- 每层 provider 集中在真实的 `wire.go`：
  `internal/config/wire.go`、`internal/repository/wire.go`、
  `internal/service/wire.go`、`internal/payment/wire.go`、
  `internal/server/middleware/wire.go`、`internal/handler/wire.go` 和
  `internal/server/http.go`。
- 新增 constructor 后，把它加入所属层 `ProviderSet`；聚合 handler 则同步更新
  `handler.Handlers`/`AdminHandlers` 及 `ProvideHandlers`/`ProvideAdminHandlers`。
- `backend/cmd/server/wire.go` 带 `wireinject` build tag，是手写 injector；
  `backend/cmd/server/wire_gen.go` 首行是 `Code generated by Wire. DO NOT EDIT.`，禁止手改。
  使用 `make -C backend generate`（其调用 `go generate ./cmd/server`）重新生成。
- `backend/ent/generate.go` 是手写生成入口；`backend/ent/schema/**` 是手写模型。其余带
  `Code generated by ent, DO NOT EDIT.` 的 `backend/ent/**` 文件由
  `go generate ./ent` 生成，不在生成文件中修 bug。当前生成特性包括
  `sql/upsert`、`intercept`、`sql/execquery`、`sql/lock` 和 `int64` ID。

## 文件与测试命名

- Go package/file 使用小写；多词文件使用 snake_case。常见后缀为 `_handler.go`、
  `_service.go`、`_repo.go`、`_cache.go`、`_test.go`。
- 测试与被测 package 相邻，不建立独立 `tests/` 树。无 build tag 的回归测试由普通
  `go test ./...` 运行；较重测试使用 `//go:build unit`、`integration` 或 `e2e`。
- HTTP 路由契约可参考 `backend/internal/server/api_contract_test.go`；handler stub 测试可参考
  `backend/internal/handler/user_handler_test.go`；service 规则测试可参考
  `backend/internal/service/auth_service_test.go`；真实 PostgreSQL/Redis 集成环境由
  `backend/internal/repository/integration_harness_test.go` 使用 testcontainers 建立。
- 标准入口来自 `backend/Makefile`：`test`、`test-unit`、`test-integration`、`test-e2e`。
  根 `Makefile:test-backend` 只是转发到 backend Makefile。

## 新功能落位清单

1. 在 `internal/server/routes/<feature>.go` 注册 URL，并明确 middleware 顺序。
2. 在 `internal/handler/` 或 `internal/handler/admin/` 增加薄 HTTP 适配；需要稳定传输结构时
   使用 `handler/dto`。
3. 在 `internal/service/` 定义业务模型、错误、端口接口和编排；不要让 service import 具体
   repository。
4. 在 `internal/repository/` 实现持久化/cache/upstream 端口；如涉及表结构，同时更新
   `ent/schema` 和新增 forward SQL migration。
5. 把 constructor 加入对应 `ProviderSet`，重新生成 Wire/Ent 代码。
6. 在被修改层旁增加最小有效测试；数据库事务和 migration 优先用 repository unit/integration
   测试锁定。

## 反模式

- 不在 `cmd/server/main.go`、route 注册函数或 handler 中写可复用业务/数据库流程。
- 不让生产 handler 直接构造 repository、Redis client 或 SQL connection。
- 不让普通 service import `internal/repository`；运维例外名单不是新功能的通行证。
- 不手改 `wire_gen.go` 或带 Ent 生成标记的文件。
- 不创建与现有层平行的 `backend/src`、`controllers`、`dao` 等新目录来表达同一职责。
- 不因单个文件较大就跨 package 拆出循环依赖；本项目优先在同一 package 以内按功能前缀拆文件。
