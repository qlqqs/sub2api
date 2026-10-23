<template>
  <UpstreamAuthLayout v-if="!qxThemeConfig.enabled">
    <slot />
    <template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </UpstreamAuthLayout>
  <div v-else class="qx qx-auth qx-dots qx-dots-center">
    <QxSiteHeader plain />
    <main class="qx-auth-main">
      <div class="qx-card">
        <slot />
      </div>
      <div v-if="$slots.footer" class="qx-auth-footer">
        <slot name="footer" />
      </div>
      <div class="qx-stats qx-auth-stats">
        <span class="qx-kicker">{{ t('qxTheme.console.endpoint') }}</span>
        <span v-for="e in endpoints" :key="e.protocol" class="qx-stats-item">
          {{ QX_PROTOCOL_LABELS[e.protocol] }}<code>{{ e.url }}</code>
        </span>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import UpstreamAuthLayout from '@/components/layout/AuthLayout.vue'
import QxSiteHeader from './QxSiteHeader.vue'
import { QX_PROTOCOL_LABELS, qxProtocolEndpoint, qxThemeConfig } from './config'
import { useQxSite } from './useQxSite'
import './theme.css'

/**
 * CUSTOM: 登录 / 注册 / 找回密码等页面的外壳，插槽与上游 AuthLayout 相同（default + footer）。
 * 由 `components/layout/index.ts` 以 AuthLayout 的名字导出；主题关闭时渲染上游原版。
 * 组件名沿用 AuthLayout，按名字 stub 布局的现有测试无需改动。
 */
defineOptions({ name: 'AuthLayout' })

const { t } = useI18n()
const appStore = useAppStore()
const { apiOrigin } = useQxSite()

const endpoints = computed(() =>
  qxThemeConfig.protocols.map((protocol) => ({ protocol, url: qxProtocolEndpoint(protocol, apiOrigin.value) }))
)

onMounted(() => {
  if (qxThemeConfig.enabled) appStore.fetchPublicSettings()
})
</script>
