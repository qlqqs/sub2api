import { computed } from 'vue'
import {
  onBeforeRouteLeave,
  useRoute,
  useRouter,
  type LocationQuery,
  type LocationQueryRaw,
  type RouteLocationRaw
} from 'vue-router'
import type { AdminAffiliateTeamLevel, AffiliateTeamAgentSort, AffiliateTeamSortOrder } from '@/types'
import {
  AFFILIATE_TEAM_DEFAULT_PAGE_SIZE,
  affiliateTeamPageSizes,
  defaultAffiliateTeamRange,
  parseRange,
  positiveInt,
  sameQuery,
  single
} from './useAffiliateTeamQuery'

/**
 * 管理端团队消费汇总页的 URL 状态：`start, end, level, q, sort, order, page, size`。
 * 规则同代理端：白名单解析、默认值不写入 URL、页内变化用 router.replace、下钻只带时间范围。
 * 离开汇总页时记下 query，代理视角面包屑的首项用它回到原来的筛选。
 */

export interface AdminAffiliateTeamQueryState {
  start: string
  end: string
  level: AdminAffiliateTeamLevel
  q: string
  sort: AffiliateTeamAgentSort
  order: AffiliateTeamSortOrder
  page: number
  size: number
}

const LEVELS: readonly AdminAffiliateTeamLevel[] = ['1', '2', 'downgraded']
export const ADMIN_AFFILIATE_TEAM_SORTS: readonly AffiliateTeamAgentSort[] = ['team_cost', 'earnings', 'active_customers']
const DEFAULT_SORT: AffiliateTeamAgentSort = 'team_cost'
const MAX_TEXT_LENGTH = 100

export function parseAdminAffiliateTeamQuery(query: LocationQuery, today: Date = new Date()): AdminAffiliateTeamQueryState {
  const { start, end } = parseRange(query, today)
  const rawLevel = single(query.level) as AdminAffiliateTeamLevel
  const rawSort = single(query.sort) as AffiliateTeamAgentSort
  const rawSize = positiveInt(single(query.size))
  return {
    start,
    end,
    level: LEVELS.includes(rawLevel) ? rawLevel : '',
    q: single(query.q).slice(0, MAX_TEXT_LENGTH),
    sort: ADMIN_AFFILIATE_TEAM_SORTS.includes(rawSort) ? rawSort : DEFAULT_SORT,
    order: single(query.order) === 'asc' ? 'asc' : 'desc',
    page: positiveInt(single(query.page)) ?? 1,
    size: rawSize !== null && affiliateTeamPageSizes().includes(rawSize) ? rawSize : AFFILIATE_TEAM_DEFAULT_PAGE_SIZE
  }
}

/** 只写入非默认值。 */
export function buildAdminAffiliateTeamQuery(state: AdminAffiliateTeamQueryState, today: Date = new Date()): Record<string, string> {
  const query: Record<string, string> = {}
  const defaults = defaultAffiliateTeamRange(today)
  if (state.start !== defaults.start || state.end !== defaults.end) {
    query.start = state.start
    query.end = state.end
  }
  if (state.level) query.level = state.level
  if (state.q) query.q = state.q
  if (state.sort !== DEFAULT_SORT) query.sort = state.sort
  if (state.order !== 'desc') query.order = state.order
  if (state.page > 1) query.page = String(state.page)
  if (state.size !== AFFILIATE_TEAM_DEFAULT_PAGE_SIZE) query.size = String(state.size)
  return query
}

/** 最近一次离开汇总页时的 query；刷新页面后为空，即回到默认筛选。 */
let rememberedSummaryQuery: LocationQuery = {}

/** 汇总页路由：带回离开时的筛选、排序与页码，日期取调用方给的范围（代理视角当前的日期）。 */
export function adminAffiliateTeamSummaryRoute(range: { start: string; end: string }): RouteLocationRaw {
  const state = parseAdminAffiliateTeamQuery(rememberedSummaryQuery)
  return { name: 'AdminAffiliateTeam', query: buildAdminAffiliateTeamQuery({ ...state, start: range.start, end: range.end }) }
}

export function useAdminAffiliateTeamQuery() {
  const route = useRoute()
  const router = useRouter()

  const state = computed(() => parseAdminAffiliateTeamQuery(route.query))

  // 在离开时记下：此时 from 仍是汇总页的完整 query。
  onBeforeRouteLeave((_to, from) => {
    rememberedSummaryQuery = { ...from.query }
  })

  /** 页内状态变化：合并补丁后 replace，浏览器返回直接回到上一页。 */
  async function update(patch: Partial<AdminAffiliateTeamQueryState>): Promise<void> {
    const query = buildAdminAffiliateTeamQuery({ ...state.value, ...patch })
    if (sameQuery(query, route.query)) return
    await router.replace({ query: query as LocationQueryRaw })
  }

  /** 下钻（某代理的视角）只携带时间范围。 */
  const drillDownQuery = computed(() => ({ start: state.value.start, end: state.value.end }))

  return { state, update, drillDownQuery }
}
