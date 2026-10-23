import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useRoute, useRouter, type LocationQuery, type LocationQueryRaw } from 'vue-router'
import type {
  AffiliateTeamAgentSort,
  AffiliateTeamCustomerSort,
  AffiliateTeamSortOrder
} from '@/types'
import { getConfiguredTablePageSizeOptions } from '@/utils/tablePreferences'

/**
 * 代理团队消费页的 URL 状态（route query 是唯一权威来源）。
 *
 * - 白名单解析 `start, end, tab, sort, order, page, size, q, model, group`，非法值回落默认；
 * - 页内变化用 router.replace（浏览器返回直接回到上一层），等于默认值的键不写入 URL；
 * - 下钻链接只携带时间范围。
 */

/** team = /affiliate/team；agent = /affiliate/team/agents/:id；customer = /affiliate/team/customers/:id */
export type AffiliateTeamView = 'team' | 'agent' | 'customer'
export type AffiliateTeamTab = 'customers' | 'agents'
export type AffiliateTeamSort = AffiliateTeamCustomerSort | AffiliateTeamAgentSort

export interface AffiliateTeamQueryState {
  start: string
  end: string
  tab: AffiliateTeamTab
  sort: AffiliateTeamSort
  order: AffiliateTeamSortOrder
  page: number
  size: number
  q: string
  model: string
  /** null = 全部分组；0 = 无分组。 */
  group: number | null
}

export const AFFILIATE_TEAM_DEFAULT_RANGE_DAYS = 7
export const AFFILIATE_TEAM_MAX_RANGE_DAYS = 90
export const AFFILIATE_TEAM_DEFAULT_PAGE_SIZE = 20
/** 与后端 affiliateTeamMaxPageSize 一致。 */
const AFFILIATE_TEAM_MAX_PAGE_SIZE = 100
const MAX_TEXT_LENGTH = 100

const CUSTOMER_SORTS: readonly AffiliateTeamCustomerSort[] = ['cost', 'requests', 'earnings', 'last_used_at']
const AGENT_SORTS: readonly AffiliateTeamAgentSort[] = ['team_cost', 'earnings', 'active_customers']

export function defaultAffiliateTeamSort(tab: AffiliateTeamTab): AffiliateTeamSort {
  return tab === 'agents' ? 'team_cost' : 'cost'
}

export function affiliateTeamSortKeys(tab: AffiliateTeamTab): readonly AffiliateTeamSort[] {
  return tab === 'agents' ? AGENT_SORTS : CUSTOMER_SORTS
}

/** 站点配置的表格分页选项（共享 Pagination 组件使用同一组），截到后端上限 100。 */
export function affiliateTeamPageSizes(): number[] {
  const sizes = getConfiguredTablePageSizeOptions().filter((size) => size <= AFFILIATE_TEAM_MAX_PAGE_SIZE)
  return sizes.length > 0 ? sizes : [AFFILIATE_TEAM_DEFAULT_PAGE_SIZE]
}

// ---------- 日期（本地自然日，与 DateRangePicker 的快捷项一致） ----------

const DATE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/

export function formatLocalDate(date: Date): string {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function parseLocalDate(value: string): Date | null {
  const match = DATE_PATTERN.exec(value)
  if (!match) return null
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]))
  // 拒绝 2026-02-30 这类被 Date 自动进位的日期。
  return formatLocalDate(date) === value ? date : null
}

export function shiftLocalDate(value: string, days: number): string {
  const date = parseLocalDate(value)
  if (!date) return value
  date.setDate(date.getDate() + days)
  return formatLocalDate(date)
}

/** 闭区间天数；非法日期返回 0。 */
export function affiliateTeamRangeDays(start: string, end: string): number {
  const s = parseLocalDate(start)
  const e = parseLocalDate(end)
  if (!s || !e) return 0
  // 用 UTC 计算差值，避开夏令时导致的 23/25 小时。
  const diff = Date.UTC(e.getFullYear(), e.getMonth(), e.getDate()) - Date.UTC(s.getFullYear(), s.getMonth(), s.getDate())
  return Math.round(diff / 86_400_000) + 1
}

export function defaultAffiliateTeamRange(today: Date = new Date()): { start: string; end: string } {
  const end = formatLocalDate(today)
  return { start: shiftLocalDate(end, -(AFFILIATE_TEAM_DEFAULT_RANGE_DAYS - 1)), end }
}

/** 超过 90 天时把开始日期收到 end - 89。 */
export function clampAffiliateTeamRange(start: string, end: string): { start: string; end: string; clamped: boolean } {
  if (affiliateTeamRangeDays(start, end) > AFFILIATE_TEAM_MAX_RANGE_DAYS) {
    return { start: shiftLocalDate(end, -(AFFILIATE_TEAM_MAX_RANGE_DAYS - 1)), end, clamped: true }
  }
  return { start, end, clamped: false }
}

// ---------- 解析 / 生成 ----------

function single(value: LocationQuery[string] | undefined): string {
  const raw = Array.isArray(value) ? value.find((item): item is string => typeof item === 'string') : value
  return typeof raw === 'string' ? raw.trim() : ''
}

function positiveInt(value: string): number | null {
  if (!/^\d+$/.test(value)) return null
  const n = Number(value)
  return Number.isSafeInteger(n) && n > 0 ? n : null
}

function parseRange(query: LocationQuery, today: Date): { start: string; end: string } {
  const start = single(query.start)
  const end = single(query.end)
  const days = affiliateTeamRangeDays(start, end)
  if (days < 1) return defaultAffiliateTeamRange(today)
  const { start: s, end: e } = clampAffiliateTeamRange(start, end)
  return { start: s, end: e }
}

export function parseAffiliateTeamQuery(
  query: LocationQuery,
  view: AffiliateTeamView,
  today: Date = new Date()
): AffiliateTeamQueryState {
  const { start, end } = parseRange(query, today)
  const tab: AffiliateTeamTab = view === 'team' && single(query.tab) === 'agents' ? 'agents' : 'customers'
  const rawSort = single(query.sort) as AffiliateTeamSort
  const sort = view !== 'customer' && affiliateTeamSortKeys(tab).includes(rawSort) ? rawSort : defaultAffiliateTeamSort(tab)
  const order: AffiliateTeamSortOrder = single(query.order) === 'asc' ? 'asc' : 'desc'
  const page = positiveInt(single(query.page)) ?? 1
  const rawSize = positiveInt(single(query.size))
  const size = rawSize !== null && affiliateTeamPageSizes().includes(rawSize) ? rawSize : AFFILIATE_TEAM_DEFAULT_PAGE_SIZE
  const q = view === 'customer' || tab === 'agents' ? '' : single(query.q).slice(0, MAX_TEXT_LENGTH)
  const model = view === 'customer' ? single(query.model).slice(0, MAX_TEXT_LENGTH) : ''
  const rawGroup = single(query.group)
  const group = view === 'customer' && /^\d+$/.test(rawGroup) && Number.isSafeInteger(Number(rawGroup)) ? Number(rawGroup) : null
  return { start, end, tab, sort, order, page, size, q, model, group }
}

/** 只写入非默认值与当前视图相关的键。 */
export function buildAffiliateTeamQuery(
  state: AffiliateTeamQueryState,
  view: AffiliateTeamView,
  today: Date = new Date()
): Record<string, string> {
  const query: Record<string, string> = {}
  const defaults = defaultAffiliateTeamRange(today)
  if (state.start !== defaults.start || state.end !== defaults.end) {
    query.start = state.start
    query.end = state.end
  }
  if (view === 'team' && state.tab === 'agents') query.tab = 'agents'
  if (view !== 'customer') {
    if (state.sort !== defaultAffiliateTeamSort(state.tab)) query.sort = state.sort
    if (state.order !== 'desc') query.order = state.order
    // 搜索只作用于客户表。
    if (state.q && state.tab === 'customers') query.q = state.q
  } else {
    if (state.model) query.model = state.model
    if (state.group !== null) query.group = String(state.group)
  }
  if (state.page > 1) query.page = String(state.page)
  if (state.size !== AFFILIATE_TEAM_DEFAULT_PAGE_SIZE) query.size = String(state.size)
  return query
}

function sameQuery(next: Record<string, string>, current: LocationQuery): boolean {
  const currentKeys = Object.keys(current).filter((key) => current[key] !== undefined)
  const nextKeys = Object.keys(next)
  if (currentKeys.length !== nextKeys.length) return false
  return nextKeys.every((key) => current[key] === next[key])
}

export function useAffiliateTeamQuery(view: MaybeRefOrGetter<AffiliateTeamView>) {
  const route = useRoute()
  const router = useRouter()

  const state = computed(() => parseAffiliateTeamQuery(route.query, toValue(view)))

  /** 页内状态变化：合并补丁后 replace，浏览器返回直接回到上一层。 */
  async function update(patch: Partial<AffiliateTeamQueryState>): Promise<void> {
    const query = buildAffiliateTeamQuery({ ...state.value, ...patch }, toValue(view))
    if (sameQuery(query, route.query)) return
    await router.replace({ query: query as LocationQueryRaw })
  }

  /** 下钻（客户详情 / 二级团队）只携带时间范围。 */
  const drillDownQuery = computed(() => ({ start: state.value.start, end: state.value.end }))

  return { state, update, drillDownQuery }
}
