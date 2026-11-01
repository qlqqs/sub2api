/**
 * CUSTOM: 合伙人管理端接口：/api/v1/admin/affiliates/partners/*。
 * 接口约定见 `.trellis/tasks/10-31-affiliate-partner/contract.md` §1；金额、比例都是字符串，只展示不运算。
 */

import { apiClient } from '../client'
import {
  normalizePartnerAdjustment,
  normalizePartnerMonthTotals,
  normalizePartnerStatementDetail,
  normalizePartnerStatements,
  normalizePartnerUserRef,
  type PartnerAdjustment,
  type PartnerAmount,
  type PartnerMonthTotals,
  type PartnerStatementDetail,
  type PartnerUserRef
} from '../affiliatePartner'

export interface PartnerTeamSize {
  /** agent_owner_id = 他的人数。 */
  owned: number
  /** agent_upline_id = 他的人数。 */
  via_upline: number
}

/** 按快照当前所属的区域（仅作提示）。 */
export interface PartnerRegionRef {
  partner: PartnerUserRef
  region_name: string
}

export interface AdminPartnerItem {
  user: PartnerUserRef
  region_name: string
  share_rate: string
  next_share_rate: string | null
  next_rate_month: string | null
  since: string
  agent_count: number
  member_count: number
  /** 截至最后一个已结束月份的期末累计亏损。 */
  loss_carried: PartnerAmount
  /** 已结束但未确认的月份数。 */
  unconfirmed_months: number
  current_month: PartnerMonthTotals
}

export type PartnerAttachSource = 'manual' | 'promote'

export interface PartnerAgentMonth {
  consumption: PartnerAmount
  revenue: PartnerAmount
  account_cost: PartnerAmount
  commission: PartnerAmount
  profit: PartnerAmount
}

export interface AdminPartnerAgentItem {
  user: PartnerUserRef
  attached_at: string
  source: PartnerAttachSource
  /** 升级自动挂靠且为自动升级时为 null。 */
  attached_by_email: string | null
  agent_level: number
  team: PartnerTeamSize
  current_month: PartnerAgentMonth
}

export interface AdminPartnerDetail extends AdminPartnerItem {
  agents: AdminPartnerAgentItem[]
}

export interface PartnerCreatePreview {
  user: PartnerUserRef
  agent_level: number
  is_partner: boolean
  is_attached: boolean
  team: PartnerTeamSize
  current_region: PartnerRegionRef | null
  can_create: boolean
  /** 不能设立时的原因。 */
  reason: string | null
}

export interface PartnerAttachPreview {
  agent: PartnerUserRef
  agent_level: number
  team: PartnerTeamSize
  current_region: PartnerRegionRef | null
  can_attach: boolean
  reason: string | null
}

export interface CreatePartnerRequest {
  user_id: number
  region_name: string
  /** 小数，"0.4000" = 40%。 */
  share_rate: string
}

export interface UpdatePartnerRequest {
  region_name?: string
  share_rate?: string
}

export interface PayStatementRequest {
  /** YYYY-MM-DD；省略为今天（北京时间）。 */
  paid_on?: string
  note: string
}

export interface CreateAdjustmentRequest {
  month: string
  /** 两位小数，可正可负，不为 0。 */
  amount: string
  note: string
}

// ==================== 形状兜底 ====================

function obj(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
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

function amount(value: unknown): PartnerAmount {
  if (typeof value === 'string' && value.trim() !== '') return value
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return '0'
}

const EMPTY_USER: PartnerUserRef = { id: 0, email: '', username: '', deleted: false }

function userRef(value: unknown): PartnerUserRef {
  return normalizePartnerUserRef(value) ?? EMPTY_USER
}

function teamSize(value: unknown): PartnerTeamSize {
  const o = obj(value)
  return { owned: int(o.owned), via_upline: int(o.via_upline) }
}

export function normalizePartnerRegionRef(value: unknown): PartnerRegionRef | null {
  if (!value || typeof value !== 'object') return null
  const o = obj(value)
  return { partner: userRef(o.partner), region_name: str(o.region_name) }
}

export function normalizeAdminPartnerItem(data: unknown): AdminPartnerItem {
  const o = obj(data)
  return {
    user: userRef(o.user),
    region_name: str(o.region_name),
    share_rate: amount(o.share_rate),
    next_share_rate: nullableStr(o.next_share_rate),
    next_rate_month: nullableStr(o.next_rate_month),
    since: str(o.since),
    agent_count: int(o.agent_count),
    member_count: int(o.member_count),
    loss_carried: amount(o.loss_carried),
    unconfirmed_months: int(o.unconfirmed_months),
    current_month: normalizePartnerMonthTotals(o.current_month)
  }
}

export function normalizeAdminPartnerAgentItem(data: unknown): AdminPartnerAgentItem {
  const o = obj(data)
  const month = obj(o.current_month)
  return {
    user: userRef(o.user),
    attached_at: str(o.attached_at),
    source: o.source === 'promote' ? 'promote' : 'manual',
    attached_by_email: nullableStr(o.attached_by_email),
    agent_level: int(o.agent_level),
    team: teamSize(o.team),
    current_month: {
      consumption: amount(month.consumption),
      revenue: amount(month.revenue),
      account_cost: amount(month.account_cost),
      commission: amount(month.commission),
      profit: amount(month.profit)
    }
  }
}

export function normalizeAdminPartnerDetail(data: unknown): AdminPartnerDetail {
  const o = obj(data)
  const agents = Array.isArray(o.agents) ? o.agents : []
  return { ...normalizeAdminPartnerItem(o), agents: agents.map(normalizeAdminPartnerAgentItem) }
}

export function normalizePartnerCreatePreview(data: unknown): PartnerCreatePreview {
  const o = obj(data)
  return {
    user: userRef(o.user),
    agent_level: int(o.agent_level),
    is_partner: o.is_partner === true,
    is_attached: o.is_attached === true,
    team: teamSize(o.team),
    current_region: normalizePartnerRegionRef(o.current_region),
    can_create: o.can_create === true,
    reason: nullableStr(o.reason)
  }
}

export function normalizePartnerAttachPreview(data: unknown): PartnerAttachPreview {
  const o = obj(data)
  return {
    agent: userRef(o.agent),
    agent_level: int(o.agent_level),
    team: teamSize(o.team),
    current_region: normalizePartnerRegionRef(o.current_region),
    can_attach: o.can_attach === true,
    reason: nullableStr(o.reason)
  }
}

// ==================== 请求 ====================

const BASE = '/admin/affiliates/partners'

export async function listPartners(options: { signal?: AbortSignal } = {}): Promise<AdminPartnerItem[]> {
  const { data } = await apiClient.get(BASE, { signal: options.signal })
  const items = obj(data).items
  return (Array.isArray(items) ? items : []).map(normalizeAdminPartnerItem)
}

export async function previewPartner(userId: number, options: { signal?: AbortSignal } = {}): Promise<PartnerCreatePreview> {
  const { data } = await apiClient.get(`${BASE}/preview`, { params: { user_id: userId }, signal: options.signal })
  return normalizePartnerCreatePreview(data)
}

export async function createPartner(payload: CreatePartnerRequest): Promise<AdminPartnerItem> {
  const { data } = await apiClient.post(BASE, payload)
  return normalizeAdminPartnerItem(data)
}

export async function getPartner(userId: number, options: { signal?: AbortSignal } = {}): Promise<AdminPartnerDetail> {
  const { data } = await apiClient.get(`${BASE}/${userId}`, { signal: options.signal })
  return normalizeAdminPartnerDetail(data)
}

export async function updatePartner(userId: number, payload: UpdatePartnerRequest): Promise<AdminPartnerItem> {
  const { data } = await apiClient.put(`${BASE}/${userId}`, payload)
  return normalizeAdminPartnerItem(data)
}

export async function previewAttach(
  partnerId: number,
  agentUserId: number,
  options: { signal?: AbortSignal } = {}
): Promise<PartnerAttachPreview> {
  const { data } = await apiClient.get(`${BASE}/${partnerId}/agents/preview`, {
    params: { agent_user_id: agentUserId },
    signal: options.signal
  })
  return normalizePartnerAttachPreview(data)
}

export async function attachAgent(partnerId: number, agentUserId: number): Promise<AdminPartnerAgentItem> {
  const { data } = await apiClient.post(`${BASE}/${partnerId}/agents`, { agent_user_id: agentUserId })
  return normalizeAdminPartnerAgentItem(data)
}

export async function listStatements(partnerId: number, options: { signal?: AbortSignal } = {}): Promise<PartnerMonthTotals[]> {
  const { data } = await apiClient.get(`${BASE}/${partnerId}/statements`, { signal: options.signal })
  return normalizePartnerStatements(data)
}

export async function getStatement(
  partnerId: number,
  month: string,
  options: { signal?: AbortSignal } = {}
): Promise<PartnerStatementDetail> {
  const { data } = await apiClient.get(`${BASE}/${partnerId}/statements/${month}`, { signal: options.signal })
  return normalizePartnerStatementDetail(data)
}

export async function confirmStatement(partnerId: number, month: string): Promise<PartnerStatementDetail> {
  const { data } = await apiClient.post(`${BASE}/${partnerId}/statements/${month}/confirm`)
  return normalizePartnerStatementDetail(data)
}

export async function payStatement(partnerId: number, month: string, payload: PayStatementRequest): Promise<PartnerStatementDetail> {
  const { data } = await apiClient.post(`${BASE}/${partnerId}/statements/${month}/pay`, payload)
  return normalizePartnerStatementDetail(data)
}

export async function createAdjustment(partnerId: number, payload: CreateAdjustmentRequest): Promise<PartnerAdjustment> {
  const { data } = await apiClient.post(`${BASE}/${partnerId}/adjustments`, payload)
  return normalizePartnerAdjustment(data)
}

export async function deleteAdjustment(partnerId: number, adjustmentId: number): Promise<void> {
  await apiClient.delete(`${BASE}/${partnerId}/adjustments/${adjustmentId}`)
}

/** CSV（UTF-8 BOM），返回 Blob，由页面触发下载。 */
export async function exportStatement(partnerId: number, month: string): Promise<Blob> {
  const response = await apiClient.get(`${BASE}/${partnerId}/statements/${month}/export`, { responseType: 'blob' })
  return response.data as Blob
}

export const affiliatePartnersAPI = {
  listPartners,
  previewPartner,
  createPartner,
  getPartner,
  updatePartner,
  previewAttach,
  attachAgent,
  listStatements,
  getStatement,
  confirmStatement,
  payStatement,
  createAdjustment,
  deleteAdjustment,
  exportStatement
}

export default affiliatePartnersAPI
