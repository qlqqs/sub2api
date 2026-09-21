# 组件规范

## SFC 模式

组件使用 Vue 3 Composition API 和 `<script setup lang="ts">`。当前文件大多按 `<template>` 后 `<script setup>` 排列；在所改目录保持局部格式一致。组件逻辑应围绕 props、emits、refs/computed、handlers 和生命周期组织，复杂可复用状态移到 composable。

```ts
const props = withDefaults(defineProps<{
  modelValue: number | null
  min?: number
}>(), { min: 0 })

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
}>()
```

`components/payment/AmountInput.vue` 是 typed props、默认值和 `v-model` 事件的简洁样例；`components/common/BaseDialog.vue` 展示 slots、Teleport、焦点和生命周期管理。

## Props、事件与组合

- props 只读。需要可编辑副本时创建本地 ref/reactive，并用 `watch` 明确同步；不要直接修改对象 props。
- 可选 props 用 `withDefaults` 提供稳定默认值；数组/对象默认值使用工厂函数。
- emits 必须类型化；双向绑定遵循 `modelValue`/`update:modelValue`，其余事件用能表达结果的名称，如 `close`、`success`、`selectionChange`。
- 父组件负责数据和业务副作用，展示型子组件通过 props/slots 接收内容并 emit 意图。复杂表格单元格使用 named scoped slots，参见 `DataTable.vue`。
- 只在父级确实需要命令式调用时 `defineExpose`，如输入框的 `focus`/`select`；普通状态通过 props/emits 传递。
- 异步按钮要有 loading/disabled 防重入，失败交给调用方或统一 toast，不留下无法退出的中间状态。

## 样式与响应式布局

- 主要使用 Tailwind utilities，并复用 `style.css` 中的语义类，如 `btn`、`input`、`modal-*`、`table-*`。
- 同时提供 light/dark 类；主色使用 Tailwind 配置中的 `primary`，中性色使用 gray/dark 体系。
- 复用已有 `Icon`、`BaseDialog`、`DataTable`、`Pagination`、表单控件和 layout，不复制一套相近交互。
- 保持移动端和桌面行为。`DataTable.vue` 在窄屏改为卡片列表，在桌面保留表格和虚拟化，是复杂响应式组件的参考。
- 状态变化不应改变固定控件的占位尺寸，避免 loading、图标或长文本造成布局跳动/覆盖。

## 文案与安全

- 面向用户的文本通过 `useI18n()` 和 `t('...')` 获取；新增 key 同步更新中文和英文 locale，并运行 locale 测试。
- 动态 URL 先走 `utils/url.ts` 的 `sanitizeUrl`。外链使用 `target="_blank"` 时带 `rel="noopener noreferrer"`。
- `v-html` 仅用于可信或经过 DOMPurify 处理的内容。Markdown 示例见 `views/public/LegalDocumentView.vue`；SVG 使用 `utils/sanitize.ts` 的 `sanitizeSvg`。不要照搬未经 sanitize 的历史用法。

## 无障碍

- 交互使用原生 `button`/`input`/`a`，button 明确 `type="button"`（表单提交除外）。
- 只有图标的按钮提供本地化 `aria-label`；表单 label 与 input `id` 对应，错误/提示保持可读。
- 自定义开关声明 `role="switch"` 与 `aria-checked`，参考 `components/payment/ToggleSwitch.vue`。
- dialog 使用 `role="dialog"`、`aria-modal`、标题关联，支持 Escape、初始焦点和关闭后的焦点恢复，参考 `BaseDialog.vue`。
- 表格 header 使用 `scope="col"`，排序状态用 `aria-sort`；选择框提供明确 label。

## 常见错误

- 直接修改 props，或用双向 watch 造成循环与陈旧覆盖。
- 在多个 modal 重复实现 body scroll lock、Escape 和焦点处理，而不复用 `BaseDialog`。
- 把可点击 `div` 当按钮却没有键盘语义。
- 在模板中拼接未过滤的 HTML、SVG 或管理端可配置 URL。
- 只实现桌面尺寸，或只用颜色表达状态。
