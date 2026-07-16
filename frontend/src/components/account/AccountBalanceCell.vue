<!-- CUSTOM: Displays transient upstream balance query state without persistence. -->
<template>
  <div class="min-w-[6.67rem] text-xs">
    <span v-if="accountType !== 'upstream'" class="text-gray-400 dark:text-gray-500">
      {{ t('admin.accounts.upstreamBalance.notApplicable') }}
    </span>

    <template v-else>
      <div class="flex items-start justify-between gap-1.5">
        <div class="min-w-0 space-y-1">
          <div v-if="displayedResult" class="space-y-0.5">
            <div class="font-medium text-gray-900 dark:text-white">
              {{ scopeLabel(displayedResult.scope) }}:
              {{ formatBalance(displayedResult.remaining, displayedResult.unit) }}
            </div>
            <div v-if="displayedResult.api_key_rate" class="text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.upstreamBalance.apiKeyRate') }}: {{ displayedResult.api_key_rate }}x
            </div>
          </div>

          <div
            v-if="state.phase === 'idle'"
            class="text-gray-500 dark:text-gray-400"
          >
            {{ t('admin.accounts.upstreamBalance.notQueried') }}
          </div>
          <div
            v-else-if="state.phase === 'loading'"
            class="sr-only"
            role="status"
          >
            {{ t('admin.accounts.upstreamBalance.loading') }}
          </div>
          <div
            v-else-if="state.phase === 'unsupported'"
            class="text-amber-600 dark:text-amber-400"
          >
            {{ t('admin.accounts.upstreamBalance.unsupported') }}
          </div>
          <div
            v-else-if="state.phase === 'failed'"
            class="text-red-600 dark:text-red-400"
            role="alert"
          >
            {{ state.error_message || t('admin.accounts.upstreamBalance.errors.fallback') }}
          </div>
        </div>

        <div class="flex shrink-0 items-center gap-0.5">
          <button
            type="button"
            class="flex h-5 w-5 items-center justify-center rounded-full text-gray-400 transition-colors hover:bg-gray-100 hover:text-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-1 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-500 dark:hover:bg-gray-700 dark:hover:text-primary-400"
            :disabled="state.phase === 'loading'"
            :aria-label="refreshButtonLabel"
            :aria-busy="state.phase === 'loading'"
            :title="refreshButtonLabel"
            @click="$emit('refresh')"
          >
            <svg
              class="h-4 w-4"
              :class="state.phase === 'loading' ? 'animate-spin' : ''"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              aria-hidden="true"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M20 11a8.1 8.1 0 0 0-15.5-2M4 5v4h4" />
              <path stroke-linecap="round" stroke-linejoin="round" d="M4 13a8.1 8.1 0 0 0 15.5 2M20 19v-4h-4" />
            </svg>
          </button>
          <span
            v-if="displayedResult"
            class="flex h-5 w-5 items-center justify-center text-gray-400 dark:text-gray-500"
            role="img"
            tabindex="0"
            :aria-label="queriedAtLabel(displayedResult.queried_at)"
            :title="queriedAtLabel(displayedResult.queried_at)"
          >
            <svg
              class="h-3.5 w-3.5"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              aria-hidden="true"
            >
              <circle cx="12" cy="12" r="8.5" />
              <path stroke-linecap="round" d="M12 7.5V12l3 2" />
            </svg>
          </span>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  AccountBalanceRowState,
  AccountType,
  UpstreamBalanceResult,
  UpstreamBalanceScope
} from '@/types'

const props = defineProps<{
  accountType: AccountType
  state: AccountBalanceRowState
}>()

defineEmits<{
  refresh: []
}>()

const { t, locale } = useI18n()

const displayedResult = computed<UpstreamBalanceResult | undefined>(() => {
  if (props.state.phase === 'unsupported') return undefined
  return props.state.latest_result ?? props.state.last_successful_result
})

const refreshButtonLabel = computed(() =>
  props.state.phase === 'loading'
    ? t('admin.accounts.upstreamBalance.loading')
    : t('admin.accounts.upstreamBalance.refresh')
)

function scopeLabel(scope: UpstreamBalanceScope): string {
  switch (scope) {
    case 'user':
      return t('admin.accounts.upstreamBalance.scope.user')
    case 'api_key':
      return t('admin.accounts.upstreamBalance.scope.apiKey')
    default:
      return t('admin.accounts.upstreamBalance.scope.unknown')
  }
}

function formatQueriedAt(queriedAt: string): string {
  const date = new Date(queriedAt)
  if (Number.isNaN(date.getTime())) return queriedAt
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'short',
    timeStyle: 'medium'
  }).format(date)
}

function queriedAtLabel(queriedAt: string): string {
  return `${t('admin.accounts.upstreamBalance.queriedAt')}: ${formatQueriedAt(queriedAt)}`
}

function formatFixedTwoDecimalPlaces(decimalValue: string): string {
  const normalizedValue = decimalValue.trim()
  const decimalParts = normalizedValue.match(/^([+-]?)(\d*)(?:\.(\d*))?$/)
  const hasDigits = decimalParts && (decimalParts[2] !== '' || decimalParts[3] !== '')
  if (!decimalParts || !hasDigits) return decimalValue

  const sign = decimalParts[1] === '-' ? '-' : ''
  const integerPart = (decimalParts[2] || '0').replace(/^0+(?=\d)/, '')
  const fractionalPart = decimalParts[3] || ''
  const truncatedFraction = fractionalPart.slice(0, 2).padEnd(2, '0')
  const shouldRoundUp = Number(fractionalPart[2] || '0') >= 5
  const absoluteHundredths = BigInt(`${integerPart}${truncatedFraction}`) + (shouldRoundUp ? 1n : 0n)
  const absoluteDigits = absoluteHundredths.toString().padStart(3, '0')
  const formattedInteger = absoluteDigits.slice(0, -2)
  const formattedFraction = absoluteDigits.slice(-2)
  const formattedSign = sign && absoluteHundredths !== 0n ? sign : ''

  return `${formattedSign}${formattedInteger}.${formattedFraction}`
}

function formatBalance(remaining: string | undefined, unit: string | undefined): string {
  if (remaining === undefined || remaining === '') return '-'

  const formattedBalance = formatFixedTwoDecimalPlaces(remaining)

  if (unit?.toUpperCase() === 'USD') return `$${formattedBalance}`
  return unit ? `${formattedBalance} ${unit}` : formattedBalance
}
</script>
