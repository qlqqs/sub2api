<template>
  <BaseDialog :show="show" :title="operation === 'add' ? t('admin.users.deposit') : t('admin.users.withdraw')" width="narrow" @close="$emit('close')">
    <form v-if="user" id="balance-form" @submit.prevent="handleBalanceSubmit" class="space-y-5">
      <div class="flex items-center gap-3 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-primary-100"><span class="text-lg font-medium text-primary-700">{{ user.email.charAt(0).toUpperCase() }}</span></div>
        <div class="flex-1"><p class="font-medium text-gray-900 dark:text-gray-100">{{ user.email }}</p><p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.users.currentBalance') }}: {{ BALANCE_SYMBOL }}{{ formatBalance(user.balance) }}</p></div>
      </div>
      <!-- CUSTOM(fork): No default; an administrator must classify every new credit. -->
      <fieldset v-if="operation === 'add'" class="space-y-2">
        <legend class="input-label">{{ t('admin.users.offlineRecharge.kind') }}</legend>
        <div class="flex flex-wrap gap-4">
          <label v-for="kind in (['gift', 'offline'] as const)" :key="kind" class="flex items-center gap-2 text-sm text-gray-800 dark:text-gray-200">
            <input v-model="form.kind" type="radio" name="balance-kind" :value="kind" />
            {{ t(`admin.users.offlineRecharge.${kind}`) }}
          </label>
        </div>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t(`admin.users.offlineRecharge.${form.kind ? form.kind + 'Hint' : 'chooseKind'}`) }}</p>
      </fieldset>
      <OfflineReceiptFields v-if="isOffline" id-prefix="balance" v-model:paid="form.paid_amount" v-model:channel="form.offline_channel" v-model:trade="form.trade_no" />
      <div>
        <label for="balance-amount" class="input-label">{{ isOffline ? t('admin.users.offlineRecharge.credited') : operation === 'add' ? t('admin.users.depositAmount') : t('admin.users.withdrawAmount') }}</label>
        <div class="relative flex gap-2">
          <div class="relative flex-1"><div class="absolute left-3 top-1/2 -translate-y-1/2 font-medium text-gray-500">{{ BALANCE_SYMBOL }}</div><input id="balance-amount" v-model.number="form.amount" type="number" :step="isOffline ? '0.01' : 'any'" :max="isOffline ? 100000000 : undefined" min="0" required class="input pl-8" /></div>
          <button v-if="operation === 'subtract'" type="button" @click="fillAllBalance" class="btn btn-secondary whitespace-nowrap">{{ t('admin.users.withdrawAll') }}</button>
        </div>
      </div>
      <div><label for="balance-notes" class="input-label">{{ t('admin.users.notes') }}</label><textarea id="balance-notes" maxlength="10000" v-model="form.notes" rows="3" class="input"></textarea></div>
      <p v-if="isOffline && validReceipt && form.amount > 0" class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.users.offlineRecharge.preview', { principal: Math.min(form.amount, form.paid_amount).toFixed(2), bonus: Math.max(0, form.amount - form.paid_amount).toFixed(2), balance: calculateNewBalance().toFixed(2) }) }}</p>
      <div v-if="form.amount > 0 && !isOffline" class="rounded-xl border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-950"><div class="flex items-center justify-between text-sm"><span class="text-gray-700 dark:text-gray-300">{{ t('admin.users.newBalance') }}:</span><span class="font-bold text-gray-900 dark:text-gray-100">{{ BALANCE_SYMBOL }}{{ formatBalance(calculateNewBalance()) }}</span></div></div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button type="submit" form="balance-form" :disabled="submitting || !validForm" class="btn" :class="operation === 'add' ? 'bg-emerald-600 text-white' : 'btn-danger'">{{ submitting ? t('common.saving') : t('common.confirm') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { BALANCE_SYMBOL } from '@/utils/balanceCurrency'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { AdminUser } from '@/types'
import OfflineReceiptFields from './OfflineReceiptFields.vue'
import type { OfflineReceipt } from '@/api/admin/users'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{ show: boolean, user: AdminUser | null, operation: 'add' | 'subtract' }>()
const emit = defineEmits(['close', 'success']); const { t } = useI18n(); const appStore = useAppStore()

const submitting = ref(false)
const freshForm = () => ({ amount: 0, notes: '', kind: '' as '' | 'gift' | 'offline', paid_amount: 0, offline_channel: '' as OfflineReceipt['offline_channel'], trade_no: '' })
const form = reactive(freshForm())
watch(() => [props.show, props.user?.id, props.operation], () => { Object.assign(form, freshForm()) })
const isOffline = computed(() => props.operation === 'add' && form.kind === 'offline')
const validMoney = (v: number) => Number.isFinite(v) && v > 0 && v <= 100000000 && Math.abs(v * 100 - Math.round(v * 100)) < 0.000001
const validReceipt = computed(() => validMoney(form.paid_amount) && !!form.offline_channel)
const validForm = computed(() => Number.isFinite(form.amount) && form.amount > 0 &&
  (props.operation !== 'add' || !!form.kind) && (!isOffline.value || (validReceipt.value && validMoney(form.amount))))

// 格式化余额：显示完整精度，去除尾部多余的0
const formatBalance = (value: number) => {
  if (value === 0) return '0.00'
  // 最多保留8位小数，去除尾部的0
  const formatted = value.toFixed(8).replace(/\.?0+$/, '')
  // 确保至少有2位小数
  const parts = formatted.split('.')
  if (parts.length === 1) return formatted + '.00'
  if (parts[1].length === 1) return formatted + '0'
  return formatted
}

// 填入全部余额
const fillAllBalance = () => {
  if (props.user) {
    form.amount = props.user.balance
  }
}

const calculateNewBalance = () => {
  if (!props.user) return 0
  const result = props.operation === 'add' ? props.user.balance + form.amount : props.user.balance - form.amount
  // 避免浮点数精度问题导致的 -0.00 显示
  return Math.abs(result) < 1e-10 ? 0 : result
}
const handleBalanceSubmit = async () => {
  if (!props.user || submitting.value || !validForm.value) return
  if (!form.amount || form.amount <= 0) {
    appStore.showError(t('admin.users.amountRequired'))
    return
  }
  // 退款时验证金额不超过实际余额
  if (props.operation === 'subtract' && form.amount > props.user.balance) {
    appStore.showError(t('admin.users.insufficientBalance'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.users.updateBalance(props.user.id, form.amount, props.operation, form.notes, isOffline.value ? { kind: 'offline', paid_amount: form.paid_amount, offline_channel: form.offline_channel, trade_no: form.trade_no.trim() } : { kind: 'gift' })
    appStore.showSuccess(t('common.success')); emit('success'); emit('close')
  } catch (e: unknown) {
    console.error('Failed to update balance:', e)
    appStore.showError(extractI18nErrorMessage(e, t, 'payment.errors', t('common.error')))
  } finally { submitting.value = false }
}
</script>
