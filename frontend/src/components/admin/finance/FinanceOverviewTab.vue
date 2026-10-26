<template>
  <!-- CUSTOM: 财务总览：期间卡片、余额消耗构成、当前余额快照、趋势图 -->
  <div class="space-y-4">
    <section>
      <h3 class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.overview.periodTitle') }}</h3>
      <div class="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
        <div v-for="card in periodCards" :key="card.key" class="card p-4" :data-testid="`finance-card-${card.key}`">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ card.label }}</p>
          <p class="mt-1 text-xl font-bold text-gray-900 dark:text-white">
            <span
              v-if="card.ratio !== undefined"
              class="tabular-nums"
              :class="card.ratioNegative ? 'text-red-600 dark:text-red-400' : undefined"
            >{{ card.ratio }}</span>
            <FinanceMoney v-else :value="card.value" />
          </p>
          <p v-if="card.hint" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ card.hint }}</p>
        </div>
      </div>
    </section>

    <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
      <section class="card p-4">
        <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.overview.consumptionTitle') }}</h3>
        <dl class="space-y-2 text-sm">
          <div class="flex items-center justify-between gap-3">
            <dt class="text-gray-600 dark:text-gray-300">{{ t('admin.finance.overview.consumption') }}</dt>
            <dd class="font-medium text-gray-900 dark:text-white"><FinanceMoney :value="totals.consumption" /></dd>
          </div>
          <div class="flex items-center justify-between gap-3">
            <dt class="text-gray-600 dark:text-gray-300">{{ t('admin.finance.profit.columns.revenuePaid') }}</dt>
            <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="totals.revenue_paid" /></dd>
          </div>
          <div class="flex items-center justify-between gap-3">
            <dt class="text-gray-600 dark:text-gray-300">{{ t('admin.finance.profit.columns.revenueCommission') }}</dt>
            <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="totals.revenue_commission" /></dd>
          </div>
          <div class="flex items-center justify-between gap-3">
            <dt class="text-gray-600 dark:text-gray-300" :title="t('admin.finance.overview.freeConsumedHint')">
              {{ t('admin.finance.overview.freeConsumed') }}
            </dt>
            <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="totals.free_consumed" /></dd>
          </div>
          <div class="flex items-center justify-between gap-3">
            <dt class="text-gray-600 dark:text-gray-300">{{ t('admin.finance.overview.overdraftConsumed') }}</dt>
            <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="totals.overdraft_consumed" /></dd>
          </div>
        </dl>
        <p class="mt-3 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.overview.freeConsumedHint') }}</p>
      </section>

      <section class="card p-4 lg:col-span-2" data-testid="finance-snapshot">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.overview.snapshotTitle') }}</h3>
        <p class="mb-3 mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.overview.snapshotHint') }}</p>
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
          <div v-for="item in snapshotItems" :key="item.key" class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800/60">
            <p class="text-xs text-gray-500 dark:text-gray-400" :title="item.title">{{ item.label }}</p>
            <p class="mt-0.5 text-base font-semibold text-gray-900 dark:text-white">
              <FinanceMoney :value="item.value" />
            </p>
            <p v-if="item.sub" class="text-xs text-gray-500 dark:text-gray-400">{{ item.sub }}</p>
          </div>
        </div>
      </section>
    </div>

    <FinanceTrendChart :series="summary.series" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FinanceSummary } from '@/api/admin/finance'
import { formatCny, formatRatio, isNegativeAmount } from '@/utils/financeFormat'
import FinanceMoney from './FinanceMoney.vue'
import FinanceTrendChart from './FinanceTrendChart.vue'

const props = defineProps<{ summary: FinanceSummary }>()

const { t } = useI18n()

const totals = computed(() => props.summary.totals)

interface PeriodCard {
  key: string
  label: string
  value?: string
  ratio?: string
  ratioNegative?: boolean
  hint?: string
}

const periodCards = computed<PeriodCard[]>(() => {
  const s = totals.value
  return [
    {
      key: 'revenue',
      label: t('admin.finance.overview.cards.revenue'),
      value: s.revenue,
      hint: t('admin.finance.overview.cards.revenueHint', {
        paid: formatCny(s.revenue_paid),
        commission: formatCny(s.revenue_commission)
      })
    },
    {
      key: 'account_cost',
      label: t('admin.finance.overview.cards.accountCost'),
      value: s.account_cost,
      hint: t('admin.finance.overview.cards.accountCostHint', { internal: formatCny(s.account_cost_internal) })
    },
    { key: 'commission', label: t('admin.finance.overview.cards.commission'), value: s.commission },
    {
      key: 'expenses',
      label: t('admin.finance.overview.cards.expenses'),
      value: s.expenses,
      hint: t('admin.finance.overview.cards.expensesHint')
    },
    {
      key: 'profit',
      label: t('admin.finance.overview.cards.profit'),
      value: s.profit,
      hint: t('admin.finance.overview.cards.profitHint')
    },
    {
      key: 'profit_margin',
      label: t('admin.finance.overview.cards.profitMargin'),
      ratio: formatRatio(s.profit_margin),
      ratioNegative: s.profit_margin != null && isNegativeAmount(s.profit_margin, 4),
      hint: t('admin.finance.overview.cards.profitMarginHint')
    }
  ]
})

const snapshotItems = computed(() => {
  const s = props.summary.snapshot
  return [
    { key: 'balance_total', label: t('admin.finance.overview.snapshot.balanceTotal'), value: s.balance_total },
    {
      key: 'balance_paid',
      label: t('admin.finance.overview.snapshot.balancePaid'),
      value: s.balance_paid,
      sub: t('admin.finance.overview.snapshot.balancePaidCash', { amount: formatCny(s.balance_paid_cash_value) })
    },
    { key: 'balance_commission', label: t('admin.finance.overview.snapshot.balanceCommission'), value: s.balance_commission },
    { key: 'balance_free', label: t('admin.finance.overview.snapshot.balanceFree'), value: s.balance_free },
    {
      key: 'balance_unattributed',
      label: t('admin.finance.overview.snapshot.balanceUnattributed'),
      title: t('admin.finance.overview.snapshot.balanceUnattributedHint'),
      value: s.balance_unattributed,
      sub: t('admin.finance.overview.snapshot.balanceUnattributedHint')
    },
    { key: 'frozen_balance', label: t('admin.finance.overview.snapshot.frozenBalance'), value: s.frozen_balance },
    { key: 'affiliate_available', label: t('admin.finance.overview.snapshot.affiliateAvailable'), value: s.affiliate_available },
    { key: 'affiliate_frozen', label: t('admin.finance.overview.snapshot.affiliateFrozen'), value: s.affiliate_frozen }
  ] as Array<{ key: string; label: string; value: string; sub?: string; title?: string }>
})
</script>
