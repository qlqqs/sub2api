# 前端质量规范

## 工具与命令

前端只使用 pnpm；CI 安装采用 `pnpm install --frozen-lockfile`。常用检查：

```bash
pnpm lint:check     # 不修改文件的 ESLint 检查
pnpm typecheck      # vue-tsc --noEmit
pnpm test:run       # Vitest 单次运行
pnpm test:coverage  # 全局 80% 覆盖率阈值
pnpm build          # vue-tsc -b + Vite production build
```

`pnpm lint` 带 `--fix`，会修改文件；只在确认需要自动修复时运行。更改 `package.json` 后运行 pnpm 更新 `pnpm-lock.yaml`，不要混用 npm/yarn lockfile。

## 测试模式

- 纯 util/API：Vitest + `vi.mock`/adapter stub，直接断言返回值、请求参数和错误结构。参考 `api/__tests__/client.spec.ts`、`utils/__tests__/authError.spec.ts`。
- composable/store：每个测试重建 Pinia/模块状态；timer 用 fake timers，结束时恢复；覆盖缓存、去重、失败和 clear。参考 `stores/__tests__/subscriptions.spec.ts`。
- Vue 组件/view：使用 `@vue/test-utils` 的 `mount`/`shallowMount`、`flushPromises`，通过用户可观察文本、属性、emit 或稳定的 `data-test(id)` 选择器断言。
- router：覆盖匿名、普通用户、管理员、功能开关、重定向和标题；参考 `router/__tests__/guards.spec.ts` 与 `feature-access.spec.ts`。
- i18n：新增/移动 key 后运行 `i18n/__tests__/localesMessageCompile.spec.ts` 和 `localesNoKeyCollision.spec.ts`。

新增行为至少测试正常路径、错误/空状态和关键边界。并发请求、刷新、polling、OAuth/payment 回调必须测试重复触发、陈旧响应与 cleanup。

## 质量与安全检查

- 页面进入、loading、empty、error、disabled 和成功状态完整，不让 Promise rejection 漏到控制台。
- API 调用经共享 client；支持取消的列表/搜索把 `AbortSignal` 传到底层。
- auth、权限和功能开关的 UI 与路由一致，但安全判断仍由后端执行。
- 所有动态 HTML/SVG/URL 经过 sanitize，外链隔离 opener。
- 新控件可键盘使用，有 label/role/ARIA，modal 有焦点与 Escape 行为。
- 中文和英文文案同步，长文本与移动端布局不溢出。
- timer、listener、observer、object URL、popup 和请求在卸载/结束时清理。
- 依赖升级同时审查 bundle、浏览器兼容性、audit 和 lockfile 差异。

## 当前已知宽松点

ESLint 当前允许 explicit `any`、ban-ts-comment、单词组件名及 `v-if`/`v-for` 组合；存量代码也存在中英文注释混合和少数未统一 sanitize 的 `v-html`。新代码不要借此扩大技术债：优先 typed boundary、清晰结构与 DOMPurify；独立清理存量问题应另开任务，避免在无关变更中大面积重写。

## 审查清单

- 改动放在正确目录，复用了现有组件/composable/store/API helper。
- props、emits、API payload、错误和可空状态有准确类型。
- async 竞态、loading 和错误展示可预测，卸载后不回写状态。
- 测试验证用户行为和协议，不只验证内部实现细节。
- lint、typecheck、相关 Vitest 与 production build 均通过；未运行项有明确原因。
