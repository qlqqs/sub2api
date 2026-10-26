<template>
  <div class="space-y-4" data-testid="finance-settings">
    <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.currentOnly') }}</p>
    <div class="overflow-x-auto">
      <nav class="tabs w-max" :aria-label="t('admin.finance.tabs.settings')">
        <button v-for="item in sections" :key="item" type="button" class="tab whitespace-nowrap" :class="{ 'tab-active': section === item }" :aria-current="section === item ? 'page' : undefined" :data-testid="`finance-settings-${item}`" @click="section = item">{{ t(`admin.finance.costSettings.${item}`) }}</button>
      </nav>
    </div>
    <FinanceAccountCostSettings ref="accountSettings" v-if="section === 'accounts'" :refresh-token="refreshToken" @changed="emit('changed')" @expenses="section = 'expenses'" />
    <FinancePaymentSettings v-else-if="section === 'payment'" :refresh-token="refreshToken" @changed="emit('changed')" />
    <section v-else-if="section === 'expenses'" class="space-y-4">
      <div class="card space-y-2 p-4">
        <label class="block text-sm">{{ t('admin.finance.costSettings.expenseRange') }}</label>
        <DateRangePicker v-model:start-date="expenseStart" v-model:end-date="expenseEnd" />
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.expenseSource') }}</p>
        <p v-if="rangeError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ rangeError }}</p>
      </div>
      <FinanceExpensesTab :start="applied.start" :end="applied.end" :refresh-token="refreshToken" @changed="emit('changed')" />
    </section>
    <slot v-else name="history" :refresh-token="refreshToken" :changed="notifyChanged">
      <p class="card p-4 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.historyPending') }}</p>
    </slot>
  </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter, onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import FinanceAccountCostSettings from './FinanceAccountCostSettings.vue'
import FinancePaymentSettings from './FinancePaymentSettings.vue'
import FinanceExpensesTab from './FinanceExpensesTab.vue'
import { defaultFinanceRange, inclusiveDays } from '@/utils/financeDate'
import { FINANCE_MAX_RANGE_DAYS } from '@/api/admin/finance'
defineProps<{ refreshToken: number }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const route = useRoute(), router = useRouter()
const sections = ['accounts', 'payment', 'expenses', 'history'] as const
type Section = typeof sections[number]
const accountSettings = ref<InstanceType<typeof FinanceAccountCostSettings> | null>(null)
const section = computed<Section>({
  get: () => sections.find(value => value === route.query.section) ?? 'accounts',
  set: value => {
    const navigate = () => { void router.replace({ query: { ...route.query, section: value === 'accounts' ? undefined : value } }) }
    if (accountSettings.value) accountSettings.value.requestLeave(navigate)
    else navigate()
  }
})
const initial = defaultFinanceRange()
function queryDate(value: unknown, fallback: string) { return typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value) ? value : fallback }
const expenseStart = ref(queryDate(route.query.expense_start, initial.start))
const expenseEnd = ref(queryDate(route.query.expense_end, initial.end))
const applied = ref({ ...initial })
const rangeError = computed(() => {
  const days = inclusiveDays(expenseStart.value, expenseEnd.value)
  if (!Number.isFinite(days) || days < 1) return t('admin.finance.toolbar.rangeInvalid')
  if (days > FINANCE_MAX_RANGE_DAYS) return t('admin.finance.toolbar.rangeTooLarge', { max: FINANCE_MAX_RANGE_DAYS })
  return ''
})
watch([expenseStart, expenseEnd], () => {
  if (rangeError.value) return
  applied.value = { start: expenseStart.value, end: expenseEnd.value }
  if (route.query.expense_start === expenseStart.value && route.query.expense_end === expenseEnd.value) return
  if (section.value !== 'expenses') return
  void router.replace({ query: { ...route.query, expense_start: expenseStart.value, expense_end: expenseEnd.value } })
}, { immediate: true })
watch(() => [route.query.expense_start, route.query.expense_end], () => {
  expenseStart.value = queryDate(route.query.expense_start, initial.start)
  expenseEnd.value = queryDate(route.query.expense_end, initial.end)
})
async function confirmLeave() { return await accountSettings.value?.confirmLeave() ?? true }
onBeforeRouteLeave(confirmLeave)
onBeforeRouteUpdate(confirmLeave)
defineExpose({ confirmLeave })
function notifyChanged() { emit('changed') }
</script>
