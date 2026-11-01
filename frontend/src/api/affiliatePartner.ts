/**
 * CUSTOM: 合伙人（按区域利润分成）用户端接口：/api/v1/user/aff/partner/*。
 * 接口约定见 `.trellis/tasks/10-31-affiliate-partner/contract.md`。
 *
 * 金额是十进制字符串、比例是字符串小数（"0.4000" = 40%），前端只展示不运算；
 * normalize 只做形状兜底（缺字段补 "0"、数组补 []），不改数值。管理端（api/admin/affiliatePartners.ts）共用这些类型和函数。
 */

import { apiClient } from './client'
import type {
  AffiliatePartnerInfo,
  AffiliateTeamAgent,
  AffiliateTeamCustomer,
  AffiliateTeamOverview,
  AffiliateTeamRequest,
  AffiliateTeamUser,
  BasePaginationResponse,
  FetchOptions
} from '@/types'
import type {
  AffiliateTeamAgentsParams,
  AffiliateTeamCustomersParams,
  AffiliateTeamDataSource,
  AffiliateTeamOverviewParams,
  AffiliateTeamRangeParams,
  AffiliateTeamRequestsParams
} from './affiliateTeam'

export type { AffiliatePartnerInfo }

export type PartnerAmount = string
/** current = 当月预估；draft = 已结束未确认。 */
export type PartnerMonthStatus = 'current' | 'draft' | 'confirmed' | 'paid'

/** 管理端为邮箱原文，合伙人端为脱敏邮箱。 */
export interface PartnerUserRef {
  id: number
  email: string
  username: string
  deleted: boolean
}

export interface PartnerMonthTotals {
  month: string
  status: PartnerMonthStatus
  share_rate: string
  consumption: PartnerAmount
  revenue: PartnerAmount
  account_cost: PartnerAmount
  commission: PartnerAmount
  region_expense: PartnerAmount
  adjustment: PartnerAmount
  profit: PartnerAmount
  loss_before: PartnerAmount
  loss_after: PartnerAmount
  share_base: PartnerAmount
  share_amount: PartnerAmount
  confirmed_at: string | null
  paid_on: string | null
  paid_note: string | null
}

/** line_agent 为 null 表示合伙人直属团队。 */
export interface PartnerLine {
  line_agent: PartnerUserRef | null
  members: number
  consumption: PartnerAmount
  revenue: PartnerAmount
  account_cost: PartnerAmount
  commission: PartnerAmount
  profit: PartnerAmount
}

export interface PartnerCostByGroup {
  group_id: number
  group_name: string
  consumption: PartnerAmount
  account_cost: PartnerAmount
}

export interface PartnerCostByModel {
  model: string
  consumption: PartnerAmount
  account_cost: PartnerAmount
}

export interface PartnerAdjustment {
  id: number
  amount: PartnerAmount
  note: string
  created_at: string
  /** 只在管理端返回。 */
  created_by_email: string | null
}

export interface PartnerStatementDetail {
  totals: PartnerMonthTotals
  lines: PartnerLine[]
  cost_by_group: PartnerCostByGroup[]
  cost_by_model: PartnerCostByModel[]
  adjustments: PartnerAdjustment[]
  warnings: string[]
}

export interface PartnerOverview extends AffiliatePartnerInfo {
  loss_carried: PartnerAmount
  agent_count: number
  member_count: number
  current_month: PartnerMonthTotals
  lines: PartnerLine[]
}

/** 区域团队消费：一条渠道经理线（line_agent 为 null 是合伙人直属团队）。 */
export interface PartnerTeamLine {
  line_agent: AffiliateTeamUser | null
  customers: number
  active_customers: number
  requests: number
  team_cost: number
  cost_share: number
}

// ==================== 形状兜底 ====================

function obj(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

function arr(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

function amount(value: unknown): PartnerAmount {
  if (typeof value === 'string' && value.trim() !== '') return value
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return '0'
}

function int(value: unknown): number {
  const n = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(n) ? n : 0
}

function num(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function str(value: unknown): string {
  return typeof value === 'string' ? value : value == null ? '' : String(value)
}

function nullableStr(value: unknown): string | null {
  return typeof value === 'string' && value !== '' ? value : null
}

function amounts<K extends string>(source: Record<string, unknown>, keys: readonly K[]): Record<K, PartnerAmount> {
  return Object.fromEntries(keys.map((key) => [key, amount(source[key])])) as Record<K, PartnerAmount>
}

const STATUSES: PartnerMonthStatus[] = ['current', 'draft', 'confirmed', 'paid']

const MONTH_AMOUNT_KEYS = [
  'consumption',
  'revenue',
  'account_cost',
  'commission',
  'region_expense',
  'adjustment',
  'profit',
  'loss_before',
  'loss_after',
  'share_base',
  'share_amount'
] as const

const LINE_AMOUNT_KEYS = ['consumption', 'revenue', 'account_cost', 'commission', 'profit'] as const

export function normalizePartnerUserRef(value: unknown): PartnerUserRef | null {
  if (!value || typeof value !== 'object') return null
  const o = obj(value)
  return { id: int(o.id), email: str(o.email), username: str(o.username), deleted: o.deleted === true }
}

/** 团队页组件用的用户形状。 */
export function partnerRefToTeamUser(ref: PartnerUserRef | null): AffiliateTeamUser | null {
  return ref ? { user_id: ref.id, nickname: ref.username, email: ref.email, deleted: ref.deleted } : null
}

export function normalizePartnerMonthTotals(data: unknown): PartnerMonthTotals {
  const o = obj(data)
  return {
    month: str(o.month),
    status: STATUSES.find((s) => s === o.status) ?? 'current',
    share_rate: amount(o.share_rate),
    ...amounts(o, MONTH_AMOUNT_KEYS),
    confirmed_at: nullableStr(o.confirmed_at),
    paid_on: nullableStr(o.paid_on),
    paid_note: nullableStr(o.paid_note)
  }
}

export function normalizePartnerLine(raw: unknown): PartnerLine {
  const r = obj(raw)
  return { line_agent: normalizePartnerUserRef(r.line_agent), members: int(r.members), ...amounts(r, LINE_AMOUNT_KEYS) }
}

export function normalizePartnerAdjustment(raw: unknown): PartnerAdjustment {
  const r = obj(raw)
  return {
    id: int(r.id),
    amount: amount(r.amount),
    note: str(r.note),
    created_at: str(r.created_at),
    created_by_email: nullableStr(r.created_by_email)
  }
}

export function normalizePartnerStatementDetail(data: unknown): PartnerStatementDetail {
  const o = obj(data)
  return {
    totals: normalizePartnerMonthTotals(o.totals),
    lines: arr(o.lines).map(normalizePartnerLine),
    cost_by_group: arr(o.cost_by_group).map((raw) => {
      const r = obj(raw)
      return { group_id: int(r.group_id), group_name: str(r.group_name), ...amounts(r, ['consumption', 'account_cost'] as const) }
    }),
    cost_by_model: arr(o.cost_by_model).map((raw) => {
      const r = obj(raw)
      return { model: str(r.model), ...amounts(r, ['consumption', 'account_cost'] as const) }
    }),
    adjustments: arr(o.adjustments).map(normalizePartnerAdjustment),
    warnings: arr(o.warnings).filter((w): w is string => typeof w === 'string' && w !== '')
  }
}

export function normalizePartnerStatements(data: unknown): PartnerMonthTotals[] {
  return arr(obj(data).items).map(normalizePartnerMonthTotals)
}

export function normalizeAffiliatePartnerInfo(data: unknown): AffiliatePartnerInfo | null {
  if (!data || typeof data !== 'object') return null
  const o = obj(data)
  return {
    region_name: str(o.region_name),
    share_rate: amount(o.share_rate),
    next_share_rate: nullableStr(o.next_share_rate),
    next_rate_month: nullableStr(o.next_rate_month),
    since: str(o.since)
  }
}

export function normalizePartnerOverview(data: unknown): PartnerOverview {
  const o = obj(data)
  return {
    region_name: str(o.region_name),
    share_rate: amount(o.share_rate),
    next_share_rate: nullableStr(o.next_share_rate),
    next_rate_month: nullableStr(o.next_rate_month),
    since: str(o.since),
    loss_carried: amount(o.loss_carried),
    agent_count: int(o.agent_count),
    member_count: int(o.member_count),
    current_month: normalizePartnerMonthTotals(o.current_month),
    lines: arr(o.lines).map(normalizePartnerLine)
  }
}

export function normalizePartnerTeamLine(raw: unknown): PartnerTeamLine {
  const r = obj(raw)
  return {
    line_agent: partnerRefToTeamUser(normalizePartnerUserRef(r.line_agent)),
    customers: int(r.customers),
    active_customers: int(r.active_customers),
    requests: int(r.requests),
    team_cost: num(r.team_cost),
    cost_share: num(r.cost_share)
  }
}

/** 区域客户：在团队页客户结构上带归属代理与所在渠道经理线（不返回收益）。 */
export function normalizePartnerTeamCustomer(raw: unknown): AffiliateTeamCustomer {
  const r = obj(raw)
  return {
    ...(r as unknown as AffiliateTeamCustomer),
    owner: partnerRefToTeamUser(normalizePartnerUserRef(r.owner)),
    line_agent: partnerRefToTeamUser(normalizePartnerUserRef(r.line_agent))
  }
}

// ==================== 合伙人本人 ====================

export async function getOverview(options: FetchOptions = {}): Promise<PartnerOverview> {
  const { data } = await apiClient.get('/user/aff/partner/overview', { signal: options.signal })
  return normalizePartnerOverview(data)
}

export async function listStatements(options: FetchOptions = {}): Promise<PartnerMonthTotals[]> {
  const { data } = await apiClient.get('/user/aff/partner/statements', { signal: options.signal })
  return normalizePartnerStatements(data)
}

export async function getStatement(month: string, options: FetchOptions = {}): Promise<PartnerStatementDetail> {
  const { data } = await apiClient.get(`/user/aff/partner/statements/${month}`, { signal: options.signal })
  return normalizePartnerStatementDetail(data)
}

// ==================== 区域团队消费 ====================

const TEAM_BASE = '/user/aff/partner/team'

/** 以 base() 为根的团队页四个接口（与代理端同结构）；区域客户额外带归属列。 */
function teamSource(base: () => string): AffiliateTeamDataSource {
  return {
    async getOverview(params: AffiliateTeamOverviewParams, options: FetchOptions = {}) {
      const { data } = await apiClient.get<AffiliateTeamOverview>(`${base()}/overview`, { params, signal: options.signal })
      return data
    },
    async listCustomers(params: AffiliateTeamCustomersParams, options: FetchOptions = {}) {
      const { data } = await apiClient.get<BasePaginationResponse<AffiliateTeamCustomer>>(`${base()}/customers`, {
        params,
        signal: options.signal
      })
      return { ...data, items: arr(data.items).map(normalizePartnerTeamCustomer) }
    },
    async listAgents(params: AffiliateTeamAgentsParams, options: FetchOptions = {}) {
      const { data } = await apiClient.get<BasePaginationResponse<AffiliateTeamAgent>>(`${base()}/agents`, {
        params,
        signal: options.signal
      })
      return data
    },
    async listRequests(userId: number, params: AffiliateTeamRequestsParams, options: FetchOptions = {}) {
      const { data } = await apiClient.get<BasePaginationResponse<AffiliateTeamRequest>>(
        `${base()}/customers/${userId}/requests`,
        { params, signal: options.signal }
      )
      return data
    }
  }
}

/** 区域视角：整个区域。区域没有「下级代理」表，根页的下级列表用 listTeamLines。 */
export const partnerRegionTeamAPI: AffiliateTeamDataSource = teamSource(() => TEAM_BASE)

/** 以挂靠的渠道经理为视角；agentId 在每次调用时读取。 */
export function partnerAgentTeamAPI(agentId: () => number): AffiliateTeamDataSource {
  return teamSource(() => `${TEAM_BASE}/agents/${agentId()}`)
}

export async function listTeamLines(
  params: AffiliateTeamRangeParams,
  options: FetchOptions = {}
): Promise<PartnerTeamLine[]> {
  const { data } = await apiClient.get(`${TEAM_BASE}/lines`, { params, signal: options.signal })
  return arr(obj(data).items).map(normalizePartnerTeamLine)
}

export const affiliatePartnerAPI = {
  getOverview,
  listStatements,
  getStatement,
  listTeamLines
}

export default affiliatePartnerAPI
