<template>
  <!-- CUSTOM: 结算单按渠道经理拆分；合伙人直属团队单列一行 -->
  <div class="card overflow-hidden" data-test="partner-lines">
    <h3 class="px-4 pb-3 pt-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('partner.lines.title') }}</h3>
    <div class="overflow-x-auto">
      <table class="table min-w-max whitespace-nowrap">
        <thead>
          <tr>
            <th scope="col">{{ t('partner.lines.line') }}</th>
            <th scope="col">{{ t('affiliate.team.columns.email') }}</th>
            <th scope="col" class="!text-right">{{ t('partner.lines.members') }}</th>
            <th v-for="col in COLUMNS" :key="col" scope="col" class="!text-right">{{ t(`partner.lines.${col}`) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="lines.length === 0">
            <td :colspan="COLUMNS.length + 3" class="py-6 text-center text-gray-500 dark:text-dark-400">{{ t('partner.lines.empty') }}</td>
          </tr>
          <tr v-for="({ line, to }, index) in rows" :key="line.line_agent?.id ?? `direct-${index}`" data-test="partner-line-row">
            <td>
              <RouterLink
                v-if="to"
                :to="to"
                class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              >
                {{ lineName(line) }}
              </RouterLink>
              <span v-else class="font-medium text-gray-900 dark:text-white">{{ lineName(line) }}</span>
            </td>
            <td :class="line.line_agent?.email && !line.line_agent.deleted ? 'text-gray-600 dark:text-gray-300' : 'text-gray-400 dark:text-dark-500'">
              {{ line.line_agent?.email && !line.line_agent.deleted ? line.line_agent.email : '-' }}
            </td>
            <td class="text-right tabular-nums">{{ line.members }}</td>
            <td v-for="col in COLUMNS" :key="col" class="text-right">
              <FinanceMoney :value="line[col]" :color-negative="col === 'profit'" />
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'
import FinanceMoney from '@/components/admin/finance/FinanceMoney.vue'
import type { PartnerLine } from '@/api/affiliatePartner'
import { partnerUserName } from '@/utils/affiliatePartner'

const props = defineProps<{
  lines: PartnerLine[]
  /** 可点进的行（用户端：直属团队 / 渠道经理的团队消费页）。 */
  lineRoute?: (line: PartnerLine) => RouteLocationRaw | undefined
}>()

const { t } = useI18n()

const rows = computed(() => props.lines.map((line) => ({ line, to: props.lineRoute?.(line) })))

const COLUMNS = ['consumption', 'revenue', 'account_cost', 'commission', 'profit'] as const

function lineName(line: PartnerLine): string {
  return line.line_agent ? partnerUserName(line.line_agent, t('partner.deleted')) : t('partner.region.directTeam')
}
</script>
