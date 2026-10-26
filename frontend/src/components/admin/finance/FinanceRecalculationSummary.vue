<template>
  <div class="space-y-4 text-sm">
    <div class="flex flex-wrap items-center gap-3">
      <strong>#{{ batch.id }} · {{ t(`${ns}.kinds.${batch.kind}`) }}</strong>
      <span class="badge badge-gray" role="status">{{ t(`${ns}.statuses.${batch.status}`) }}</span>
    </div>
    <p v-if="['queued', 'applying', 'syncing'].includes(batch.status)" class="rounded-lg bg-amber-50 p-3 text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">{{ t(`${ns}.syncWarning`) }}</p>
    <p v-if="batch.status === 'completed'" class="text-green-700 dark:text-green-400">{{ t(`${ns}.completed`) }}</p>
    <p v-if="batch.error_code || batch.error_message" role="alert" class="break-words text-red-600 dark:text-red-400">{{ batch.error_code }} · {{ batch.error_message }}</p>
    <dl class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <div><dt class="text-gray-500">{{ t(`${ns}.scope`) }}</dt><dd>{{ batch.start }} — {{ batch.end }} ({{ t(`${ns}.beijing`) }})</dd></div>
      <div><dt class="text-gray-500">{{ t(`${ns}.actor`) }}</dt><dd>#{{ batch.actor_id }}</dd></div>
      <div v-if="batch.account_id"><dt class="text-gray-500">{{ t(`${ns}.account`) }}</dt><dd>#{{ batch.account_id }}</dd></div>
      <div v-if="batch.provider_id"><dt class="text-gray-500">{{ t(`${ns}.provider`) }}</dt><dd>#{{ batch.provider_id }}</dd></div>
      <div v-if="batch.model"><dt class="text-gray-500">{{ t(`${ns}.model`) }}</dt><dd class="break-all">{{ batch.model }}</dd></div>
      <div><dt class="text-gray-500">{{ t(`${ns}.cutoff`) }}</dt><dd>{{ time(batch.cutoff_at) }}</dd></div>
      <div><dt class="text-gray-500">{{ t(`${ns}.expires`) }}</dt><dd>{{ time(batch.expires_at) }}</dd></div>
      <div class="sm:col-span-2"><dt class="text-gray-500">{{ t(`${ns}.snapshot`) }}</dt><dd class="break-all font-mono text-xs">{{ batch.config_fingerprint || '—' }}</dd></div>
    </dl>
    <div v-if="batch.kind === 'usage_cost' && batch.accept_retained_baseline" data-testid="retained-baseline-summary" class="space-y-2 rounded-lg bg-amber-50 p-3 text-amber-900 dark:bg-amber-900/20 dark:text-amber-200">
      <strong>{{ t(`${ns}.baseline.accepted`) }}</strong>
      <p>{{ t(`${ns}.baseline.condition`) }}</p>
      <p>{{ t(`${ns}.baseline.backup`) }}</p>
    </div>
    <ul v-if="batch.snapshot_summary?.length" class="list-inside list-disc break-words"><li v-for="source in batch.snapshot_summary" :key="source">{{ source }}</li></ul>
    <p v-else-if="batch.status !== 'previewing'" class="text-gray-500 dark:text-gray-400">{{ t(`${ns}.sourceHint`) }}</p>
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
      <div v-for="key in countKeys" :key="key" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
        <div class="text-gray-500 dark:text-gray-400">{{ t(`${ns}.counts.${key}`) }}</div><strong>{{ batch[key] }}</strong>
      </div>
    </div>
    <template v-if="batch.status !== 'previewing' && batch.changed + batch.unchanged > 0">
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t(`${ns}.totalsHint`) }} · {{ t(`${ns}.units.${batch.kind}`) }}</p>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div v-for="key in moneyKeys" :key="key"><span class="text-gray-500">{{ t(`${ns}.${key}`) }}</span><strong class="ml-2 break-all font-mono">{{ batch[key] }}</strong></div>
      </div>
    </template>
    <p v-else class="text-gray-500 dark:text-gray-400">{{ t(`${ns}.noComparableTotals`) }}</p>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { RecalculationBatch } from '@/api/admin/financeRecalculation'
defineProps<{ batch: RecalculationBatch }>()
const { t, locale } = useI18n()
const ns = 'admin.financeRecalculation'
const countKeys = ['scanned', 'changed', 'unchanged', 'skipped'] as const
const moneyKeys = ['old_total', 'new_total', 'delta'] as const
function time(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString(locale.value, { timeZone: 'Asia/Shanghai', hour12: false })
}
</script>
