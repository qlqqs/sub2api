<template>
  <section class="card space-y-4 p-4" data-testid="finance-payment-settings" :aria-busy="loading">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.costSettings.paymentTitle') }}</h3>
        <p class="mt-1 max-w-2xl text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.paymentHint') }}</p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.paymentSource') }}</p>
      </div>
      <button type="button" class="btn btn-secondary" :disabled="loading || saving" data-testid="finance-payment-reload" @click="load">
        {{ t('common.refresh') }}
      </button>
    </div>
    <input
      v-model="keyword"
      type="search"
      class="input w-full sm:w-80"
      :placeholder="t('admin.finance.costSettings.paymentSearch')"
      :aria-label="t('admin.finance.costSettings.paymentSearch')"
      data-testid="finance-payment-search"
    />
    <FinanceLoadState :loading="loading" :error="error" :has-data="data !== null" @retry="load">
      <DataTable :columns="columns" :data="pageRows" :expandable-actions="false">
        <template #cell-name="{ row }">
          <span class="block max-w-xs break-words font-medium">{{ row.name }}</span>
          <span class="text-xs text-gray-400">#{{ row.id }}</span>
        </template>
        <template #cell-channel_fee_rate="{ row }">
          <span class="font-mono">{{ formatRatio(row.channel_fee_rate) }}</span>
        </template>
        <template #cell-enabled="{ row }">{{ t(row.enabled ? 'common.enabled' : 'common.disabled') }}</template>
        <template #cell-actions="{ row }">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || saving" :data-testid="`finance-payment-edit-${row.id}`" @click="openEditor(row)">
            {{ t('common.edit') }}
          </button>
        </template>
        <template #empty><p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.paymentEmpty') }}</p></template>
      </DataTable>
      <Pagination
        v-if="filteredRows.length > 0"
        :total="filteredRows.length"
        :page="page"
        :page-size="pageSize"
        @update:page="page = $event"
        @update:page-size="changePageSize"
      />
    </FinanceLoadState>

    <BaseDialog
      :show="editing !== null"
      :title="t('admin.finance.costSettings.paymentEditTitle')"
      width="normal"
      :close-on-escape="!saving"
      :show-close-button="!saving"
      @close="closeEditor"
    >
      <form id="finance-payment-fee-form" class="space-y-4" novalidate @submit.prevent="save">
        <p class="break-words text-sm text-gray-700 dark:text-gray-300">#{{ editing?.id }} {{ editing?.name }}</p>
        <p id="finance-payment-fee-hint" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.costSettings.paymentHint') }}</p>
        <div>
          <label for="finance-payment-fee" class="input-label">{{ t('admin.finance.channelFee.label') }}</label>
          <input
            id="finance-payment-fee"
            v-model="draftPercent"
            type="text"
            inputmode="decimal"
            class="input"
            :disabled="saving || loading"
            :aria-invalid="invalid"
            :aria-describedby="invalid ? 'finance-payment-fee-hint finance-payment-fee-error' : 'finance-payment-fee-hint'"
            data-testid="finance-payment-fee"
            @input="invalid = false"
          />
          <p v-if="invalid" id="finance-payment-fee-error" class="mt-1 text-xs text-red-600 dark:text-red-400" role="alert">
            {{ t('admin.finance.channelFee.invalid', { max: CHANNEL_FEE_PERCENT_MAX }) }}
          </p>
        </div>
        <p v-if="saveError" class="text-sm text-red-600 dark:text-red-400" role="alert" data-testid="finance-payment-save-error">{{ saveError }}</p>
      </form>
      <template #footer>
        <div class="flex flex-wrap justify-end gap-3">
          <button type="button" class="btn btn-secondary mr-auto" :disabled="saving || loading" data-testid="finance-payment-editor-reload" @click="reloadEditor">{{ t('common.refresh') }}</button>
          <button type="button" class="btn btn-secondary" :disabled="saving" data-testid="finance-payment-cancel" @click="closeEditor">{{ t('common.cancel') }}</button>
          <button type="submit" form="finance-payment-fee-form" class="btn btn-primary" :disabled="saving || loading" data-testid="finance-payment-save">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { ProviderInstance } from '@/types/payment'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { CHANNEL_FEE_PERCENT_MAX, decimalRateToPercent, formatRatio, percentToDecimalRate } from '@/utils/financeFormat'
import FinanceLoadState from './FinanceLoadState.vue'

const props = withDefaults(defineProps<{ refreshToken?: number }>(), { refreshToken: 0 })
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const appStore = useAppStore()

// Keep only display/cost fields. Provider credentials never enter the editor draft or a write payload.
type PaymentCostRow = Pick<ProviderInstance, 'id' | 'name' | 'provider_key' | 'enabled' | 'channel_fee_rate'>
const { data, loading, error, load } = useFinanceLoad<PaymentCostRow[]>(async () => {
  const response = await adminPaymentAPI.getProviders()
  return response.data.map(({ id, name, provider_key, enabled, channel_fee_rate }) => ({
    id, name, provider_key, enabled, channel_fee_rate
  }))
}, () => t('common.unknownError'))
watch(() => props.refreshToken, load, { immediate: true })

const keyword = ref('')
const page = ref(1)
const pageSize = ref(20)
const filteredRows = computed(() => {
  const search = keyword.value.trim().toLocaleLowerCase()
  return (data.value ?? []).filter(row => `${row.id} ${row.name} ${row.provider_key}`.toLocaleLowerCase().includes(search))
})
const pageRows = computed(() => filteredRows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
const columns = computed(() => [
  { key: 'name', label: t('common.name') },
  { key: 'provider_key', label: t('admin.finance.costSettings.paymentProvider') },
  { key: 'channel_fee_rate', label: t('admin.finance.channelFee.label') },
  { key: 'enabled', label: t('common.status') },
  { key: 'actions', label: t('common.actions') }
])
watch(keyword, () => { page.value = 1 })
watch(() => filteredRows.value.length, count => {
  page.value = Math.min(page.value, Math.max(1, Math.ceil(count / pageSize.value)))
})
function changePageSize(size: number) {
  pageSize.value = size
  page.value = 1
}

const editing = ref<PaymentCostRow | null>(null)
const draftPercent = ref('')
const invalid = ref(false)
const saving = ref(false)
const saveError = ref('')
let disposed = false
onBeforeUnmount(() => { disposed = true })

function openEditor(row: PaymentCostRow) {
  if (saving.value || loading.value) return
  editing.value = { ...row }
  draftPercent.value = decimalRateToPercent(row.channel_fee_rate)
  invalid.value = false
  saveError.value = ''
}
function closeEditor() {
  if (saving.value) return
  editing.value = null
  draftPercent.value = ''
  invalid.value = false
  saveError.value = ''
}
async function reloadEditor() {
  if (saving.value || loading.value || !editing.value) return
  const id = editing.value.id
  await load()
  if (disposed || editing.value?.id !== id) return
  if (error.value) {
    saveError.value = error.value
    return
  }
  const current = data.value?.find(row => row.id === id)
  if (current) openEditor(current)
  else closeEditor()
}
async function save() {
  if (saving.value || loading.value || !editing.value) return
  const rate = percentToDecimalRate(draftPercent.value)
  // Unlike the full provider form, clearing an existing fee must not silently save zero.
  if (!draftPercent.value.trim() || 'error' in rate) {
    invalid.value = true
    return
  }
  saving.value = true
  invalid.value = false
  saveError.value = ''
  try {
    // Existing endpoint is PUT with partial-update semantics. Never spread the provider object.
    await adminPaymentAPI.updateProvider(editing.value.id, { channel_fee_rate: rate.value })
    if (disposed) return
    saving.value = false
    closeEditor()
    appStore.showSuccess(t('admin.finance.costSettings.paymentSaved'))
    emit('changed')
    void load()
  } catch (err) {
    if (!disposed) saveError.value = extractApiErrorMessage(err, t('common.unknownError'))
  } finally {
    if (!disposed) saving.value = false
  }
}
</script>
