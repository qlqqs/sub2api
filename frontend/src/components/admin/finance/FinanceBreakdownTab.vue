<template>
  <!-- CUSTOM: 消耗与成本：按账号 / 分组 / 模型 / 用户拆分 -->
  <div class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="tabs w-fit" role="tablist" :aria-label="t('admin.finance.tabs.breakdown')">
        <button
          v-for="d in DIMS"
          :key="d"
          type="button"
          role="tab"
          class="tab !px-3 !py-1.5"
          :class="{ 'tab-active': dim === d }"
          :aria-selected="dim === d"
          :data-testid="`finance-breakdown-dim-${d}`"
          @click="dim = d"
        >
          {{ t(`admin.finance.breakdown.dims.${d}`) }}
        </button>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t(dim === 'account' ? 'admin.finance.breakdown.grossProfitHintAccount' : 'admin.finance.breakdown.grossProfitHint') }}
      </p>
    </div>

    <FinanceLoadState :loading="loading" :error="error" :has-data="!!data" @retry="load">
      <div v-if="data" class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="table min-w-max whitespace-nowrap" data-testid="finance-breakdown-table">
            <thead>
              <tr>
                <th scope="col">{{ t('admin.finance.breakdown.columns.name') }}</th>
                <th v-if="data.dim === 'account'" scope="col">{{ t('admin.finance.breakdown.columns.costMode') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.breakdown.columns.requests') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.breakdown.columns.images') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.breakdown.columns.consumption') }}</th>
                <th scope="col" class="text-right" :title="t('admin.finance.breakdown.subscriptionHint')">
                  {{ t('admin.finance.breakdown.columns.subscriptionConsumption') }}
                </th>
                <th scope="col" class="text-right">{{ t('admin.finance.breakdown.columns.accountCost') }}</th>
                <th v-if="data.dim === 'account'" scope="col" class="text-right" :title="t('admin.finance.breakdown.expenseCostHint')">
                  {{ t('admin.finance.breakdown.columns.expenseCost') }}
                </th>
                <th scope="col" class="text-right">{{ t('admin.finance.breakdown.columns.grossProfit') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="data.rows.length === 0">
                <td :colspan="data.dim === 'account' ? 9 : 7" class="py-8 text-center text-gray-500 dark:text-gray-400">
                  {{ t('admin.finance.noData') }}
                </td>
              </tr>
              <tr v-for="row in data.rows" :key="row.key">
                <td class="max-w-xs truncate" :title="row.label">
                  <span :class="{ 'font-mono text-xs': data.dim === 'model' }">{{ row.label }}</span>
                  <span v-if="data.dim !== 'model'" class="ml-1 text-xs text-gray-400">#{{ row.key }}</span>
                </td>
                <td v-if="data.dim === 'account'">
                  <span v-if="row.cost_mode" class="badge" :class="costModeClass(row.cost_mode)">
                    {{ t(`admin.finance.breakdown.costModes.${row.cost_mode}`) }}
                  </span>
                  <span v-else class="text-gray-400">—</span>
                </td>
                <td class="text-right tabular-nums">{{ row.requests.toLocaleString() }}</td>
                <td class="text-right tabular-nums">{{ row.images.toLocaleString() }}</td>
                <td class="text-right"><FinanceMoney :value="row.consumption" :color-negative="false" /></td>
                <td class="text-right text-gray-500 dark:text-gray-400">
                  <FinanceMoney :value="row.subscription_consumption" :color-negative="false" />
                </td>
                <td class="text-right"><FinanceMoney :value="row.account_cost" :color-negative="false" /></td>
                <td v-if="data.dim === 'account'" class="text-right">
                  <FinanceMoney :value="row.expense_cost ?? '0'" :color-negative="false" />
                </td>
                <td class="text-right"><FinanceMoney :value="row.gross_profit" /></td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="data.truncated" class="px-4 py-2 text-xs text-amber-600 dark:text-amber-400">
          {{ t('admin.finance.breakdown.truncated', { limit: LIMIT }) }}
        </p>
      </div>
    </FinanceLoadState>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { FinanceBreakdownDim, FinanceCostMode } from '@/api/admin/finance'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import FinanceLoadState from './FinanceLoadState.vue'
import FinanceMoney from './FinanceMoney.vue'

const props = defineProps<{ start: string; end: string; refreshToken: number }>()

const { t } = useI18n()

const DIMS: FinanceBreakdownDim[] = ['account', 'group', 'model', 'user']
const LIMIT = 200

const dim = ref<FinanceBreakdownDim>('account')

const { data, loading, error, load } = useFinanceLoad(
  (signal) =>
    adminAPI.finance.getBreakdown({ start: props.start, end: props.end, dim: dim.value, limit: LIMIT }, { signal }),
  () => t('common.unknownError')
)

watch(() => [props.start, props.end, props.refreshToken, dim.value], load, { immediate: true })

function costModeClass(mode: FinanceCostMode): string {
  switch (mode) {
    case 'model_rate':
      return 'badge-primary'
    case 'per_image':
      return 'badge-purple'
    case 'fixed':
      return 'badge-warning'
    default:
      return 'badge-gray'
  }
}
</script>
