<template>
  <!-- CUSTOM: 其他支出：按发生日期筛选，增删改查 -->
  <div class="space-y-3">
    <div class="flex flex-wrap items-center gap-3">
      <div class="w-48">
        <Select v-model="category" :options="categoryFilterOptions" :aria-label="t('admin.finance.expenses.columns.category')" />
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.expenses.rangeHint') }}</p>
      <button type="button" class="btn btn-primary ml-auto" data-testid="finance-expense-add" @click="openCreate">
        <Icon name="plus" size="sm" class="mr-1" />
        {{ t('admin.finance.expenses.add') }}
      </button>
    </div>

    <FinanceLoadState :loading="loading" :error="error" :has-data="!!data" @retry="load">
      <div v-if="data" class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="table min-w-max whitespace-nowrap" data-testid="finance-expenses-table">
            <thead>
              <tr>
                <th scope="col">{{ t('admin.finance.expenses.columns.occurredOn') }}</th>
                <th scope="col">{{ t('admin.finance.expenses.columns.category') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.expenses.columns.amount') }}</th>
                <th scope="col">{{ t('admin.finance.expenses.columns.amortize') }}</th>
                <th scope="col">{{ t('admin.finance.expenses.columns.account') }}</th>
                <th scope="col">{{ t('admin.finance.expenses.columns.note') }}</th>
                <th scope="col">{{ t('admin.finance.expenses.columns.createdBy') }}</th>
                <th scope="col" class="text-right">{{ t('admin.finance.expenses.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="data.items.length === 0">
                <td colspan="8" class="py-8 text-center text-gray-500 dark:text-gray-400">{{ t('admin.finance.noData') }}</td>
              </tr>
              <tr v-for="item in data.items" :key="item.id">
                <td class="font-mono text-xs">{{ item.occurred_on }}</td>
                <td>{{ t(`admin.finance.expenses.categories.${item.category}`) }}</td>
                <td class="text-right"><FinanceMoney :value="item.amount" /></td>
                <td class="text-xs">
                  {{ item.amortize_days > 1 ? t('admin.finance.expenses.amortizeDays', { days: item.amortize_days }) : t('admin.finance.expenses.amortizeOnce') }}
                  <span v-if="item.amortize_days > 1" class="block text-gray-400">
                    {{ t('admin.finance.expenses.dailyAmount', { amount: formatCny(item.daily_amount, 4) }) }}
                  </span>
                </td>
                <td class="text-xs">
                  <template v-if="item.account_id != null">#{{ item.account_id }} {{ item.account_name ?? '' }}</template>
                  <span v-else class="text-gray-400">—</span>
                </td>
                <td class="max-w-xs truncate text-xs" :title="item.note">{{ item.note || '—' }}</td>
                <td class="text-xs text-gray-500 dark:text-gray-400">{{ item.created_by_email ?? '—' }}</td>
                <td class="text-right">
                  <div class="inline-flex gap-1">
                    <button
                      type="button"
                      class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700"
                      :title="t('common.edit')"
                      :aria-label="t('common.edit')"
                      @click="openEdit(item)"
                    >
                      <Icon name="edit" size="sm" />
                    </button>
                    <button
                      type="button"
                      class="rounded-lg p-1.5 text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-500/10"
                      :title="t('common.delete')"
                      :aria-label="t('common.delete')"
                      @click="deleting = item"
                    >
                      <Icon name="trash" size="sm" />
                    </button>
                  </div>
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

    <FinanceExpenseDialog
      :show="dialogOpen"
      :expense="editing"
      :accounts="accounts"
      @close="dialogOpen = false"
      @saved="onSaved"
    />
    <ConfirmDialog
      :show="deleting !== null"
      :title="t('admin.finance.expenses.deleteConfirmTitle')"
      :message="deleteMessage"
      :confirm-text="t('common.delete')"
      danger
      @confirm="confirmDelete"
      @cancel="deleting = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import {
  FINANCE_EXPENSE_CATEGORIES,
  type FinanceExpense,
  type FinanceExpenseCategory,
  type FinanceSettingsAccount
} from '@/api/admin/finance'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCny } from '@/utils/financeFormat'
import FinanceExpenseDialog from './FinanceExpenseDialog.vue'
import FinanceLoadState from './FinanceLoadState.vue'
import FinanceMoney from './FinanceMoney.vue'

const props = defineProps<{ start: string; end: string; refreshToken: number }>()
/** 支出变化会影响总览的利润，通知页面刷新汇总。 */
const emit = defineEmits<{ changed: [] }>()

const { t } = useI18n()
const appStore = useAppStore()

const category = ref<FinanceExpenseCategory | ''>('')
const page = ref(1)
const pageSize = ref(20)

const categoryFilterOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.finance.expenses.allCategories') },
  ...FINANCE_EXPENSE_CATEGORIES.map((c) => ({ value: c, label: t(`admin.finance.expenses.categories.${c}`) }))
])

const { data, loading, error, load } = useFinanceLoad(
  (signal) =>
    adminAPI.finance.listExpenses(
      { start: props.start, end: props.end, category: category.value, page: page.value, page_size: pageSize.value },
      { signal }
    ),
  () => t('common.unknownError')
)

watch(
  () => [props.start, props.end, category.value],
  () => {
    if (page.value !== 1) page.value = 1
    else void load()
  }
)
watch(() => [page.value, pageSize.value, props.refreshToken], load, { immediate: true })

function onPageSizeChange(size: number) {
  pageSize.value = size
  page.value = 1
}

// 关联账号的候选：与「设置」同源（全部未删除账号 + 已选中但已删除的）
const accounts = ref<FinanceSettingsAccount[]>([])
async function loadAccounts() {
  try {
    accounts.value = (await adminAPI.finance.getSettings()).accounts
  } catch {
    accounts.value = []
  }
}
void loadAccounts()

const dialogOpen = ref(false)
const editing = ref<FinanceExpense | null>(null)

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}

function openEdit(item: FinanceExpense) {
  editing.value = item
  dialogOpen.value = true
}

function onSaved() {
  dialogOpen.value = false
  void load()
  emit('changed')
}

const deleting = ref<FinanceExpense | null>(null)
const deleteMessage = computed(() => {
  const item = deleting.value
  if (!item) return ''
  return t('admin.finance.expenses.deleteConfirm', {
    date: item.occurred_on,
    category: t(`admin.finance.expenses.categories.${item.category}`),
    amount: formatCny(item.amount)
  })
})

async function confirmDelete() {
  const item = deleting.value
  if (!item) return
  deleting.value = null
  try {
    await adminAPI.finance.deleteExpense(item.id)
    appStore.showSuccess(t('admin.finance.expenses.deleted'))
    void load()
    emit('changed')
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.unknownError')))
  }
}
</script>
