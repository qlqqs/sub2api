<template>
  <HomeView v-if="useUpstreamHome" />
  <QxHome v-else />
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { qxThemeConfig } from './config'
import QxHome from './QxHome.vue'

/**
 * CUSTOM: 首页路由入口。主题关闭，或后台配置了自定义首页内容 / 简洁首页时，
 * 渲染上游原版 HomeView（这两项设置由上游首页负责解释）。
 */
const HomeView = defineAsyncComponent(() => import('@/views/HomeView.vue'))

const appStore = useAppStore()

const useUpstreamHome = computed(() => {
  const settings = appStore.cachedPublicSettings
  return (
    !qxThemeConfig.enabled ||
    (settings?.home_content || '').trim().length > 0 ||
    settings?.compact_home_enabled === true
  )
})

onMounted(() => {
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
