# 前端开发规范

> `frontend/` 是 Vue 3 + TypeScript + Vite 单页应用，使用 Pinia、Vue Router、vue-i18n、Axios、Tailwind CSS 和 Vitest。

## 使用方式

根据改动范围阅读对应专题；规范以当前源码和测试为依据，并显式记录现有宽松配置，不把它们误写成推荐模式。

| 规范 | 内容 |
|------|------|
| [目录结构](./directory-structure.md) | 页面、组件、API、状态、类型和工具的归属 |
| [组件](./component-guidelines.md) | SFC、props/emits、组合、样式、i18n 与无障碍 |
| [组合式函数](./hook-guidelines.md) | composable 边界、请求取消、资源清理和测试 |
| [状态管理](./state-management.md) | 本地、Pinia、服务端缓存、URL 与持久状态 |
| [类型安全](./type-safety.md) | strict TypeScript、API 类型、运行时校验和 `any` 边界 |
| [质量](./quality-guidelines.md) | lint、类型检查、测试、构建和安全审查 |

## 开发前检查

- 先搜索 `components/common/`、`components/layout/` 和 `composables/`，避免重复实现已有交互。
- 明确状态属于组件、URL、Pinia 还是 API 缓存，并确认卸载/登出后的清理责任。
- API 变更同时核对后端 response envelope、`api/client.ts` 解包行为及 `src/types/`。
- 所有用户可见文本优先进入 `i18n/locales/zh` 与 `i18n/locales/en`，并保持 key 对齐。

## 完成检查

从 `frontend/` 运行：

```bash
pnpm lint:check
pnpm typecheck
pnpm test:run
pnpm build
```

`package.json` 变化后必须用 pnpm 更新并提交 `pnpm-lock.yaml`。覆盖率任务 `pnpm test:coverage` 的全局阈值为 statements/branches/functions/lines 各 80%。
