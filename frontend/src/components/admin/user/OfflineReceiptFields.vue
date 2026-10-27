<template>
  <!-- CUSTOM(fork): Shared receipt inputs for new credits and historical backfill. -->
  <div class="space-y-4">
    <div>
      <label :for="`${idPrefix}-paid`" class="input-label">{{ t('admin.users.offlineRecharge.paid') }}</label>
      <input :id="`${idPrefix}-paid`" v-model.number="paid" type="number" min="0.01" max="100000000" step="0.01" required class="input" />
    </div>
    <div>
      <label :for="`${idPrefix}-channel`" class="input-label">{{ t('admin.users.offlineRecharge.channel') }}</label>
      <select :id="`${idPrefix}-channel`" v-model="channel" required class="input">
        <option disabled value="">{{ t('admin.users.offlineRecharge.chooseChannel') }}</option>
        <option v-for="item in channels" :key="item" :value="item">{{ t(`admin.users.offlineRecharge.${item}`) }}</option>
      </select>
    </div>
    <div>
      <label :for="`${idPrefix}-trade`" class="input-label">{{ t('admin.users.offlineRecharge.trade') }}</label>
      <input :id="`${idPrefix}-trade`" v-model="trade" maxlength="128" class="input" />
    </div>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { OfflineReceipt } from '@/api/admin/users'
defineProps<{ idPrefix: string }>()
const paid = defineModel<number>('paid', { required: true })
const channel = defineModel<OfflineReceipt['offline_channel']>('channel', { required: true })
const trade = defineModel<string>('trade', { required: true })
const { t } = useI18n()
const channels = ['wechat', 'alipay', 'bank', 'other'] as const
</script>
