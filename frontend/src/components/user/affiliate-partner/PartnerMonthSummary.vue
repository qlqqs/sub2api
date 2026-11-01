<template>
  <!-- CUSTOM: 合伙人某月的各项合计：利润构成、累计亏损、比例与分成 -->
  <section class="card p-5" data-test="partner-month-summary">
    <div class="flex flex-wrap items-center gap-2">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ title || totals.month }}</h3>
      <PartnerStatusBadge :status="totals.status" />
      <span v-if="totals.paid_on" class="text-xs text-gray-500 dark:text-dark-400" data-test="partner-paid-on">
        {{ t('partner.summary.paidOn', { date: totals.paid_on }) }}<template v-if="totals.paid_note"> · {{ totals.paid_note }}</template>
      </span>
    </div>

    <dl class="mt-4 grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-3 lg:grid-cols-6">
      <div v-for="item in items" :key="item.key" class="min-w-0" :data-test="`partner-summary-${item.key}`">
        <dt class="text-xs text-gray-500 dark:text-dark-400">{{ item.label }}</dt>
        <dd class="mt-0.5 text-lg font-semibold text-gray-900 dark:text-white">
          <FinanceMoney :value="item.value" :color-negative="item.key === 'profit' || item.key === 'adjustment'" />
        </dd>
      </div>
    </dl>

    <div class="mt-4 flex flex-wrap items-end justify-between gap-4 border-t border-gray-100 pt-4 dark:border-dark-700">
      <dl class="flex flex-wrap gap-x-6 gap-y-2 text-sm">
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('partner.summary.lossBefore') }}</dt>
          <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="totals.loss_before" /></dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('partner.summary.lossAfter') }}</dt>
          <dd class="text-gray-900 dark:text-white" data-test="partner-loss-after"><FinanceMoney :value="totals.loss_after" /></dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('partner.summary.shareBase') }}</dt>
          <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="totals.share_base" /></dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('partner.summary.rate') }}</dt>
          <dd class="text-gray-900 dark:text-white">{{ formatPartnerRate(totals.share_rate) }}</dd>
        </div>
      </dl>
      <div class="sm:text-right">
        <p class="text-xs text-gray-500 dark:text-dark-400">
          {{ totals.status === 'current' || totals.status === 'draft' ? t('partner.summary.shareEstimated') : t('partner.summary.share') }}
        </p>
        <p class="text-2xl font-semibold text-primary-600 dark:text-primary-400" data-test="partner-share-amount">
          <FinanceMoney :value="totals.share_amount" :color-negative="false" />
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import FinanceMoney from '@/components/admin/finance/FinanceMoney.vue'
import type { PartnerMonthTotals } from '@/api/affiliatePartner'
import { formatPartnerRate } from '@/utils/affiliatePartner'
import PartnerStatusBadge from './PartnerStatusBadge.vue'

const props = defineProps<{
  totals: PartnerMonthTotals
  /** 默认显示月份。 */
  title?: string
}>()

const { t } = useI18n()

const items = computed(() => [
  { key: 'revenue', label: t('partner.summary.revenue'), value: props.totals.revenue },
  { key: 'account_cost', label: t('partner.summary.accountCost'), value: props.totals.account_cost },
  { key: 'commission', label: t('partner.summary.commission'), value: props.totals.commission },
  { key: 'region_expense', label: t('partner.summary.regionExpense'), value: props.totals.region_expense },
  { key: 'adjustment', label: t('partner.summary.adjustment'), value: props.totals.adjustment },
  { key: 'profit', label: t('partner.summary.profit'), value: props.totals.profit }
])
</script>
