<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-start gap-3">
          <div class="relative w-full md:w-72">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="filters.search"
              type="text"
              class="input pl-10"
              :placeholder="t('admin.affiliates.agents.searchPlaceholder')"
              :aria-label="t('admin.affiliates.agents.searchPlaceholder')"
              data-test="agents-search"
              @input="debounceReload"
            />
          </div>
          <select
            v-model="filters.level"
            class="input w-full sm:w-44"
            :aria-label="t('admin.affiliates.agents.levelFilter')"
            data-test="agents-level-filter"
            @change="reloadFromFirstPage"
          >
            <option value="">{{ t('admin.affiliates.agents.levelFilterAll') }}</option>
            <option v-for="level in AGENT_LEVELS" :key="level" :value="String(level)">{{ levelLabel(level) }}</option>
          </select>
          <div class="w-full sm:w-64">
            <AffiliateUserLookup
              input-id="agents-owner-filter"
              :selected="filters.owner"
              :placeholder="t('admin.affiliates.agents.ownerFilterAll')"
              :clear-label="t('admin.affiliates.agents.ownerFilterAll')"
              test-id="agents-owner-filter"
              @select="selectOwnerFilter"
              @clear="selectOwnerFilter(null)"
            />
          </div>
          <button class="btn btn-secondary px-2 md:px-3" :disabled="loading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadEntries">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <div class="ml-auto flex flex-wrap gap-2">
            <button type="button" class="btn btn-primary" data-test="agents-add-open" @click="openAddDialog">
              + {{ t('admin.affiliates.agents.addAgent') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="selectedIds.length === 0"
              data-test="agents-batch-open"
              @click="openBatchDialog"
            >
              {{ t('admin.affiliates.agents.batchLevel') }}<template v-if="selectedIds.length > 0"> ({{ selectedIds.length }})</template>
            </button>
          </div>
        </div>
        <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.agents.scopeHint') }}</p>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="entries"
          :loading="loading"
          row-key="user_id"
          selectable
          :selected-keys="selectedIds"
          :selection-label="t('admin.affiliates.agents.selectRow')"
          @update:selected-keys="handleSelectedKeys"
        >
          <template #cell-user="{ row }">
            <div class="space-y-0.5">
              <div class="max-w-56 truncate text-sm font-medium text-gray-900 dark:text-white">{{ row.email || '-' }}</div>
              <div class="max-w-56 truncate text-xs text-gray-500 dark:text-dark-400">
                #{{ row.user_id }}<template v-if="row.username"> · {{ row.username }}</template>
              </div>
              <div v-if="row.aff_code_custom" class="text-xs">
                <span class="rounded bg-primary-100 px-1.5 py-0.5 font-mono text-primary-700 dark:bg-primary-900/30 dark:text-primary-300" :title="t('admin.affiliates.agents.customCode')">{{ row.aff_code }}</span>
              </div>
            </div>
          </template>
          <template #cell-agent_level="{ row }">
            <div class="space-y-0.5" data-test="agent-level-cell">
              <div class="flex items-center gap-1 text-sm">
                <span :class="row.agent_level === 3 ? 'text-gray-500 dark:text-dark-400' : 'font-medium text-gray-900 dark:text-white'">
                  {{ levelLabel(row.agent_level) }}
                </span>
                <Icon
                  v-if="row.agent_level_locked"
                  name="lock"
                  size="xs"
                  class="text-amber-500"
                  :title="t('admin.affiliates.agents.locked')"
                  :aria-label="t('admin.affiliates.agents.locked')"
                />
              </div>
              <div class="text-xs text-gray-400 dark:text-dark-500" data-test="agent-level-source">{{ sourceLabel(row) }}</div>
            </div>
          </template>
          <template #cell-owner="{ row }">
            <span v-if="row.agent_owner_id == null" class="text-sm text-gray-400 dark:text-dark-500">—</span>
            <span v-else-if="row.agent_owner_deleted" class="text-sm text-gray-400 dark:text-dark-500" data-test="agent-owner-deleted">{{ t('admin.affiliates.deleted') }}</span>
            <div v-else class="space-y-0.5">
              <div class="max-w-56 truncate text-sm text-gray-700 dark:text-gray-300">{{ row.agent_owner_email || '#' + row.agent_owner_id }}</div>
              <div v-if="row.agent_owner_level" class="text-xs text-gray-400 dark:text-dark-500">{{ levelShort(row.agent_owner_level) }}</div>
            </div>
          </template>
          <template #cell-aff_count="{ row }">
            <span class="text-sm text-gray-900 dark:text-white">{{ row.aff_count }}</span>
          </template>
          <template #cell-owned_customer_count="{ row }">
            <span v-if="!row.owned_customer_count" class="text-sm text-gray-400 dark:text-dark-500">—</span>
            <span v-else class="text-sm text-gray-900 dark:text-white">{{ row.owned_customer_count }}</span>
          </template>
          <template #cell-agent_profit_total="{ row }">
            <span v-if="!row.agent_profit_total" class="text-sm text-gray-400 dark:text-dark-500">—</span>
            <span v-else class="text-sm font-semibold text-emerald-600 dark:text-emerald-400">{{ BALANCE_SYMBOL }}{{ formatAmount(row.agent_profit_total) }}</span>
          </template>
          <template #cell-custom_rate="{ row }">
            <span v-if="row.aff_rebate_rate_percent == null" class="text-sm text-gray-400 dark:text-dark-500">—</span>
            <span v-else class="text-sm text-gray-900 dark:text-white">{{ formatRatePercent(row.aff_rebate_rate_percent) }}%</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <button type="button" class="text-sm text-primary-600 hover:underline dark:text-primary-400" data-test="agent-adjust" @click="openAdjustDialog(row)">
                {{ t('admin.affiliates.agents.adjust') }}
              </button>
              <button
                type="button"
                class="rounded px-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-900 dark:hover:bg-dark-700 dark:hover:text-white"
                :aria-label="t('admin.affiliates.agents.more')"
                :aria-expanded="menu.entry?.user_id === row.user_id ? 'true' : 'false'"
                aria-haspopup="menu"
                data-test="agent-more"
                @click.stop="toggleMenu(row, $event)"
              >
                ⋯
              </button>
            </div>
          </template>
          <template #empty>
            <div class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.agents.empty') }}</div>
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

    <!-- ⋯ menu -->
    <Teleport to="body">
      <div
        v-if="menu.entry && menu.position"
        role="menu"
        class="agent-action-menu fixed z-[9999] w-48 overflow-hidden rounded-xl bg-white py-1 shadow-lg ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10"
        :style="{ top: menu.position.top + 'px', left: menu.position.left + 'px' }"
        data-test="agent-menu"
        @click.stop
      >
        <button type="button" role="menuitem" class="block w-full px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-700" data-test="agent-menu-customers" @click="openCustomers(menu.entry)">
          {{ t('admin.affiliates.agents.viewCustomers') }}
        </button>
        <button type="button" role="menuitem" class="block w-full px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-700" data-test="agent-menu-rebates" @click="viewRebates(menu.entry)">
          {{ t('admin.affiliates.agents.viewRebates') }}
        </button>
        <button type="button" role="menuitem" class="block w-full px-4 py-2 text-left text-sm text-red-600 hover:bg-gray-100 dark:hover:bg-dark-700" data-test="agent-menu-clear" @click="askClear(menu.entry)">
          {{ t('admin.affiliates.agents.clearSettings') }}
        </button>
      </div>
    </Teleport>

    <!-- 调整 -->
    <BaseDialog :show="adjust.open" :title="t('admin.affiliates.agents.adjustDialog.title')" width="normal" @close="closeAdjust">
      <form v-if="adjust.entry" class="space-y-4" data-test="agent-adjust-form" @submit.prevent="submitAdjust">
        <div>
          <span class="input-label">{{ t('admin.affiliates.agents.adjustDialog.user') }}</span>
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ adjust.entry.email }} <span class="text-xs text-gray-500">#{{ adjust.entry.user_id }}</span></p>
        </div>
        <div>
          <label for="agent-adjust-level" class="input-label">{{ t('admin.affiliates.agents.adjustDialog.level') }}</label>
          <select id="agent-adjust-level" v-model.number="adjust.level" class="input" data-test="agent-adjust-level">
            <option v-for="level in AGENT_LEVELS" :key="level" :value="level">{{ levelLabel(level) }}</option>
          </select>
          <p class="mt-1 text-xs text-gray-400">{{ t('admin.affiliates.agents.adjustDialog.levelHint') }}</p>
        </div>
        <div>
          <label for="agent-adjust-rate" class="input-label">{{ t('admin.affiliates.agents.adjustDialog.customRate') }}</label>
          <div class="relative">
            <input
              id="agent-adjust-rate"
              v-model="adjust.rate"
              type="number"
              step="0.01"
              min="0"
              max="100"
              class="input pr-8"
              :placeholder="t('admin.affiliates.agents.adjustDialog.customRatePlaceholder')"
              data-test="agent-adjust-rate"
            />
            <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">%</span>
          </div>
          <p class="mt-1 text-xs text-gray-400">{{ t('admin.affiliates.agents.adjustDialog.customRateHint') }}</p>
        </div>
        <div>
          <label for="agent-adjust-code" class="input-label">{{ t('admin.affiliates.agents.adjustDialog.inviteCode') }}</label>
          <input
            id="agent-adjust-code"
            v-model="adjust.code"
            type="text"
            class="input font-mono"
            maxlength="32"
            :placeholder="t('admin.affiliates.agents.adjustDialog.inviteCodePlaceholder')"
            data-test="agent-adjust-code"
          />
          <p class="mt-1 text-xs text-gray-400">{{ t('admin.affiliates.agents.adjustDialog.inviteCodeHint') }}</p>
        </div>
        <label v-if="adjust.entry.agent_level_locked" class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="adjust.unlock" type="checkbox" data-test="agent-adjust-unlock" />
          {{ t('admin.affiliates.agents.adjustDialog.unlock') }}
        </label>
        <p v-if="adjust.error" role="alert" class="text-sm text-red-600">{{ adjust.error }}</p>
      </form>
      <template #footer>
        <p v-if="!adjustHasChanges" class="mr-auto text-xs text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.agents.adjustDialog.noChanges') }}</p>
        <button type="button" class="btn btn-secondary" :disabled="adjust.saving" @click="closeAdjust">{{ t('common.cancel') }}</button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="adjust.saving || !adjustHasChanges"
          data-test="agent-adjust-submit"
          @click="submitAdjust"
        >
          {{ adjust.saving ? t('common.saving') : t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <!-- + 设置代理 -->
    <BaseDialog :show="addDialog.open" :title="t('admin.affiliates.agents.addDialog.title')" width="normal" @close="closeAddDialog">
      <div class="space-y-4">
        <AffiliateUserLookup
          input-id="agent-add-user"
          :label="t('admin.affiliates.agents.addDialog.user')"
          :placeholder="t('admin.affiliates.agents.addDialog.userPlaceholder')"
          :clear-label="t('admin.affiliates.agents.addDialog.changeUser')"
          :empty-text="t('admin.affiliates.agents.addDialog.noUserFound')"
          :selected="addDialog.user"
          test-id="agent-add-user"
          @select="addDialog.user = $event"
          @clear="addDialog.user = null"
        />
        <div>
          <label for="agent-add-level" class="input-label">{{ t('admin.affiliates.agents.addDialog.level') }}</label>
          <select id="agent-add-level" v-model.number="addDialog.level" class="input" data-test="agent-add-level">
            <option :value="1">{{ levelLabel(1) }}</option>
            <option :value="2">{{ levelLabel(2) }}</option>
          </select>
          <p class="mt-1 text-xs text-gray-400">{{ t('admin.affiliates.agents.addDialog.hint') }}</p>
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="addDialog.saving" @click="closeAddDialog">{{ t('common.cancel') }}</button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="addDialog.saving || !addDialog.user"
          data-test="agent-add-submit"
          @click="submitAddDialog"
        >
          {{ addDialog.saving ? t('common.saving') : t('admin.affiliates.agents.addDialog.submit') }}
        </button>
      </template>
    </BaseDialog>

    <!-- 批量设等级 -->
    <BaseDialog :show="batch.open" :title="t('admin.affiliates.agents.batchDialog.title', { count: selectedIds.length })" width="narrow" @close="batch.open = false">
      <div class="space-y-2">
        <select v-model.number="batch.level" class="input" :aria-label="t('admin.affiliates.agents.adjustDialog.level')" data-test="agents-batch-level">
          <option v-for="level in AGENT_LEVELS" :key="level" :value="level">{{ levelLabel(level) }}</option>
        </select>
        <p class="text-xs text-gray-400">{{ t('admin.affiliates.agents.batchDialog.hint') }}</p>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="batch.saving" @click="batch.open = false">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-primary" :disabled="batch.saving" data-test="agents-batch-submit" @click="submitBatch">
          {{ batch.saving ? t('common.saving') : t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <!-- 查看客户 -->
    <BaseDialog
      :show="customers.open"
      :title="t('admin.affiliates.agents.customers.title', { email: customers.entry?.email || '#' + (customers.entry?.user_id ?? '') })"
      width="wide"
      @close="customers.open = false"
    >
      <div v-if="customers.loading" class="flex justify-center py-8">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>
      <div v-else-if="customers.items.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400" data-test="agent-customers-empty">
        {{ t('admin.affiliates.agents.customers.empty') }}
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[560px] text-left text-sm" data-test="agent-customers-table">
          <thead>
            <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
              <th scope="col" class="px-3 py-2 font-medium">{{ t('admin.affiliates.agents.customers.user') }}</th>
              <th scope="col" class="px-3 py-2 font-medium">{{ t('admin.affiliates.agents.customers.level') }}</th>
              <th scope="col" class="px-3 py-2 font-medium">{{ t('admin.affiliates.agents.customers.relation') }}</th>
              <th scope="col" class="px-3 py-2 font-medium">{{ t('admin.affiliates.agents.customers.boundAt') }}</th>
              <th scope="col" class="px-3 py-2 text-right font-medium">{{ t('admin.affiliates.agents.customers.contribution') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in customers.items" :key="item.user_id + '-' + item.relation" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800">
              <td class="px-3 py-2">
                <div class="text-gray-900 dark:text-white">{{ item.email || '-' }}</div>
                <div class="text-xs text-gray-500 dark:text-dark-400">#{{ item.user_id }}<template v-if="item.username"> · {{ item.username }}</template></div>
              </td>
              <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{{ levelShort(item.agent_level) }}</td>
              <td class="px-3 py-2">
                <span :class="['badge whitespace-nowrap', item.relation === 'owner' ? 'badge-primary' : 'badge-warning']">
                  {{ item.relation === 'owner' ? t('admin.affiliates.agents.customers.relationOwner') : t('admin.affiliates.agents.customers.relationUpline') }}
                </span>
              </td>
              <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{{ item.bound_at ? formatDateTime(item.bound_at) : '-' }}</td>
              <td class="px-3 py-2 text-right font-medium text-emerald-600 dark:text-emerald-400">{{ BALANCE_SYMBOL }}{{ formatAmount(item.contribution) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <template #footer>
        <Pagination
          v-if="customers.total > customers.page_size"
          :page="customers.page"
          :total="customers.total"
          :page-size="customers.page_size"
          :show-page-size-selector="false"
          @update:page="loadCustomers"
        />
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="confirm.show"
      :title="confirm.title"
      :message="confirm.message"
      :confirm-text="confirm.confirmText"
      danger
      @confirm="runConfirm"
      @cancel="cancelConfirm"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { BALANCE_SYMBOL } from '@/utils/balanceCurrency'
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import AffiliateUserLookup from './AffiliateUserLookup.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import {
  affiliatesAPI,
  type AffiliateAdminEntry,
  type AffiliateAgentCustomer,
  type AgentLevel,
  type SimpleUser,
  type UpdateAffiliateUserRequest,
} from '@/api/admin/affiliates'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import { formatRatePercent } from '@/utils/affiliateRewards'

const AGENT_LEVELS: readonly AgentLevel[] = [1, 2, 3]

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(false)
const entries = ref<AffiliateAdminEntry[]>([])
const selectedIds = ref<number[]>([])
const filters = reactive<{ search: string; level: string; owner: SimpleUser | null }>({
  search: '',
  level: '',
  owner: null,
})
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let searchTimer: ReturnType<typeof setTimeout> | undefined
let loadSequence = 0

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.affiliates.agents.col.user') },
  { key: 'agent_level', label: t('admin.affiliates.agents.col.level') },
  { key: 'owner', label: t('admin.affiliates.agents.col.owner') },
  { key: 'aff_count', label: t('admin.affiliates.agents.col.inviteCount') },
  { key: 'owned_customer_count', label: t('admin.affiliates.agents.col.customerCount') },
  { key: 'agent_profit_total', label: t('admin.affiliates.agents.col.profitTotal') },
  { key: 'custom_rate', label: t('admin.affiliates.agents.col.customRate') },
  { key: 'actions', label: t('admin.affiliates.agents.col.actions') },
])

function levelLabel(level: number | null | undefined): string {
  return t(`admin.affiliates.levels.${normalizeLevel(level)}`)
}

function levelShort(level: number | null | undefined): string {
  return t(`admin.affiliates.levelShort.${normalizeLevel(level)}`)
}

function normalizeLevel(level: number | null | undefined): AgentLevel {
  return level === 1 || level === 2 ? level : 3
}

function sourceLabel(entry: AffiliateAdminEntry): string {
  switch (entry.agent_level_source) {
    case 'agent':
      return entry.agent_level_updated_by_email
        ? t('admin.affiliates.agents.source.agent', { email: entry.agent_level_updated_by_email })
        : t('admin.affiliates.agents.source.agentUnknown')
    case 'auto':
      return t('admin.affiliates.agents.source.auto')
    case 'manual':
      return t('admin.affiliates.agents.source.manual')
    default:
      return t('admin.affiliates.agents.source.default')
  }
}

function formatAmount(value: number | null | undefined): string {
  return Number(value || 0).toFixed(2)
}

function showError(error: unknown) {
  appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
}

async function loadEntries() {
  const sequence = ++loadSequence
  loading.value = true
  try {
    const res = await affiliatesAPI.listUsers({
      page: pagination.page,
      page_size: pagination.page_size,
      search: filters.search.trim(),
      agent_level: filters.level ? (Number(filters.level) as AgentLevel) : null,
      owner_id: filters.owner?.id ?? null,
    })
    if (sequence !== loadSequence) return
    entries.value = res.items ?? []
    pagination.total = res.total ?? 0
  } catch (error) {
    if (sequence === loadSequence) showError(error)
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadEntries()
}

function debounceReload() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(reloadFromFirstPage, 300)
}

function selectOwnerFilter(user: SimpleUser | null) {
  filters.owner = user
  reloadFromFirstPage()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadEntries()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadEntries()
}

function handleSelectedKeys(keys: Array<string | number>) {
  selectedIds.value = keys.map((key) => Number(key)).filter((id) => Number.isFinite(id))
}

// ---------- ⋯ menu ----------
const menu = reactive<{ entry: AffiliateAdminEntry | null; position: { top: number; left: number } | null }>({
  entry: null,
  position: null,
})

function toggleMenu(entry: AffiliateAdminEntry, event: MouseEvent) {
  if (menu.entry?.user_id === entry.user_id) {
    closeMenu()
    return
  }
  const target = event.currentTarget as HTMLElement | null
  const rect = target?.getBoundingClientRect()
  const menuWidth = 192
  const menuHeight = 132
  const padding = 8
  let left = rect ? rect.right - menuWidth : 0
  let top = rect ? rect.bottom + 4 : 0
  if (typeof window !== 'undefined') {
    left = Math.max(padding, Math.min(left, window.innerWidth - menuWidth - padding))
    if (rect && top + menuHeight > window.innerHeight - padding) top = rect.top - menuHeight - 4
  }
  menu.entry = entry
  menu.position = { top, left }
}

function closeMenu() {
  menu.entry = null
  menu.position = null
}

function handleDocumentClick() {
  if (menu.entry) closeMenu()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && menu.entry) closeMenu()
}

function viewRebates(entry: AffiliateAdminEntry | null) {
  closeMenu()
  if (!entry) return
  void router.push({ path: '/admin/affiliates/rebates', query: { search: entry.email || String(entry.user_id) } })
}

// ---------- confirm ----------
const confirm = reactive<{ show: boolean; title: string; message: string; confirmText: string; action: (() => Promise<void>) | null }>({
  show: false,
  title: '',
  message: '',
  confirmText: '',
  action: null,
})

function openConfirm(title: string, message: string, confirmText: string, action: () => Promise<void>) {
  confirm.title = title
  confirm.message = message
  confirm.confirmText = confirmText
  confirm.action = action
  confirm.show = true
}

async function runConfirm() {
  const action = confirm.action
  confirm.show = false
  confirm.action = null
  if (action) await action()
}

function cancelConfirm() {
  confirm.show = false
  confirm.action = null
}

function askClear(entry: AffiliateAdminEntry | null) {
  closeMenu()
  if (!entry) return
  openConfirm(
    t('admin.affiliates.agents.clearTitle'),
    t('admin.affiliates.agents.clearMessage', { email: entry.email || `#${entry.user_id}` }),
    t('admin.affiliates.agents.clearSettings'),
    async () => {
      try {
        await affiliatesAPI.clearUserSettings(entry.user_id)
        appStore.showSuccess(t('common.saved'))
        await loadEntries()
      } catch (error) {
        showError(error)
      }
    },
  )
}

// ---------- 调整 ----------
// `rate` is string | number because <input type="number"> v-model casts typed values to numbers.
const adjust = reactive<{
  open: boolean
  saving: boolean
  entry: AffiliateAdminEntry | null
  level: AgentLevel
  rate: string | number
  code: string
  unlock: boolean
  error: string
}>({
  open: false,
  saving: false,
  entry: null,
  level: 3,
  rate: '',
  code: '',
  unlock: false,
  error: '',
})

function openAdjustDialog(entry: AffiliateAdminEntry) {
  closeMenu()
  adjust.entry = entry
  adjust.level = normalizeLevel(entry.agent_level)
  adjust.rate = entry.aff_rebate_rate_percent != null ? String(entry.aff_rebate_rate_percent) : ''
  adjust.code = entry.aff_code_custom ? entry.aff_code : ''
  adjust.unlock = false
  adjust.error = ''
  adjust.open = true
}

function closeAdjust() {
  if (!adjust.saving) adjust.open = false
}

/** Builds the PUT payload from the dialog; returns null for an invalid rate. */
function buildAdjustPayload(): UpdateAffiliateUserRequest | null {
  const entry = adjust.entry
  if (!entry) return {}
  const payload: UpdateAffiliateUserRequest = {}
  if (adjust.level !== normalizeLevel(entry.agent_level)) payload.agent_level = adjust.level

  const rateText = String(adjust.rate ?? '').trim()
  if (rateText === '') {
    if (entry.aff_rebate_rate_percent != null) payload.clear_rebate_rate = true
  } else {
    const rate = Number(rateText)
    if (!Number.isFinite(rate) || rate < 0 || rate > 100) return null
    if (rate !== entry.aff_rebate_rate_percent) payload.aff_rebate_rate_percent = rate
  }

  const code = adjust.code.trim().toUpperCase()
  if (code && !(entry.aff_code_custom && code === entry.aff_code)) payload.aff_code = code
  if (adjust.unlock && entry.agent_level_locked) payload.agent_level_locked = false
  return payload
}

const adjustHasChanges = computed(() => {
  const payload = buildAdjustPayload()
  return payload === null || Object.keys(payload).length > 0
})

async function saveAdjust(payload: UpdateAffiliateUserRequest) {
  if (!adjust.entry) return
  adjust.saving = true
  adjust.error = ''
  try {
    await affiliatesAPI.updateUserSettings(adjust.entry.user_id, payload)
    appStore.showSuccess(t('common.saved'))
    adjust.open = false
    await loadEntries()
  } catch (error) {
    adjust.error = extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error'))
  } finally {
    adjust.saving = false
  }
}

function submitAdjust() {
  if (adjust.saving || !adjust.entry) return
  const payload = buildAdjustPayload()
  if (payload === null) {
    adjust.error = t('admin.affiliates.agents.adjustDialog.errorBadRate')
    return
  }
  if (Object.keys(payload).length === 0) return
  // Higher number = lower rank: 1 一级 > 2 二级 > 3 普通.
  const demoting = payload.agent_level != null && payload.agent_level > normalizeLevel(adjust.entry.agent_level)
  if (demoting) {
    openConfirm(
      t('admin.affiliates.agents.demoteTitle'),
      t('admin.affiliates.agents.demoteMessage'),
      t('common.confirm'),
      () => saveAdjust(payload),
    )
    return
  }
  void saveAdjust(payload)
}

// ---------- + 设置代理 ----------
const addDialog = reactive<{ open: boolean; saving: boolean; user: SimpleUser | null; level: 1 | 2 }>({
  open: false,
  saving: false,
  user: null,
  level: 2,
})

function openAddDialog() {
  addDialog.user = null
  addDialog.level = 2
  addDialog.open = true
}

function closeAddDialog() {
  if (!addDialog.saving) addDialog.open = false
}

async function submitAddDialog() {
  if (!addDialog.user || addDialog.saving) return
  addDialog.saving = true
  try {
    await affiliatesAPI.updateUserSettings(addDialog.user.id, { agent_level: addDialog.level })
    appStore.showSuccess(t('common.saved'))
    addDialog.open = false
    reloadFromFirstPage()
  } catch (error) {
    showError(error)
  } finally {
    addDialog.saving = false
  }
}

// ---------- 批量设等级 ----------
const batch = reactive<{ open: boolean; saving: boolean; level: AgentLevel }>({ open: false, saving: false, level: 2 })

function openBatchDialog() {
  if (selectedIds.value.length === 0) return
  batch.level = 2
  batch.open = true
}

async function submitBatch() {
  if (batch.saving || selectedIds.value.length === 0) return
  batch.saving = true
  try {
    await affiliatesAPI.batchSetAgentLevel({ user_ids: [...selectedIds.value], agent_level: batch.level })
    appStore.showSuccess(t('common.saved'))
    batch.open = false
    selectedIds.value = []
    await loadEntries()
  } catch (error) {
    showError(error)
  } finally {
    batch.saving = false
  }
}

// ---------- 查看客户 ----------
const customers = reactive<{
  open: boolean
  loading: boolean
  entry: AffiliateAdminEntry | null
  items: AffiliateAgentCustomer[]
  page: number
  page_size: number
  total: number
}>({
  open: false,
  loading: false,
  entry: null,
  items: [],
  page: 1,
  page_size: 20,
  total: 0,
})
let customersSequence = 0

function openCustomers(entry: AffiliateAdminEntry | null) {
  closeMenu()
  if (!entry) return
  customers.entry = entry
  customers.items = []
  customers.total = 0
  customers.open = true
  void loadCustomers(1)
}

async function loadCustomers(page: number) {
  if (!customers.entry) return
  const sequence = ++customersSequence
  customers.page = page
  customers.loading = true
  try {
    const res = await affiliatesAPI.listUserCustomers(customers.entry.user_id, { page, page_size: customers.page_size })
    if (sequence !== customersSequence) return
    customers.items = res.items ?? []
    customers.total = res.total ?? 0
  } catch (error) {
    if (sequence === customersSequence) showError(error)
  } finally {
    if (sequence === customersSequence) customers.loading = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleDocumentClick)
  document.addEventListener('keydown', handleKeydown)
  void loadEntries()
})

onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
  document.removeEventListener('click', handleDocumentClick)
  document.removeEventListener('keydown', handleKeydown)
})
</script>
