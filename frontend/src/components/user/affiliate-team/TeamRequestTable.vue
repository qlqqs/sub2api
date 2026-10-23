<template>
  <DataTable
    :columns="columns"
    :data="rows"
    :loading="loading"
    sticky-first-column
    always-table
    data-test="team-request-table"
  >
    <template #cell-created_at="{ row }">
      <span class="tabular-nums text-gray-600 dark:text-gray-300" :title="formatDateTime(row.created_at)">
        {{ formatShortDateTime(row.created_at) }}
      </span>
    </template>
    <template #cell-model="{ row }">
      <span class="text-gray-900 dark:text-gray-100">{{ row.model || '-' }}</span>
    </template>
    <template #cell-group_name="{ row }">
      <span v-if="row.group_name" class="text-gray-700 dark:text-gray-300">{{ row.group_name }}</span>
      <span v-else class="text-gray-400 dark:text-dark-500">{{ t('affiliate.team.filters.noGroup') }}</span>
    </template>
    <template #cell-tokens="{ row }">
      <span v-if="row.image_count > 0" class="tabular-nums" data-test="team-request-images">
        {{ t('affiliate.team.tokens.images', row.image_count) }}
      </span>
      <span v-else-if="row.video_count > 0" class="tabular-nums" data-test="team-request-videos">
        {{ t('affiliate.team.tokens.videos', row.video_count) }}
      </span>
      <span
        v-else-if="tokenTotal(row) > 0"
        class="inline-flex cursor-help items-center gap-2.5 tabular-nums"
        :title="tokenTitle(row)"
        data-test="team-request-tokens"
      >
        <span v-if="row.input_tokens > 0" class="inline-flex items-center gap-0.5">
          <Icon name="arrowDown" size="xs" class="text-emerald-500" />
          <span class="text-gray-900 dark:text-white">{{ formatCacheTokens(row.input_tokens) }}</span>
        </span>
        <span v-if="row.output_tokens > 0" class="inline-flex items-center gap-0.5">
          <Icon name="arrowUp" size="xs" class="text-violet-500" />
          <span class="text-gray-900 dark:text-white">{{ formatCacheTokens(row.output_tokens) }}</span>
        </span>
        <span v-if="row.cache_read_tokens > 0" class="inline-flex items-center gap-0.5">
          <Icon name="database" size="xs" class="text-sky-500" />
          <span class="text-sky-600 dark:text-sky-400">{{ formatCacheTokens(row.cache_read_tokens) }}</span>
        </span>
        <span v-if="row.cache_creation_tokens > 0" class="inline-flex items-center gap-0.5">
          <Icon name="edit" size="xs" class="text-amber-500" />
          <span class="text-amber-600 dark:text-amber-400">{{ formatCacheTokens(row.cache_creation_tokens) }}</span>
        </span>
      </span>
      <span v-else class="text-gray-400 dark:text-dark-500">-</span>
    </template>
    <template #cell-cost="{ row }">
      <span class="font-medium tabular-nums">{{ formatTeamCharge(row.cost) }}</span>
    </template>
    <template #empty>
      <slot name="empty" />
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import type { AffiliateTeamRequest } from '@/types'
import { formatTeamCharge, formatTeamCount } from '@/utils/affiliateTeam'
import { formatDateTime } from '@/utils/format'
import { formatCacheTokens } from '@/utils/formatters'

defineProps<{
  rows: AffiliateTeamRequest[]
  loading?: boolean
}>()

const { t } = useI18n()

const columns = computed<Column[]>(() => [
  { key: 'created_at', label: t('affiliate.team.columns.time') },
  { key: 'model', label: t('affiliate.team.columns.model') },
  { key: 'group_name', label: t('affiliate.team.columns.group') },
  { key: 'tokens', label: t('affiliate.team.columns.tokens') },
  { key: 'cost', label: t('affiliate.team.columns.charge'), class: 'text-right' }
])

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

/** MM-DD HH:mm:ss（本地时间）；悬停显示完整时间。 */
function formatShortDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function tokenTotal(row: AffiliateTeamRequest): number {
  return row.input_tokens + row.output_tokens + row.cache_read_tokens + row.cache_creation_tokens
}

/** 悬停：为 0 的项省略，最后一行合计。 */
function tokenTitle(row: AffiliateTeamRequest): string {
  const parts: Array<[string, number]> = [
    [t('affiliate.team.tokens.input'), row.input_tokens],
    [t('affiliate.team.tokens.output'), row.output_tokens],
    [t('affiliate.team.tokens.cacheRead'), row.cache_read_tokens],
    [t('affiliate.team.tokens.cacheWrite'), row.cache_creation_tokens]
  ]
  return [
    ...parts.filter(([, value]) => value > 0).map(([label, value]) => `${label} ${formatTeamCount(value)}`),
    `${t('affiliate.team.tokens.total')} ${formatTeamCount(tokenTotal(row))}`
  ].join('\n')
}
</script>
