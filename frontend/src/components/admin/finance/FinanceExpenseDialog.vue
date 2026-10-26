<template>
  <!-- CUSTOM: 新增 / 编辑其他支出 -->
  <BaseDialog
    :show="show"
    :title="expense ? t('admin.finance.expenses.editTitle') : t('admin.finance.expenses.createTitle')"
    width="normal"
    @close="emit('close')"
  >
    <form id="finance-expense-form" class="space-y-4" novalidate @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <label class="input-label" for="finance-expense-date">{{ t('admin.finance.expenses.form.occurredOn') }}</label>
          <input
            id="finance-expense-date"
            v-model="draft.occurred_on"
            type="date"
            class="input"
            :class="{ 'border-red-400 dark:border-red-500': errors.occurred_on }"
            data-testid="finance-expense-date"
          />
          <p v-if="errors.occurred_on" class="mt-1 text-xs text-red-500">{{ errorText(errors.occurred_on) }}</p>
        </div>
        <div>
          <label class="input-label" for="finance-expense-category">{{ t('admin.finance.expenses.form.category') }}</label>
          <Select
            id="finance-expense-category"
            v-model="draft.category"
            :options="categoryOptions"
            data-testid="finance-expense-category"
          />
        </div>
        <div>
          <label class="input-label" for="finance-expense-amount">{{ t('admin.finance.expenses.form.amount') }}</label>
          <input
            id="finance-expense-amount"
            v-model="draft.amount"
            type="text"
            inputmode="decimal"
            class="input"
            :class="{ 'border-red-400 dark:border-red-500': errors.amount }"
            placeholder="0.00"
            data-testid="finance-expense-amount"
          />
          <p v-if="errors.amount" class="mt-1 text-xs text-red-500">{{ errorText(errors.amount) }}</p>
        </div>
        <div>
          <span id="finance-expense-mode-label" class="input-label">{{ t('admin.finance.expenses.form.amortizeMode') }}</span>
          <div
            role="radiogroup"
            aria-labelledby="finance-expense-mode-label"
            class="grid grid-cols-2 gap-1 rounded-lg bg-gray-100 p-1 dark:bg-dark-700"
            data-testid="finance-expense-mode"
          >
            <button
              v-for="mode in AMORTIZE_MODES"
              :key="mode"
              type="button"
              role="radio"
              :aria-checked="draft.amortize_mode === mode"
              class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
              :class="draft.amortize_mode === mode
                ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-500 dark:text-primary-300'
                : 'text-gray-600 hover:text-gray-900 dark:text-gray-300 dark:hover:text-white'"
              :data-testid="`finance-expense-mode-${mode}`"
              @click="draft.amortize_mode = mode"
            >
              {{ t(`admin.finance.expenses.form.amortizeModes.${mode}`) }}
            </button>
          </div>
        </div>
        <div v-if="draft.amortize_mode === 'daily'">
          <label class="input-label" for="finance-expense-days">{{ t('admin.finance.expenses.form.amortizeDays') }}</label>
          <input
            id="finance-expense-days"
            v-model="draft.amortize_days"
            type="text"
            inputmode="numeric"
            class="input"
            :class="{ 'border-red-400 dark:border-red-500': errors.amortize_days }"
            data-testid="finance-expense-days"
          />
          <p v-if="errors.amortize_days" class="mt-1 text-xs text-red-500">{{ errorText(errors.amortize_days) }}</p>
        </div>
      </div>
      <p class="input-hint -mt-2">
        {{ t(draft.amortize_mode === 'daily' ? 'admin.finance.expenses.form.amortizeDailyHint' : 'admin.finance.expenses.form.amortizeOnceHint') }}
      </p>
      <div>
        <label class="input-label" for="finance-expense-account">{{ t('admin.finance.expenses.form.account') }}</label>
        <Select
          id="finance-expense-account"
          v-model="draft.account_id"
          :options="accountOptions"
          searchable="auto"
          data-testid="finance-expense-account"
        />
      </div>
      <div>
        <label class="input-label" for="finance-expense-note">{{ t('admin.finance.expenses.form.note') }}</label>
        <textarea
          id="finance-expense-note"
          v-model="draft.note"
          rows="2"
          class="input"
          :class="{ 'border-red-400 dark:border-red-500': errors.note }"
          :maxlength="EXPENSE_NOTE_MAX"
          :placeholder="t('admin.finance.expenses.form.notePlaceholder')"
          data-testid="finance-expense-note"
        />
        <p v-if="errors.note" class="mt-1 text-xs text-red-500">{{ errorText(errors.note) }}</p>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="finance-expense-form" class="btn btn-primary" :disabled="saving" data-testid="finance-expense-submit">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import {
  FINANCE_EXPENSE_CATEGORIES,
  type FinanceExpense,
  type FinanceExpenseCategory,
  type FinanceSettingsAccount
} from '@/api/admin/finance'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { beijingDate } from '@/utils/financeDate'
import {
  EXPENSE_NOTE_MAX,
  amortizeFieldsFromDays,
  amortizeModeAfterCategoryChange,
  newExpenseDraft,
  type ExpenseAmortizeMode,
  validateExpenseDraft,
  type ExpenseDraft,
  type ExpenseFieldError
} from '@/utils/financeExpense'

const props = withDefaults(
  defineProps<{
    show: boolean
    /** 编辑的条目；为空表示新增。 */
    expense?: FinanceExpense | null
    accounts?: FinanceSettingsAccount[]
  }>(),
  { expense: null, accounts: () => [] }
)

const emit = defineEmits<{ close: []; saved: [expense: FinanceExpense] }>()

const { t } = useI18n()
const appStore = useAppStore()

const draft = reactive<ExpenseDraft>(newExpenseDraft(beijingDate()))
const errors = ref<Partial<Record<keyof ExpenseDraft, ExpenseFieldError>>>({})
const saving = ref(false)
/** 程序性重置表单时不触发「切换类别改默认天数」。 */
let resetting = false

function reset() {
  resetting = true
  const e = props.expense
  Object.assign(
    draft,
    e
      ? {
          occurred_on: e.occurred_on,
          category: e.category,
          amount: e.amount,
          ...amortizeFieldsFromDays(e.amortize_days),
          account_id: e.account_id,
          note: e.note
        }
      : newExpenseDraft(beijingDate())
  )
  errors.value = {}
  // 等类别 watcher 跑完（flush: 'sync'）后再放开
  resetting = false
}

watch(
  () => [props.show, props.expense] as const,
  ([show]) => {
    if (show) reset()
  },
  { immediate: true }
)

watch(
  () => draft.category,
  (to: FinanceExpenseCategory, from: FinanceExpenseCategory | undefined) => {
    if (resetting || !from || to === from) return
    draft.amortize_mode = amortizeModeAfterCategoryChange(draft.amortize_mode, from, to)
  },
  { flush: 'sync' }
)

const AMORTIZE_MODES: ExpenseAmortizeMode[] = ['once', 'daily']

const categoryOptions = computed<SelectOption[]>(() =>
  FINANCE_EXPENSE_CATEGORIES.map((c) => ({ value: c, label: t(`admin.finance.expenses.categories.${c}`) }))
)

const accountOptions = computed<SelectOption[]>(() => {
  const list: SelectOption[] = props.accounts.map((a) => ({
    value: a.id,
    label: `#${a.id} ${a.name}${a.deleted ? `（${t('admin.finance.settings.deletedTag')}）` : ''}`
  }))
  // 编辑时关联的账号不在列表里也要能显示
  const current = props.expense
  if (current?.account_id != null && !list.some((a) => a.value === current.account_id)) {
    list.push({ value: current.account_id, label: `#${current.account_id} ${current.account_name ?? ''}`.trim() })
  }
  return [{ value: null, label: t('admin.finance.expenses.form.accountNone') }, ...list]
})

function errorText(error: ExpenseFieldError): string {
  return t(`admin.finance.expenses.errors.${error}`)
}

async function submit() {
  if (saving.value) return
  const result = validateExpenseDraft(draft)
  if (!result.ok) {
    errors.value = result.errors
    return
  }
  errors.value = {}
  saving.value = true
  try {
    const saved = props.expense
      ? await adminAPI.finance.updateExpense(props.expense.id, result.payload)
      : await adminAPI.finance.createExpense(result.payload)
    appStore.showSuccess(t(props.expense ? 'admin.finance.expenses.updated' : 'admin.finance.expenses.created'))
    emit('saved', saved)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.unknownError')))
  } finally {
    saving.value = false
  }
}

defineExpose({ draft })
</script>
