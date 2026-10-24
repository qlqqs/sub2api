<template>
  <div class="qx-shell" :class="{ 'is-collapsed': sidebarCollapsed }" :style="shellVars">
    <QxSidebar />
    <div class="qx-shell-col">
      <AppHeader />
      <main class="qx-shell-main" :class="route.meta?.qxFullBleed ? 'qx-shell-main--bleed' : 'p-4 md:p-6 lg:p-8'">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * CUSTOM: QxAPI 控制台 / 管理后台外壳，由上游 AppLayout.vue 在主题开启时渲染（见 config.ts）。
 * 侧边栏换成 QxSidebar；顶栏直接复用上游 AppHeader.vue，只在 shell.css 里改样式。
 * 顶栏标题上方的小标签和余额前的「余额」字样无法改模板，用 CSS 变量传给 ::before。
 * 非管理页面给 <html> 加 qx-line 类和 data-qx-page（路由名），pages/ 下的极简线条样式只在这些页面生效；
 * 放在 <html> 上是为了让传送到 body 的弹窗、下拉也能匹配到。
 */
import { computed, onBeforeUnmount, watchEffect } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import AppHeader from '@/components/layout/AppHeader.vue'
import QxSidebar from './QxSidebar.vue'
import './shell.css'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)

/** 顶栏小标签：路由路径里的英文段，如 /admin/affiliates/agents → admin / affiliates / agents。 */
const pageKicker = computed(() =>
  route.path
    .split('/')
    .filter((seg) => /^[a-z][a-z-]*$/.test(seg))
    .map((seg) => seg.replace(/-/g, ' '))
    .join(' / ')
)

const LINE_CLASS = 'qx-line'
const lineScope = computed(() => !route.path.startsWith('/admin'))
watchEffect(() => {
  const root = document.documentElement
  root.classList.toggle(LINE_CLASS, lineScope.value)
  if (lineScope.value && typeof route.name === 'string') root.dataset.qxPage = route.name
  else delete root.dataset.qxPage
})
onBeforeUnmount(() => {
  document.documentElement.classList.remove(LINE_CLASS)
  delete document.documentElement.dataset.qxPage
})

// CSS 字符串值：JSON.stringify 生成带引号的字符串，内容只含字母、空格、斜杠和本地化文字。
const shellVars = computed(() => ({
  '--qx-page-kicker': JSON.stringify(pageKicker.value),
  '--qx-balance-label': JSON.stringify(t('common.balance'))
}))
</script>
