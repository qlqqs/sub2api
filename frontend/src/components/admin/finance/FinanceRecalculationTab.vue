<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div><h3 class="font-semibold">{{ t(`${ns}.title`) }}</h3><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t(`${ns}.description`) }}</p></div>
      <div class="flex gap-2">
        <button type="button" class="btn btn-secondary" @click="refresh">{{ t(`${ns}.refresh`) }}</button>
        <button type="button" class="btn btn-primary" data-testid="new-preview" @click="openNew">{{ t(`${ns}.newPreview`) }}</button>
      </div>
    </div>
    <p class="text-sm text-gray-500 dark:text-gray-400">{{ t(`${ns}.durable`) }}</p>
    <p v-if="listError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ listError }}</p>
    <DataTable :columns="batchColumns" :data="batches?.items ?? []" :loading="listLoading">
      <template #cell-kind="{ row }">{{ t(`${ns}.kinds.${row.kind}`) }}</template>
      <template #cell-status="{ row }"><span role="status">{{ t(`${ns}.statuses.${row.status}`) }}</span></template>
      <template #cell-scope="{ row }">{{ row.start }} — {{ row.end }}</template>
      <template #cell-actions="{ row }"><button type="button" class="btn btn-secondary btn-sm" :data-testid="`batch-${row.id}`" @click="openBatch(row.id)">{{ t(`${ns}.details`) }}</button></template>
    </DataTable>
    <Pagination v-if="batches && batches.total > 0" :total="batches.total" :page="page" :page-size="pageSize" @update:page="page = $event" @update:page-size="setPageSize" />

    <BaseDialog :show="dialogOpen" :title="t(`${ns}.dialogTitle`)" width="wide" @close="closeDialog">
      <ol class="mb-5 grid grid-cols-3 gap-2 text-sm">
        <li v-for="n in [1, 2, 3]" :key="n" :aria-current="step === n ? 'step' : undefined" :class="step === n ? 'font-semibold text-primary-600 dark:text-primary-400' : 'text-gray-500'">{{ n }}. {{ t(`${ns}.steps.${n}`) }}</li>
      </ol>
      <p v-if="actionError || detailError" role="alert" class="mb-4 break-words text-sm text-red-600 dark:text-red-400">{{ actionError || detailError }}</p>
      <p v-if="needsNewPreview || previewExpired" class="mb-4 text-sm text-amber-700 dark:text-amber-300">{{ t(`${ns}.conflictHint`) }}</p>
      <form v-if="step === 1" id="finance-recalculation-scope" class="space-y-4" @submit.prevent="createPreview">
        <div><label for="recalculation-kind" class="label">{{ t(`${ns}.kind`) }}</label><Select id="recalculation-kind" v-model="kind" :options="kindOptions" /></div>
        <div><p id="recalculation-dates" class="label">{{ t(`${ns}.scope`) }} · {{ t(`${ns}.beijing`) }}</p><DateRangePicker v-model:start-date="start" v-model:end-date="end" aria-labelledby="recalculation-dates" /></div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t(`${ns}.rangeHint`) }}</p>
        <template v-if="kind === 'usage_cost'">
          <div><label for="recalculation-account" class="label">{{ t(`${ns}.account`) }}</label><Select id="recalculation-account" v-model="accountId" :options="accountOptions" searchable /></div>
          <div><label for="recalculation-model" class="label">{{ t(`${ns}.model`) }}</label><input id="recalculation-model" v-model="model" class="input" list="recalculation-models" :placeholder="t(`${ns}.allModels`)" maxlength="256" /><datalist id="recalculation-models"><option v-for="name in models" :key="name" :value="name" /></datalist><p class="mt-1 text-xs text-gray-500">{{ t(`${ns}.modelHint`) }}</p></div>
          <div class="space-y-2 rounded-lg bg-amber-50 p-3 text-sm text-amber-900 dark:bg-amber-900/20 dark:text-amber-200">
            <label class="flex items-start gap-2"><input v-model="acceptRetainedBaseline" data-testid="accept-retained-baseline" type="checkbox" class="mt-1" aria-describedby="retained-baseline-hint" :disabled="mutating" /><span>{{ t(`${ns}.baseline.label`) }}</span></label>
            <p id="retained-baseline-hint" class="text-xs">{{ t(`${ns}.baseline.hint`) }}</p>
            <p class="text-xs">{{ t(`${ns}.baseline.backup`) }}</p>
          </div>
        </template>
        <div v-else><label for="recalculation-provider" class="label">{{ t(`${ns}.provider`) }}</label><Select id="recalculation-provider" v-model="providerId" :options="providerOptions" searchable /></div>
        <p v-if="optionsError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ optionsError }} <button type="button" class="underline" @click="loadOptions">{{ t(`${ns}.refresh`) }}</button></p>
        <p class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800">{{ t(`${ns}.boundaries`) }}</p>
      </form>
      <template v-else-if="selected">
        <FinanceRecalculationSummary :batch="selected" />
        <template v-if="step === 2">
          <p class="my-4 text-xs text-gray-500 dark:text-gray-400">{{ t(`${ns}.itemsHint`) }}</p>
          <p v-if="itemsError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ itemsError }}</p>
          <DataTable :columns="itemColumns" :data="items?.items ?? []" :loading="itemsLoading">
            <template #cell-status="{ row }">{{ itemLabel(row.status) }}</template>
            <template #cell-source="{ row }">{{ detailLabel('sources', row.source) }}</template>
            <template #cell-reason="{ row }"><span class="break-words">{{ detailLabel('reasons', row.reason) }}</span></template>
            <template #cell-old_cost="{ row }"><span class="font-mono">{{ row.old_cost ?? '—' }}</span></template>
            <template #cell-new_cost="{ row }"><span class="font-mono">{{ row.status === 'skipped' ? '—' : row.new_cost ?? '—' }}</span></template>
            <template #cell-delta="{ row }"><span class="font-mono">{{ row.status === 'skipped' ? '—' : row.delta ?? '—' }}</span></template>
            <template #cell-applied="{ row }">{{ t(`${ns}.${row.applied ? 'yes' : 'no'}`) }}</template>
          </DataTable>
          <Pagination v-if="items && items.total > 0" :total="items.total" :page="itemPage" :page-size="itemPageSize" @update:page="itemPage = $event" @update:page-size="setItemPageSize" />
          <p v-if="selected.status === 'ready' && !isCreator" class="mt-3 text-sm text-amber-700 dark:text-amber-300">{{ t(`${ns}.creatorOnly`) }}</p>
        </template>
        <div v-else class="mt-5 space-y-4 rounded-xl border border-red-200 bg-red-50 p-4 text-sm dark:border-red-800 dark:bg-red-950/20">
          <strong class="text-red-700 dark:text-red-300">{{ t(`${ns}.irreversible`) }}</strong>
          <p>{{ t(`${ns}.confirmHint`) }}</p>
          <label class="flex items-start gap-2"><input v-model="acknowledged" data-testid="acknowledge" type="checkbox" class="mt-1" :disabled="mutating" /><span>{{ t(`${ns}.acknowledge`) }}</span></label>
        </div>
      </template>
      <p v-else class="text-sm text-gray-500">{{ t(`${ns}.loading`) }}</p>
      <template #footer>
        <div class="flex w-full flex-wrap justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeDialog">{{ t(`${ns}.close`) }}</button>
          <button v-if="step > 1" type="button" class="btn btn-secondary" :disabled="mutating" @click="refresh">{{ t(`${ns}.refresh`) }}</button>
          <button v-if="step === 1" type="submit" form="finance-recalculation-scope" class="btn btn-primary" data-testid="create-preview" :disabled="mutating">{{ t(`${ns}.${mutating ? 'loading' : 'preview'}`) }}</button>
          <button v-if="step === 2 && canConfirm" type="button" class="btn btn-primary" data-testid="review-confirm" @click="step = 3; acknowledged = false">{{ t(`${ns}.reviewConfirm`) }}</button>
          <button v-if="step === 3" type="button" class="btn btn-secondary" :disabled="mutating" @click="step = 2; acknowledged = false">{{ t(`${ns}.back`) }}</button>
          <button v-if="step === 3" type="button" class="btn btn-danger" data-testid="confirm-execute" :disabled="!acknowledged || !canConfirm || mutating" @click="execute">{{ t(`${ns}.${mutating ? 'loading' : 'execute'}`) }}</button>
          <button v-if="canRetry" type="button" class="btn btn-primary" data-testid="retry-sync" :disabled="mutating" @click="retrySync">{{ t(`${ns}.retrySync`) }}</button>
          <button v-if="needsNewPreview || previewExpired || (selected && ['conflict', 'expired', 'failed'].includes(selected.status))" type="button" class="btn btn-secondary" :disabled="mutating" @click="openNew">{{ t(`${ns}.newPreview`) }}</button>
        </div>
      </template>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { financeRecalculationAPI as api, recalculationIsRunning, type RecalculationBatch, type RecalculationKind, type RecalculationScope } from '@/api/admin/financeRecalculation'
import { getSettings } from '@/api/admin/finance'
import { getAvailableModels } from '@/api/admin/accounts'
import { adminPaymentAPI } from '@/api/admin/payment'
import { useAuthStore } from '@/stores/auth'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { extractApiErrorMessage } from '@/utils/apiError'
import { defaultFinanceRange, inclusiveDays } from '@/utils/financeDate'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import FinanceRecalculationSummary from './FinanceRecalculationSummary.vue'

const props = withDefaults(defineProps<{ refreshToken?: number }>(), { refreshToken: 0 })
const emit = defineEmits<{ changed: [] }>()
const { t, te } = useI18n()
const auth = useAuthStore()
const ns = 'admin.financeRecalculation'
const fallback = () => t(`${ns}.loadFailed`)
const page = ref(1)
const pageSize = ref(20)
const itemPage = ref(1)
const itemPageSize = ref(20)
const selectedId = ref<number | null>(null)
const selected = ref<RecalculationBatch | null>(null)
const dialogOpen = ref(false)
const step = ref(1)
const acknowledged = ref(false)
const actionError = ref('')
const detailError = ref('')
const needsNewPreview = ref(false)
const mutating = ref(false)
const now = ref(Date.now())
let disposed = false
let dialogGeneration = 0
let detailGeneration = 0
let detailController: AbortController | null = null
let timer: ReturnType<typeof setTimeout> | undefined
const observed = new Map<number, string>()
const notifiedCompleted = new Set<number>()
function observe(batch: RecalculationBatch) {
  const previous = observed.get(batch.id)
  // Completion is terminal: an overlapping older list read cannot make it incomplete again.
  if (previous === 'completed') return
  observed.set(batch.id, batch.status)
  if (batch.status === 'completed' && previous && previous !== 'completed' && !notifiedCompleted.has(batch.id)) {
    notifiedCompleted.add(batch.id)
    emit('changed')
  }
}
const { data: batches, loading: listLoading, error: listError, load: loadList } = useFinanceLoad(
  signal => api.list({ page: page.value, page_size: pageSize.value }, signal), fallback
)
watch(batches, value => value?.items.forEach(observe))
const { data: items, loading: itemsLoading, error: itemsError, load: loadItems } = useFinanceLoad(
  signal => selectedId.value == null ? Promise.resolve({ items: [], total: 0 }) : api.items(selectedId.value, { page: itemPage.value, page_size: itemPageSize.value }, signal), fallback
)
watch(() => [page.value, pageSize.value, props.refreshToken], () => void refresh(), { immediate: true })
watch(() => [selectedId.value, itemPage.value, itemPageSize.value], () => { items.value = null; void loadItems() })
const batchColumns = computed(() => ['id', 'kind', 'scope', 'status', 'scanned', 'changed', 'actions'].map(key => ({ key, label: t(`${ns}.columns.${key}`) })))
const itemColumns = computed(() => ['id', 'status', 'source', 'old_cost', 'new_cost', 'delta', 'reason', 'applied'].map(key => ({ key, label: t(`${ns}.columns.${key}`) })))
const isCreator = computed(() => selected.value?.actor_id === auth.user?.id)
const previewExpired = computed(() => selected.value?.status === 'ready' && Date.parse(selected.value.expires_at) <= now.value)
const canConfirm = computed(() => selected.value?.status === 'ready' && isCreator.value && !needsNewPreview.value && !!selected.value.config_fingerprint && Date.parse(selected.value.expires_at) > now.value && selected.value.changed > 0)
const canRetry = computed(() => selected.value?.status === 'failed' && selected.value.failed_phase === 'syncing' && isCreator.value)
function itemLabel(status: string) { return te(`${ns}.itemStatuses.${status}`) ? t(`${ns}.itemStatuses.${status}`) : status }
function detailLabel(group: 'sources' | 'reasons', value: string): string {
  if (!value) return '—'
  const key = `${ns}.${group}.${value}`
  return te(key) ? `${t(key)} (${value})` : value
}
function setPageSize(size: number) { pageSize.value = size; page.value = 1 }
function setItemPageSize(size: number) { itemPageSize.value = size; itemPage.value = 1 }

async function loadDetail() {
  const id = selectedId.value
  if (id == null || mutating.value) return
  detailController?.abort()
  const controller = new AbortController()
  detailController = controller
  const generation = ++detailGeneration
  try {
    const batch = await api.get(id, controller.signal)
    if (disposed || generation !== detailGeneration || selectedId.value !== id) return
    if (selected.value && selected.value.config_fingerprint !== batch.config_fingerprint) {
      acknowledged.value = false; step.value = 2
    }
    selected.value = batch
    detailError.value = ''
    observe(batch)
    if (batch.status !== 'ready') { step.value = 2; acknowledged.value = false }
  } catch (error) {
    if (!disposed && generation === detailGeneration && selectedId.value === id) detailError.value = extractApiErrorMessage(error, fallback())
  }
}
async function refresh() {
  if (disposed) return
  now.value = Date.now()
  await Promise.all([loadList(), loadDetail(), selectedId.value == null ? Promise.resolve() : loadItems()])
}
async function poll() {
  if (disposed) return
  now.value = Date.now()
  if (batches.value?.items.some(b => recalculationIsRunning(b.status)) || (selected.value && recalculationIsRunning(selected.value.status))) await refresh()
  if (!disposed) timer = setTimeout(poll, 3000)
}
function closeDialog() {
  dialogOpen.value = false
  ++dialogGeneration
  ++detailGeneration
  detailController?.abort()
  selectedId.value = null
  selected.value = null
  acknowledged.value = false
  mutating.value = false
}
function resetDialog() {
  closeDialog()
  actionError.value = ''; detailError.value = ''; needsNewPreview.value = false
  itemPage.value = 1
  dialogOpen.value = true
}
function openBatch(id: number) {
  resetDialog(); step.value = 2; selectedId.value = id
  void loadDetail()
}

const range = defaultFinanceRange()
const kind = ref<RecalculationKind>('usage_cost')
const acceptRetainedBaseline = ref(false)
watch(kind, () => { acceptRetainedBaseline.value = false })
const start = ref(range.start)
const end = ref(range.end)
const accountId = ref<number | ''>('')
const providerId = ref<number | ''>('')
const model = ref('')
const accounts = ref<SelectOption[]>([])
const providers = ref<SelectOption[]>([])
const models = ref<string[]>([])
const optionsError = ref('')
let optionsGeneration = 0
let modelsGeneration = 0
const kindOptions = computed(() => (['usage_cost', 'payment_fee'] as const).map(value => ({ value, label: t(`${ns}.kinds.${value}`) })))
const accountOptions = computed(() => [{ value: '', label: t(`${ns}.allAccounts`) }, ...accounts.value])
const providerOptions = computed(() => [{ value: '', label: t(`${ns}.allProviders`) }, ...providers.value])
async function loadOptions() {
  const generation = ++optionsGeneration
  optionsError.value = ''
  try {
    const [settings, payment] = await Promise.all([getSettings(), adminPaymentAPI.getProviders()])
    if (disposed || generation !== optionsGeneration) return
    accounts.value = settings.accounts.filter(a => !a.deleted).map(a => ({ value: a.id, label: `#${a.id} ${a.name}` }))
    // Explicit whitelist: do not retain or render provider.config/credentials.
    providers.value = payment.data.map(p => ({ value: p.id, label: `#${p.id} ${p.name}` }))
  } catch (error) {
    if (!disposed && generation === optionsGeneration) optionsError.value = extractApiErrorMessage(error, fallback())
  }
}
watch(accountId, async id => {
  model.value = ''; models.value = []
  const generation = ++modelsGeneration
  if (!id) return
  try {
    const result = await getAvailableModels(id)
    if (!disposed && generation === modelsGeneration) models.value = result.map(m => m.id)
  } catch {
    // Historical upstream names remain manually enterable even if the account no longer advertises them.
    if (!disposed && generation === modelsGeneration) models.value = []
  }
})
let previewRequest: { signature: string; key: string } | null = null
function openNew() {
  resetDialog(); step.value = 1; previewRequest = null; acceptRetainedBaseline.value = false
  void loadOptions()
}
function actionFailed(error: unknown) {
  const e = error && typeof error === 'object' ? error as { status?: number; response?: { status?: number } } : {}
  if (e.status === 409 || e.response?.status === 409) {
    needsNewPreview.value = true; acknowledged.value = false; step.value = 2
    actionError.value = t(`${ns}.conflictHint`)
  } else actionError.value = extractApiErrorMessage(error, fallback())
}
async function createPreview() {
  if (mutating.value) return
  const days = inclusiveDays(start.value, end.value)
  if (!Number.isFinite(days) || days < 1 || days > 732) { actionError.value = t(`${ns}.invalidRange`); return }
  const scope: RecalculationScope = { kind: kind.value, start: start.value, end: end.value }
  if (kind.value === 'usage_cost') { scope.accept_retained_baseline = acceptRetainedBaseline.value; if (accountId.value) scope.account_id = accountId.value; if (model.value.trim()) scope.model = model.value.trim() }
  else if (providerId.value) scope.provider_id = providerId.value
  const signature = JSON.stringify(scope)
  if (previewRequest?.signature !== signature) previewRequest = { signature, key: crypto.randomUUID() }
  const generation = dialogGeneration
  mutating.value = true; actionError.value = ''
  try {
    const batch = await api.create({ ...scope, idempotency_key: previewRequest.key })
    if (disposed) return
    // Closing a dialog does not cancel a durable job; discover it in the list without reopening UI.
    void loadList()
    if (generation !== dialogGeneration) return
    selected.value = batch; selectedId.value = batch.id; step.value = 2; observe(batch)
  } catch (error) {
    if (!disposed && generation === dialogGeneration) actionFailed(error)
  } finally {
    if (!disposed && generation === dialogGeneration) mutating.value = false
  }
}
async function mutate(operation: (batch: RecalculationBatch) => Promise<RecalculationBatch>) {
  if (!selected.value || mutating.value) return
  const batch = selected.value
  const generation = dialogGeneration
  ++detailGeneration; detailController?.abort()
  mutating.value = true; actionError.value = ''
  try {
    const result = await operation(batch)
    if (disposed) return
    if (generation !== dialogGeneration) { void loadList(); return }
    selected.value = result; step.value = 2; acknowledged.value = false; observe(result)
  } catch (error) {
    if (!disposed && generation === dialogGeneration) actionFailed(error)
  } finally {
    if (!disposed && generation === dialogGeneration) { mutating.value = false; void refresh() }
  }
}
function execute() { now.value = Date.now(); if (canConfirm.value && acknowledged.value) void mutate(b => api.confirm(b.id, b.config_fingerprint)) }
function retrySync() { if (canRetry.value) void mutate(b => api.retrySync(b.id)) }
onMounted(() => { timer = setTimeout(poll, 3000) })
onBeforeUnmount(() => {
  disposed = true; ++dialogGeneration; ++detailGeneration; ++optionsGeneration; ++modelsGeneration
  clearTimeout(timer); detailController?.abort()
})
</script>
