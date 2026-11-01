<template>
  <!-- CUSTOM: 合伙人区域视角的渠道经理线：每个挂靠的渠道经理一行，合伙人直属团队单列一行 -->
  <DataTable
    :columns="columns"
    :data="rows"
    :loading="loading"
    :row-key="rowKey"
    clickable-rows
    sticky-first-column
    always-table
    data-test="team-line-table"
    @row-click="openRow"
  >
    <template #cell-line="{ row }">
      <TeamUserCell v-if="row.line_agent" :user="row.line_agent" :to="lineRoute(row)" />
      <RouterLink
        v-else-if="directRoute"
        :to="directRoute"
        class="font-medium text-gray-900 transition-colors hover:text-primary-600 dark:text-white dark:hover:text-primary-400"
        data-test="team-line-direct"
        @click.stop
      >
        {{ t('partner.region.directTeam') }}
      </RouterLink>
      <span v-else class="font-medium text-gray-900 dark:text-white">{{ t('partner.region.directTeam') }}</span>
    </template>
    <template #cell-email="{ row }">
      <TeamEmailCell :user="row.line_agent" />
    </template>
    <template #cell-active_customers="{ row }">
      <span class="tabular-nums">
        {{ formatTeamCount(row.active_customers) }}<span class="text-gray-400 dark:text-dark-500"> / {{ formatTeamCount(row.customers) }}</span>
      </span>
    </template>
    <template #cell-requests="{ row }">
      <span class="tabular-nums">{{ formatTeamCount(row.requests) }}</span>
    </template>
    <template #cell-team_cost="{ row }">
      <TeamShareCell :value="formatTeamMoney(row.team_cost)" :share="row.cost_share" />
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
import type { PartnerTeamLine } from '@/api/affiliatePartner'
import { formatTeamCount, formatTeamMoney } from '@/utils/affiliateTeam'
import TeamShareCell from './TeamShareCell.vue'
import TeamEmailCell from './TeamEmailCell.vue'
import TeamUserCell from './TeamUserCell.vue'
import { useAffiliateTeamContext } from './teamContext'

const props = defineProps<{
  rows: PartnerTeamLine[]
  loading?: boolean
  /** 下钻只携带时间范围。 */
  drillQuery: Record<string, string>
}>()

const { t } = useI18n()
const router = useRouter()
const context = useAffiliateTeamContext()

const columns = computed<Column[]>(() => [
  { key: 'line', label: t('partner.lines.line') },
  { key: 'email', label: t('affiliate.team.columns.email') },
  { key: 'active_customers', label: t('affiliate.team.columns.activeCustomers'), class: 'text-right' },
  { key: 'requests', label: t('affiliate.team.columns.requests'), class: 'text-right' },
  { key: 'team_cost', label: t('affiliate.team.columns.teamCost'), class: 'text-right' }
])

const rowKey = (row: PartnerTeamLine) => row.line_agent?.user_id ?? 0

const directRoute = computed(() => context.routes.direct?.(props.drillQuery))

function lineRoute(row: PartnerTeamLine): RouteLocationRaw | undefined {
  if (!row.line_agent) return directRoute.value
  return row.line_agent.deleted ? undefined : context.routes.agent(row.line_agent.user_id, props.drillQuery)
}

function openRow(row: PartnerTeamLine) {
  const to = lineRoute(row)
  if (to) void router.push(to)
}
</script>
