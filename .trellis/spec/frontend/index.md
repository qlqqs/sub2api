# 前端开发规范索引

本目录记录 `frontend/` 当前真实使用的 Vue 3、Pinia、Vue Router、Axios、TypeScript 与
Vitest 约定。规范来自源码、配置和测试，不是通用框架模板。实现或评审前按变更范围读取对应文件；跨层变更通常需要同时读取多份规范。

## Pre-Development Checklist

- [ ] 在本 fork 上做自定义功能或修改上游核心路径前，阅读[二次开发与上游同步约束](../guides/secondary-development.md)。
- [ ] 按变更落点先读下方对应规范；新增或移动文件先核对
  [目录结构](./directory-structure.md)，组件与共享逻辑分别核对
  [组件规范](./component-guidelines.md) 和 [Composable 规范](./hook-guidelines.md)。
- [ ] 身份、持久化、缓存或轮询变更同时阅读 [状态管理](./state-management.md)、
  [API 访问规范](./api-guidelines.md) 和 [类型安全](./type-safety.md)，确认公共接口、运行时
  边界及失效语义。
- [ ] 页面入口、权限或 feature gate 变更先核对 [路由规范](./router-guidelines.md)，并追踪
  route meta、守卫、store 与 API 的完整数据流。
- [ ] 写测试或选择验证命令前先读 [质量规范](./quality-guidelines.md)，确认生产源码 typecheck
  与 Vitest 测试编译是不同边界。

## 规范列表

| 规范 | 适用范围 | 状态 |
|---|---|---|
| [目录结构](./directory-structure.md) | `src` 分层、功能目录、命名与放置位置 | 已完成（源码核验） |
| [组件规范](./component-guidelines.md) | Vue SFC、props/emits、组合、样式与可访问性 | 已完成（源码核验） |
| [Composable 规范](./hook-guidelines.md) | `use*` composable、共享响应式逻辑、清理与请求模式 | 已完成（源码核验） |
| [状态管理](./state-management.md) | Pinia setup store、局部/URL 状态、持久化、缓存与去重 | 已完成（源码核验） |
| [路由规范](./router-guidelines.md) | route meta、权限/功能守卫、懒加载、预取与路由测试 | 已完成（源码核验） |
| [API 访问规范](./api-guidelines.md) | Axios client、拦截器、refresh、响应解包、错误与 admin API | 已完成（源码核验） |
| [类型安全](./type-safety.md) | strict TypeScript、类型分布、泛型、`unknown`/`any`、运行时边界 | 已完成（源码核验） |
| [质量规范](./quality-guidelines.md) | ESLint、typecheck 边界、Vitest、coverage 与 Makefile 门禁 | 已完成（源码核验） |

## 按任务选择

- 新页面或组件：目录结构 + 组件规范；提取共享逻辑时再读 Composable 规范。
- 身份、全局设置、缓存或轮询：状态管理；涉及请求时同时读 API 访问规范。
- 新增页面入口、权限或 feature gate：路由规范 + 状态管理。
- 新增接口或调整错误处理：API 访问规范 + 类型安全。
- 上游账号余额列/批量刷新/余额专用凭证：API 访问规范 + 状态管理 + [backend 上游余额契约](../backend/upstream-balance.md)。
- 提交前验证或测试变更：质量规范；特别注意主 `tsconfig.json` 明确排除测试文件。
  `make test-frontend` 会执行 lint、生产源码 typecheck 和 critical tests，但其中 Vitest 部分
  只运行六个指定文件，并非完整 Vitest 套件。

## Quality Check

- [ ] 运行 `pnpm --dir frontend run lint:check` 检查 ESLint。
- [ ] 运行 `pnpm --dir frontend run typecheck` 检查生产 `src` 下的 TypeScript/Vue 源码；该
  命令不覆盖被主 `tsconfig.json` 排除的测试文件。
- [ ] 对改动涉及的测试运行聚焦 Vitest；需要完整前端测试套件时运行
  `pnpm --dir frontend run test:run`，不要用 critical tests 代替完整回归。
- [ ] 若运行 `make test-frontend`，按其真实门禁解读结果：它依次执行 `lint:check`、生产源码
  `typecheck`，再执行 `test-frontend-critical`；其中 Vitest 部分仅运行 Makefile 列出的六项
  critical tests，不是完整 Vitest 套件。命令选择和覆盖范围详见
  [质量规范](./quality-guidelines.md)。

规范默认使用中文书写；代码符号、文件路径和工具命令保留仓库原文，便于直接检索与执行。
