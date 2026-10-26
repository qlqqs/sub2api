/**
 * CUSTOM: 财务明细页（管理员）。接口约定见任务 `.trellis/tasks/10-25-finance-ledger/contract.md`。
 *
 * 金额一律是十进制字符串、比例是字符串小数（分母为 0 时为 null），前端只展示不运算；
 * 这里只做形状兜底（缺字段补 "0"、数组补 []），不改数值。
 */

import { apiClient } from '../client'

export type FinanceAmount = string
export type FinanceGranularity = 'day' | 'month'
export type FinanceBreakdownDim = 'account' | 'group' | 'model' | 'user'
export type FinanceCostMode = 'rate' | 'model_rate' | 'per_image' | 'fixed'
export type FinanceExpenseCategory = 'upstream_subscription' | 'server' | 'domain' | 'other'

export const FINANCE_EXPENSE_CATEGORIES: FinanceExpenseCategory[] = [
  'upstream_subscription',
  'server',
  'domain',
  'other'
]

/** 契约 §8：订阅类别默认摊销 30 天，其他默认 1 天（当天全额）。 */
export function defaultAmortizeDays(category: FinanceExpenseCategory | string): number {
  return category === 'upstream_subscription' ? 30 : 1
}

export const FINANCE_MAX_RANGE_DAYS = 732

export interface FinanceRangeQuery {
  start: string
  end: string
}

export interface FinancePeriodTotals {
  revenue: FinanceAmount
  revenue_paid: FinanceAmount
  revenue_commission: FinanceAmount
  free_consumed: FinanceAmount
  overdraft_consumed: FinanceAmount
  consumption: FinanceAmount
  account_cost: FinanceAmount
  account_cost_internal: FinanceAmount
  commission: FinanceAmount
  expenses: FinanceAmount
  profit: FinanceAmount
  profit_margin: string | null
}

export interface FinanceSeriesRow extends FinancePeriodTotals {
  period: string
}

export interface FinanceSnapshot {
  balance_total: FinanceAmount
  balance_paid: FinanceAmount
  balance_paid_cash_value: FinanceAmount
  balance_commission: FinanceAmount
  balance_free: FinanceAmount
  balance_unattributed: FinanceAmount
  frozen_balance: FinanceAmount
  affiliate_available: FinanceAmount
  affiliate_frozen: FinanceAmount
}

export interface FinanceSummary {
  range: { start: string; end: string; granularity: FinanceGranularity }
  totals: FinancePeriodTotals
  series: FinanceSeriesRow[]
  snapshot: FinanceSnapshot
  warnings: string[]
  computed_at: string
}

export interface FinanceGiftTotals {
  recharge_bonus: FinanceAmount
  invitee_bonus: FinanceAmount
  promo: FinanceAmount
  admin_adjust_in: FinanceAmount
  admin_adjust_out: FinanceAmount
  redeem_code: FinanceAmount
  total: FinanceAmount
}

export interface FinanceRechargeOrder {
  id: number
  user_id: number
  user_email: string
  completed_at: string
  payment_type: string
  pay_amount: FinanceAmount
  channel_fee_rate: string | null
  channel_fee: FinanceAmount
  principal: FinanceAmount
  bonus: FinanceAmount
  refund_cash: FinanceAmount
  refund_balance_deducted: FinanceAmount
  status: string
}

export interface FinanceRecharge {
  totals: {
    order_count: number
    cash_received: FinanceAmount
    channel_fee: FinanceAmount
    refund_cash: FinanceAmount
    net_cash: FinanceAmount
    principal: FinanceAmount
    gift: FinanceGiftTotals
    commission_transfer_in: FinanceAmount
  }
  orders: FinanceRechargeOrder[]
  orders_truncated: boolean
}

export interface FinanceBreakdownRow {
  key: string
  label: string
  cost_mode?: FinanceCostMode | null
  requests: number
  images: number
  consumption: FinanceAmount
  subscription_consumption: FinanceAmount
  account_cost: FinanceAmount
  /** 仅 dim=account：期间内关联到该账号的其他支出摊销额。 */
  expense_cost?: FinanceAmount
  /** dim=account 时 = 余额消耗 − 账号成本 − 关联支出；其他维度 = 余额消耗 − 账号成本。 */
  gross_profit: FinanceAmount
}

export interface FinanceBreakdown {
  dim: FinanceBreakdownDim
  rows: FinanceBreakdownRow[]
  truncated: boolean
}

export interface FinanceAffiliateBeneficiary {
  user_id: number
  user_email: string
  accrued: FinanceAmount
  transferred: FinanceAmount
  withdrawn: FinanceAmount
  available: FinanceAmount
}

export interface FinanceAffiliate {
  accrued: {
    agent_direct: FinanceAmount
    agent_diff: FinanceAmount
    invite_percent: FinanceAmount
    invite_fixed: FinanceAmount
    total: FinanceAmount
  }
  transferred: FinanceAmount
  withdrawn: FinanceAmount
  available: FinanceAmount
  frozen: FinanceAmount
  top_beneficiaries: FinanceAffiliateBeneficiary[]
}

export interface FinanceReconcileRow {
  user_id: number
  user_email: string
  role: string
  deleted: boolean
  paid_in: FinanceAmount
  paid_cash: FinanceAmount
  free_in: FinanceAmount
  free_out: FinanceAmount
  commission_in: FinanceAmount
  refund_deducted: FinanceAmount
  consumption: FinanceAmount
  overdraft: FinanceAmount
  expected_balance: FinanceAmount
  actual_balance: FinanceAmount
  difference: FinanceAmount
  remaining: { paid: FinanceAmount; commission: FinanceAmount; free: FinanceAmount }
}

export interface FinancePage<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface FinanceReconcileQuery {
  page?: number
  page_size?: number
  only_mismatch?: boolean
  search?: string
}

export interface FinanceSettingsAccount {
  id: number
  name: string
  platform: string
  deleted: boolean
}

export interface FinanceSettings {
  fixed_cost_account_ids: number[]
  accounts: FinanceSettingsAccount[]
}

export interface FinanceExpenseInput {
  occurred_on: string
  category: FinanceExpenseCategory
  amount: FinanceAmount
  amortize_days: number
  account_id: number | null
  note: string
}

export interface FinanceExpense extends FinanceExpenseInput {
  id: number
  account_name: string | null
  daily_amount: FinanceAmount
  created_by_email: string | null
  created_at: string
  updated_at: string
}

export interface FinanceExpenseQuery {
  start?: string
  end?: string
  category?: FinanceExpenseCategory | ''
  page?: number
  page_size?: number
}

// ==================== 形状兜底 ====================

type Loose = Record<string, unknown> | null | undefined

function obj(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

function arr(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

/** 金额字段：字符串原样保留；数字转字符串（兼容）；缺省为 "0"。 */
function amount(value: unknown): FinanceAmount {
  if (typeof value === 'string' && value.trim() !== '') return value
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return '0'
}

function ratio(value: unknown): string | null {
  if (typeof value === 'string' && value.trim() !== '') return value
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return null
}

function int(value: unknown): number {
  const n = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(n) ? n : 0
}

function str(value: unknown): string {
  return typeof value === 'string' ? value : value == null ? '' : String(value)
}

function nullableStr(value: unknown): string | null {
  return typeof value === 'string' && value !== '' ? value : null
}

function amounts<K extends string>(source: Loose, keys: readonly K[]): Record<K, FinanceAmount> {
  const o = obj(source)
  return Object.fromEntries(keys.map((key) => [key, amount(o[key])])) as Record<K, FinanceAmount>
}

const PERIOD_AMOUNT_KEYS = [
  'revenue',
  'revenue_paid',
  'revenue_commission',
  'free_consumed',
  'overdraft_consumed',
  'consumption',
  'account_cost',
  'account_cost_internal',
  'commission',
  'expenses',
  'profit'
] as const

function periodTotals(source: unknown): FinancePeriodTotals {
  return { ...amounts(obj(source), PERIOD_AMOUNT_KEYS), profit_margin: ratio(obj(source).profit_margin) }
}

const SNAPSHOT_KEYS = [
  'balance_total',
  'balance_paid',
  'balance_paid_cash_value',
  'balance_commission',
  'balance_free',
  'balance_unattributed',
  'frozen_balance',
  'affiliate_available',
  'affiliate_frozen'
] as const

export function normalizeSummary(data: unknown): FinanceSummary {
  const o = obj(data)
  const range = obj(o.range)
  return {
    range: {
      start: str(range.start),
      end: str(range.end),
      granularity: range.granularity === 'month' ? 'month' : 'day'
    },
    totals: periodTotals(o.totals),
    series: arr(o.series).map((row) => ({ period: str(obj(row).period), ...periodTotals(row) })),
    snapshot: amounts(obj(o.snapshot), SNAPSHOT_KEYS),
    warnings: arr(o.warnings).filter((w): w is string => typeof w === 'string' && w !== ''),
    computed_at: str(o.computed_at)
  }
}

const GIFT_KEYS = [
  'recharge_bonus',
  'invitee_bonus',
  'promo',
  'admin_adjust_in',
  'admin_adjust_out',
  'redeem_code',
  'total'
] as const

export function normalizeRecharge(data: unknown): FinanceRecharge {
  const o = obj(data)
  const totals = obj(o.totals)
  return {
    totals: {
      order_count: int(totals.order_count),
      ...amounts(totals, ['cash_received', 'channel_fee', 'refund_cash', 'net_cash', 'principal'] as const),
      gift: amounts(obj(totals.gift), GIFT_KEYS),
      commission_transfer_in: amount(totals.commission_transfer_in)
    },
    orders: arr(o.orders).map((raw) => {
      const r = obj(raw)
      return {
        id: int(r.id),
        user_id: int(r.user_id),
        user_email: str(r.user_email),
        completed_at: str(r.completed_at),
        payment_type: str(r.payment_type),
        channel_fee_rate: ratio(r.channel_fee_rate),
        status: str(r.status),
        ...amounts(r, [
          'pay_amount',
          'channel_fee',
          'principal',
          'bonus',
          'refund_cash',
          'refund_balance_deducted'
        ] as const)
      }
    }),
    orders_truncated: o.orders_truncated === true
  }
}

const COST_MODES: FinanceCostMode[] = ['rate', 'model_rate', 'per_image', 'fixed']

export function normalizeBreakdown(data: unknown, fallbackDim: FinanceBreakdownDim): FinanceBreakdown {
  const o = obj(data)
  const dim = (['account', 'group', 'model', 'user'] as const).find((d) => d === o.dim) ?? fallbackDim
  return {
    dim,
    rows: arr(o.rows).map((raw) => {
      const r = obj(raw)
      const mode = COST_MODES.find((m) => m === r.cost_mode) ?? null
      return {
        key: str(r.key),
        label: str(r.label) || str(r.key),
        cost_mode: mode,
        requests: int(r.requests),
        images: int(r.images),
        ...amounts(r, ['consumption', 'subscription_consumption', 'account_cost', 'gross_profit'] as const)
      }
    }),
    truncated: o.truncated === true
  }
}

export function normalizeAffiliate(data: unknown): FinanceAffiliate {
  const o = obj(data)
  return {
    accrued: amounts(obj(o.accrued), ['agent_direct', 'agent_diff', 'invite_percent', 'invite_fixed', 'total'] as const),
    ...amounts(o, ['transferred', 'withdrawn', 'available', 'frozen'] as const),
    top_beneficiaries: arr(o.top_beneficiaries).map((raw) => {
      const r = obj(raw)
      return {
        user_id: int(r.user_id),
        user_email: str(r.user_email),
        ...amounts(r, ['accrued', 'transferred', 'withdrawn', 'available'] as const)
      }
    })
  }
}

function page<T>(data: unknown, mapItem: (raw: unknown) => T): FinancePage<T> {
  const o = obj(data)
  return {
    items: arr(o.items).map(mapItem),
    total: int(o.total),
    page: int(o.page) || 1,
    page_size: int(o.page_size),
    pages: int(o.pages)
  }
}

export function normalizeReconcileRow(raw: unknown): FinanceReconcileRow {
  const r = obj(raw)
  return {
    user_id: int(r.user_id),
    user_email: str(r.user_email),
    role: str(r.role),
    deleted: r.deleted === true,
    ...amounts(r, [
      'paid_in',
      'paid_cash',
      'free_in',
      'free_out',
      'commission_in',
      'refund_deducted',
      'consumption',
      'overdraft',
      'expected_balance',
      'actual_balance',
      'difference'
    ] as const),
    remaining: amounts(obj(r.remaining), ['paid', 'commission', 'free'] as const)
  }
}

export function normalizeExpense(raw: unknown): FinanceExpense {
  const r = obj(raw)
  const category = FINANCE_EXPENSE_CATEGORIES.find((c) => c === r.category) ?? 'other'
  const accountId = r.account_id == null || r.account_id === '' ? null : int(r.account_id)
  return {
    id: int(r.id),
    occurred_on: str(r.occurred_on),
    category,
    amount: amount(r.amount),
    amortize_days: int(r.amortize_days) || 1,
    account_id: accountId,
    account_name: nullableStr(r.account_name),
    note: str(r.note),
    daily_amount: amount(r.daily_amount),
    created_by_email: nullableStr(r.created_by_email),
    created_at: str(r.created_at),
    updated_at: str(r.updated_at)
  }
}

export function normalizeSettings(data: unknown): FinanceSettings {
  const o = obj(data)
  return {
    fixed_cost_account_ids: arr(o.fixed_cost_account_ids).map(int).filter((id) => id > 0),
    accounts: arr(o.accounts).map((raw) => {
      const r = obj(raw)
      return { id: int(r.id), name: str(r.name), platform: str(r.platform), deleted: r.deleted === true }
    })
  }
}

/** 差额是否视为不平：|difference| ≥ 0.01（契约 §5）。按字符串判断，不经浮点。 */
export function isReconcileMismatch(difference: FinanceAmount): boolean {
  const text = String(difference ?? '').trim().replace(/^[+-]/, '')
  const match = /^(\d*)(?:\.(\d*))?$/.exec(text)
  if (!match) return false
  const intPart = match[1].replace(/^0+/, '')
  if (intPart !== '') return true
  return /[1-9]/.test((match[2] ?? '').slice(0, 2))
}

// ==================== 请求 ====================

export async function getSummary(
  query: FinanceRangeQuery & { granularity: FinanceGranularity },
  options: { signal?: AbortSignal } = {}
): Promise<FinanceSummary> {
  const { data } = await apiClient.get('/admin/finance/summary', { params: query, signal: options.signal })
  return normalizeSummary(data)
}

export async function getRecharge(query: FinanceRangeQuery, options: { signal?: AbortSignal } = {}): Promise<FinanceRecharge> {
  const { data } = await apiClient.get('/admin/finance/recharge', { params: query, signal: options.signal })
  return normalizeRecharge(data)
}

export async function getBreakdown(
  query: FinanceRangeQuery & { dim: FinanceBreakdownDim; limit?: number },
  options: { signal?: AbortSignal } = {}
): Promise<FinanceBreakdown> {
  const { data } = await apiClient.get('/admin/finance/breakdown', { params: query, signal: options.signal })
  return normalizeBreakdown(data, query.dim)
}

export async function getAffiliate(query: FinanceRangeQuery, options: { signal?: AbortSignal } = {}): Promise<FinanceAffiliate> {
  const { data } = await apiClient.get('/admin/finance/affiliate', { params: query, signal: options.signal })
  return normalizeAffiliate(data)
}

export async function getReconcile(
  query: FinanceReconcileQuery,
  options: { signal?: AbortSignal } = {}
): Promise<FinancePage<FinanceReconcileRow>> {
  const params: Record<string, string | number | boolean> = {
    page: query.page ?? 1,
    page_size: query.page_size ?? 50,
    only_mismatch: query.only_mismatch === true
  }
  const search = query.search?.trim()
  if (search) params.search = search
  const { data } = await apiClient.get('/admin/finance/reconcile', { params, signal: options.signal })
  return page(data, normalizeReconcileRow)
}

/** CSV 导出（带 BOM），返回 Blob，由页面触发下载。 */
export async function exportCsv(query: FinanceRangeQuery & { granularity: FinanceGranularity }): Promise<Blob> {
  const response = await apiClient.get('/admin/finance/export', { params: query, responseType: 'blob' })
  return response.data as Blob
}

export async function getSettings(): Promise<FinanceSettings> {
  const { data } = await apiClient.get('/admin/finance/settings')
  return normalizeSettings(data)
}

export async function updateSettings(payload: { fixed_cost_account_ids: number[] }): Promise<FinanceSettings> {
  const { data } = await apiClient.put('/admin/finance/settings', payload)
  return normalizeSettings(data)
}

export async function listExpenses(
  query: FinanceExpenseQuery,
  options: { signal?: AbortSignal } = {}
): Promise<FinancePage<FinanceExpense>> {
  const params: Record<string, string | number> = { page: query.page ?? 1, page_size: query.page_size ?? 20 }
  if (query.start) params.start = query.start
  if (query.end) params.end = query.end
  if (query.category) params.category = query.category
  const { data } = await apiClient.get('/admin/finance/expenses', { params, signal: options.signal })
  return page(data, normalizeExpense)
}

export async function createExpense(payload: FinanceExpenseInput): Promise<FinanceExpense> {
  const { data } = await apiClient.post('/admin/finance/expenses', payload)
  return normalizeExpense(data)
}

export async function updateExpense(id: number, payload: FinanceExpenseInput): Promise<FinanceExpense> {
  const { data } = await apiClient.put(`/admin/finance/expenses/${id}`, payload)
  return normalizeExpense(data)
}

export async function deleteExpense(id: number): Promise<void> {
  await apiClient.delete(`/admin/finance/expenses/${id}`)
}

export const financeAPI = {
  getSummary,
  getRecharge,
  getBreakdown,
  getAffiliate,
  getReconcile,
  exportCsv,
  getSettings,
  updateSettings,
  listExpenses,
  createExpense,
  updateExpense,
  deleteExpense
}

export default financeAPI
