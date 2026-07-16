# 后端开发规范

本目录记录 `backend/` Go 服务当前采用的工程约定。规则以真实源码、测试、构建脚本和 lint 配置为依据；实施改动前先读取与目标层相关的规范，冲突时以可执行代码和配置为准并同步更新规范。

## Pre-Development Checklist

- [ ] 在本 fork 上做自定义功能或修改上游核心路径前，阅读[二次开发与上游同步约束](../guides/secondary-development.md)。
- [ ] 新增或移动后端文件前，阅读[目录结构](./directory-structure.md)，确认 Route、Handler、Service、Repository、基础设施和生成代码的职责边界。
- [ ] 涉及 Ent schema、Repository、事务、Redis 或 SQL migration 时，同时阅读[数据库规范](./database-guidelines.md)和[质量规范](./quality-guidelines.md)，确认生成、迁移与测试要求。
- [ ] 涉及失败路径、HTTP envelope、网关协议、流式响应或 panic 边界时，阅读[错误处理](./error-handling.md)。
- [ ] 涉及 request ID、上游错误体、结构化字段或敏感数据时，阅读[日志规范](./logging-guidelines.md)。
- [ ] 修改前搜索现有接口、构造函数、错误、migration 和测试，优先沿用项目现有边界，不扩散文档中标记的历史例外。
- [ ] 涉及上游账号余额查询、平台协议适配、余额专用凭证或管理端余额 DTO 时，阅读[上游账号余额查询](./upstream-balance.md)。

## 规范索引

| 规范 | 内容 | 状态 |
|---|---|---|
| [目录结构](./directory-structure.md) | Handler、Service、Repository、基础包与生成代码的职责和放置位置 | 已完成 |
| [数据库规范](./database-guidelines.md) | Ent、事务、Repository、SQL 迁移及缓存访问模式 | 已完成 |
| [错误处理](./error-handling.md) | `ApplicationError`、持久化错误翻译、HTTP/协议响应、错误链与 Recovery | 已完成 |
| [日志规范](./logging-guidelines.md) | Zap/slog/stdlog 桥接、请求关联、access log、结构化字段与脱敏 | 已完成 |
| [质量规范](./quality-guidelines.md) | golangci-lint、依赖边界、build tags、Testcontainers、迁移测试和代码生成 | 已完成 |
| [上游账号余额查询](./upstream-balance.md) | `POST .../upstream-balance`、协议选择、错误分类、敏感凭证与只读边界 | 已完成 |

## 使用顺序

1. 涉及包归属或跨层依赖时，先读[目录结构](./directory-structure.md)。
2. 涉及 Ent schema、Repository、事务、Redis 或迁移时，同时读[数据库规范](./database-guidelines.md)和[质量规范](./quality-guidelines.md)。
3. 涉及 Handler/Service 失败路径、网关协议或 panic 边界时，读[错误处理](./error-handling.md)。
4. 涉及可观测性、请求 ID、上游错误体或敏感数据时，读[日志规范](./logging-guidelines.md)。
5. 涉及上游余额查询、平台探测或余额专用凭证时，读[上游账号余额查询](./upstream-balance.md)，并同时核对[错误处理](./error-handling.md)与[日志规范](./logging-guidelines.md)的脱敏与 envelope 约束。
6. 提交前按[质量规范](./quality-guidelines.md)选择正确的默认、unit、integration 或 e2e 命令，并确认生成代码与迁移测试已同步。

## Quality Check

- [ ] 对修改包先运行最窄的 `go test`；跨层后端改动运行 `make -C backend test`，同时覆盖默认测试与 golangci-lint。
- [ ] 需要 unit build tag 时运行 `make -C backend test-unit`；涉及真实 PostgreSQL、Redis、事务或 migration 语义时运行 `make -C backend test-integration`，并确认 Docker 环境可用且测试未被跳过。
- [ ] 修改 Ent schema、ProviderSet、构造函数或 Wire binding 后运行 `make -C backend generate`，检查生成 diff 只包含预期内容。
- [ ] 新增 migration 时核对完整文件名字典序、checksum 不可变性和 `*_notx.sql` 限制，并运行相关 migration runner 与 schema integration 测试。
- [ ] 按[错误处理](./error-handling.md)与[日志规范](./logging-guidelines.md)复核客户端响应、错误链、重复日志和敏感信息脱敏。

文档默认使用中文；代码标识、命令、字段名和协议名称保持源码原文。
