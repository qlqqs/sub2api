# sub2api 跨层思考指南

> 目的：在修改 API、持久化字段、权限、错误或页面数据流前，沿真实链路确认每一层的输入、输出和责任，避免边界两侧各自假设。

## 先画真实数据流

跨层功能开始前，列出本次会经过的文件，不要只写抽象层名。用户“可用渠道”是现成参考：

```text
backend/internal/server/routes/common.go
  -> backend/internal/handler/available_channel_handler.go
  -> backend/internal/service/channel_available.go
  -> backend/internal/repository/channel_repo.go 与分组 repository
  -> backend/internal/pkg/response/response.go
  -> frontend/src/api/channels.ts
  -> frontend/src/views/user/AvailableChannelsView.vue
  -> frontend/src/router/index.ts
```

这条链路中，repository 读取持久化数据，service 组合渠道、分组和定价，handler 依据认证主体过滤可见数据并构造公开 DTO，`response` 统一 HTTP 输出，前端 API 模块声明同一响应类型，view 负责页面展示，router 负责入口和访问元数据。新增功能应先确认是否沿用这条职责分配。

## 逐边界确认契约

| 边界 | 必须确认 | sub2api 中的落点 |
| --- | --- | --- |
| 路由 -> handler | 方法、路径、中间件、认证主体和参数来源 | `backend/internal/server/routes/`、`backend/internal/server/middleware/` |
| handler -> service | 输入是否已完成 HTTP 解析，service 是否仍保持传输层无关 | `backend/internal/handler/`、`backend/internal/service/` |
| service -> repository | 事务、查询范围、排序、空值和错误包装 | `backend/internal/service/channel_available.go`、`backend/internal/repository/` |
| schema -> migration -> repository | 列名、类型、默认值、约束和旧数据兼容 | `backend/ent/schema/`、`backend/migrations/`、`backend/internal/repository/` |
| backend -> frontend API | JSON 字段名、可空性、数组形态、数值单位和错误状态 | handler DTO、`frontend/src/api/` |
| API/store/composable -> view | 缓存归属、并发请求、加载/错误状态和刷新时机 | `frontend/src/stores/`、`frontend/src/composables/`、`frontend/src/views/` |
| router -> view | 路由名、懒加载、认证/管理员/功能开关元数据 | `frontend/src/router/index.ts` |

每个边界至少回答：输入是什么、输出是什么、谁验证、谁转换、失败时返回什么、空值如何表示。不能回答时先读实现，不要用“应该如此”补全。

## 后端边界规则

### Handler 负责 HTTP 与公开 DTO

`backend/internal/handler/available_channel_handler.go` 从 middleware 取得认证主体，调用 service，再通过字段白名单隐藏内部 ID、状态和管理字段。类似用户接口必须继续在 handler 明确公开字段，不能直接序列化 repository 或 Ent 实体。

handler 统一使用 `backend/internal/pkg/response/response.go` 的 `Success`、`ErrorFrom` 等入口。业务失败由 service/repository 返回可包装的错误，handler 不应根据错误字符串临时决定状态码，也不应为单个接口创造新的响应外壳。

### Service 负责业务组合

`backend/internal/service/channel_available.go` 同时读取渠道和活跃分组，执行排序、支持模型计算与定价回落。此类跨 repository 规则属于 service。handler 只为不同受众做认证、请求解析和 DTO 过滤；repository 不应知道页面展示语义。

service 包装下层错误时保留原始错误链，例如使用 `%w`。这样 `response.ErrorFrom` 仍可识别错误类别，日志也能保留原因。

### Repository 与 migration 负责持久化事实

查询和写入集中在 `backend/internal/repository/`，数据库结构由 `backend/ent/schema/` 与 `backend/migrations/` 的现有机制共同演进。新增持久化字段时必须同时检查：

1. migration 对现有数据的默认值、回填、约束和索引是否安全；
2. Ent schema 的类型、optional/nillable/default 是否表达同一语义；
3. repository 的创建、更新、扫描和筛选是否覆盖新字段；
4. service 与 handler 是否真的需要暴露该字段，不能因为数据库新增就自动透传。

不要修改已执行迁移来伪装新状态；按 `backend/migrations/` 的编号与现有约定新增迁移。不要在 handler 或 service 中散落原始 SQL 绕过 repository。

## 前后端契约规则

`backend/internal/handler/available_channel_handler.go` 使用 snake_case JSON tag；`frontend/src/api/channels.ts` 以相同字段名声明 `UserAvailableChannel`、`UserChannelPlatformSection` 和定价类型。改变任一字段时必须同时核对两端，尤其注意：

- Go 指针价格字段会输出 JSON `null`，TypeScript 对应 `number | null`；
- handler 应初始化需要稳定返回的切片，避免前端在 `[]` 与 `null` 之间猜测；
- 金额、倍率、token 数、时间和 ID 不能只凭 TypeScript 的 `number` 判断单位与含义；
- handler 的字段白名单是安全边界，前端类型不能要求仅存在于 service/repository 的内部字段；
- 排序属于哪一层要明确；不要让 backend 和 view 各自做不同的“最终排序”。

前端请求和响应类型放在对应的 `frontend/src/api/` 模块。view、store 和 composable 导入这些类型，不各自复制接口。`frontend/src/api/client.ts` 负责共享 HTTP 行为时，业务 API 不应绕过它另建请求客户端。

## 前端状态与导航边界

- `frontend/src/stores/subscriptions.ts` 展示了 Pinia 对跨页面状态、缓存 TTL、并发请求去重和轮询生命周期的管理。只有同类共享状态才进入 store。
- `frontend/src/composables/useBatchImageAccess.ts` 封装可复用的访问判断和响应式状态。与组件生命周期或复用交互相关的逻辑放 composable，不把它复制进多个 view。
- `frontend/src/views/user/AvailableChannelsView.vue` 等 view 负责页面编排、局部加载状态和展示，不重新定义后端业务规则。
- `frontend/src/router/index.ts` 集中声明路由、懒加载组件、标题和权限元数据。路由守卫改善前端访问体验，但不能替代后端 `middleware` 和 handler 的授权检查。

新增页面时同时检查 API 模块、可选的 store/composable、view、router，以及实际导航入口。仅注册 route 而遗漏菜单，或只隐藏菜单而保留无后端授权的接口，都不是完整实现。

## 错误、权限与空状态

跨层错误应保持层次：repository 提供持久化原因，service 添加业务上下文，handler 交给统一 response 映射，前端 API/client 传播结构化失败，view 决定用户提示。禁止通过匹配英文错误字符串在前端推导业务类型。

权限至少检查两处：后端路由/middleware/handler 是强制边界，前端 router/view 是体验边界。“可用渠道”handler 在返回前按用户可访问分组和平台再次过滤，说明读取成功不等于可以完整暴露实体。

空结果、未配置和失败必须区分。例如空渠道列表应是成功的空数组；可空价格是 `null`；未认证由统一错误响应处理；不能把请求失败吞掉后伪装成“没有数据”，除非现有产品行为明确如此。

## 修改后的端到端检查

### API 或业务字段

- [ ] 已从 `backend/internal/server/routes/` 追到 handler、service、repository
- [ ] 已确认公开 DTO，而不是直接暴露 Ent/repository 对象
- [ ] JSON 字段、可空性、数组、单位和排序与 `frontend/src/api/` 一致
- [ ] store/composable/view 没有重复解析或重写同一业务规则
- [ ] 错误经 service 包装后仍可由 `response.ErrorFrom` 正确处理

### 持久化字段

- [ ] 已检查 `backend/migrations/` 与 `backend/ent/schema/` 的一致性
- [ ] 已考虑旧数据、默认值、约束、索引和回滚失败风险
- [ ] repository 的读写路径都覆盖新字段
- [ ] 只有确需公开的字段进入 handler DTO 和前端 API 类型

### 页面、权限或导航

- [ ] 已检查 `frontend/src/router/index.ts` 与实际导航入口
- [ ] 前端守卫和显示条件不被当作后端授权
- [ ] loading、empty、error、unauthorized、forbidden 状态可区分
- [ ] 多个消费者共享的缓存、请求去重或访问判断已放到 store/composable

### 验证

- [ ] 覆盖至少一个正常链路和一个边界状态
- [ ] 对 `null`、空数组、缺省配置、无权限和下层错误做了针对性验证
- [ ] 修改后搜索旧字段、旧路由和旧错误用法，确认没有遗漏
- [ ] 重新读取相关实现，确认文档、代码和实际落盘结果一致
