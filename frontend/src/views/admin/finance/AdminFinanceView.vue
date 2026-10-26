<template>
  <!-- CUSTOM: 财务明细页（仅管理员）。接口约定见 .trellis/tasks/10-25-finance-ledger/contract.md -->
  <AppLayout>
    <div class="space-y-4">
      <div v-if="activeTab !== 'settings'" class="card p-4" data-testid="finance-report-toolbar">
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.finance.toolbar.range') }}:</span>
            <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" />
          </div>
          <div class="tabs !p-0.5" role="radiogroup" :aria-label="t('admin.finance.toolbar.granularity')">
            <button
              v-for="g in GRANULARITIES"
              :key="g"
              type="button"
              role="radio"
              class="tab !px-3 !py-1"
              :class="{ 'tab-active': granularity === g }"
              :aria-checked="granularity === g"
              :data-testid="`finance-granularity-${g}`"
              @click="granularity = g"
            >
              {{ t(`admin.finance.toolbar.${g}`) }}
            </button>
          </div>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="summaryLoading"
            :title="t('admin.finance.toolbar.refresh')"
            :aria-label="t('admin.finance.toolbar.refresh')"
            data-testid="finance-refresh"
            @click="refresh"
          >
            <Icon name="refresh" size="md" :class="summaryLoading ? 'animate-spin' : ''" />
          </button>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="exporting || !!rangeError"
            data-testid="finance-export"
            @click="exportCsv"
          >
            <Icon name="download" size="sm" class="mr-1" />
            {{ exporting ? t('admin.finance.toolbar.exporting') : t('admin.finance.toolbar.export') }}
          </button>
          <span v-if="summary?.computed_at" class="ml-auto text-xs text-gray-400 dark:text-gray-500">
            {{ t('admin.finance.toolbar.computedAt', { time: formatDateTime(summary.computed_at) }) }}
          </span>
        </div>
        <p v-if="rangeError" class="mt-2 text-sm text-red-600 dark:text-red-400" role="alert" data-testid="finance-range-error">
          {{ rangeError }}
        </p>
      </div>

      <div
        v-if="activeTab !== 'settings' && summary && summary.warnings.length > 0"
        class="flex gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200"
        role="status"
        data-testid="finance-warnings"
      >
        <Icon name="exclamationTriangle" size="md" class="mt-0.5 shrink-0" />
        <div>
          <p class="font-medium">{{ t('admin.finance.warningsTitle') }}</p>
          <ul class="mt-1 list-disc space-y-0.5 pl-4">
            <li v-for="(warning, index) in summary.warnings" :key="index">{{ warning }}</li>
          </ul>
        </div>
      </div>

      <div class="overflow-x-auto">
        <nav class="tabs w-max" role="tablist" :aria-label="t('admin.finance.title')">
          <button
            v-for="tab in TABS"
            :key="tab"
            type="button"
            role="tab"
            class="tab whitespace-nowrap"
            :class="{ 'tab-active': activeTab === tab }"
            :aria-selected="activeTab === tab"
            :data-testid="`finance-tab-${tab}`"
            @click="selectTab(tab)"
          >
            {{ t(`admin.finance.tabs.${tab}`) }}
          </button>
        </nav>
      </div>

      <section role="tabpanel">
        <FinanceLoadState
          v-if="activeTab === 'overview' || activeTab === 'profit'"
          :loading="summaryLoading"
          :error="summaryError"
          :has-data="!!summary"
          @retry="loadSummary"
        >
          <FinanceOverviewTab v-if="summary && activeTab === 'overview'" :summary="summary" />
          <FinanceProfitTable v-else-if="summary" :summary="summary" />
        </FinanceLoadState>
        <FinanceRechargeTab
          v-else-if="activeTab === 'recharge'"
          :start="applied.start"
          :end="applied.end"
          :refresh-token="refreshToken"
        />
        <FinanceBreakdownTab
          v-else-if="activeTab === 'breakdown'"
          :start="applied.start"
          :end="applied.end"
          :refresh-token="refreshToken"
        />
        <FinanceAffiliateTab
          v-else-if="activeTab === 'affiliate'"
          :start="applied.start"
          :end="applied.end"
          :refresh-token="refreshToken"
        />
        <FinanceExpensesTab
          v-else-if="activeTab === 'expenses'"
          :start="applied.start"
          :end="applied.end"
          :refresh-token="refreshToken"
          @changed="loadSummary"
        />
        <FinanceReconcileTab v-else-if="activeTab === 'reconcile'" :refresh-token="refreshToken" />
        <FinanceSettingsTab ref="settingsTab" v-else-if="activeTab === 'settings'" :refresh-token="refreshToken" @changed="loadSummary">
          <template #history="slot">
            <FinanceRecalculationTab :refresh-token="slot.refreshToken" @changed="slot.changed" />
          </template>
        </FinanceSettingsTab>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Icon from '@/components/icons/Icon.vue'
import FinanceAffiliateTab from '@/components/admin/finance/FinanceAffiliateTab.vue'
import FinanceBreakdownTab from '@/components/admin/finance/FinanceBreakdownTab.vue'
import FinanceExpensesTab from '@/components/admin/finance/FinanceExpensesTab.vue'
import FinanceLoadState from '@/components/admin/finance/FinanceLoadState.vue'
import FinanceOverviewTab from '@/components/admin/finance/FinanceOverviewTab.vue'
import FinanceProfitTable from '@/components/admin/finance/FinanceProfitTable.vue'
import FinanceRechargeTab from '@/components/admin/finance/FinanceRechargeTab.vue'
import FinanceReconcileTab from '@/components/admin/finance/FinanceReconcileTab.vue'
import FinanceSettingsTab from '@/components/admin/finance/FinanceSettingsTab.vue'
import FinanceRecalculationTab from '@/components/admin/finance/FinanceRecalculationTab.vue'
import { adminAPI } from '@/api/admin'
import { FINANCE_MAX_RANGE_DAYS, type FinanceGranularity } from '@/api/admin/finance'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { defaultFinanceRange, inclusiveDays } from '@/utils/financeDate'
import { formatDateTime } from '@/utils/format'

const TABS = ['overview', 'profit', 'recharge', 'breakdown', 'affiliate', 'expenses', 'reconcile', 'settings'] as const
type FinanceTab = (typeof TABS)[number]
const GRANULARITIES: FinanceGranularity[] = ['day', 'month']

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

// ---------- 标签页（同步到 ?tab=，刷新 / 分享链接保持位置） ----------
function tabFromQuery(value: unknown): FinanceTab {
  return TABS.find((tab) => tab === value) ?? 'overview'
}
const activeTab = ref<FinanceTab>(tabFromQuery(route.query.tab))
const settingsTab = ref<InstanceType<typeof FinanceSettingsTab> | null>(null)
let checkingLeave = false
async function selectTab(tab: FinanceTab) {
  if (tab === activeTab.value || checkingLeave) return
  if (activeTab.value === 'settings' && settingsTab.value) {
    checkingLeave = true
    try { if (!(await settingsTab.value.confirmLeave())) return }
    finally { checkingLeave = false }
  }
  activeTab.value = tab
}
watch(activeTab, (tab) => {
  if (route.query.tab === tab || (tab === 'overview' && route.query.tab == null)) return
  void router.replace({ query: { ...route.query, tab: tab === 'overview' ? undefined : tab } })
})
watch(
  () => route.query.tab,
  (value) => {
    activeTab.value = tabFromQuery(value)
  }
)

// ---------- 日期范围（北京时间本月）与粒度 ----------
const initialRange = defaultFinanceRange()
const startDate = ref(initialRange.start)
const endDate = ref(initialRange.end)
const granularity = ref<FinanceGranularity>('day')
/** 最近一次合法的范围；非法输入时各标签页保留上次的数据。 */
const applied = ref({ ...initialRange })
const refreshToken = ref(0)

const rangeError = computed(() => {
  const days = inclusiveDays(startDate.value, endDate.value)
  if (!Number.isFinite(days) || days < 1) return t('admin.finance.toolbar.rangeInvalid')
  if (days > FINANCE_MAX_RANGE_DAYS) return t('admin.finance.toolbar.rangeTooLarge', { max: FINANCE_MAX_RANGE_DAYS })
  return ''
})

watch([startDate, endDate], () => {
  if (rangeError.value) return
  if (applied.value.start === startDate.value && applied.value.end === endDate.value) return
  applied.value = { start: startDate.value, end: endDate.value }
})

// ---------- 总览 / 利润明细共用的汇总 ----------
const {
  data: summaryData,
  loading: summaryLoading,
  error: summaryError,
  load: loadSummary
} = useFinanceLoad(
  (signal) =>
    adminAPI.finance.getSummary(
      { start: applied.value.start, end: applied.value.end, granularity: granularity.value },
      { signal }
    ),
  () => t('common.unknownError')
)
const summary = computed(() => summaryData.value)

watch([applied, granularity, refreshToken], loadSummary, { immediate: true })

function refresh() {
  refreshToken.value++
}

// ---------- CSV 导出 ----------
const exporting = ref(false)
async function exportCsv() {
  if (exporting.value || rangeError.value) return
  exporting.value = true
  const { start, end } = applied.value
  try {
    const blob = await adminAPI.finance.exportCsv({ start, end, granularity: granularity.value })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `finance-${start}-${end}.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
  } catch (error) {
    appStore.showError(
      t('admin.finance.toolbar.exportFailed', { error: extractApiErrorMessage(error, t('common.unknownError')) })
    )
  } finally {
    exporting.value = false
  }
}
</script>
