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
    data-test="team-agent-table"
    @sort="(key: string, next: 'asc' | 'desc') => emit('sort', key, next)"
    @row-click="openRow"
  >
    <template #cell-user="{ row }">
      <TeamUserCell :user="row.user" :to="row.user.deleted ? undefined : agentRoute(row)">
        <TeamBadge v-if="row.historical" tone="gray" :hint="label('affiliate.team.historicalHint')">
          {{ t('affiliate.team.historical') }}
        </TeamBadge>
      </TeamUserCell>
    </template>
    <template #cell-active_customers="{ row }">
      <span class="tabular-nums">
        {{ formatTeamCount(row.active_customers) }}<span class="text-gray-400 dark:text-dark-500"> / {{ formatTeamCount(row.customers) }}</span>
      </span>
    </template>
    <template #cell-team_cost="{ row }">
      <TeamShareCell :value="formatTeamMoney(row.team_cost)" :share="row.cost_share" :muted="row.user.deleted" />
    </template>
    <template #cell-earnings="{ row }">
      <span class="tabular-nums">{{ formatTeamMoney(row.earnings) }}</span>
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
import type { AffiliateTeamAgent, AffiliateTeamSortOrder } from '@/types'
import { formatTeamCount, formatTeamMoney } from '@/utils/affiliateTeam'
import TeamBadge from './TeamBadge.vue'
import TeamShareCell from './TeamShareCell.vue'
import TeamUserCell from './TeamUserCell.vue'
import { useAffiliateTeamContext, useTeamLabel } from './teamContext'

const props = defineProps<{
  rows: AffiliateTeamAgent[]
  loading?: boolean
  sort: string
  order: AffiliateTeamSortOrder
  drillQuery: Record<string, string>
}>()

const emit = defineEmits<{
  sort: [key: string, order: AffiliateTeamSortOrder]
}>()

const { t } = useI18n()
const router = useRouter()
const context = useAffiliateTeamContext()
const label = useTeamLabel(context)

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('affiliate.team.columns.agent') },
  { key: 'active_customers', label: t('affiliate.team.columns.activeCustomers'), sortable: true, class: 'text-right' },
  { key: 'team_cost', label: t('affiliate.team.columns.teamCost'), sortable: true, class: 'text-right' },
  { key: 'earnings', label: label('affiliate.team.columns.earnings'), sortable: true, class: 'text-right' }
])

const rowKey = (row: AffiliateTeamAgent) => row.user.user_id

function agentRoute(row: AffiliateTeamAgent): RouteLocationRaw {
  return context.routes.agent(row.user.user_id, props.drillQuery)
}

function openRow(row: AffiliateTeamAgent) {
  if (row.user.deleted) return
  void router.push(agentRoute(row))
}
</script>
