<template>
  <!-- 后台内嵌形态：?embedded=1 且已登录，套完整后台布局（同模型广场） -->
  <AppLayout v-if="isEmbedded">
    <ClientDownloadsContent :clients="clients" />
  </AppLayout>

  <!-- 独立形态：QxAPI 主题用站点顶栏，主题关闭时用模型广场的导航条 -->
  <div v-else-if="qxEnabled" class="qx">
    <QxSiteHeader />
    <main class="qx-wrap py-8 sm:py-12">
      <ClientDownloadsContent :clients="clients" />
    </main>
  </div>
  <div v-else class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <PlazaNavBar />
    <main class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
      <ClientDownloadsContent :clients="clients" />
    </main>
  </div>
</template>

<script setup lang="ts">
/** CUSTOM: 客户端下载页，内容来自公共设置 client_downloads（后台「系统设置 → 通用」配置）。 */
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PlazaNavBar from '@/components/modelPlaza/PlazaNavBar.vue'
import ClientDownloadsContent from '@/components/downloads/ClientDownloadsContent.vue'
import QxSiteHeader from '@/themes/qx/QxSiteHeader.vue'
import { qxThemeConfig } from '@/themes/qx/config'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { visibleClientDownloads } from '@/utils/clientDownloads'
import '@/themes/qx/theme.css'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()

const qxEnabled = qxThemeConfig.enabled
// embedded=1 但未登录（如转发的链接）自动降级为独立形态。
const isEmbedded = computed(() => route.query.embedded === '1' && authStore.isAuthenticated)
const clients = computed(() => visibleClientDownloads(appStore.cachedPublicSettings))

onMounted(() => {
  if (!appStore.publicSettingsLoaded) void appStore.fetchPublicSettings()
})
</script>
