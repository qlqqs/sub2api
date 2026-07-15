# 前端组件规范

本项目组件使用 Vue 3 单文件组件、Composition API、`<script setup lang="ts">` 和 Tailwind CSS。下面的规则来自 `src/components/common/`、领域组件、页面组合方式和邻近 Vitest 测试。

## 组件边界与组织

- 路由页面负责取数、权限/导航和领域组件编排；可复用展示与交互下沉到组件。`src/views/user/ProfileView.vue` 组合 `AppLayout`、`ProfileInfoCard`、密码/TOTP/余额通知组件，`ProfileInfoCard.vue` 再组合 profile 子组件。
- `src/components/common/` 只收业务无关原语。`Input.vue`、`DataTable.vue`、`BaseDialog.vue` 可以被多个领域使用；`src/components/admin/usage/UsageTable.vue` 因认识 usage 行结构而留在管理员 usage 领域。
- 组件只在父子边界通过 props、emits 和 slots 通信。跨页面持久状态使用 Pinia；不要通过模块级可变变量制造隐式共享状态。`BaseDialog.vue` 的 `dialogIdCounter` 是生成 DOM ID 的有限例外，不承载业务数据。
- 组件内部可直接组合 Composable，例如 `AppLayout.vue` 使用 `useOnboardingTour`；网络请求应通过 `src/api/` 或封装后的 Composable，而不是写进模板表达式。

## SFC 结构

- 新组件必须使用 `<script setup lang="ts">`。现有文件既有 template-first（`Input.vue`、`DataTable.vue`）也有 script-first（`App.vue`），不要为了统一顺序制造无意义改动；在同一文件中保持 template、script、style 分区清晰即可。
- 导入按来源与职责保持可读：Vue/Vue Router/i18n、store/API/Composable、组件、类型。类型导入使用 `import type`，代表例为 `ProfileInfoCard.vue` 从 `@/types` 导入 `User` 等类型。
- 简单派生值用 `computed`，可变 UI 状态用 `ref`/`reactive`，副作用使用 `watch` 和生命周期钩子。监听器、Observer、定时器和请求取消必须有对称清理；`BaseDialog.vue` 和 `DataTable.vue` 展示了事件监听与 ResizeObserver/media query 的清理。
- 模板中避免复杂业务计算；将格式化、归一化和状态判断提取为命名函数或 computed。`ProfileInfoCard.vue` 的 `primaryEmailDisplay`、`memberSinceLabel` 和 source 解析函数是可参考形态。

## Props、emits 与 `v-model`

### Props

- 使用类型化 `defineProps`。需要默认值时使用 `withDefaults(defineProps<Props>(), defaults)`：
  - `src/components/common/Input.vue` 用局部 `Props` 接口描述输入属性。
  - `src/components/common/DataTable.vue` 用 `withDefaults` 统一 loading、sticky、sort 等默认行为。
  - `src/components/user/profile/ProfileInfoCard.vue` 直接对内联 props 类型应用 `withDefaults`。
- Props 表达父组件拥有的数据；组件不得直接修改 prop。需要本地编辑副本时显式复制，并在 prop 变化时说明同步策略。
- 只在当前组件使用的 props 类型留在 SFC；跨组件复用才导出到兄弟 `types.ts` 或领域/API 类型模块。不要为了一个简单 props 接口扩大 `src/types/`。
- 布尔属性使用能读出状态的名字（`loading`、`disabled`、`clickableRows`、`closeOnEscape`），事件处理属性不要伪装成布尔值。

### Emits

- 使用类型化 `defineEmits`，优先采用参数元组或调用签名。`DataTable.vue` 声明 `sort: [key, order]` 与 `rowClick: [row]`，`Input.vue` 为 `change`、`blur`、`focus`、`enter` 指定载荷类型。
- 事件名描述已发生的用户/领域动作，例如 `close`、`success`、`sort`、`rowClick`。父组件拥有状态时，子组件发事件而不是直接访问父组件状态。
- 模板中可以转发简单事件；一旦涉及校验、异步或多个状态变化，使用命名处理函数，不在模板中堆叠长表达式。

### `modelValue`

- 双向控件沿用 `modelValue` + `update:modelValue`，当前项目不使用 `defineModel`。参考：
  - `Input.vue` 接收 `string | number | null | undefined`，在原生 input 事件中发出字符串。
  - `Toggle.vue` 接收 boolean，并发出取反后的 boolean。
- 载荷类型必须与控件承诺一致。`Input.vue` 虽允许 number 作为输入值，实际发出的仍是字符串，这是现有兼容行为；需要数字的调用端应显式解析，新的专用数字控件不得假装发出 number 却返回字符串。
- 不同时发出多个含义相同的更新事件。`change` 只用于原生 change 语义或父层明确需要的提交时机，实时绑定使用 `update:modelValue`。

## Slots 与组合

- 用 slot 表达结构扩展点，而不是为每种内容增加布尔 prop：
  - `Input.vue` 提供 `prefix`、`suffix`。
  - `AppLayout.vue` 提供默认内容 slot。
  - `BaseDialog.vue` 提供 body 默认 slot 与 `footer`。
  - `DataTable.vue` 提供 `empty`、`header-${column.key}`、`cell-${column.key}`，并向 cell slot 传递 `row`、`value`、`expanded`。
- Scoped slot 的名称与载荷属于公共 API。修改 `DataTable` slot 参数前必须检查 `src/components/admin/usage/UsageTable.vue` 等调用点和 `DataTable.spec.ts`。
- 仅在确有命令式需求时使用 `defineExpose`。`Input.vue` 暴露 `focus()`/`select()`，`AppLayout.vue` 暴露 `replayTour`；普通数据流仍通过 props/emits。

## 样式与响应式布局

- 优先在模板使用 Tailwind utility，并同时处理 class-based dark mode：`text-gray-900 dark:text-white`、`bg-white dark:bg-dark-800`。主题颜色、阴影、动画和断点扩展来自 `tailwind.config.js`。
- 重复的设计原语使用 `src/style.css` 中的组件类，如 `btn`/`btn-primary`、`input`、`card`、`badge`、modal/table 类；不要在每个领域重新发明一套按钮和输入框。
- 组件布局应按移动端优先添加 `sm:`/`md:`/`lg:`。`DataTable.vue` 在移动端渲染卡片、桌面端渲染语义 table；`AppLayout.vue` 使用响应式侧栏边距。
- 只有值确实由运行时计算时才使用 `:style`，例如 Teleport 菜单坐标、虚拟表格 padding 或用户配置的 z-index。固定视觉样式应留在 Tailwind/class 中。
- 局部复杂过渡或浏览器行为可以使用 SFC style；不要用全局 CSS 选择器绑定某个领域组件的内部 DOM，除非它是 `src/style.css` 明确定义的共享原语。

## 可访问性与原生语义

- 优先使用原生元素：动作使用 `<button type="button">`，导航使用链接，输入使用 `<input>`/`select`，数据表使用 `<table>`/`th scope="col">`。不要用可点击 `div` 替代按钮。
- 表单标签必须通过 `for`/`id` 关联输入。`Input.vue` 会把可选 `id` prop 同时绑定到
  label 的 `for` 与 input 的 `id`，因此关联是否成立依赖调用方实际传入 `id`；它当前也
  尚未设置 `aria-invalid`，或用 `aria-describedby` 关联 hint/error，不能作为完整合规范例。
- 图标按钮必须有可访问名称；可见文本优先，纯图标按钮使用本地化 `aria-label`。
  `BaseDialog.vue` 的关闭按钮当前缺少 `type="button"`，且使用硬编码英文
  `aria-label="Close modal"`，这是待修复缺口，不是完整合规范例。
- 自定义控件需要完整 ARIA 状态。`Toggle.vue` 使用 `role="switch"` 和 `aria-checked`；
  `BaseDialog.vue` 已实现 `role="dialog"`、`aria-modal`、`aria-labelledby`、Escape、
  初始聚焦和焦点恢复等部分弹窗行为，但结合上述关闭按钮缺口，只能作为局部实现参考。
- 注册全局键盘、焦点、滚动锁或 Observer 时必须在卸载时恢复。关闭弹窗不仅要隐藏 DOM，也要恢复焦点和 body 状态。
- 颜色不能成为唯一状态信号；同时提供文本、图标、`aria-*` 或其他结构信息。交互测试应优先断言语义属性和用户可见行为。

## 组件测试

- 使用 Vitest + `@vue/test-utils` 的 `mount`，测试放在组件最近的 `__tests__/`。`src/components/common/__tests__/DataTable.spec.ts` 和 `src/components/user/profile/__tests__/ProfileInfoCard.spec.ts` 是代表性测试。
- 测试组件的公开契约：props 渲染、emits、slot、ARIA、响应式分支和用户交互。`DataTable.spec.ts` 断言 `aria-sort`、排序方向、虚拟化阈值及稳定 row key，而不是做整页快照。
- i18n、store、Router 和浏览器 API 按测试边界 mock。全局 `matchMedia`、ResizeObserver、IntersectionObserver 等基础 mock 位于 `src/__tests__/setup.ts`；场景特定行为可在 spec 内覆盖。
- 使用 `data-testid` 只定位缺少稳定语义查询的业务区域，示例见 `ProfileInfoCard.spec.ts`。按钮、输入和文本优先按原生语义或可见内容查找。
- `Input.vue` 当前没有邻近专用测试，这是已有覆盖缺口，不是免测先例；修改 v-model、事件、错误/ARIA 或 slot 行为时应在 `src/components/common/__tests__/` 增加聚焦测试。

## 已知例外与反模式

- `DataTable.vue` 和 `src/components/common/types.ts` 为兼容多种行数据仍使用 `any`。新领域组件不要继续扩散 `any`；先在调用端保留具体行类型，改公共表格契约时再以兼容方式引入泛型。
- `DataTable.vue` 的可点击行/移动卡片通过 click 发出 `rowClick`，但行本身不是完整键盘控件。不要把它作为唯一关键操作入口；关键操作应提供真实 button/link，并用 `@click.stop` 避免触发行点击。
- `Input.vue` 的 label/input 关联依赖调用方传入 `id`；当前 error/hint 文本也尚未通过
  `aria-describedby` 关联，且未设置 `aria-invalid`。它是需要逐步修复的缺口，不是无障碍
  完整范例。
- `BaseDialog.vue` 的关闭按钮当前缺少 `type="button"`，并使用硬编码英文
  `aria-label="Close modal"`。在表单内可能触发默认提交，且名称未本地化；不要把它称为
  完整无障碍合规范例。
- 部分历史大型页面包含内联 SVG、长模板和大量事件逻辑。新增可复用交互时应提取 Icon/领域组件，而不是复制长模板或把业务组件塞进 `common/`。
- 不要用过时 README 中的静态示例覆盖当前 `script setup + TypeScript + Tailwind` 约定；以实际 SFC、测试和构建配置为准。
