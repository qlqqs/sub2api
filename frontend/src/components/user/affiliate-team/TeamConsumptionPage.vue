<template>
  <div class="space-y-4">
    <div class="space-y-1.5">
      <!-- 代理端根页无权访问时只留下方的说明卡片 -->
      <TeamToolbar
        v-if="isAdmin || !(view === 'team' && notFound)"
        :crumbs="crumbs"
        :section-tabs="!isAdmin && view === 'team'"
        :start-date="query.start"
        :end-date="query.end"
        :refreshing="refreshing"
        @range-change="onRangeChange"
        @refresh="reload"
      />

      <!-- 管理端根页：被查看的代理本人 -->
      <div
        v-if="!notFound && view === 'team' && subject"
        class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-gray-600 dark:text-gray-300"
        data-test="team-viewer"
      >
        <template v-if="subject.user.nickname && subject.user.email">
          <span>{{ subject.user.email }}</span>
          <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
        </template>
        <!-- 已删除的账号面包屑已是「已删除 #ID」，不再重复 -->
        <template v-if="!subject.user.deleted">
          <span class="tabular-nums" data-test="team-viewer-id">#{{ subject.user.user_id }}</span>
          <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
        </template>
        <span v-if="overview?.viewer.downgraded" data-test="team-viewer-level">{{ t('admin.affiliates.team.meta.downgraded') }}</span>
        <template v-else-if="overview?.viewer.agent_level === 2">
          <span data-test="team-viewer-level">{{ t('admin.affiliates.team.meta.level2') }}</span>
          <template v-if="subject.upline">
            <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
            <i18n-t keypath="admin.affiliates.team.meta.upline" tag="span" scope="global" data-test="team-viewer-upline">
              <template #name>
                <RouterLink
                  v-if="!subject.upline.deleted && context.routes.viewer"
                  :to="context.routes.viewer(subject.upline.user_id, drillDownQuery)"
                  class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                >
                  {{ teamUserName(subject.upline, deletedLabel(subject.upline)) }}
                </RouterLink>
                <span v-else>{{ teamUserName(subject.upline, deletedLabel(subject.upline)) }}</span>
              </template>
            </i18n-t>
          </template>
        </template>
        <span v-else data-test="team-viewer-level">{{ t('admin.affiliates.team.meta.level1') }}</span>
        <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
        <span class="tabular-nums">{{ label('affiliate.team.lifetimeEarnings', { amount: formatTeamMoney(subject.lifetime_earnings) }) }}</span>
      </div>

      <!-- 二级团队 / 客户的元信息行 -->
      <div
        v-else-if="!notFound && overview?.agent"
        class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-gray-600 dark:text-gray-300"
        data-test="team-subject"
      >
        <!-- 面包屑已显示昵称（无昵称时显示邮箱），这里只补邮箱，不重复 -->
        <template v-if="overview.agent.user.nickname && overview.agent.user.email">
          <span>{{ overview.agent.user.email }}</span>
          <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
        </template>
        <template v-if="isAdmin && !overview.agent.user.deleted">
          <span class="tabular-nums" data-test="team-subject-id">#{{ overview.agent.user.user_id }}</span>
          <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
        </template>
        <TeamBadge v-if="overview.agent.historical" tone="gray" :hint="label('affiliate.team.historicalHint')">
          {{ t('affiliate.team.historical') }}
        </TeamBadge>
        <span v-else>{{ t('affiliate.team.subAgent') }}</span>
      </div>
      <div
        v-else-if="!notFound && customer"
        class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-gray-600 dark:text-gray-300"
        data-test="team-subject"
      >
        <span v-if="customer.user.nickname && customer.user.email">{{ customer.user.email }}</span>
        <template v-if="isAdmin && !customer.user.deleted">
          <span v-if="customer.user.nickname && customer.user.email" class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
          <span class="tabular-nums" data-test="team-subject-id">#{{ customer.user.user_id }}</span>
        </template>
        <RouterLink
          v-if="customer.is_sub_agent"
          :to="agentRoute(customer.user.user_id)"
          class="rounded transition-opacity hover:opacity-80"
          data-test="team-subject-sub-agent"
        >
          <TeamBadge>{{ t('affiliate.team.subAgent') }}</TeamBadge>
        </RouterLink>
        <span
          v-if="(customer.user.nickname && customer.user.email) || customer.is_sub_agent || (isAdmin && !customer.user.deleted)"
          class="text-gray-300 dark:text-dark-600"
          aria-hidden="true"
        >·</span>
        <span v-if="customer.relation === 'owner' || !customer.owner_agent">{{ label('affiliate.team.myCustomer') }}</span>
        <i18n-t v-else keypath="affiliate.team.customerOf" tag="span" scope="global" data-test="team-subject-owner">
          <template #name>
            <RouterLink
              v-if="!customer.owner_agent.deleted"
              :to="agentRoute(customer.owner_agent.user_id)"
              class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
            >
              {{ teamUserName(customer.owner_agent, deletedLabel(customer.owner_agent)) }}
            </RouterLink>
            <span v-else>{{ deletedLabel(customer.owner_agent) }}</span>
          </template>
        </i18n-t>
        <template v-if="customer.bound_at">
          <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
          <span class="tabular-nums">{{ t('affiliate.team.boundAt', { date: formatLocalDate(new Date(customer.bound_at)) }) }}</span>
        </template>
        <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
        <span class="tabular-nums">{{ label('affiliate.team.lifetimeEarnings', { amount: formatTeamMoney(customer.lifetime_earnings) }) }}</span>
        <template v-if="context.routes.usage">
          <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">·</span>
          <RouterLink
            :to="context.routes.usage(customer.user.user_id, { start: query.start, end: query.end })"
            class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
            data-test="team-subject-usage"
          >
            {{ t('admin.affiliates.team.meta.usage') }} →
          </RouterLink>
        </template>
      </div>

      <p v-if="!isAdmin && overview?.viewer.downgraded" class="text-sm text-gray-500 dark:text-dark-400" data-test="team-downgraded">
        {{ t('affiliate.team.downgradedNotice') }}
      </p>
    </div>

    <div
      v-if="notFound"
      class="card flex flex-col items-center gap-3 px-6 py-16 text-center"
      data-test="team-not-found"
    >
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ notFoundState.message }}</p>
      <RouterLink
        :to="notFoundState.to"
        class="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
      >
        {{ notFoundState.link }}
      </RouterLink>
    </div>

    <template v-else>
      <TeamSummaryStrip
        :view="view"
        :summary="overview?.summary ?? null"
        :counts="overview?.counts"
        :settle-interval-minutes="overview?.viewer.settle_interval_minutes"
      />

      <div class="grid gap-4 lg:grid-cols-3">
        <TeamTrendChart class="lg:col-span-2" :trend="overview?.trend ?? []" :loading="!overview" />
        <TeamModelShare
          :models="overview?.models ?? []"
          :loading="!overview"
          :clickable="view === 'customer'"
          :active-model="query.model"
          @select="onModelShareSelect"
        />
      </div>

      <section class="card overflow-hidden" data-test="team-list">
        <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
          <div v-if="showTabs" role="tablist" class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800">
            <button
              v-for="tab in tabs"
              :key="tab.key"
              type="button"
              role="tab"
              :aria-selected="listKind === tab.key"
              :class="['tab px-3 py-1.5', listKind === tab.key && 'tab-active']"
              :data-test="`team-tab-${tab.key}`"
              @click="onTabChange(tab.key)"
            >
              {{ tab.label }}
              <span class="ml-1 tabular-nums text-gray-400 dark:text-dark-500">{{ formatTeamCount(tab.count) }}</span>
            </button>
          </div>
          <h3 v-else class="text-sm font-semibold text-gray-900 dark:text-white" data-test="team-list-title">
            {{ listTitle }}
            <span v-if="listCount !== null" class="ml-1 font-normal tabular-nums text-gray-400 dark:text-dark-500">
              {{ formatTeamCount(listCount) }}
            </span>
          </h3>

          <div v-if="listKind === 'customers'" class="w-full sm:w-64">
            <SearchInput
              v-model="searchDraft"
              :placeholder="label('affiliate.team.searchPlaceholder')"
              :debounce-ms="300"
              @search="onSearch"
            />
          </div>
          <div v-else-if="listKind === 'requests'" class="flex w-full items-center gap-3 sm:w-auto sm:gap-4">
            <div class="flex min-w-0 flex-1 items-center gap-2 sm:flex-none">
              <label for="team-filter-model" class="shrink-0 text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.team.filters.model') }}</label>
              <div class="min-w-0 flex-1 sm:w-52 sm:flex-none">
                <Select
                  id="team-filter-model"
                  :model-value="query.model"
                  :options="modelOptions"
                  @update:model-value="onModelFilter"
                />
              </div>
            </div>
            <div class="flex min-w-0 flex-1 items-center gap-2 sm:flex-none">
              <label for="team-filter-group" class="shrink-0 text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.team.filters.group') }}</label>
              <div class="min-w-0 flex-1 sm:w-36 sm:flex-none">
                <Select
                  id="team-filter-group"
                  :model-value="query.group"
                  :options="groupOptions"
                  @update:model-value="onGroupFilter"
                />
              </div>
            </div>
          </div>
        </div>

        <TeamProgressBar :active="refreshing && listReady" />

        <TeamCustomerTable
          v-if="listKind === 'customers'"
          :rows="customers"
          :loading="!listReady"
          :sort="query.sort"
          :order="query.order"
          :drill-query="drillDownQuery"
          @sort="onSort"
        >
          <template #empty>
            <div class="flex flex-col items-center gap-2 text-sm text-gray-500 dark:text-dark-400" data-test="team-empty">
              <span>{{ query.q ? t('affiliate.team.empty.noMatches') : t('affiliate.team.empty.noCustomers') }}</span>
              <RouterLink
                v-if="!query.q && view === 'team' && !isAdmin"
                :to="{ name: 'Affiliate' }"
                class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              >
                {{ t('affiliate.team.empty.invite') }}
              </RouterLink>
            </div>
          </template>
        </TeamCustomerTable>
        <TeamAgentTable
          v-else-if="listKind === 'agents'"
          :rows="agents"
          :loading="!listReady"
          :sort="query.sort"
          :order="query.order"
          :drill-query="drillDownQuery"
          @sort="onSort"
        >
          <template #empty>
            <span class="text-sm text-gray-500 dark:text-dark-400" data-test="team-empty">{{ t('affiliate.team.empty.noAgents') }}</span>
          </template>
        </TeamAgentTable>
        <TeamRequestTable v-else :rows="requests" :loading="!listReady">
          <template #empty>
            <span class="text-sm text-gray-500 dark:text-dark-400" data-test="team-empty">{{ t('affiliate.team.empty.noRequests') }}</span>
          </template>
        </TeamRequestTable>

        <Pagination
          v-if="listReady && total > 0"
          :total="total"
          :page="query.page"
          :page-size="query.size"
          :show-page-size-selector="listKind === 'requests'"
          @update:page="(page: number) => update({ page })"
          @update:pageSize="(size: number) => update({ size, page: 1 })"
        />
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, toRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import TeamAgentTable from './TeamAgentTable.vue'
import TeamBadge from './TeamBadge.vue'
import TeamCustomerTable from './TeamCustomerTable.vue'
import TeamModelShare from './TeamModelShare.vue'
import TeamProgressBar from './TeamProgressBar.vue'
import TeamRequestTable from './TeamRequestTable.vue'
import TeamSummaryStrip from './TeamSummaryStrip.vue'
import TeamToolbar, { type TeamCrumb } from './TeamToolbar.vue'
import TeamTrendChart from './TeamTrendChart.vue'
import {
  provideAffiliateTeamContext,
  useAffiliateTeamContext,
  useTeamLabel,
  type AffiliateTeamContext
} from './teamContext'
import {
  AFFILIATE_TEAM_MAX_RANGE_DAYS,
  affiliateTeamSortKeys,
  clampAffiliateTeamRange,
  defaultAffiliateTeamSort,
  formatLocalDate,
  useAffiliateTeamQuery,
  type AffiliateTeamSort,
  type AffiliateTeamTab,
  type AffiliateTeamView
} from '@/composables/useAffiliateTeamQuery'
import { useAppStore } from '@/stores/app'
import type {
  AffiliateTeamAgent,
  AffiliateTeamAgentSort,
  AffiliateTeamCustomer,
  AffiliateTeamCustomerSort,
  AffiliateTeamOverview,
  AffiliateTeamRequest,
  AffiliateTeamSortOrder,
  AffiliateTeamUser,
  BasePaginationResponse
} from '@/types'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatTeamCount, formatTeamMoney, teamUserName } from '@/utils/affiliateTeam'

type ListKind = 'customers' | 'agents' | 'requests'

/**
 * 团队消费页主体（代理端 /affiliate/team 与管理端代理视角共用）。接口、路由与受众相关文案取自
 * teamContext；view / subjectId 由所在的路由视图算出。
 */
const props = defineProps<{
  view: AffiliateTeamView
  /** 二级代理 / 客户 ID；团队根页为 0；路径参数非法时为 -1（直接按不存在处理）。 */
  subjectId: number
}>()

const { t } = useI18n()
const appStore = useAppStore()

const view = toRef(props, 'view')
const subjectId = toRef(props, 'subjectId')

const { state: query, update, drillDownQuery } = useAffiliateTeamQuery(view)

// ---------- 数据 ----------

const overview = ref<AffiliateTeamOverview | null>(null)
const notFound = ref(false)
const customers = ref<AffiliateTeamCustomer[]>([])
const agents = ref<AffiliateTeamAgent[]>([])
const requests = ref<AffiliateTeamRequest[]>([])
const total = ref(0)
/** 当前列表数据属于哪种表；与 listKind 不一致时显示骨架。 */
const loadedListKind = ref<ListKind | null>(null)
const overviewLoading = ref(false)
const listLoading = ref(false)

// ---------- 受众 ----------

const baseContext = useAffiliateTeamContext()
const isAdmin = baseContext.audience === 'admin'
/** 管理端：被查看的代理本人（各层 overview 都带）。 */
const subject = computed(() => overview.value?.subject ?? null)

/** 已删除用户的名称：管理端带 ID。 */
function deletedLabel(user: AffiliateTeamUser): string {
  return label('affiliate.team.deleted', { id: user.user_id })
}
const subjectName = computed(() => (subject.value ? teamUserName(subject.value.user, deletedLabel(subject.value.user)) : ''))

// 管理端替换文案可以引用被查看代理的名字（如「已不是 {name} 的二级代理」），下层组件从这里取。
const context: AffiliateTeamContext = {
  ...baseContext,
  labelParams: () => ({ ...baseContext.labelParams?.(), name: subjectName.value })
}
provideAffiliateTeamContext(context)
const label = useTeamLabel(context)

const customer = computed(() => overview.value?.customer ?? null)
const listKind = computed<ListKind>(() => {
  if (view.value === 'customer') return 'requests'
  return view.value === 'team' && query.value.tab === 'agents' ? 'agents' : 'customers'
})
const listReady = computed(() => loadedListKind.value === listKind.value)
const refreshing = computed(() => overviewLoading.value || listLoading.value)

const scopeKey = computed(() => `${view.value}:${subjectId.value}`)
const overviewKey = computed(() => `${scopeKey.value}|${query.value.start}|${query.value.end}`)
const listKey = computed(() => {
  const q = query.value
  return JSON.stringify([overviewKey.value, listKind.value, q.sort, q.order, q.page, q.size, q.q, q.model, q.group])
})

let overviewController: AbortController | null = null
let listController: AbortController | null = null
let loadedScope = ''
let loadedOverviewKey = ''

function isCancel(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const e = error as { name?: unknown; code?: unknown }
  return e.name === 'AbortError' || e.name === 'CanceledError' || e.code === 'ERR_CANCELED'
}

function isNotFound(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const e = error as { status?: unknown; reason?: unknown }
  return e.reason === 'AFFILIATE_TEAM_NOT_FOUND' || e.reason === 'AFFILIATE_TEAM_FORBIDDEN' || e.status === 403 || e.status === 404
}

/** 换了层级（团队 ↔ 二级团队 ↔ 客户）时清空旧数据，回到骨架。 */
function resetScope() {
  overviewController?.abort()
  listController?.abort()
  overviewController = null
  listController = null
  overviewLoading.value = false
  listLoading.value = false
  overview.value = null
  notFound.value = false
  customers.value = []
  agents.value = []
  requests.value = []
  total.value = 0
  loadedListKind.value = null
  loadedOverviewKey = ''
}

function rangeParams() {
  return { start_date: query.value.start, end_date: query.value.end }
}

async function loadOverview(): Promise<void> {
  overviewController?.abort()
  const controller = new AbortController()
  overviewController = controller
  overviewLoading.value = true
  const params = {
    ...rangeParams(),
    agent_id: view.value === 'agent' ? subjectId.value : undefined,
    user_id: view.value === 'customer' ? subjectId.value : undefined
  }
  try {
    const data = await context.api.getOverview(params, { signal: controller.signal })
    if (overviewController !== controller) return
    overview.value = data
    notFound.value = false
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

async function fetchList(kind: ListKind, signal: AbortSignal): Promise<BasePaginationResponse<unknown>> {
  const q = query.value
  const page = { page: q.page, page_size: q.size }
  if (kind === 'requests') {
    return context.api.listRequests(subjectId.value, {
      ...rangeParams(),
      ...page,
      model: q.model || undefined,
      group_id: q.group ?? undefined
    }, { signal })
  }
  const sorting = { sort_order: q.order }
  if (kind === 'agents') {
    return context.api.listAgents({ ...rangeParams(), ...page, ...sorting, sort_by: q.sort as AffiliateTeamAgentSort }, { signal })
  }
  return context.api.listCustomers({
    ...rangeParams(),
    ...page,
    ...sorting,
    sort_by: q.sort as AffiliateTeamCustomerSort,
    agent_id: view.value === 'agent' ? subjectId.value : undefined,
    search: q.q || undefined
  }, { signal })
}

async function loadList(): Promise<void> {
  listController?.abort()
  const controller = new AbortController()
  listController = controller
  listLoading.value = true
  const kind = listKind.value
  try {
    const res = await fetchList(kind, controller.signal)
    if (listController !== controller) return
    const items = res.items ?? []
    if (kind === 'requests') requests.value = items as AffiliateTeamRequest[]
    else if (kind === 'agents') agents.value = items as AffiliateTeamAgent[]
    else customers.value = items as AffiliateTeamCustomer[]
    total.value = res.total ?? 0
    loadedListKind.value = kind
    // URL 里的页码超出范围（数据变少 / 手改链接）时回到最后一页。
    if (items.length === 0 && total.value > 0 && query.value.page > 1) {
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
  if (scopeKey.value !== loadedScope) {
    resetScope()
    loadedScope = scopeKey.value
  }
  if (subjectId.value < 0) {
    notFound.value = true
    return
  }
  const tasks: Promise<void>[] = []
  if (force || overviewKey.value !== loadedOverviewKey) {
    loadedOverviewKey = overviewKey.value
    tasks.push(loadOverview())
  }
  tasks.push(loadList())
  const failure = (await Promise.allSettled(tasks)).find(
    (result): result is PromiseRejectedResult => result.status === 'rejected'
  )
  if (!failure) return
  if (isNotFound(failure.reason)) {
    notFound.value = true
    return
  }
  appStore.showError(extractI18nErrorMessage(failure.reason, t, 'affiliate.team.errors', t('affiliate.team.loadFailed')))
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

// URL 带了 tab=agents 但没有下级代理：回到「我的客户」。
watch(() => overview.value?.counts, (counts) => {
  if (counts && counts.sub_agents === 0 && view.value === 'team' && query.value.tab === 'agents') {
    onTabChange('customers')
  }
})

// ---------- 面包屑（按归属层级生成，与来路无关） ----------

const teamRoute = computed<RouteLocationRaw>(() => context.routes.team(drillDownQuery.value))

function agentRoute(id: number): RouteLocationRaw {
  return context.routes.agent(id, drillDownQuery.value)
}

/** 管理端：回到汇总页（带回离开时的筛选，日期取当前页的）。 */
const summaryRoute = computed<RouteLocationRaw | null>(() =>
  context.routes.summary ? context.routes.summary({ start: query.value.start, end: query.value.end }) : null
)

const crumbs = computed<TeamCrumb[]>(() => {
  // 代理端：团队消费 / 二级 / 客户；管理端在前面加汇总页，团队根为被查看的代理。
  const prefix: TeamCrumb[] = summaryRoute.value
    ? [{ label: t('admin.affiliates.team.title'), to: summaryRoute.value }]
    : []
  if (notFound.value && (isAdmin || view.value !== 'team')) {
    return isAdmin ? prefix : [{ label: t('affiliate.team.title'), to: teamRoute.value }]
  }
  const data = overview.value
  const rootLabel = isAdmin ? subjectName.value : t('affiliate.team.title')
  const rootLoading = isAdmin && !subject.value
  if (view.value === 'team') {
    return [...prefix, rootLoading ? { label: '', loading: true } : { label: rootLabel }]
  }
  const root: TeamCrumb = rootLoading ? { label: '', loading: true } : { label: rootLabel, to: teamRoute.value }
  if (view.value === 'agent') {
    return data?.agent
      ? [...prefix, root, { label: teamUserName(data.agent.user, deletedLabel(data.agent.user)) }]
      : [...prefix, root, { label: '', loading: true }]
  }
  const c = data?.customer
  if (!c) return [...prefix, root, { label: '', loading: true }]
  const self: TeamCrumb = { label: teamUserName(c.user, deletedLabel(c.user)) }
  if (c.relation === 'upline' && c.owner_agent) {
    const owner = c.owner_agent
    return [...prefix, root, { label: teamUserName(owner, deletedLabel(owner)), to: owner.deleted ? undefined : agentRoute(owner.user_id) }, self]
  }
  return [...prefix, root, self]
})

const notFoundState = computed<{ message: string; to: RouteLocationRaw; link: string }>(() => {
  if (summaryRoute.value) {
    return { message: label('affiliate.team.notFound'), to: summaryRoute.value, link: label('affiliate.team.backToTeam') }
  }
  if (view.value === 'team') {
    return { message: t('affiliate.team.forbidden'), to: { name: 'Affiliate' }, link: t('affiliate.team.backToAffiliate') }
  }
  return { message: t('affiliate.team.notFound'), to: teamRoute.value, link: t('affiliate.team.backToTeam') }
})

// ---------- 列表头 ----------

const showTabs = computed(() => view.value === 'team' && (overview.value?.counts?.sub_agents ?? 0) > 0)
const tabs = computed(() => [
  { key: 'customers' as const, label: label('affiliate.team.tabs.customers'), count: overview.value?.counts?.direct_customers ?? 0 },
  { key: 'agents' as const, label: t('affiliate.team.tabs.agents'), count: overview.value?.counts?.sub_agents ?? 0 }
])

const listTitle = computed(() => {
  if (view.value === 'customer') return t('affiliate.team.requestsTitle')
  const agent = overview.value?.agent
  if (view.value === 'agent') {
    return agent ? t('affiliate.team.agentCustomers', { name: teamUserName(agent.user, deletedLabel(agent.user)) }) : ''
  }
  return label('affiliate.team.tabs.customers')
})
const listCount = computed<number | null>(() => {
  const data = overview.value
  if (!data || view.value === 'customer') return null
  return view.value === 'agent' ? data.summary.customers : data.counts?.direct_customers ?? null
})

const modelOptions = computed(() => {
  const models = overview.value?.filters?.models ?? []
  const current = query.value.model
  const list = current && !models.includes(current) ? [current, ...models] : models
  return [
    { value: '', label: t('affiliate.team.filters.all') },
    ...list.map((model) => ({ value: model, label: model }))
  ]
})
const groupOptions = computed(() => [
  { value: null, label: t('affiliate.team.filters.all') },
  ...(overview.value?.filters?.groups ?? []).map((group) => ({
    value: group.id,
    label: group.id === 0 || !group.name ? t('affiliate.team.filters.noGroup') : group.name
  }))
])

// ---------- 交互 ----------

const searchDraft = ref(query.value.q)
watch(() => query.value.q, (q) => {
  if (q !== searchDraft.value.trim()) searchDraft.value = q
})

function onSearch(value: string) {
  void update({ q: value.trim(), page: 1 })
}

function onTabChange(tab: AffiliateTeamTab) {
  if (tab === query.value.tab) return
  void update({ tab, sort: defaultAffiliateTeamSort(tab), order: 'desc', page: 1, q: '' })
}

function onSort(key: string, order: AffiliateTeamSortOrder) {
  const sortable = affiliateTeamSortKeys(listKind.value === 'agents' ? 'agents' : 'customers')
  if (!sortable.includes(key as AffiliateTeamSort)) return
  // 换列先按降序（大的在前），同一列再点切换方向。
  void update({ sort: key as AffiliateTeamSort, order: key === query.value.sort ? order : 'desc', page: 1 })
}

function onRangeChange(range: { start: string; end: string }) {
  const next = clampAffiliateTeamRange(range.start, range.end)
  if (next.clamped) {
    appStore.showInfo(t('affiliate.team.rangeClamped', { days: AFFILIATE_TEAM_MAX_RANGE_DAYS }))
  }
  // 摘要、图表、表格一起刷新；页码归 1，排序保留。
  void update({ start: next.start, end: next.end, page: 1 })
}

/** 模型分布点击：设置明细的模型筛选，再点取消。 */
function onModelShareSelect(model: string) {
  void update({ model: query.value.model === model ? '' : model, page: 1 })
}

function onModelFilter(value: string | number | boolean | null) {
  void update({ model: typeof value === 'string' ? value : '', page: 1 })
}

function onGroupFilter(value: string | number | boolean | null) {
  void update({ group: typeof value === 'number' ? value : null, page: 1 })
}
</script>
