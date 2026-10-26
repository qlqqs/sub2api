<template>
  <div class="space-y-4" data-testid="finance-account-cost-settings">
    <section class="card space-y-4 p-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold">{{ t('admin.finance.costSettings.accounts') }}</h3>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.accountSource') }}</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading || metaLoading || busy" @click="reload">{{ t('common.refresh') }}</button>
      </div>
      <div class="flex flex-wrap gap-3">
        <input v-model="keyword" type="search" class="input w-full sm:w-72" :aria-label="t('admin.finance.settings.searchPlaceholder')" :placeholder="t('admin.finance.settings.searchPlaceholder')" data-testid="finance-account-search" />
        <select v-model="platform" class="input w-full sm:w-auto" :aria-label="t('admin.finance.costSettings.platform')">
          <option value="">{{ t('admin.finance.costSettings.allPlatforms') }}</option>
          <option v-for="item in platforms" :key="item" :value="item">{{ item }}</option>
        </select>
      </div>
      <FinanceLoadState :loading="loading || metaLoading" :error="error || metaError" :has-data="!!data && !!meta" @retry="reload">
        <DataTable :columns="columns" :data="data?.items || []" :expandable-actions="false">
          <template #cell-name="{ row }"><span class="break-words">#{{ row.id }} {{ row.name }}</span></template>
          <template #cell-rate="{ row }"><span class="font-mono">{{ row.rate_multiplier ?? 1 }}×</span></template>
          <template #cell-model="{ row }">{{ t('admin.finance.accountModelCost.count', { count: meta?.models[String(row.id)]?.length || 0 }) }}</template>
          <template #cell-fixed="{ row }">{{ t(meta?.settings.fixed_cost_account_ids.includes(row.id) ? 'common.yes' : 'common.no') }}</template>
          <template #cell-rules="{ row }"><span v-for="channel in matchingChannels(row)" :key="channel.id" class="block text-xs">#{{ channel.id }} {{ channel.name }}</span><span v-if="!matchingChannels(row).length">—</span></template>
          <template #cell-actions="{ row }"><button type="button" class="btn btn-secondary btn-sm" :disabled="busy || opening" :data-testid="`finance-account-edit-${row.id}`" @click="openAccount(row.id)">{{ t('common.edit') }}</button></template>
        </DataTable>
        <Pagination v-if="data?.total" :total="data.total" :page="page" :page-size="pageSize" @update:page="page = $event" @update:page-size="pageSize = $event; page = 1" />
      </FinanceLoadState>
    </section>

    <section v-if="meta" class="card space-y-3 p-4">
      <h3 class="text-sm font-semibold">{{ t('admin.finance.costSettings.sharedRules') }}</h3>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.channelSource') }}</p>
      <input v-model="channelKeyword" type="search" class="input w-full sm:w-72" :placeholder="t('admin.finance.costSettings.channelSearch')" :aria-label="t('admin.finance.costSettings.channelSearch')" />
      <DataTable :columns="channelColumns" :data="channelPageRows" :expandable-actions="false">
        <template #cell-name="{ row }">#{{ row.id }} {{ row.name }}</template>
        <template #cell-rules="{ row }">{{ row.account_stats_pricing_rules?.length || 0 }}</template>
        <template #cell-actions="{ row }"><button type="button" class="btn btn-secondary btn-sm" :disabled="busy || opening" :data-testid="`finance-channel-edit-${row.id}`" @click="openChannel(row)">{{ t('common.edit') }}</button></template>
      </DataTable>
      <Pagination v-if="filteredChannels.length" :total="filteredChannels.length" :page="channelPage" :page-size="20" @update:page="channelPage = $event" />
    </section>

    <!-- Keep dialog props stable during saves so nested editors retain their draft; close handlers guard busy. -->
    <BaseDialog :show="editing !== null" :title="t('admin.finance.costSettings.editAccount')" width="normal" @close="closeAccount">
      <div v-if="editing" class="space-y-5">
        <p class="break-words text-sm">#{{ editing.id }} {{ editing.name }}</p>
        <p class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.finance.costSettings.independentSaves') }}</p>
        <p class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.finance.costSettings.accountConcurrency') }}</p>
        <fieldset :disabled="busy" class="space-y-2">
          <label for="finance-default-rate" class="input-label">{{ t('admin.finance.costSettings.defaultRate') }}</label>
          <input id="finance-default-rate" v-model="defaultRate" class="input" inputmode="decimal" data-testid="finance-default-rate" :disabled="rateManaged" />
          <p class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.finance.costSettings.schedulingHint') }}</p>
          <p v-if="rateManaged" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBilling.syncRateManagedHint') }}</p>
          <button type="button" class="btn btn-primary btn-sm" :disabled="rateManaged || Number(defaultRate) === (editing.rate_multiplier ?? 1)" data-testid="finance-default-save" @click="saveDefault">{{ t('admin.finance.costSettings.saveDefault') }}</button>
          <p v-if="statuses.default" role="status" class="text-xs">{{ statuses.default }}</p>
        </fieldset>
        <fieldset :disabled="busy" class="space-y-2 border-t border-gray-200 pt-4 dark:border-dark-600">
          <AccountModelCostEditor ref="modelEditor" :account-id="editing.id" />
          <button type="button" class="btn btn-primary btn-sm" :disabled="!modelEditor?.hasChanges" data-testid="finance-model-save" @click="saveModels">{{ t('admin.finance.costSettings.saveModels') }}</button>
          <p v-if="statuses.models" role="status" class="text-xs">{{ statuses.models }}</p>
        </fieldset>
        <fieldset :disabled="busy" class="space-y-2 border-t border-gray-200 pt-4 dark:border-dark-600">
          <label class="flex items-center gap-2 text-sm"><input v-model="fixedCost" type="checkbox" data-testid="finance-fixed-cost" />{{ t('admin.finance.settings.fixedCostTitle') }}</label>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.fixedHint') }}</p>
          <button type="button" class="btn btn-primary btn-sm" :disabled="fixedCost === savedFixed" data-testid="finance-fixed-save" @click="saveFixed">{{ t('admin.finance.costSettings.saveFixed') }}</button>
          <p v-if="statuses.fixed" role="status" class="text-xs">{{ statuses.fixed }}</p>
          <button type="button" class="btn btn-secondary btn-sm" @click="goExpenses">{{ t('admin.finance.costSettings.expenses') }}</button>
        </fieldset>
      </div>
      <template #footer><button type="button" class="btn btn-secondary" :disabled="busy" @click="closeAccount">{{ t('common.close') }}</button></template>
    </BaseDialog>

    <BaseDialog :show="channelEditing !== null" :title="t('admin.finance.costSettings.sharedRules')" width="normal" @close="closeChannel">
      <div v-if="channelEditing && channelDraft" class="space-y-4">
        <p class="break-words text-sm">#{{ channelEditing.id }} {{ channelEditing.name }}</p>
        <p class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.finance.costSettings.sharedImpact') }}</p>
        <div class="max-h-40 overflow-auto rounded border border-gray-200 p-2 text-xs dark:border-dark-600" data-testid="finance-channel-impact">
          <p>{{ t('admin.finance.costSettings.affectedGroups') }}: {{ affectedGroups.map(id => `#${id} ${meta?.groups.find(g => g.id === id)?.name || ''}`).join(', ') || '—' }}</p>
          <p>{{ t('admin.finance.costSettings.affectedAccounts') }}: {{ affectedAccounts.map(a => `#${a.id} ${a.name}`).join(', ') || '—' }}</p>
        </div>
        <fieldset :disabled="busy || channelConflict" class="space-y-4">
          <label class="flex items-center gap-2 text-sm"><input v-model="channelDraft.apply_pricing_to_account_stats" type="checkbox" />{{ t('admin.channels.form.applyPricingToAccountStats') }}</label>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.channelPricingHint') }}</p>
          <AccountStatsPricingRulesEditor ref="rulesEditor" v-model="channelDraft.account_stats_pricing_rules" :groups="meta?.groups.filter(g => channelEditing?.group_ids.includes(g.id)) || []" />
        </fieldset>
        <p v-if="channelError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ channelError }}</p>
      </div>
      <template #footer>
        <div class="flex flex-wrap justify-end gap-2">
          <button type="button" class="btn btn-secondary mr-auto" :disabled="busy || opening" @click="reloadChannel">{{ t('common.refresh') }}</button>
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="closeChannel">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="busy || channelConflict || !channelDirty" data-testid="finance-channel-save" @click="saveChannel">{{ busy ? t('common.saving') : t('common.save') }}</button>
        </div>
      </template>
    </BaseDialog>
    <!-- Mount only for an active confirmation: the opening Escape must not reach a pre-mounted hidden dialog. -->
    <ConfirmDialog v-if="discardAction !== null" :show="true" :title="t('admin.finance.costSettings.discardTitle')" :message="t('admin.finance.costSettings.discardHint')" :confirm-text="t('admin.finance.costSettings.discard')" @confirm="discard" @cancel="cancelDiscard" />
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Account, AccountListItem } from '@/types'
import type { Channel } from '@/api/admin/channels'
import { getChannelCostConfig, updateChannelCostConfig, type FinanceChannelCostConfig } from '@/api/admin/financeCostConfig'
import AccountModelCostEditor from '@/components/account/AccountModelCostEditor.vue'
import AccountStatsPricingRulesEditor from '@/components/admin/channel/AccountStatsPricingRulesEditor.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import FinanceLoadState from './FinanceLoadState.vue'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'
import { listPlatformIds } from '@/constants/platformCatalog'
const props = defineProps<{ refreshToken: number }>()
const emit = defineEmits<{ changed: []; expenses: [] }>()
const { t } = useI18n()
const app = useAppStore()
const platforms = listPlatformIds()
const keyword = ref(''), platform = ref(''), page = ref(1), pageSize = ref(20)
const channelKeyword = ref(''), channelPage = ref(1)
const busy = ref(false), opening = ref(false)
const { data, loading, error, load } = useFinanceLoad(signal => adminAPI.accounts.list(page.value, pageSize.value, { search: keyword.value, platform: platform.value, lite: 'true' }, { signal }), () => t('common.unknownError'))
async function allChannels(signal: AbortSignal) {
  const rows: Channel[] = []
  for (let page = 1; ; page++) {
    const response = await adminAPI.channels.list(page, 100, {}, { signal })
    rows.push(...response.items)
    if (!response.items.length || rows.length >= response.total) return rows
  }
}
const { data: meta, loading: metaLoading, error: metaError, load: loadMeta } = useFinanceLoad(async signal => {
  const [settings, channels, groups, models] = await Promise.all([adminAPI.finance.getSettings(), allChannels(signal), adminAPI.groups.getAllIncludingInactive(), adminAPI.accountModelCost.listAll()])
  return { settings, channels, groups, models }
}, () => t('common.unknownError'))
function reload() { void load(); void loadMeta() }
watch(() => props.refreshToken, reload, { immediate: true })
watch([page, pageSize, platform], load)
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(keyword, () => { clearTimeout(searchTimer); searchTimer = setTimeout(() => { if (page.value === 1) void load(); else page.value = 1 }, 250) })
watch(platform, () => { page.value = 1 })
watch(channelKeyword, () => { channelPage.value = 1 })
const columns = computed(() => ['name', 'platform', 'rate', 'model', 'fixed', 'rules', 'actions'].map(key => ({ key, label: t(`admin.finance.costSettings.column${key[0].toUpperCase()}${key.slice(1)}`) })))
const channelColumns = computed(() => ['name', 'rules', 'actions'].map(key => ({ key, label: t(`admin.finance.costSettings.column${key[0].toUpperCase()}${key.slice(1)}`) })))
const filteredChannels = computed(() => (meta.value?.channels || []).filter(c => `${c.id} ${c.name}`.toLowerCase().includes(channelKeyword.value.trim().toLowerCase())))
const channelPageRows = computed(() => filteredChannels.value.slice((channelPage.value - 1) * 20, channelPage.value * 20))
function matchingChannels(account: AccountListItem) {
  const groups = account.group_ids || []
  return (meta.value?.channels || []).filter(channel => (channel.apply_pricing_to_account_stats && channel.group_ids.some(id => groups.includes(id))) || channel.account_stats_pricing_rules?.some(rule => rule.account_ids.includes(account.id) || rule.group_ids.some(id => groups.includes(id))))
}
const editing = ref<Account | null>(null), defaultRate = ref('1'), fixedCost = ref(false), savedFixed = ref(false)
const rateManaged = computed(() => editing.value?.type === 'apikey' && editing.value.extra?.upstream_billing_probe_enabled === true && editing.value.extra?.upstream_billing_rate_sync_enabled === true)
const statuses = reactive({ default: '', models: '', fixed: '' })
const modelEditor = ref<InstanceType<typeof AccountModelCostEditor> | null>(null)
let generation = 0
onBeforeUnmount(() => { generation++; clearTimeout(searchTimer); window.removeEventListener('beforeunload', beforeUnload); cancelDiscard() })
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
function beforeUnload(event: BeforeUnloadEvent) {
  if (!busy.value && !accountDirty.value && !channelDirty.value) return
  event.preventDefault(); event.returnValue = ''
}
async function openAccount(id: number) {
  opening.value = true; const current = ++generation
  try {
    const account = await adminAPI.accounts.getById(id)
    if (current !== generation) return
    editing.value = account; defaultRate.value = String(account.rate_multiplier ?? 1)
    fixedCost.value = savedFixed.value = meta.value?.settings.fixed_cost_account_ids.includes(id) ?? false
    statuses.default = statuses.models = statuses.fixed = ''
  } catch (error) { if (current === generation) app.showError(extractApiErrorMessage(error, t('common.unknownError'))) }
  finally { if (current === generation) opening.value = false }
}
const accountDirty = computed(() => !!editing.value && (defaultRate.value !== String(editing.value.rate_multiplier ?? 1) || fixedCost.value !== savedFixed.value || modelEditor.value?.hasChanges))
const discardAction = ref<(() => void) | null>(null)
let cancelPendingLeave: (() => void) | null = null
function cancelDiscard() { discardAction.value = null; cancelPendingLeave?.(); cancelPendingLeave = null }
function guard(action: () => void, dirty: boolean) { if (busy.value) return; if (dirty) discardAction.value = action; else action() }
function discard() { const action = discardAction.value; discardAction.value = null; cancelPendingLeave = null; action?.() }
function confirmLeave(): Promise<boolean> {
  if (busy.value) return Promise.resolve(false)
  if (!accountDirty.value && !channelDirty.value) return Promise.resolve(true)
  cancelDiscard()
  return new Promise(resolve => {
    cancelPendingLeave = () => resolve(false)
    discardAction.value = () => { editing.value = null; channelDraft.value = null; channelEditing.value = null; resolve(true) }
  })
}
async function requestLeave(action: () => void) { if (await confirmLeave()) action() }
defineExpose({ requestLeave, confirmLeave })
function closeAccount() { guard(() => { editing.value = null }, !!accountDirty.value) }
function goExpenses() { guard(() => { editing.value = null; emit('expenses') }, !!accountDirty.value) }
async function savePart(part: keyof typeof statuses, action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; statuses[part] = ''
  try { await action(); statuses[part] = t('admin.finance.costSettings.partSaved'); emit('changed'); void load(); void loadMeta() }
  catch (error) { statuses[part] = t('admin.finance.costSettings.partFailed', { error: extractApiErrorMessage(error, t('common.unknownError')) }) }
  finally { busy.value = false }
}
async function saveDefault() {
  if (!editing.value || rateManaged.value) return
  const value = Number(defaultRate.value)
  if (!/^(?:\d+(?:\.\d*)?|\.\d+)$/.test(defaultRate.value.trim()) || !Number.isFinite(value) || value < 0) { statuses.default = t('admin.finance.costSettings.invalidRate'); return }
  const account = editing.value
  await savePart('default', async () => { const saved = await adminAPI.accounts.update(account.id, { rate_multiplier: value }); editing.value = saved; defaultRate.value = String(saved.rate_multiplier ?? 1) })
}
async function saveModels() { await savePart('models', async () => { const error = modelEditor.value?.validate(); if (error) throw new Error(error); await modelEditor.value?.save() }) }
async function saveFixed() {
  if (!editing.value) return
  const id = editing.value.id, selected = fixedCost.value
  await savePart('fixed', async () => {
    // Reduce stale whole-list overwrites, but this read/modify/write is NOT a conditional write.
    const latest = await adminAPI.finance.getSettings()
    const ids = new Set(latest.fixed_cost_account_ids)
    if (selected) ids.add(id); else ids.delete(id)
    await adminAPI.finance.updateSettings({ fixed_cost_account_ids: [...ids].sort((a, b) => a - b) })
    savedFixed.value = selected
  })
}
const channelEditing = ref<Channel | null>(null), channelDraft = ref<FinanceChannelCostConfig | null>(null)
const channelBaseline = ref(''), channelError = ref(''), channelConflict = ref(false)
const impactAccounts = ref<AccountListItem[]>([])
const rulesEditor = ref<InstanceType<typeof AccountStatsPricingRulesEditor> | null>(null)
const channelDirty = computed(() => !!channelDraft.value && JSON.stringify(channelDraft.value) !== channelBaseline.value)
const baselineConfig = computed<FinanceChannelCostConfig | null>(() => channelBaseline.value ? JSON.parse(channelBaseline.value) : null)
const impactedRules = computed(() => [...(baselineConfig.value?.account_stats_pricing_rules || []), ...(channelDraft.value?.account_stats_pricing_rules || [])])
const affectedGroups = computed(() => [...new Set([...((channelDraft.value?.apply_pricing_to_account_stats || baselineConfig.value?.apply_pricing_to_account_stats) ? channelEditing.value?.group_ids || [] : []), ...impactedRules.value.flatMap(r => r.group_ids)])])
const affectedAccounts = computed(() => {
  const explicit = new Set(impactedRules.value.flatMap(r => r.account_ids))
  const accounts = impactAccounts.value.filter(a => explicit.has(a.id) || a.group_ids?.some(id => affectedGroups.value.includes(id)))
  return [...accounts, ...[...explicit].filter(id => !accounts.some(a => a.id === id)).map(id => ({ id, name: meta.value?.settings.accounts.find(a => a.id === id)?.name || '' }))]
})
async function openChannel(channel: Channel) {
  opening.value = true; const current = ++generation
  try {
    const config = await getChannelCostConfig(channel.id)
    const rows: AccountListItem[] = []
    for (let page = 1; ; page++) {
      const response = await adminAPI.accounts.list(page, 100, { lite: 'true' })
      if (current !== generation) return
      rows.push(...response.items)
      if (!response.items.length || rows.length >= response.total) break
    }
    if (current !== generation) return
    impactAccounts.value = rows; channelDraft.value = config; channelBaseline.value = JSON.stringify(config)
    channelEditing.value = channel; channelError.value = ''; channelConflict.value = false
  } catch (error) { if (current === generation) app.showError(extractApiErrorMessage(error, t('common.unknownError'))) }
  finally { if (current === generation) opening.value = false }
}
function closeChannel() { guard(() => { channelEditing.value = null; channelDraft.value = null }, channelDirty.value) }
function reloadChannel() { const channel = channelEditing.value; if (channel) guard(() => { void openChannel(channel) }, channelDirty.value) }
async function saveChannel() {
  if (!channelDraft.value || !channelEditing.value || busy.value || channelConflict.value) return
  const validation = rulesEditor.value?.validate()
  if (validation) { channelError.value = validation; return }
  busy.value = true; channelError.value = ''
  try {
    const draft = channelDraft.value
    const saved = await updateChannelCostConfig(channelEditing.value.id, { expected_version: draft.version, apply_pricing_to_account_stats: draft.apply_pricing_to_account_stats, account_stats_pricing_rules: draft.account_stats_pricing_rules })
    channelDraft.value = saved; channelBaseline.value = JSON.stringify(saved)
    app.showSuccess(t('admin.finance.costSettings.partSaved')); emit('changed'); void loadMeta()
  } catch (error) {
    const e = error as { response?: { status?: number }; status?: number; reason?: string }
    channelConflict.value = e?.response?.status === 409 || e?.status === 409 || e?.reason === 'FINANCE_COST_CONFIG_CHANGED'
    channelError.value = channelConflict.value ? t('admin.finance.costSettings.conflict') : extractApiErrorMessage(error, t('common.unknownError'))
  } finally { busy.value = false }
}
</script>
