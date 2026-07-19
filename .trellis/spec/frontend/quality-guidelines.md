# 前端质量规范

## 工具链基线

前端命令以 `frontend/package.json` 为准：

| 目标 | 命令 | 实际行为 |
|---|---|---|
| 非修改式 lint | `pnpm --dir frontend run lint:check` | ESLint 扫描 Vue/JS/TS 等文件，不写回 |
| 自动修复 lint | `pnpm --dir frontend run lint` | 同一范围执行 `eslint --fix`，会修改文件 |
| 生产源类型检查 | `pnpm --dir frontend run typecheck` | `vue-tsc --noEmit`；排除测试 |
| 完整 Vitest 单次运行 | `pnpm --dir frontend run test:run` | `vitest run`，运行 config include 的全部测试 |
| 交互/监听测试 | `pnpm --dir frontend run test` | `vitest`，会持续监听，不适合作为一次性质量门 |
| 覆盖率 | `pnpm --dir frontend run test:coverage` | V8 coverage + 阈值检查 |
| 构建 | `pnpm --dir frontend run build` | `vue-tsc -b && vite build` |

自动化或提交前检查优先使用非监听、非修改式命令；不要用 `lint` 代替 `lint:check` 后又忽略它的自动写回。

## ESLint 的真实约束

`frontend/.eslintrc.cjs` 使用 `vue-eslint-parser`、`@typescript-eslint/parser`，继承
`eslint:recommended`、`plugin:vue/vue3-essential`、
`plugin:@typescript-eslint/recommended`。

项目有意放宽若干规则：

- `@typescript-eslint/no-unused-vars` 是 warning，且 `_` 前缀参数/变量忽略；主
  `tsconfig.json` 对生产源仍开启 `noUnusedLocals` / `noUnusedParameters`，所以生产代码不能仅靠 warning 判断。
- `@typescript-eslint/no-explicit-any`、`ban-types`、`ban-ts-comment` 关闭；这不等于鼓励
  `any` 或 `@ts-ignore`，新边界按 `type-safety.md` 优先使用 `unknown` 和缩窄。
- `vue/multi-word-component-names` 与 `vue/no-use-v-if-with-v-for` 关闭；评审时仍要检查渲染逻辑是否清楚。
- `no-constant-condition`、`no-mixed-spaces-and-tabs`、`no-useless-escape` 关闭。

`frontend/.eslintignore` 排除依赖、dist、Vite 缓存和生成的 Vite config JS/声明文件。不要把业务源文件加入 ignore 来绕过问题。

## Typecheck 的明确边界

`frontend/tsconfig.json` 明确排除：

```text
src/**/__tests__/**
src/**/*.spec.ts
src/**/*.test.ts
```

因此 `typecheck` 只证明生产 `src` 的 TS/Vue 类型通过；测试中的 mock、Vitest globals、
测试 helper 类型错误不在该门内。`tsconfig.node.json` 只 include `vite.config.ts`，
`vitest.config.ts` 也不由该 node project 覆盖。测试变更必须运行 Vitest，不能报告“typecheck 已覆盖测试”。

## Vitest 配置与覆盖率

`frontend/vitest.config.ts` 定义：

- `jsdom` 环境、Vitest globals；
- 全局 setup：`frontend/src/__tests__/setup.ts`；
- alias `@ -> src`，并使用 vue-i18n runtime bundler 构建；
- include `src/**/*.{test,spec}.{js,ts,jsx,tsx}`；
- 排除 `node_modules`、`dist`。

coverage 使用 V8，输出 text/json/html；统计 `src/**/*.{js,ts,vue}`，排除声明、测试与
`src/main.ts`。全局 statements、branches、functions、lines 阈值均为 80%。覆盖率命令是独立质量门，不是 `test:run` 或根 Makefile 默认步骤。

新增测试应优先覆盖可观察行为和高风险分支，而不是复述实现：

- API 拦截器：`frontend/src/api/__tests__/client.spec.ts`；
- 路由功能访问：`frontend/src/router/__tests__/feature-access.spec.ts`；
- store 缓存/并发：`frontend/src/stores/__tests__/app.spec.ts`；
- 组件交互：`frontend/src/components/admin/usage/__tests__/UsageStatsCards.spec.ts`；
- 页面流程：`frontend/src/views/user/__tests__/PaymentView.spec.ts`。

## 根 Makefile：critical 与完整套件不同

根 `Makefile` 的 `test-frontend` 依次运行 `lint:check`、`typecheck` 和
`test-frontend-critical`。critical 列表目前只有六个文件：

```text
src/views/auth/__tests__/LinuxDoCallbackView.spec.ts
src/views/auth/__tests__/WechatCallbackView.spec.ts
src/views/user/__tests__/PaymentView.spec.ts
src/views/user/__tests__/PaymentResultView.spec.ts
src/components/user/profile/__tests__/ProfileInfoCard.spec.ts
src/views/admin/__tests__/SettingsView.spec.ts
```

所以以下名称不能混淆：

- `make test-frontend-critical`：只运行上述六个测试；
- `make test-frontend`：lint + 生产源 typecheck + 上述六个测试；
- `make test`：后端测试 + `make test-frontend`；
- `pnpm --dir frontend run test:run`：完整前端 Vitest 套件。

Router、API、store 等多数测试不在根 critical 列表内。改动这些层时应额外运行相关目录或完整套件，而不能只报告 `make test-frontend`。

## 变更验证矩阵

| 变更范围 | 最低聚焦验证 | 合并前建议 |
|---|---|---|
| TS/Vue 生产代码 | `lint:check` + `typecheck` | 相关 Vitest；跨层时 `test:run` |
| API client/模块 | `src/api/__tests__` | `lint:check`、`typecheck`、完整套件 |
| router/meta/guard | `src/router/__tests__` | navigation integration + 完整套件 |
| Pinia store/缓存 | 对应 `src/stores/__tests__` | 完整套件 |
| critical 列表页面 | 对应 spec | `make test-frontend` |
| 构建配置或入口 | `pnpm --dir frontend run build` | lint、typecheck、完整套件 |

文档-only 规范改动不需要伪造代码测试价值，但仍要检查 Markdown 链接、路径和命令是否真实。

## 评审清单

- 类型：生产代码在 strict 配置下通过；没有把测试误称为 typecheck 覆盖。
- 状态：请求并发、失效、loading 与计时器都有成对清理；失败不会把未知状态写成“明确关闭”。
- API：复用 `apiClient`，泛型是解包后的业务类型，错误按结构化 plain object 处理。
- Router：公共路由显式 meta，权限/feature guard 有真实测试，动态 import 保留。
- 测试：运行的是改动对应测试，而非仅依赖不覆盖该层的 critical 列表。
- 命令：自动化使用 `lint:check`、`test:run` 等一次性命令，不意外启动 watcher 或写回全仓。

## 禁止模式

- 不要通过 ESLint ignore、`@ts-ignore` 或宽泛 `any` 隐藏可建模的问题。
- 不要声称 `make test-frontend` 是完整前端测试，或声称 `typecheck` 检查了 spec 文件。
- 不要只测试复制出来的守卫/转换逻辑而不覆盖生产入口。
- 不要在未观察副作用的情况下添加“只为覆盖行数”的低价值测试。
- 不要在一次性验证中启动 `pnpm --dir frontend run test` watcher。
