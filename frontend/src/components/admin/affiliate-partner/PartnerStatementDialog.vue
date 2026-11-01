<template>
  <!-- CUSTOM: 结算单明细与操作：调整项（未确认）、确认（按月份顺序，当月不能确认）、标记已付、导出 -->
  <BaseDialog :show="show" :title="t('admin.partners.statement.title', { month })" width="extra-wide" @close="emit('close')">
    <FinanceLoadState :loading="loading" :error="error" :has-data="!!detail" @retry="load">
      <PartnerStatementDetailBody v-if="detail" :detail="detail">
        <template v-if="actions.adjust" #actions="{ item }">
          <button
            type="button"
            class="text-sm text-red-600 hover:underline disabled:opacity-50"
            :disabled="busy"
            data-test="partner-adjustment-delete"
            @click="removeAdjustment(item.id)"
          >
            {{ t('common.delete') }}
          </button>
        </template>
        <template v-if="actions.adjust" #footer>
          <form class="flex flex-wrap items-start gap-2 border-t border-gray-100 p-4 dark:border-dark-700" data-test="partner-adjustment-form" @submit.prevent="addAdjustment">
            <input
              v-model="adjustment.amount"
              type="text"
              inputmode="decimal"
              class="input w-32"
              :placeholder="t('admin.partners.statement.adjustmentAmount')"
              :aria-label="t('admin.partners.statement.adjustmentAmount')"
              data-test="partner-adjustment-amount"
            />
            <input
              v-model="adjustment.note"
              type="text"
              class="input min-w-0 flex-1"
              maxlength="500"
              :placeholder="t('admin.partners.statement.adjustmentNote')"
              :aria-label="t('admin.partners.statement.adjustmentNote')"
              data-test="partner-adjustment-note"
            />
            <button type="submit" class="btn btn-secondary" :disabled="busy || !adjustmentValid" data-test="partner-adjustment-add">
              {{ t('admin.partners.statement.addAdjustment') }}
            </button>
          </form>
        </template>
      </PartnerStatementDetailBody>
    </FinanceLoadState>

    <form v-if="paying" class="mt-4 grid gap-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700 sm:grid-cols-[10rem_1fr]" data-test="partner-pay-form" @submit.prevent="pay">
      <div>
        <label for="partner-pay-date" class="input-label">{{ t('admin.partners.statement.paidOn') }}</label>
        <input id="partner-pay-date" v-model="payForm.paid_on" type="date" class="input" data-test="partner-pay-date" />
      </div>
      <div>
        <label for="partner-pay-note" class="input-label">{{ t('admin.partners.statement.paidNote') }}</label>
        <input id="partner-pay-note" v-model="payForm.note" type="text" class="input" maxlength="500" data-test="partner-pay-note" />
      </div>
      <div class="flex justify-end gap-2 sm:col-span-2">
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="paying = false">{{ t('common.cancel') }}</button>
        <button type="submit" class="btn btn-primary" :disabled="busy || !payForm.note.trim()" data-test="partner-pay-submit">
          {{ t('admin.partners.statement.markPaid') }}
        </button>
      </div>
    </form>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-2">
        <span v-if="confirmBlockedReason" class="mr-auto text-xs text-gray-500 dark:text-dark-400" data-test="partner-confirm-hint">
          {{ confirmBlockedReason }}
        </span>
        <button type="button" class="btn btn-secondary" :disabled="exporting" data-test="partner-statement-export" @click="exportCsv">
          <Icon name="download" size="sm" class="mr-1" />
          {{ t('admin.partners.statement.export') }}
        </button>
        <button
          v-if="actions.pay && !paying"
          type="button"
          class="btn btn-secondary"
          :disabled="busy"
          data-test="partner-statement-pay"
          @click="openPay"
        >
          {{ t('admin.partners.statement.markPaid') }}
        </button>
        <button
          v-if="status === 'draft' || status === 'current'"
          type="button"
          class="btn btn-primary"
          :disabled="busy || !actions.confirm"
          data-test="partner-statement-confirm"
          @click="confirming = true"
        >
          {{ t('admin.partners.statement.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="confirming"
    :title="t('admin.partners.statement.confirmTitle', { month })"
    :message="t('admin.partners.statement.confirmMessage')"
    :confirm-text="t('admin.partners.statement.confirm')"
    @confirm="confirm"
    @cancel="confirming = false"
  />
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import FinanceLoadState from '@/components/admin/finance/FinanceLoadState.vue'
import PartnerStatementDetailBody from '@/components/user/affiliate-partner/PartnerStatementDetailBody.vue'
import type { PartnerMonthTotals } from '@/api/affiliatePartner'
import affiliatePartnersAPI from '@/api/admin/affiliatePartners'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { normalizeAdjustmentAmount, partnerStatementActions } from '@/utils/affiliatePartner'
import { beijingDate } from '@/utils/financeDate'

const props = defineProps<{
  show: boolean
  partnerId: number
  month: string
  /** 整张结算单列表：确认要求之前的月份都已确认。 */
  statements: PartnerMonthTotals[]
}>()

/** changed：结算单状态或金额变了，父级刷新列表。 */
const emit = defineEmits<{ close: []; changed: [] }>()

const { t } = useI18n()
const appStore = useAppStore()

const { data: detail, loading, error, load } = useFinanceLoad(
  (signal) => affiliatePartnersAPI.getStatement(props.partnerId, props.month, { signal }),
  () => t('common.unknownError')
)

const status = computed(() => detail.value?.totals.status ?? props.statements.find((item) => item.month === props.month)?.status)
const actions = computed(() => partnerStatementActions(props.statements, props.month))

const confirmBlockedReason = computed(() => {
  if (status.value === 'current') return t('admin.partners.statement.currentMonthHint')
  if (status.value === 'draft' && !actions.value.confirm) return t('admin.partners.statement.previousHint')
  return ''
})

const busy = ref(false)
const confirming = ref(false)
const paying = ref(false)
const exporting = ref(false)

async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  try {
    await action()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    busy.value = false
  }
}

function confirm() {
  confirming.value = false
  void run(async () => {
    detail.value = await affiliatePartnersAPI.confirmStatement(props.partnerId, props.month)
    appStore.showSuccess(t('admin.partners.statement.confirmed'))
    emit('changed')
  })
}

const payForm = reactive({ paid_on: '', note: '' })

function openPay() {
  payForm.paid_on = beijingDate()
  payForm.note = ''
  paying.value = true
}

function pay() {
  const note = payForm.note.trim()
  if (!note) return
  void run(async () => {
    detail.value = await affiliatePartnersAPI.payStatement(props.partnerId, props.month, {
      paid_on: payForm.paid_on || undefined,
      note
    })
    paying.value = false
    appStore.showSuccess(t('admin.partners.statement.paid'))
    emit('changed')
  })
}

const adjustment = reactive({ amount: '', note: '' })
const adjustmentValid = computed(() => normalizeAdjustmentAmount(adjustment.amount) !== null && adjustment.note.trim() !== '')

function addAdjustment() {
  const amount = normalizeAdjustmentAmount(adjustment.amount)
  const note = adjustment.note.trim()
  if (amount === null || !note) return
  void run(async () => {
    await affiliatePartnersAPI.createAdjustment(props.partnerId, { month: props.month, amount, note })
    adjustment.amount = ''
    adjustment.note = ''
    await load()
    emit('changed')
  })
}

function removeAdjustment(id: number) {
  void run(async () => {
    await affiliatePartnersAPI.deleteAdjustment(props.partnerId, id)
    await load()
    emit('changed')
  })
}

watch(
  () => [props.show, props.month] as const,
  ([show]) => {
    if (!show) return
    detail.value = null
    paying.value = false
    adjustment.amount = ''
    adjustment.note = ''
    void load()
  },
  { immediate: true }
)

async function exportCsv() {
  if (exporting.value) return
  exporting.value = true
  try {
    const blob = await affiliatePartnersAPI.exportStatement(props.partnerId, props.month)
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `partner-${props.partnerId}-${props.month}.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.unknownError')))
  } finally {
    exporting.value = false
  }
}
</script>
