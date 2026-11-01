<template>
  <DataTable
    :key="`${sort}:${order}`"
    :columns="columns"
    :data="rows"
    :loading="loading"
    :row-key="rowKey"
    :default-sort-key="sort"
    :default-sort-order="order"
    server-side-sort
    clickable-rows
    sticky-first-column
    always-table
    data-test="team-customer-table"
    @sort="(key: string, next: 'asc' | 'desc') => emit('sort', key, next)"
    @row-click="openRow"
  >
    <template #cell-user="{ row }">
      <TeamUserCell :user="row.user" :to="row.user.deleted ? undefined : customerRoute(row)">
        <TeamBadge v-if="row.agent_level === 2">{{ t('affiliate.team.subAgent') }}</TeamBadge>
      </TeamUserCell>
    </template>
    <template #cell-line_agent="{ row }">
      <span :class="mutedClass(row) || 'text-gray-600 dark:text-gray-300'">
        {{ row.line_agent ? teamUserName(row.line_agent, label('affiliate.team.deleted', { id: row.line_agent.user_id })) : t('partner.region.directTeam') }}
      </span>
    </template>
    <template #cell-owner="{ row }">
      <span :class="mutedClass(row) || 'text-gray-600 dark:text-gray-300'">
        {{ row.owner ? teamUserName(row.owner, label('affiliate.team.deleted', { id: row.owner.user_id })) : '-' }}
      </span>
    </template>
    <template #cell-cost="{ row }">
      <TeamShareCell :value="formatTeamMoney(row.cost)" :share="row.cost_share" :muted="row.user.deleted" />
    </template>
    <template #cell-requests="{ row }">
      <span :class="['tabular-nums', mutedClass(row)]">{{ formatTeamCount(row.requests) }}</span>
    </template>
    <template #cell-earnings="{ row }">
      <span :class="['tabular-nums', mutedClass(row)]">{{ formatTeamMoney(row.earnings) }}</span>
    </template>
    <template #cell-last_used_at="{ row }">
      <span
        v-if="row.last_used_at"
        :class="mutedClass(row) || 'text-gray-600 dark:text-gray-300'"
        :title="formatDateTime(row.last_used_at)"
      >
        {{ formatRelativeTime(row.last_used_at) }}
      </span>
      <span v-else class="text-gray-400 dark:text-dark-500">-</span>
    </template>
    <template #empty>
      <slot name="empty" />
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import type { AffiliateTeamCustomer, AffiliateTeamSortOrder } from '@/types'
import { formatTeamCount, formatTeamMoney, teamUserName } from '@/utils/affiliateTeam'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import TeamBadge from './TeamBadge.vue'
import TeamShareCell from './TeamShareCell.vue'
import TeamUserCell from './TeamUserCell.vue'
import { useAffiliateTeamContext, useTeamLabel } from './teamContext'

const props = defineProps<{
  rows: AffiliateTeamCustomer[]
  loading?: boolean
  sort: string
  order: AffiliateTeamSortOrder
  /** 下钻只携带时间范围。 */
  drillQuery: Record<string, string>
}>()

const emit = defineEmits<{
  sort: [key: string, order: AffiliateTeamSortOrder]
}>()

const { t } = useI18n()
const router = useRouter()
const context = useAffiliateTeamContext()
const label = useTeamLabel(context)

// 合伙人的区域视角：加「所属渠道经理 / 归属代理」，不显示收益。
const columns = computed<Column[]>(() => [
  { key: 'user', label: t('affiliate.team.columns.customer') },
  ...(context.region
    ? [
        { key: 'line_agent', label: t('partner.region.columns.lineAgent') },
        { key: 'owner', label: t('partner.region.columns.owner') }
      ]
    : []),
  { key: 'cost', label: t('affiliate.team.columns.cost'), sortable: true, class: 'text-right' },
  { key: 'requests', label: t('affiliate.team.columns.requests'), sortable: true, class: 'text-right' },
  ...(context.region ? [] : [{ key: 'earnings', label: label('affiliate.team.columns.earnings'), sortable: true, class: 'text-right' }]),
  { key: 'last_used_at', label: t('affiliate.team.columns.lastUsed'), sortable: true, class: 'text-right' }
])

const rowKey = (row: AffiliateTeamCustomer) => row.user.user_id

function customerRoute(row: AffiliateTeamCustomer): RouteLocationRaw {
  return context.routes.customer(row.user.user_id, props.drillQuery)
}

function mutedClass(row: AffiliateTeamCustomer): string {
  return row.user.deleted ? 'text-gray-400 dark:text-dark-500' : ''
}

function openRow(row: AffiliateTeamCustomer) {
  if (row.user.deleted) return
  void router.push(customerRoute(row))
}
</script>
