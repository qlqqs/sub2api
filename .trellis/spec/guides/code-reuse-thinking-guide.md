# sub2api 代码复用思考指南

> 目的：在新增 handler、service、repository、页面或状态逻辑前，先确认现有代码由谁负责，优先复用项目已有边界和实现。

## 先搜索，再决定

不要按函数名猜测。先用代码搜索确认同一业务对象在哪些层出现，并阅读相邻实现：

- 后端入口与返回：`backend/internal/handler/`、`backend/internal/pkg/response/response.go`
- 业务规则：`backend/internal/service/`
- 持久化：`backend/internal/repository/`、`backend/ent/schema/`、`backend/migrations/`
- 前端请求契约：`frontend/src/api/`
- 共享状态与交互逻辑：`frontend/src/stores/`、`frontend/src/composables/`
- 页面装配与导航：`frontend/src/views/`、`frontend/src/router/index.ts`

搜索时同时查业务名、JSON 字段、路由路径、错误文案和配置键。修改一个常量或字段前，也要搜索旧值的全部引用，避免只改生产者或只改消费者。

## 按现有职责复用

### 后端

| 位置 | 当前职责 | 复用原则 |
| --- | --- | --- |
| `backend/internal/handler/available_channel_handler.go` | 读取认证上下文、调用 service、过滤对外字段并通过 `response` 返回 | handler 可以拥有 HTTP DTO 和字段白名单；不要在另一个 handler 重写渠道查询或计价规则 |
| `backend/internal/service/channel_available.go` | 组合渠道、分组和定价，形成业务视图 | 跨 repository 的业务组合留在 service；其他入口需要同一规则时调用或抽取这里的能力 |
| `backend/internal/repository/channel_repo.go`、`backend/internal/repository/api_key_repo.go` | 封装 Ent/SQL 查询和持久化细节 | 先扩展现有 repository；不要让 handler 或前端语义渗入查询层 |
| `backend/migrations/` 与 `backend/ent/schema/` | 保存数据库结构演进和 Ent schema | 持久化字段只有一个结构事实来源；不要在 service 中用临时表结构或重复 SQL 模拟迁移 |
| `backend/internal/pkg/response/response.go` | 统一成功和错误 HTTP 响应 | handler 复用 `response.Success`、`response.ErrorFrom` 等入口，不自行拼另一套响应壳或状态码映射 |

例如“可用渠道”链路已经由 `AvailableChannelHandler.List` 委托 `ChannelService.ListAvailable`。新增管理端或用户端展示时，应判断是复用 service 结果、增加明确的 service 方法，还是仅新增不同的 handler DTO；不应复制 `ListAvailable` 中的分组、排序和定价回落逻辑。

### 前端

| 位置 | 当前职责 | 复用原则 |
| --- | --- | --- |
| `frontend/src/api/channels.ts` | 定义用户可用渠道的 TypeScript 契约并调用后端 | 同一响应结构只在 API 模块定义一次；view 不重复声明接口或直接调用 `apiClient` |
| `frontend/src/stores/subscriptions.ts` | 管理跨页面订阅状态、缓存、并发请求去重和轮询 | 只有真正跨页面且需要生命周期管理的状态进入 Pinia；页面局部状态不为“统一”而迁入 store |
| `frontend/src/router/index.ts` | 集中定义懒加载路由、路由元数据和守卫 | 新页面复用现有 `meta` 与守卫约定，不在 view 中再实现一套导航权限判断 |
| `frontend/src/views/user/AvailableChannelsView.vue` | 组合 API 数据、页面状态和展示组件 | view 负责页面编排；可复用的数据获取或交互行为放入 API、store 或 composable，而不是复制到另一页面 |
| `frontend/src/composables/useBatchImageAccess.ts` | 复用批量图片访问判断、请求去重和响应式状态 | 当多个组件需要同一交互能力时复用 composable；不要只为包装一行表达式创建 composable |

## 何时抽取

优先抽取以下内容：

- 同一业务规则已经在两个入口出现，下一次改动容易遗漏；
- 同一外部或跨层 payload 被多个消费者分别解析；
- 缓存、请求去重、轮询、排序或状态转换包含边界条件；
- 同一响应、错误或权限规则需要保持一致；
- 同一组常量代表稳定的业务词汇。

抽取位置由数据所有者决定：HTTP 契约靠近 handler/API 模块，业务规则在 service，查询在 repository，跨组件行为在 composable，跨页面状态在 store。不要建立一个无边界的 `utils` 文件来隐藏职责。

以下情况通常先保留局部实现：

- 只出现一次且明显属于当前页面或函数；
- 两段代码表面相似，但业务变化方向不同；
- 一行直接表达比新抽象更清楚；
- 抽取后需要大量布尔参数或分支才能兼容调用方。

“出现三次才抽取”不是硬规则。错误映射、权限、金额、配额、计费和请求去重即使只重复两次，也应优先收敛，因为分歧成本高。

## 批量修改后的反查

修改字段、枚举、路由、配置键或数据库列后，重新搜索旧名称和新名称，并沿真实链路检查：

1. migration 与 Ent schema 是否仍由同一结构语义驱动；
2. repository、service、handler 是否各自只做本层转换；
3. Go JSON tag 与 `frontend/src/api/` 中的字段和可空性是否一致；
4. store、composable、router、view 是否仍引用共享实现；
5. 相邻测试是否覆盖了共享规则，而不是每个消费者各复制一套断言。

## 完成前检查

- [ ] 已搜索相同业务名、字段、路由、错误和配置键
- [ ] handler 没有复制 service/repository 逻辑
- [ ] service 没有绕过 repository 直接散落查询
- [ ] handler 复用了 `backend/internal/pkg/response/response.go` 的响应入口
- [ ] migration、Ent schema 与持久化代码没有描述互相冲突的结构
- [ ] 前端 API 类型没有在 store、composable 或 view 中重复定义
- [ ] 全局状态、可复用交互和页面局部状态放在合适位置
- [ ] 没有为了消除少量重复而引入更难理解的通用抽象
- [ ] 批量修改后已搜索旧值，确认没有遗漏消费者
