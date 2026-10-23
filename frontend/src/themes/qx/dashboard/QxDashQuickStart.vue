<template>
  <section class="qd-panel" data-testid="qd-quick">
    <header class="qd-panel-head">
      <div>
        <span class="qd-kicker">{{ t('qxTheme.dashboard.quick.kicker') }}</span>
        <h3>{{ t('dashboard.quickActions') }}</h3>
      </div>
    </header>
    <div class="qd-actions">
      <router-link
        v-for="action in actions"
        :key="action.id"
        :to="action.to"
        class="qd-action"
        :class="action.tone"
        data-testid="qd-action"
        :data-action="action.id"
      >
        <span class="qd-action-ic" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path :d="action.icon" /></svg>
        </span>
        <div class="qd-action-text">
          <h4>{{ t(action.title) }}<span v-if="action.badge" class="qd-action-badge">{{ t(action.badge) }}</span></h4>
          <p>{{ t(action.desc) }}</p>
        </div>
        <span class="qd-action-arrow" aria-hidden="true">→</span>
      </router-link>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import { useQxSite } from '../useQxSite'

/** CUSTOM: QxAPI 主题仪表盘的快捷操作：创建密钥、导入 CC Switch（后台可隐藏）、批量生图（有权限才显示）、兑换码。 */
interface QuickAction {
  id: string
  to: string
  tone: string
  icon: string
  title: string
  desc: string
  badge?: string
}

const { t } = useI18n()
const appStore = useAppStore()
const { ccsImportVisible } = useQxSite()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

const actions = computed<QuickAction[]>(() => {
  const list: QuickAction[] = [
    { id: 'create-key', to: '/keys', tone: 'qd-tone-blue', icon: 'M12 5v14M5 12h14', title: 'dashboard.createApiKey', desc: 'dashboard.generateNewKey' }
  ]
  if (ccsImportVisible.value) {
    list.push({
      id: 'ccs-import',
      to: '/keys',
      tone: 'qd-tone-cyan',
      icon: 'M12 4v11m0 0l-4-4m4 4l4-4M5 19h14',
      title: 'qxTheme.dashboard.quick.ccsTitle',
      desc: 'qxTheme.dashboard.quick.ccsDesc',
      badge: 'qxTheme.dashboard.quick.recommended'
    })
  }
  if (canUseBatchImage.value) {
    list.push({
      id: 'batch-image',
      to: '/batch-image',
      tone: 'qd-tone-amber',
      icon: 'M5 4h14a2 2 0 012 2v12a2 2 0 01-2 2H5a2 2 0 01-2-2V6a2 2 0 012-2zM3 9h18M8 14h3',
      title: 'dashboard.batchImageAgent',
      desc: 'dashboard.batchImageAgentDesc'
    })
  }
  list.push({
    id: 'redeem',
    to: '/redeem',
    tone: 'qd-tone-green',
    icon: 'M4.5 8h15A1.5 1.5 0 0121 9.5v10a1.5 1.5 0 01-1.5 1.5h-15A1.5 1.5 0 013 19.5v-10A1.5 1.5 0 014.5 8zM12 8v13M3 12h18',
    title: 'dashboard.redeemCode',
    desc: 'dashboard.addBalanceWithCode'
  })
  return list
})

onMounted(() => {
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
  void refreshBatchImageAccess()
})
</script>
