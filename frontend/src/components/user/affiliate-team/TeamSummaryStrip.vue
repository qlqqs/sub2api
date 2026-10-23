<template>
  <div class="card px-5 py-4" data-test="team-summary">
    <div :class="['grid grid-cols-2 gap-x-6 gap-y-3', metrics.length === 4 ? 'sm:grid-cols-4' : 'sm:grid-cols-3']">
      <div
        v-for="metric in metrics"
        :key="metric.key"
        :class="['min-w-0', summary && metric.sub && 'col-span-2 sm:col-span-1']"
        :data-test="`team-summary-${metric.key}`"
      >
        <div class="flex flex-wrap items-baseline gap-x-2">
          <span class="text-sm text-gray-500 dark:text-dark-400">{{ metric.label }}</span>
          <!-- 数值与 ⓘ 不拆行 -->
          <span class="inline-flex items-center whitespace-nowrap">
            <span
              v-if="summary"
              class="text-xl font-semibold tabular-nums text-gray-900 dark:text-white"
              data-test="team-summary-value"
            >
              {{ metric.value }}
            </span>
            <span v-else class="inline-block h-6 w-20 animate-pulse rounded bg-gray-200 dark:bg-dark-700"></span>
            <HelpTooltip v-if="metric.hint" :content="metric.hint" />
          </span>
        </div>
        <p
          v-if="summary && metric.sub"
          class="mt-0.5 text-xs tabular-nums text-gray-500 dark:text-dark-400"
          data-test="team-summary-split"
        >
          {{ metric.sub }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import type { AffiliateTeamCounts, AffiliateTeamSummary } from '@/types'
import type { AffiliateTeamView } from '@/composables/useAffiliateTeamQuery'
import { formatTeamCount, formatTeamMoney } from '@/utils/affiliateTeam'
import { formatTokensK } from '@/utils/format'

interface Metric {
  key: string
  label: string
  value: string
  sub?: string
  hint?: string
}

const props = defineProps<{
  view: AffiliateTeamView
  /** null = 首次加载（骨架）。 */
  summary: AffiliateTeamSummary | null
  counts?: AffiliateTeamCounts | null
  settleIntervalMinutes?: number
}>()

const { t } = useI18n()

const metrics = computed<Metric[]>(() => {
  const s = props.summary
  const cost: Metric = {
    key: 'cost',
    label: t('affiliate.team.summary.cost'),
    value: formatTeamMoney(s?.cost),
    // 「直属 · 下级」只对有下级代理的人显示；二级代理只有一个范围。
    sub: props.view === 'team' && (props.counts?.sub_agents ?? 0) > 0
      ? t('affiliate.team.summary.split', { direct: formatTeamMoney(s?.direct_cost), sub: formatTeamMoney(s?.sub_cost) })
      : undefined
  }
  const earnings: Metric = {
    key: 'earnings',
    label: t('affiliate.team.summary.earnings'),
    value: formatTeamMoney(s?.earnings),
    hint: t('affiliate.team.summary.earningsHint', { n: props.settleIntervalMinutes ?? 60 })
  }
  if (props.view === 'customer') {
    return [
      cost,
      { key: 'requests', label: t('affiliate.team.summary.requests'), value: formatTeamCount(s?.requests) },
      { key: 'tokens', label: t('affiliate.team.summary.tokens'), value: formatTokensK(s?.tokens ?? 0) },
      earnings
    ]
  }
  return [
    cost,
    earnings,
    {
      key: 'active',
      label: t('affiliate.team.summary.activeCustomers'),
      value: `${formatTeamCount(s?.active_customers)} / ${formatTeamCount(s?.customers)}`
    }
  ]
})
</script>
