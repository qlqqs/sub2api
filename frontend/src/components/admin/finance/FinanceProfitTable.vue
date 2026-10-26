<template>
  <!-- CUSTOM: 利润明细表：每行一个期间，末行合计（合计由后端给出） -->
  <div class="card overflow-hidden">
    <div class="overflow-x-auto">
      <table class="table min-w-max whitespace-nowrap" data-testid="finance-profit-table">
        <thead>
          <tr>
            <th scope="col" class="sticky left-0 z-10">{{ t('admin.finance.profit.columns.period') }}</th>
            <th v-for="col in COLUMNS" :key="col.key" scope="col" class="text-right">{{ t(col.label) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="rows.length === 0">
            <td :colspan="COLUMNS.length + 1" class="py-8 text-center text-gray-500 dark:text-gray-400">
              {{ t('admin.finance.noData') }}
            </td>
          </tr>
          <tr v-for="row in rows" :key="row.period">
            <td class="sticky left-0 bg-white font-mono text-xs dark:bg-dark-900">{{ row.period }}</td>
            <td v-for="col in COLUMNS" :key="col.key" class="text-right">
              <span
                v-if="col.key === 'profit_margin'"
                class="tabular-nums"
                :class="isMarginNegative(row.profit_margin) ? 'text-red-600 dark:text-red-400' : undefined"
              >{{ formatRatio(row.profit_margin) }}</span>
              <FinanceMoney v-else :value="row[col.key]" :currency="false" :color-negative="col.key === 'profit'" />
            </td>
          </tr>
        </tbody>
        <tfoot v-if="rows.length > 0">
          <tr class="bg-gray-50 font-semibold dark:bg-dark-800/50">
            <td class="sticky left-0 bg-gray-50 dark:bg-dark-800">{{ t('admin.finance.total') }}</td>
            <td v-for="col in COLUMNS" :key="col.key" class="text-right">
              <span
                v-if="col.key === 'profit_margin'"
                class="tabular-nums"
                :class="isMarginNegative(totals.profit_margin) ? 'text-red-600 dark:text-red-400' : undefined"
              >{{ formatRatio(totals.profit_margin) }}</span>
              <FinanceMoney v-else :value="totals[col.key]" :currency="false" :color-negative="col.key === 'profit'" />
            </td>
          </tr>
        </tfoot>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FinancePeriodTotals, FinanceSummary } from '@/api/admin/finance'
import { formatRatio, isNegativeAmount } from '@/utils/financeFormat'
import FinanceMoney from './FinanceMoney.vue'

const props = defineProps<{ summary: FinanceSummary }>()

const { t } = useI18n()

const COLUMNS: Array<{ key: keyof FinancePeriodTotals; label: string }> = [
  { key: 'revenue', label: 'admin.finance.profit.columns.revenue' },
  { key: 'revenue_paid', label: 'admin.finance.profit.columns.revenuePaid' },
  { key: 'revenue_commission', label: 'admin.finance.profit.columns.revenueCommission' },
  { key: 'free_consumed', label: 'admin.finance.profit.columns.freeConsumed' },
  { key: 'overdraft_consumed', label: 'admin.finance.profit.columns.overdraftConsumed' },
  { key: 'consumption', label: 'admin.finance.profit.columns.consumption' },
  { key: 'account_cost', label: 'admin.finance.profit.columns.accountCost' },
  { key: 'account_cost_internal', label: 'admin.finance.profit.columns.accountCostInternal' },
  { key: 'commission', label: 'admin.finance.profit.columns.commission' },
  { key: 'expenses', label: 'admin.finance.profit.columns.expenses' },
  { key: 'profit', label: 'admin.finance.profit.columns.profit' },
  { key: 'profit_margin', label: 'admin.finance.profit.columns.profitMargin' }
]

const rows = computed(() => props.summary.series)
const totals = computed(() => props.summary.totals)

function isMarginNegative(value: string | null): boolean {
  return value != null && isNegativeAmount(value, 4)
}
</script>
