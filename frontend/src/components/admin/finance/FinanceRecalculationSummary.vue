<template>
  <div class="space-y-4 text-sm">
    <div class="flex flex-wrap items-center gap-3">
      <strong>#{{ batch.id }} · {{ t(`${ns}.kinds.${batch.kind}`) }}</strong>
      <span class="badge badge-gray" role="status">{{ t(`${ns}.statuses.${batch.status}`) }}</span>
    </div>
    <p v-if="notice" :role="notice.alert ? 'alert' : undefined" :class="['break-words rounded-lg p-3', notice.tone]">{{ notice.text }}</p>
    <p class="break-words text-gray-700 dark:text-gray-300">{{ scopeText }}</p>
    <div v-if="batch.kind === 'usage_cost' && batch.accept_retained_baseline" data-testid="retained-baseline-summary" class="rounded-lg bg-amber-50 p-3 text-amber-900 dark:bg-amber-900/20 dark:text-amber-200">
      <strong>{{ t(`${ns}.baseline.accepted`) }}</strong>
      <p class="mt-1">{{ t(`${ns}.baseline.condition`) }}</p>
    </div>
    <template v-if="batch.scanned > 0 || batch.status === 'ready'">
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div v-for="key in countKeys" :key="key" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
          <div class="text-gray-500 dark:text-gray-400">{{ t(`${ns}.counts.${key}`) }}</div><strong>{{ batch[key] }}</strong>
        </div>
      </div>
      <template v-if="batch.changed + batch.unchanged > 0">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div v-for="key in moneyKeys" :key="key"><span class="text-gray-500">{{ t(`${ns}.${key}`) }}</span><strong class="ml-2 break-all font-mono">{{ batch[key] }}</strong></div>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t(`${ns}.totalsHint`) }} · {{ t(`${ns}.units.${batch.kind}`) }}</p>
      </template>
      <p v-else class="text-gray-500 dark:text-gray-400">{{ t(`${ns}.noComparableTotals`) }}</p>
    </template>
    <p class="text-xs text-gray-500 dark:text-gray-400">
      {{ t(`${ns}.actor`) }} #{{ batch.actor_id }} · {{ t(`${ns}.cutoff`) }} {{ time(batch.cutoff_at) }}<template v-if="['previewing', 'ready'].includes(batch.status)"> · {{ t(`${ns}.expires`) }} {{ time(batch.expires_at) }}</template>
    </p>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RecalculationBatch } from '@/api/admin/financeRecalculation'
const props = defineProps<{ batch: RecalculationBatch; accountName?: string; providerName?: string }>()
const { t, te, locale } = useI18n()
const ns = 'admin.financeRecalculation'
const countKeys = ['scanned', 'changed', 'unchanged', 'skipped'] as const
const moneyKeys = ['old_total', 'new_total', 'delta'] as const
const scopeText = computed(() => {
  const b = props.batch
  const parts = [`${b.start} — ${b.end} (${t(`${ns}.beijing`)})`]
  if (b.account_id) parts.push(`${t(`${ns}.account`)} ${props.accountName ?? `#${b.account_id}`}`)
  if (b.model) parts.push(`${t(`${ns}.model`)} ${b.model}`)
  if (b.provider_id) parts.push(`${t(`${ns}.provider`)} ${props.providerName ?? `#${b.provider_id}`}`)
  return parts.join(' · ')
})
// One plain-language line per state; error codes are translated, never shown raw.
const notice = computed(() => {
  const b = props.batch
  if (b.error_code) {
    const key = `${ns}.errors.${b.error_code}`
    return { text: te(key) ? t(key) : b.error_message || t(`${ns}.errors.recalculation_failed`), tone: 'bg-red-50 text-red-700 dark:bg-red-950/20 dark:text-red-300', alert: true }
  }
  if (['queued', 'applying', 'syncing'].includes(b.status)) return { text: t(`${ns}.syncWarning`), tone: 'bg-amber-50 text-amber-800 dark:bg-amber-900/20 dark:text-amber-200', alert: false }
  if (b.status === 'completed') return { text: t(`${ns}.completed`), tone: 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300', alert: false }
  if (b.status === 'cancelled') return { text: t(`${ns}.cancelled`), tone: 'bg-gray-50 text-gray-600 dark:bg-dark-800 dark:text-gray-300', alert: false }
  return null
})
function time(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString(locale.value, { timeZone: 'Asia/Shanghai', hour12: false })
}
</script>
