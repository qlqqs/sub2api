# sub2api 思考指南

这些指南用于在编码前确认复用边界和端到端数据流。它们补充 backend、frontend 规范，不替代各层的具体约定。

## 指南

| 指南 | 适用场景 |
| --- | --- |
| [代码复用思考指南](./code-reuse-thinking-guide.md) | 新增 handler、service、repository、API、store、composable 或 view 前，判断是否已有实现可复用 |
| [跨层思考指南](./cross-layer-thinking-guide.md) | 功能跨越路由、handler、service、repository、migration、前端 API、状态或页面时，核对完整契约 |
| [二次开发与上游同步约束](./secondary-development.md) | fork 上开发自定义功能、改上游核心文件、合入 custom 自检、或执行 upstream sync 前 |

## 触发条件

出现以下任一情况时，先读代码复用指南：

- 正在复制已有查询、校验、排序、错误映射、权限或状态逻辑；
- 同一 JSON 字段、响应类型、常量或配置键出现在多个位置；
- 准备创建新的工具函数、store 或 composable；
- 一个字段或枚举需要批量修改多个消费者。

出现以下任一情况时，先读跨层指南：

- API 请求或响应字段发生变化；
- 新增数据库字段、migration、索引或约束；
- 功能同时涉及 backend 与 frontend；
- 修改认证、授权、错误、空值、金额、倍率、时间或分页语义；
- 新页面需要 API、store/composable、router 和导航入口协同。


出现以下任一情况时，先读二次开发与上游同步约束：

- 在本 fork 的  或  上叠加自定义功能；
- 准备修改上游已有核心文件，或新增自定义 migration / 依赖；
- 开始  升级或合入  前的分叉点自检。

## 使用方式

1. 修改前搜索业务名、字段名、路由、错误和配置键；
2. 列出本次涉及的真实文件和每层职责；
3. 按对应指南检查复用位置、边界契约和失败路径；
4. 修改后搜索旧值，并重新读取相关文件确认落盘。

核心原则：记录并遵循 sub2api 当前存在的实现边界，不用泛化模板代替代码事实。
