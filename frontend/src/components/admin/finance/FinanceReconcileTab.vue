<template>
  <!-- CUSTOM: 用户对账（全时段）：理论余额 vs 实际余额，差额 |d| ≥ 0.01 标红 -->
  <div class="space-y-3">
    <div class="flex flex-wrap items-center gap-3">
      <div class="w-full sm:w-72">
        <SearchInput
          v-model="searchInput"
          :placeholder="t('admin.finance.reconcile.searchPlaceholder')"
          @search="onSearch"
        />
      </div>
      <label class="inline-flex cursor-pointer items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <Toggle v-model="onlyMismatch" :aria-label="t('admin.finance.reconcile.onlyMismatch')" data-testid="finance-reconcile-only-mismatch" />
        {{ t('admin.finance.reconcile.onlyMismatch') }}
      </label>
      <p class="text-xs text-gray-500 dark:text-gray-400 sm:ml-auto">{{ t('admin.finance.reconcile.hint') }}</p>
    </div>

    <FinanceLoadState :loading="loading" :error="error" :has-data="!!data" @retry="load">
      <div v-if="data" class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="table min-w-max whitespace-nowrap" data-testid="finance-reconcile-table">
            <thead>
              <tr>
                <th scope="col" class="sticky left-0 z-10">{{ t('admin.finance.reconcile.columns.user') }}</th>
                <th v-for="col in AMOUNT_COLUMNS" :key="col.key" scope="col" class="text-right">{{ t(col.label) }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.reconcile.columns.difference') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.reconcile.columns.remaining') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="data.items.length === 0">
                <td :colspan="AMOUNT_COLUMNS.length + 3" class="py-8 text-center text-gray-500 dark:text-gray-400">
                  {{ t('admin.finance.noData') }}
                </td>
              </tr>
              <tr
                v-for="row in data.items"
                :key="row.user_id"
                :class="{ 'bg-red-50/70 dark:bg-red-500/10': isReconcileMismatch(row.difference) }"
                :data-mismatch="isReconcileMismatch(row.difference) ? 'true' : undefined"
                data-testid="finance-reconcile-row"
              >
                <td
                  class="sticky left-0"
                  :class="isReconcileMismatch(row.difference) ? 'bg-red-50 dark:bg-red-950' : 'bg-white dark:bg-dark-900'"
                >
                  <span class="text-gray-900 dark:text-white">{{ row.user_email || `#${row.user_id}` }}</span>
                  <span class="ml-1 text-xs text-gray-400">#{{ row.user_id }}</span>
                  <span v-if="row.role === 'admin'" class="badge badge-primary ml-1">admin</span>
                  <span v-if="row.deleted" class="badge badge-gray ml-1">{{ t('admin.finance.reconcile.deletedUser') }}</span>
                </td>
                <td v-for="col in AMOUNT_COLUMNS" :key="col.key" class="text-right">
                  <FinanceMoney :value="row[col.key]" :currency="false" :color-negative="false" />
                </td>
                <td class="text-right">
                  <span
                    v-if="isReconcileMismatch(row.difference)"
                    class="inline-flex items-center gap-1 font-semibold text-red-600 dark:text-red-400"
                    data-testid="finance-reconcile-difference"
                  >
                    <Icon name="exclamationTriangle" size="xs" />
                    <span class="sr-only">{{ t('admin.finance.reconcile.mismatch') }}</span>
                    <FinanceMoney :value="row.difference" :currency="false" :color-negative="false" />
                  </span>
                  <FinanceMoney v-else :value="row.difference" :currency="false" :color-negative="false" />
                </td>
                <td class="text-right text-xs text-gray-500 dark:text-gray-400">
                  <FinanceMoney :value="row.remaining.paid" :currency="false" :color-negative="false" /> /
                  <FinanceMoney :value="row.remaining.commission" :currency="false" :color-negative="false" /> /
                  <FinanceMoney :value="row.remaining.free" :currency="false" :color-negative="false" />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </FinanceLoadState>

    <Pagination
      v-if="data && data.total > 0"
      :total="data.total"
      :page="page"
      :page-size="pageSize"
      @update:page="page = $event"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { isReconcileMismatch, type FinanceReconcileRow } from '@/api/admin/finance'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import FinanceLoadState from './FinanceLoadState.vue'
import FinanceMoney from './FinanceMoney.vue'

const props = defineProps<{ refreshToken: number }>()

const { t } = useI18n()

type AmountKey = Exclude<keyof FinanceReconcileRow, 'user_id' | 'user_email' | 'role' | 'deleted' | 'remaining' | 'difference'>

const AMOUNT_COLUMNS: Array<{ key: AmountKey; label: string }> = [
  { key: 'paid_in', label: 'admin.finance.reconcile.columns.paidIn' },
  { key: 'paid_cash', label: 'admin.finance.reconcile.columns.paidCash' },
  { key: 'free_in', label: 'admin.finance.reconcile.columns.freeIn' },
  { key: 'free_out', label: 'admin.finance.reconcile.columns.freeOut' },
  { key: 'commission_in', label: 'admin.finance.reconcile.columns.commissionIn' },
  { key: 'refund_deducted', label: 'admin.finance.reconcile.columns.refundDeducted' },
  { key: 'consumption', label: 'admin.finance.reconcile.columns.consumption' },
  { key: 'overdraft', label: 'admin.finance.reconcile.columns.overdraft' },
  { key: 'expected_balance', label: 'admin.finance.reconcile.columns.expectedBalance' },
  { key: 'actual_balance', label: 'admin.finance.reconcile.columns.actualBalance' }
]

const searchInput = ref('')
const search = ref('')
const onlyMismatch = ref(false)
const page = ref(1)
const pageSize = ref(50)

const { data, loading, error, load } = useFinanceLoad(
  (signal) =>
    adminAPI.finance.getReconcile(
      { page: page.value, page_size: pageSize.value, only_mismatch: onlyMismatch.value, search: search.value },
      { signal }
    ),
  () => t('common.unknownError')
)

function onSearch(value: string) {
  search.value = value.trim()
}

function onPageSizeChange(size: number) {
  pageSize.value = size
  page.value = 1
}

watch([search, onlyMismatch], () => {
  if (page.value !== 1) page.value = 1
  else void load()
})
watch(() => [page.value, pageSize.value, props.refreshToken], load, { immediate: true })
</script>
