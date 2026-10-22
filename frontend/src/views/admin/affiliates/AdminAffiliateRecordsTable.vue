<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full md:w-80">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model="filters.search" type="text" class="input pl-10" :placeholder="t('admin.affiliates.records.searchPlaceholder')" @input="debounceLoad" />
          </div>
          <input v-model="filters.start_at" type="date" class="input w-full sm:w-44" :title="t('admin.affiliates.records.startAt')" @change="reloadFromFirstPage" />
          <input v-model="filters.end_at" type="date" class="input w-full sm:w-44" :title="t('admin.affiliates.records.endAt')" @change="reloadFromFirstPage" />
          <select
            v-if="props.type === 'rebates'"
            v-model="filters.reward_type"
            class="input w-full sm:w-48"
            :aria-label="t('admin.affiliates.records.rewardType')"
            data-test="affiliate-reward-type-filter"
            @change="reloadFromFirstPage"
          >
            <option value="">{{ t('admin.affiliates.records.rewardTypeAll') }}</option>
            <option v-for="type in AFFILIATE_REWARD_TYPES" :key="type" :value="type">{{ rewardTypeLabel(type) }}</option>
          </select>
          <button class="btn btn-secondary px-2 md:px-3" :disabled="loading" :title="t('common.refresh')" @click="loadRecords">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button
            v-if="props.type === 'transfers'"
            type="button"
            class="btn btn-primary ml-auto"
            data-test="affiliate-withdraw-open"
            @click="withdrawDialog = true"
          >
            {{ t('admin.affiliates.withdraw.button') }}
          </button>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="records"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          :sort-storage-key="sortStorageKey"
          @sort="handleSort"
        >
          <template #cell-inviter="{ row }">
            <UserCell
              :id="row.inviter_id"
              :email="row.inviter_email"
              :username="row.inviter_username"
              :clickable="props.type !== 'transfers'"
              @open="openUserOverview"
            />
          </template>
          <template #cell-invitee="{ row }">
            <span v-if="row.invitee_id == null" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <UserCell
              v-else
              :id="row.invitee_id"
              :email="row.invitee_email"
              :username="row.invitee_username"
              :clickable="props.type !== 'transfers'"
              @open="openUserOverview"
            />
          </template>
          <template #cell-user="{ row }">
            <UserCell
              :id="row.user_id"
              :email="row.user_email"
              :username="row.username"
              :clickable="true"
              @open="openUserOverview"
            />
          </template>
          <template #cell-aff_code="{ row }">
            <span class="font-mono text-sm text-gray-700 dark:text-gray-300">{{ row.aff_code || '-' }}</span>
          </template>
          <template #cell-order="{ row }">
            <span v-if="row.order_id == null" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <div v-else class="space-y-0.5">
              <div class="font-mono text-sm text-gray-900 dark:text-white">#{{ row.order_id }}</div>
              <div class="max-w-56 truncate text-sm text-gray-500 dark:text-dark-400">{{ row.out_trade_no }}</div>
            </div>
          </template>
          <template #cell-payment_type="{ row }">
            <template v-if="row.payment_type">{{ t('payment.methods.' + row.payment_type, row.payment_type) }}</template>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>
          <template #cell-order_status="{ row }">
            <OrderStatusBadge v-if="row.order_status" :status="row.order_status" />
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>
          <template #cell-total_rebate="{ row }">
            <AmountText :value="row.total_rebate" />
          </template>
          <template #cell-order_amount="{ row }">
            <NullableAmountText :value="row.order_amount" />
          </template>
          <template #cell-pay_amount="{ row }">
            <span v-if="row.pay_amount == null" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <span v-else class="text-sm text-gray-900 dark:text-white">¥{{ formatAmount(row.pay_amount) }}</span>
          </template>
          <template #cell-rebate_amount="{ row }">
            <AmountText :value="row.rebate_amount" strong />
          </template>
          <template #cell-reward_type="{ row }">
            <span v-if="!row.reward_type" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <span v-else :class="['badge whitespace-nowrap', rewardTypeBadgeClass(row.reward_type)]">
              {{ rewardTypeLabel(row.reward_type) }}
            </span>
          </template>
          <template #cell-agent_owner="{ row }">
            <SnapshotUserCell :id="row.agent_owner_id" :email="row.agent_owner_email" :deleted="row.agent_owner_deleted" :level="row.agent_owner_level" />
          </template>
          <template #cell-agent_upline="{ row }">
            <SnapshotUserCell :id="row.agent_upline_id" :email="row.agent_upline_email" :deleted="row.agent_upline_deleted" />
          </template>
          <template #cell-action="{ row }">
            <span :class="['badge whitespace-nowrap', row.action === 'withdraw' ? 'badge-warning' : 'badge-primary']">
              {{ outflowTypeLabel(row.action) }}
            </span>
          </template>
          <template #cell-amount="{ row }">
            <AmountText :value="row.amount" strong />
          </template>
          <template #cell-balance_after="{ row }">
            <NullableAmountText :value="row.balance_after" />
          </template>
          <template #cell-available_quota_after="{ row }">
            <NullableAmountText :value="row.available_quota_after" />
          </template>
          <template #cell-frozen_quota_after="{ row }">
            <NullableAmountText :value="row.frozen_quota_after" />
          </template>
          <template #cell-history_quota_after="{ row }">
            <NullableAmountText :value="row.history_quota_after" />
          </template>
          <template #cell-created_at="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(row.created_at) }}</span>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <AffiliateOfflineWithdrawDialog
      v-if="props.type === 'transfers'"
      :show="withdrawDialog"
      @close="withdrawDialog = false"
      @success="handleWithdrawSuccess"
    />

    <BaseDialog
      :show="overviewDialog"
      :title="t('admin.affiliates.overview.title')"
      width="normal"
      @close="overviewDialog = false"
    >
      <div v-if="overviewLoading" class="flex justify-center py-8">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>
      <div v-else-if="selectedOverview" class="space-y-4">
        <div class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="font-mono text-sm text-gray-900 dark:text-white">#{{ selectedOverview.user_id }}</div>
          <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ selectedOverview.email || '-' }}</div>
          <div class="mt-0.5 text-sm text-gray-500 dark:text-dark-400">{{ selectedOverview.username || '-' }}</div>
        </div>
        <div class="grid gap-3 sm:grid-cols-2" data-test="affiliate-overview-stats">
          <OverviewStat
            :label="t('admin.affiliates.overview.agentLevel')"
            :value="levelLabel(selectedOverview.agent_level) + (selectedOverview.agent_level_locked ? ' · ' + t('admin.affiliates.overview.agentLevelLocked') : '')"
          />
          <OverviewStat
            v-if="isAgentLevel(selectedOverview.agent_level) && selectedOverview.agent_rate_percent != null"
            :label="t('admin.affiliates.overview.agentRate')"
            :value="formatPercent(selectedOverview.agent_rate_percent)"
          />
          <OverviewStat :label="t('admin.affiliates.overview.agentOwner')" :value="snapshotText(selectedOverview.agent_owner_id, selectedOverview.agent_owner_email, selectedOverview.agent_owner_deleted, selectedOverview.agent_owner_level)" />
          <OverviewStat :label="t('admin.affiliates.overview.agentUpline')" :value="snapshotText(selectedOverview.agent_upline_id, selectedOverview.agent_upline_email, selectedOverview.agent_upline_deleted)" />
          <OverviewStat :label="t('admin.affiliates.overview.affCode')" :value="selectedOverview.aff_code || '-'" mono />
          <OverviewStat :label="t('admin.affiliates.overview.ownedCustomerCount')" :value="String(selectedOverview.owned_customer_count ?? 0)" />
          <OverviewStat :label="t('admin.affiliates.overview.invitedCount')" :value="String(selectedOverview.invited_count)" />
          <OverviewStat :label="t('admin.affiliates.overview.rebatedInviteeCount')" :value="String(selectedOverview.rebated_invitee_count)" />
          <OverviewStat :label="t('admin.affiliates.overview.availableQuota')" :value="BALANCE_SYMBOL + formatAmount(selectedOverview.available_quota)" />
          <OverviewStat :label="t('admin.affiliates.overview.historyQuota')" :value="BALANCE_SYMBOL + formatAmount(selectedOverview.history_quota)" />
          <OverviewStat :label="t('admin.affiliates.overview.rewardAgent')" :value="BALANCE_SYMBOL + formatAmount(selectedOverview.reward_totals?.agent)" />
          <OverviewStat :label="t('admin.affiliates.overview.rewardInvite')" :value="BALANCE_SYMBOL + formatAmount(selectedOverview.reward_totals?.invite)" />
          <OverviewStat :label="t('admin.affiliates.overview.rewardInviteeBonus')" :value="BALANCE_SYMBOL + formatAmount(selectedOverview.reward_totals?.invitee_bonus)" />
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { BALANCE_SYMBOL } from '@/utils/balanceCurrency'
import { computed, defineComponent, h, onMounted, reactive, ref, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderStatusBadge from '@/components/payment/OrderStatusBadge.vue'
import AffiliateOfflineWithdrawDialog from './AffiliateOfflineWithdrawDialog.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { AFFILIATE_REWARD_TYPES, affiliatesAPI, type AffiliateInviteRecord, type AffiliateRebateRecord, type AffiliateTransferRecord, type AffiliateUserOverview, type ListAffiliateRecordsParams } from '@/api/admin/affiliates'
import type { PaginatedResponse } from '@/types'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime as formatDisplayDateTime } from '@/utils/format'

type RecordType = 'invites' | 'rebates' | 'transfers'
type AffiliateRecord = AffiliateInviteRecord | AffiliateRebateRecord | AffiliateTransferRecord

const props = withDefaults(defineProps<{
  type: RecordType
  /** Prefills the search box, e.g. when opened from the agent management page. */
  initialSearch?: string
}>(), {
  initialSearch: '',
})

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const records = ref<AffiliateRecord[]>([])
const filters = reactive({ search: props.initialSearch, start_at: '', end_at: '', reward_type: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const overviewDialog = ref(false)
const overviewLoading = ref(false)
const selectedOverview = ref<AffiliateUserOverview | null>(null)
const withdrawDialog = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => {
  if (props.type === 'invites') {
    return [
      { key: 'inviter', label: t('admin.affiliates.records.inviter'), sortable: true },
      { key: 'invitee', label: t('admin.affiliates.records.invitee'), sortable: true },
      { key: 'aff_code', label: t('admin.affiliates.records.affCode'), sortable: true },
      { key: 'agent_owner', label: t('admin.affiliates.records.agentOwner') },
      { key: 'agent_upline', label: t('admin.affiliates.records.agentUpline') },
      { key: 'total_rebate', label: t('admin.affiliates.records.totalRebate'), sortable: true },
      { key: 'created_at', label: t('admin.affiliates.records.invitedAt'), sortable: true },
    ]
  }
  if (props.type === 'rebates') {
    return [
      { key: 'order', label: t('admin.affiliates.records.order'), sortable: true },
      { key: 'inviter', label: t('admin.affiliates.records.inviter'), sortable: true },
      { key: 'invitee', label: t('admin.affiliates.records.invitee'), sortable: true },
      { key: 'order_amount', label: t('admin.affiliates.records.orderAmount'), sortable: true },
      { key: 'pay_amount', label: t('admin.affiliates.records.payAmount'), sortable: true },
      { key: 'rebate_amount', label: t('admin.affiliates.records.rebateAmount') },
      { key: 'reward_type', label: t('admin.affiliates.records.rewardType'), sortable: true },
      { key: 'payment_type', label: t('admin.affiliates.records.paymentType'), sortable: true },
      { key: 'order_status', label: t('admin.affiliates.records.orderStatus'), sortable: true },
      { key: 'created_at', label: t('admin.affiliates.records.rebatedAt'), sortable: true },
    ]
  }
  return [
    { key: 'user', label: t('admin.affiliates.records.user'), sortable: true },
    { key: 'action', label: t('admin.affiliates.records.outflowType'), sortable: true },
    { key: 'amount', label: t('admin.affiliates.records.transferAmount'), sortable: true },
    { key: 'balance_after', label: t('admin.affiliates.records.balanceAfter'), sortable: true },
    { key: 'available_quota_after', label: t('admin.affiliates.records.availableQuotaAfter'), sortable: true },
    { key: 'frozen_quota_after', label: t('admin.affiliates.records.frozenQuotaAfter'), sortable: true },
    { key: 'history_quota_after', label: t('admin.affiliates.records.historyQuotaAfter'), sortable: true },
    { key: 'created_at', label: t('admin.affiliates.records.transferredAt'), sortable: true },
  ]
})

const sortStorageKey = computed(() => `admin-affiliate-${props.type}-table-sort`)

function loadInitialSortState(): { sort_by: string; sort_order: 'asc' | 'desc' } {
  const fallback = { sort_by: 'created_at', sort_order: 'desc' as 'asc' | 'desc' }
  try {
    const raw = localStorage.getItem(sortStorageKey.value)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!columns.value.some((column) => column.key === key && column.sortable)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'asc' ? 'asc' : 'desc',
    }
  } catch {
    return fallback
  }
}

const sortState = reactive(loadInitialSortState())

function userTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

function buildParams(): ListAffiliateRecordsParams {
  return {
    page: pagination.page,
    page_size: pagination.page_size,
    search: filters.search.trim() || undefined,
    start_at: filters.start_at || undefined,
    end_at: filters.end_at || undefined,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order,
    timezone: userTimezone(),
    reward_type: props.type === 'rebates' ? (filters.reward_type as ListAffiliateRecordsParams['reward_type']) || undefined : undefined,
  }
}

async function fetchRecords(params: ListAffiliateRecordsParams): Promise<PaginatedResponse<AffiliateRecord>> {
  if (props.type === 'invites') {
    return affiliatesAPI.listInviteRecords(params)
  }
  if (props.type === 'rebates') {
    return affiliatesAPI.listRebateRecords(params)
  }
  return affiliatesAPI.listTransferRecords(params)
}

async function loadRecords() {
  loading.value = true
  try {
    const res = await fetchRecords(buildParams())
    records.value = res.items || []
    pagination.total = res.total || 0
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => reloadFromFirstPage(), 300)
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadRecords()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadRecords()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadRecords()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  void loadRecords()
}

function handleWithdrawSuccess() {
  withdrawDialog.value = false
  reloadFromFirstPage()
}

function outflowTypeLabel(action: string | null | undefined): string {
  return action === 'withdraw'
    ? t('admin.affiliates.outflowTypes.withdraw')
    : t('admin.affiliates.outflowTypes.transfer')
}

function rewardTypeLabel(type: string): string {
  return (AFFILIATE_REWARD_TYPES as readonly string[]).includes(type)
    ? t(`admin.affiliates.rewardTypes.${type}`)
    : type
}

function rewardTypeBadgeClass(type: string): string {
  if (type === 'agent_direct') return 'badge-primary'
  if (type === 'agent_diff') return 'badge-warning'
  return 'badge-success'
}

function levelLabel(level: number | null | undefined): string {
  return t(`admin.affiliates.levels.${level === 1 || level === 2 ? level : 3}`)
}

function isAgentLevel(level: number | null | undefined): boolean {
  return level === 1 || level === 2
}

/** Text for a bind-time ownership snapshot: email (level), 已删除, or a dash when absent. */
function snapshotText(
  id: number | null | undefined,
  email: string | null | undefined,
  deleted: boolean | null | undefined,
  level?: number | null,
): string {
  if (id == null) return '-'
  if (deleted) return t('admin.affiliates.deleted')
  const name = email || `#${id}`
  return level === 1 || level === 2 ? `${name} (${t(`admin.affiliates.levelShort.${level}`)})` : name
}

function formatAmount(value: number | null | undefined): string {
  return Number(value || 0).toFixed(2)
}

function formatPercent(value: number | null | undefined): string {
  const rounded = Math.round(Number(value || 0) * 100) / 100
  return `${Number.isInteger(rounded) ? rounded.toString() : rounded.toString()}%`
}

function formatDateTime(value: string | null | undefined): string {
  return value ? formatDisplayDateTime(value) : '-'
}

async function openUserOverview(userId: number) {
  if (!userId) return
  overviewDialog.value = true
  overviewLoading.value = true
  selectedOverview.value = null
  try {
    selectedOverview.value = await affiliatesAPI.getUserOverview(userId)
  } catch (error) {
    overviewDialog.value = false
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
  } finally {
    overviewLoading.value = false
  }
}

const UserCell = defineComponent({
  props: {
    id: { type: Number, required: true },
    email: { type: String, default: '' },
    username: { type: String, default: '' },
    clickable: { type: Boolean, default: false },
  },
  emits: ['open'],
  setup(cellProps, { emit }) {
    return () => h('div', { class: 'space-y-0.5' }, [
      h('div', { class: 'font-mono text-sm text-gray-900 dark:text-white' }, `#${cellProps.id}`),
      h(cellProps.clickable ? 'button' : 'div', {
        class: cellProps.clickable
          ? 'max-w-56 truncate text-left text-sm font-medium text-primary-600 hover:text-primary-700 hover:underline dark:text-primary-400 dark:hover:text-primary-300'
          : 'max-w-56 truncate text-sm text-gray-700 dark:text-gray-300',
        type: cellProps.clickable ? 'button' : undefined,
        onClick: cellProps.clickable ? () => emit('open', cellProps.id) : undefined,
      }, cellProps.email || '-'),
      h('div', { class: 'max-w-56 truncate text-sm text-gray-500 dark:text-dark-400' }, cellProps.username || '-'),
    ])
  },
})

const AmountText = defineComponent({
  props: {
    value: { type: Number, default: 0 },
    strong: { type: Boolean, default: false },
  },
  setup(amountProps) {
    return () => h('span', {
      class: amountProps.strong
        ? 'text-sm font-semibold text-emerald-600 dark:text-emerald-400'
        : 'text-sm text-gray-900 dark:text-white',
    }, `${BALANCE_SYMBOL}${formatAmount(amountProps.value)}`)
  },
})

const NullableAmountText = defineComponent({
  props: {
    value: { type: Number as PropType<number | null | undefined>, default: null },
  },
  setup(amountProps) {
    return () => {
      const value = amountProps.value
      if (value === null || value === undefined) {
        return h('span', { class: 'text-sm text-gray-400 dark:text-dark-500' }, '-')
      }
      return h(AmountText, { value })
    }
  },
})

const SnapshotUserCell = defineComponent({
  props: {
    id: { type: Number as PropType<number | null | undefined>, default: null },
    email: { type: String as PropType<string | null | undefined>, default: '' },
    deleted: { type: Boolean, default: false },
    level: { type: Number as PropType<number | null | undefined>, default: null },
  },
  setup(cellProps) {
    return () => {
      if (cellProps.id == null) {
        return h('span', { class: 'text-sm text-gray-400 dark:text-dark-500' }, '-')
      }
      if (cellProps.deleted) {
        return h('span', { class: 'text-sm text-gray-400 dark:text-dark-500' }, t('admin.affiliates.deleted'))
      }
      return h('div', { class: 'space-y-0.5' }, [
        h('div', { class: 'max-w-56 truncate text-sm text-gray-700 dark:text-gray-300' }, cellProps.email || `#${cellProps.id}`),
        cellProps.level === 1 || cellProps.level === 2
          ? h('div', { class: 'text-xs text-gray-400 dark:text-dark-500' }, t(`admin.affiliates.levelShort.${cellProps.level}`))
          : null,
      ])
    }
  },
})

const OverviewStat = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    mono: { type: Boolean, default: false },
  },
  setup(statProps) {
    return () => h('div', { class: 'rounded-lg border border-gray-100 bg-white p-3 dark:border-dark-700 dark:bg-dark-900' }, [
      h('div', { class: 'text-sm text-gray-500 dark:text-dark-400' }, statProps.label),
      h('div', {
        class: statProps.mono
          ? 'mt-1 font-mono text-base font-semibold text-gray-900 dark:text-white'
          : 'mt-1 text-base font-semibold text-gray-900 dark:text-white',
      }, statProps.value),
    ])
  },
})

onMounted(() => {
  void loadRecords()
})
</script>
