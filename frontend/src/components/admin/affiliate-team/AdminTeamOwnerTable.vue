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
    data-test="team-owner-table"
    @sort="(key: string, next: 'asc' | 'desc') => emit('sort', key, next)"
    @row-click="openRow"
  >
    <template #cell-user="{ row }">
      <!-- 已删除的代理名下仍有客户，照样可以点进去 -->
      <RouterLink
        v-if="row.user.deleted"
        :to="viewerRoute(row)"
        class="text-gray-400 transition-colors hover:text-primary-600 dark:text-dark-500 dark:hover:text-primary-400"
        data-test="team-owner-deleted"
        @click.stop
      >
        {{ t('admin.affiliates.team.labels.deleted', { id: row.user.user_id }) }}
      </RouterLink>
      <TeamUserCell v-else :user="row.user" :to="viewerRoute(row)" />
    </template>
    <template #cell-agent_level="{ row }">
      <span class="whitespace-nowrap text-gray-600 dark:text-gray-300" data-test="team-owner-level">{{ levelText(row) }}</span>
    </template>
    <template #cell-active_customers="{ row }">
      <span class="tabular-nums">
        {{ formatTeamCount(row.active_customers) }}<span class="text-gray-400 dark:text-dark-500"> / {{ formatTeamCount(row.customers) }}</span>
      </span>
    </template>
    <template #cell-team_cost="{ row }">
      <TeamShareCell :value="formatTeamMoney(row.team_cost)" :share="row.cost_share" :title="formatTeamShare(row.cost_share)" />
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
import TeamShareCell from '@/components/user/affiliate-team/TeamShareCell.vue'
import TeamUserCell from '@/components/user/affiliate-team/TeamUserCell.vue'
import type { AffiliateTeamOwner, AffiliateTeamSortOrder } from '@/types'
import { formatTeamCount, formatTeamMoney, formatTeamShare, teamUserName } from '@/utils/affiliateTeam'

/** 汇总页的代理表：一行一个有团队的人，点击进入该代理的视角。 */
const props = defineProps<{
  rows: AffiliateTeamOwner[]
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

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.affiliates.team.columns.agent') },
  { key: 'agent_level', label: t('admin.affiliates.team.columns.level') },
  { key: 'active_customers', label: t('affiliate.team.columns.activeCustomers'), sortable: true, class: 'text-right' },
  { key: 'team_cost', label: t('affiliate.team.columns.teamCost'), sortable: true, class: 'text-right' },
  { key: 'earnings', label: t('admin.affiliates.team.labels.earnings'), sortable: true, class: 'text-right' }
])

const rowKey = (row: AffiliateTeamOwner) => row.user.user_id

function viewerRoute(row: AffiliateTeamOwner): RouteLocationRaw {
  return { name: 'AdminAffiliateTeamViewer', params: { viewerId: String(row.user.user_id) }, query: props.drillQuery }
}

/** 一级 / 二级 · 上级（无昵称时为邮箱）/ 已降级。 */
function levelText(row: AffiliateTeamOwner): string {
  if (row.agent_level === 1) return t('admin.affiliates.team.levels.level1')
  if (row.agent_level === 2) {
    const upline = row.upline
      ? teamUserName(row.upline, t('admin.affiliates.team.labels.deleted', { id: row.upline.user_id }))
      : t('admin.affiliates.team.levels.noUpline')
    return t('admin.affiliates.team.levels.level2With', { name: upline })
  }
  return t('admin.affiliates.team.levels.downgraded')
}

function openRow(row: AffiliateTeamOwner) {
  void router.push(viewerRoute(row))
}
</script>
