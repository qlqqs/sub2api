<template>
  <div class="flex flex-wrap items-center justify-between gap-3">
    <nav
      :aria-label="t('affiliate.title')"
      class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800"
      data-test="affiliate-section-tabs"
    >
      <RouterLink
        v-for="tab in tabs"
        :key="tab.key"
        :to="tab.to"
        :aria-current="tab.active ? 'page' : undefined"
        :class="[
          'rounded-md px-4 py-1.5 text-sm font-medium transition-colors',
          tab.active
            ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
            : 'text-gray-600 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'
        ]"
        :data-test="`affiliate-section-tab-${tab.key}`"
      >
        {{ tab.label }}
      </RouterLink>
    </nav>
    <!-- 右侧内容（如团队消费页的日期范围与刷新） -->
    <slot />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

/** 邀请返利的两个分区：邀请（/affiliate）与团队消费（/affiliate/team）。按路由名判定当前分区。 */
const { t } = useI18n()
const route = useRoute()

const onTeam = computed(() => String(route.name ?? '').startsWith('AffiliateTeam'))

const tabs = computed(() => [
  { key: 'invite', label: t('affiliate.tabs.invite'), to: { name: 'Affiliate' }, active: route.name === 'Affiliate' },
  { key: 'team', label: t('affiliate.team.title'), to: { name: 'AffiliateTeam' }, active: onTeam.value }
])
</script>
