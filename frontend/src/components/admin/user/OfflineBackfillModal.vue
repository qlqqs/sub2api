<template>
  <!-- CUSTOM(fork): Receipt-only backfill, separate from the balance mutation dialog. -->
  <BaseDialog :show="true" :title="t('admin.users.offlineRecharge.backfill')" width="narrow" @close="emit('close')">
    <form id="offline-backfill-form" class="space-y-4" @submit.prevent="submit">
      <p class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">{{ t('admin.users.offlineRecharge.backfillHint') }}</p>
      <p class="text-sm text-gray-700 dark:text-gray-200">{{ t('admin.users.offlineRecharge.credited') }}: ¥{{ item.value.toFixed(2) }}</p>
      <OfflineReceiptFields id-prefix="backfill" v-model:paid="form.paid_amount" v-model:channel="form.offline_channel" v-model:trade="form.trade_no" />
      <div><label for="backfill-notes" class="input-label">{{ t('admin.users.notes') }}</label><textarea id="backfill-notes" v-model="form.notes" maxlength="10000" rows="3" class="input" /></div>
    </form>
    <template #footer><div class="flex justify-end gap-3">
      <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button type="submit" form="offline-backfill-form" class="btn btn-primary" :disabled="busy || !valid">{{ t(busy ? 'common.saving' : 'common.confirm') }}</button>
    </div></template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type BalanceHistoryItem } from '@/api/admin'
import type { OfflineReceipt } from '@/api/admin/users'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import OfflineReceiptFields from './OfflineReceiptFields.vue'
const props = defineProps<{ userId: number; item: BalanceHistoryItem }>()
const emit = defineEmits<{ close: []; success: [] }>()
const { t } = useI18n()
const app = useAppStore()
const busy = ref(false)
const form = reactive<OfflineReceipt & { notes: string }>({ paid_amount: 0, offline_channel: '', trade_no: '', notes: '' })
const valid = computed(() => Number.isFinite(form.paid_amount) && form.paid_amount > 0 && form.paid_amount <= 100000000 && Math.abs(form.paid_amount * 100 - Math.round(form.paid_amount * 100)) < 0.000001 && !!form.offline_channel)
async function submit() {
  if (busy.value || !valid.value) return
  busy.value = true
  try {
    await adminAPI.users.backfillOfflineRecharge(props.userId, props.item.id, { ...form, trade_no: form.trade_no.trim() })
    app.showSuccess(t('common.success'))
    emit('success')
  } catch (error: unknown) { app.showError(extractI18nErrorMessage(error, t, 'payment.errors', t('common.error'))) }
  finally { busy.value = false }
}
</script>
