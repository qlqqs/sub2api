<template>
  <AppLayout>
    <div class="space-y-4">
      <!-- 标题已在顶部栏，这里只放日期与刷新 -->
      <TeamToolbar
        :crumbs="[]"
        :start-date="query.start"
        :end-date="query.end"
        :refreshing="refreshing"
        @range-change="onRangeChange"
        @refresh="reload"
      />

      <TeamSummaryStrip
        view="platform"
        :platform="overview?.summary ?? null"
        :settle-interval-minutes="overview?.settle_interval_minutes"
      />

      <div class="grid gap-4 lg:grid-cols-3">
        <TeamTrendChart class="lg:col-span-2" :trend="overview?.trend ?? []" :loading="!overview" />
        <TeamModelShare :models="overview?.models ?? []" :loading="!overview" />
      </div>

      <section class="card overflow-hidden" data-test="team-owner-list">
        <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
          <div role="tablist" class="inline-flex flex-wrap rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
            <button
              v-for="tab in levelTabs"
              :key="tab.key || 'all'"
              type="button"
              role="tab"
              :aria-selected="query.level === tab.key"
              :class="['tab px-3 py-1.5', query.level === tab.key && 'tab-active']"
              :data-test="`team-level-${tab.key || 'all'}`"
              @click="onLevelChange(tab.key)"
            >
              {{ tab.label }}
              <span v-if="overview" class="ml-1 tabular-nums text-gray-400 dark:text-dark-500">{{ formatTeamCount(tab.count) }}</span>
            </button>
          </div>
          <div class="w-full sm:w-64">
            <SearchInput
              v-model="searchDraft"
              :placeholder="t('admin.affiliates.team.labels.searchPlaceholder')"
              :debounce-ms="300"
              @search="onSearch"
            />
          </div>
        </div>

        <TeamProgressBar :active="refreshing && listReady" />

        <AdminTeamOwnerTable
          :rows="owners"
          :loading="!listReady"
          :sort="query.sort"
          :order="query.order"
          :drill-query="drillDownQuery"
          @sort="onSort"
        >
          <template #empty>
            <span class="text-sm text-gray-500 dark:text-dark-400" data-test="team-empty">
              {{ query.q ? t('admin.affiliates.team.empty.noMatches') : t('admin.affiliates.team.empty.noAgents') }}
            </span>
          </template>
        </AdminTeamOwnerTable>

        <Pagination
          v-if="listReady && total > 0"
          :total="total"
          :page="query.page"
          :page-size="query.size"
          @update:page="(page: number) => update({ page })"
          @update:pageSize="(size: number) => update({ size, page: 1 })"
        />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import AdminTeamOwnerTable from '@/components/admin/affiliate-team/AdminTeamOwnerTable.vue'
import { createAdminAffiliateTeamContext } from '@/components/admin/affiliate-team/adminTeamContext'
import TeamModelShare from '@/components/user/affiliate-team/TeamModelShare.vue'
import TeamProgressBar from '@/components/user/affiliate-team/TeamProgressBar.vue'
import TeamSummaryStrip from '@/components/user/affiliate-team/TeamSummaryStrip.vue'
import TeamToolbar from '@/components/user/affiliate-team/TeamToolbar.vue'
import TeamTrendChart from '@/components/user/affiliate-team/TeamTrendChart.vue'
import { provideAffiliateTeamContext } from '@/components/user/affiliate-team/teamContext'
import adminAffiliateTeamAPI from '@/api/admin/affiliateTeam'
import { AFFILIATE_TEAM_MAX_RANGE_DAYS, clampAffiliateTeamRange } from '@/composables/useAffiliateTeamQuery'
import { ADMIN_AFFILIATE_TEAM_SORTS, useAdminAffiliateTeamQuery } from '@/composables/useAdminAffiliateTeamQuery'
import { useAppStore } from '@/stores/app'
import type {
  AdminAffiliateTeamLevel,
  AffiliateTeamAgentSort,
  AffiliateTeamOwner,
  AffiliateTeamPlatformOverview,
  AffiliateTeamSortOrder
} from '@/types'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatTeamCount } from '@/utils/affiliateTeam'

const { t } = useI18n()
const appStore = useAppStore()

// 汇总页只用到受众文案（如趋势图的「分润」）；这里没有被查看的代理，上下文里的接口与路由不会被调用。
provideAffiliateTeamContext(createAdminAffiliateTeamContext(() => 0))

const { state: query, update, drillDownQuery } = useAdminAffiliateTeamQuery()

// ---------- 数据 ----------

const overview = ref<AffiliateTeamPlatformOverview | null>(null)
const owners = ref<AffiliateTeamOwner[]>([])
const total = ref(0)
const listReady = ref(false)
const overviewLoading = ref(false)
const listLoading = ref(false)
const refreshing = computed(() => overviewLoading.value || listLoading.value)

const overviewKey = computed(() => `${query.value.start}|${query.value.end}`)
const listKey = computed(() => {
  const q = query.value
  return JSON.stringify([overviewKey.value, q.level, q.q, q.sort, q.order, q.page, q.size])
})

let overviewController: AbortController | null = null
let listController: AbortController | null = null
let loadedOverviewKey = ''

function isCancel(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const e = error as { name?: unknown; code?: unknown }
  return e.name === 'AbortError' || e.name === 'CanceledError' || e.code === 'ERR_CANCELED'
}

function rangeParams() {
  return { start_date: query.value.start, end_date: query.value.end }
}

async function loadOverview(): Promise<void> {
  overviewController?.abort()
  const controller = new AbortController()
  overviewController = controller
  overviewLoading.value = true
  try {
    const data = await adminAffiliateTeamAPI.getOverview(rangeParams(), { signal: controller.signal })
    if (overviewController === controller) overview.value = data
  } catch (error) {
    if (overviewController !== controller || isCancel(error)) return
    loadedOverviewKey = ''
    throw error
  } finally {
    if (overviewController === controller) {
      overviewController = null
      overviewLoading.value = false
    }
  }
}

async function loadList(): Promise<void> {
  listController?.abort()
  const controller = new AbortController()
  listController = controller
  listLoading.value = true
  const q = query.value
  try {
    const res = await adminAffiliateTeamAPI.listOwners({
      ...rangeParams(),
      page: q.page,
      page_size: q.size,
      level: q.level || undefined,
      search: q.q || undefined,
      sort_by: q.sort,
      sort_order: q.order
    }, { signal: controller.signal })
    if (listController !== controller) return
    owners.value = res.items ?? []
    total.value = res.total ?? 0
    listReady.value = true
    // URL 里的页码超出范围（数据变少 / 手改链接）时回到最后一页。
    if (owners.value.length === 0 && total.value > 0 && q.page > 1) {
      void update({ page: Math.max(1, res.pages) })
    }
  } catch (error) {
    if (listController !== controller || isCancel(error)) return
    throw error
  } finally {
    if (listController === controller) {
      listController = null
      listLoading.value = false
    }
  }
}

async function load(force = false): Promise<void> {
  const tasks: Promise<void>[] = []
  if (force || overviewKey.value !== loadedOverviewKey) {
    loadedOverviewKey = overviewKey.value
    tasks.push(loadOverview())
  }
  tasks.push(loadList())
  const failure = (await Promise.allSettled(tasks)).find(
    (result): result is PromiseRejectedResult => result.status === 'rejected'
  )
  if (failure) {
    appStore.showError(extractI18nErrorMessage(failure.reason, t, 'affiliate.team.errors', t('affiliate.team.loadFailed')))
  }
}

function reload() {
  void load(true)
}

watch(listKey, () => {
  void load()
}, { immediate: true })

onBeforeUnmount(() => {
  overviewController?.abort()
  listController?.abort()
})

// ---------- 等级标签 ----------

const levelTabs = computed(() => {
  const counts = overview.value?.counts
  const tabs: Array<{ key: AdminAffiliateTeamLevel; label: string; count: number }> = [
    { key: '', label: t('admin.affiliates.team.levels.all'), count: counts?.all ?? 0 },
    { key: '1', label: t('admin.affiliates.team.levels.level1'), count: counts?.level1 ?? 0 },
    { key: '2', label: t('admin.affiliates.team.levels.level2'), count: counts?.level2 ?? 0 }
  ]
  // 「已降级」只在有人时出现。
  if ((counts?.downgraded ?? 0) > 0) {
    tabs.push({ key: 'downgraded', label: t('admin.affiliates.team.levels.downgraded'), count: counts?.downgraded ?? 0 })
  }
  return tabs
})

// URL 带了 level=downgraded 但已没有降级的人：回到全部。
watch(() => overview.value?.counts, (counts) => {
  if (counts && counts.downgraded === 0 && query.value.level === 'downgraded') onLevelChange('')
})

// ---------- 交互 ----------

const searchDraft = ref(query.value.q)
watch(() => query.value.q, (q) => {
  if (q !== searchDraft.value.trim()) searchDraft.value = q
})

function onSearch(value: string) {
  void update({ q: value.trim(), page: 1 })
}

/** 换等级：页码归 1，排序与搜索保留。 */
function onLevelChange(level: AdminAffiliateTeamLevel) {
  if (level === query.value.level) return
  void update({ level, page: 1 })
}

function onSort(key: string, order: AffiliateTeamSortOrder) {
  if (!ADMIN_AFFILIATE_TEAM_SORTS.includes(key as AffiliateTeamAgentSort)) return
  // 换列先按降序（大的在前），同一列再点切换方向。
  void update({ sort: key as AffiliateTeamAgentSort, order: key === query.value.sort ? order : 'desc', page: 1 })
}

function onRangeChange(range: { start: string; end: string }) {
  const next = clampAffiliateTeamRange(range.start, range.end)
  if (next.clamped) {
    appStore.showInfo(t('affiliate.team.rangeClamped', { days: AFFILIATE_TEAM_MAX_RANGE_DAYS }))
  }
  // 汇总、图表、表格一起刷新；页码归 1，等级、搜索与排序保留。
  void update({ start: next.start, end: next.end, page: 1 })
}
</script>
