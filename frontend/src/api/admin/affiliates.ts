/**
 * Admin Affiliate API endpoints
 * Manage per-user affiliate (邀请返利) configurations:
 * exclusive invite codes (overrides aff_code) and exclusive rebate rates.
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

/** 1 = 一级代理, 2 = 二级代理, 3 = 普通用户. */
export type AgentLevel = 1 | 2 | 3

/** default | auto | manual | agent (appointed by a level-1 agent). */
export type AgentLevelSource = 'default' | 'auto' | 'manual' | 'agent'

/**
 * Agent ownership snapshot frozen when the invite relation was bound.
 * `*_deleted` is true when the referenced account no longer exists.
 */
export interface AffiliateAgentSnapshot {
  agent_owner_id: number | null
  agent_owner_email: string
  agent_owner_level: number | null
  agent_owner_deleted: boolean
  agent_upline_id: number | null
  agent_upline_email: string
  agent_upline_deleted: boolean
}

export interface AffiliateAdminEntry extends AffiliateAgentSnapshot {
  user_id: number
  email: string
  username: string
  aff_code: string
  aff_code_custom: boolean
  aff_rebate_rate_percent?: number | null
  aff_count: number
  agent_level: number
  agent_level_locked: boolean
  agent_level_source: AgentLevelSource | string
  agent_level_updated_by: number | null
  agent_level_updated_by_email: string
  owned_customer_count: number
  /** agent_direct + agent_diff received as beneficiary. */
  agent_profit_total: number
}

export interface ListAffiliateUsersParams {
  page?: number
  page_size?: number
  search?: string
  agent_level?: AgentLevel | null
  /** Filter by owning (upline) agent id. */
  owner_id?: number | null
}

/** One customer of an agent: owner = the agent owns the customer, upline = the agent earns the diff. */
export interface AffiliateAgentCustomer {
  user_id: number
  email: string
  username: string
  agent_level: number
  relation: 'owner' | 'upline'
  bound_at: string
  contribution: number
}

/** Ownership snapshot that a bind would write. Ids are null when there is no owner / upline. */
export interface AffiliateInviterPreview {
  agent_owner_id: number | null
  agent_owner_email: string
  agent_owner_level: number | null
  agent_upline_id: number | null
  agent_upline_email: string
}

export type AffiliateRewardType =
  | 'agent_direct'
  | 'agent_diff'
  | 'invite_percent'
  | 'invite_fixed'
  | 'invitee_bonus'

export const AFFILIATE_REWARD_TYPES: readonly AffiliateRewardType[] = [
  'agent_direct',
  'agent_diff',
  'invite_percent',
  'invite_fixed',
  'invitee_bonus',
]

export interface ListAffiliateRecordsParams {
  page?: number
  page_size?: number
  search?: string
  start_at?: string
  end_at?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
  timezone?: string
  /** Rebate records only. */
  reward_type?: AffiliateRewardType | ''
}

export interface AffiliateInviteRecord extends AffiliateAgentSnapshot {
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number
  invitee_email: string
  invitee_username: string
  aff_code: string
  total_rebate: number
  created_at: string
}

/**
 * One rebate accrual. Non-order sources (redeem codes, admin recharges) carry no
 * order, so order fields are null; invitee_id is null when the invitee account
 * was deleted.
 */
export interface AffiliateRebateRecord {
  order_id: number | null
  out_trade_no: string
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number | null
  invitee_email: string
  invitee_username: string
  order_amount: number | null
  pay_amount: number | null
  rebate_amount: number
  reward_type: AffiliateRewardType | string | null
  payment_type: string
  order_status: string
  created_at: string
}

/** transfer = user moved quota into balance; withdraw = admin-recorded offline withdrawal. */
export type AffiliateOutflowAction = 'transfer' | 'withdraw'

/** One affiliate quota outflow: a transfer into balance or an offline withdrawal. */
export interface AffiliateTransferRecord {
  ledger_id: number
  action: AffiliateOutflowAction
  user_id: number
  user_email: string
  username: string
  amount: number
  balance_after?: number | null
  available_quota_after?: number | null
  frozen_quota_after?: number | null
  history_quota_after?: number | null
  snapshot_available: boolean
  created_at: string
}

export interface WithdrawAffiliateQuotaRequest {
  /** Amount already paid to the user outside the site (USD). */
  amount: number
}

export interface AffiliateWithdrawResult {
  ledger_id: number
  user_id: number
  amount: number
  available_quota_after: number
  frozen_quota_after: number
  history_quota_after: number
}

export interface AffiliateWithdrawResponse {
  result: AffiliateWithdrawResult
  /** True when the key matched an earlier registration and nothing was deducted again. */
  replayed: boolean
}

export interface AffiliateUserOverview extends Partial<AffiliateAgentSnapshot> {
  user_id: number
  email: string
  username: string
  aff_code: string
  /** Only for agents (level 1/2); null otherwise. */
  agent_rate_percent?: number | null
  owned_customer_count?: number
  reward_totals?: { agent: number; invite: number; invitee_bonus: number } | null
  invited_count: number
  rebated_invitee_count: number
  available_quota: number
  history_quota: number
  agent_level: number
  agent_level_locked: boolean
  agent_level_source: string
}

export interface UpdateAffiliateUserRequest {
  aff_code?: string
  aff_rebate_rate_percent?: number | null
  /** Set true to explicitly clear the per-user rate (sets it to NULL). */
  clear_rebate_rate?: boolean
  /** Agent level 1-3. Demotion locks auto-upgrade, promotion unlocks. */
  agent_level?: number
  /** Explicitly lock/unlock auto-upgrade; applied after agent_level. */
  agent_level_locked?: boolean
}

export interface BatchSetAgentLevelRequest {
  user_ids: number[]
  agent_level: number
}

export interface ReevaluateAgentLevelsResponse {
  evaluated: number
  promoted: number
}

export interface BatchSetRateRequest {
  user_ids: number[]
  aff_rebate_rate_percent?: number | null
  /** Set true to clear rates instead of setting. */
  clear?: boolean
}

export interface BindInviterRequest {
  inviter_id: number
  reason: string
}

export interface BindInviterResponse {
  invitee_id: number
  inviter_id: number
  bound_at: string
  source: 'admin' | 'registration'
}

export interface SimpleUser {
  id: number
  email: string
  username: string
}

export async function listUsers(
  params: ListAffiliateUsersParams = {},
): Promise<PaginatedResponse<AffiliateAdminEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateAdminEntry>>(
    '/admin/affiliates/users',
    {
      params: {
        page: params.page ?? 1,
        page_size: params.page_size ?? 20,
        search: params.search ?? '',
        agent_level: params.agent_level || undefined,
        owner_id: params.owner_id || undefined,
      },
    },
  )
  return data
}

export async function listUserCustomers(
  userId: number,
  params: { page?: number; page_size?: number } = {},
): Promise<PaginatedResponse<AffiliateAgentCustomer>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateAgentCustomer>>(
    `/admin/affiliates/users/${userId}/customers`,
    { params: { page: params.page ?? 1, page_size: params.page_size ?? 20 } },
  )
  return data
}

export async function previewInviter(
  inviteeId: number,
  inviterId: number,
  options: { signal?: AbortSignal } = {},
): Promise<AffiliateInviterPreview> {
  const { data } = await apiClient.get<AffiliateInviterPreview>(
    `/admin/affiliates/users/${inviteeId}/inviter/preview`,
    { params: { inviter_id: inviterId }, signal: options.signal },
  )
  return data
}

export async function lookupUsers(
  q: string,
  options: { signal?: AbortSignal } = {},
): Promise<SimpleUser[]> {
  const { data } = await apiClient.get<SimpleUser[]>(
    '/admin/affiliates/users/lookup',
    { params: { q }, signal: options.signal },
  )
  return data
}

export async function bindInviter(
  inviteeId: number,
  payload: BindInviterRequest,
): Promise<BindInviterResponse> {
  const { data } = await apiClient.post<BindInviterResponse>(
    `/admin/affiliates/users/${inviteeId}/inviter`,
    payload,
  )
  return data
}

export async function updateUserSettings(
  userId: number,
  payload: UpdateAffiliateUserRequest,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.put<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
    payload,
  )
  return data
}

export async function clearUserSettings(
  userId: number,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.delete<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
  )
  return data
}

export async function batchSetRate(
  payload: BatchSetRateRequest,
): Promise<{ affected: number }> {
  const { data } = await apiClient.post<{ affected: number }>(
    '/admin/affiliates/users/batch-rate',
    payload,
  )
  return data
}

export async function batchSetAgentLevel(
  payload: BatchSetAgentLevelRequest,
): Promise<{ affected: number }> {
  const { data } = await apiClient.post<{ affected: number }>(
    '/admin/affiliates/users/batch-agent-level',
    payload,
  )
  return data
}

export async function reevaluateAgentLevels(): Promise<ReevaluateAgentLevelsResponse> {
  const { data } = await apiClient.post<ReevaluateAgentLevelsResponse>(
    '/admin/affiliates/agent-levels/reevaluate',
  )
  return data
}

function recordParams(params: ListAffiliateRecordsParams = {}) {
  return {
    page: params.page ?? 1,
    page_size: params.page_size ?? 20,
    search: params.search ?? '',
    start_at: params.start_at || undefined,
    end_at: params.end_at || undefined,
    sort_by: params.sort_by || undefined,
    sort_order: params.sort_order || undefined,
    timezone: params.timezone || undefined,
    reward_type: params.reward_type || undefined,
  }
}

export async function listInviteRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateInviteRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateInviteRecord>>(
    '/admin/affiliates/invites',
    { params: recordParams(params) },
  )
  return data
}

export async function listRebateRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateRebateRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateRebateRecord>>(
    '/admin/affiliates/rebates',
    { params: recordParams(params) },
  )
  return data
}

export async function listTransferRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateTransferRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateTransferRecord>>(
    '/admin/affiliates/transfers',
    { params: recordParams(params) },
  )
  return data
}

export async function getUserOverview(
  userId: number,
): Promise<AffiliateUserOverview> {
  const { data } = await apiClient.get<AffiliateUserOverview>(
    `/admin/affiliates/users/${userId}/overview`,
  )
  return data
}

/**
 * Records an offline withdrawal. `idempotencyKey` identifies one registration:
 * a retry with the same key deducts nothing and returns the first result, with
 * `replayed` set.
 */
export async function withdrawUserQuota(
  userId: number,
  payload: WithdrawAffiliateQuotaRequest,
  idempotencyKey: string,
): Promise<AffiliateWithdrawResponse> {
  const response = await apiClient.post<AffiliateWithdrawResult>(
    `/admin/affiliates/users/${userId}/withdraw`,
    payload,
    { headers: { 'Idempotency-Key': idempotencyKey } },
  )
  return {
    result: response.data,
    replayed: response.headers?.['x-idempotency-replayed'] === 'true',
  }
}

export const affiliatesAPI = {
  listUsers,
  listUserCustomers,
  previewInviter,
  lookupUsers,
  bindInviter,
  updateUserSettings,
  clearUserSettings,
  batchSetRate,
  batchSetAgentLevel,
  reevaluateAgentLevels,
  listInviteRecords,
  listRebateRecords,
  listTransferRecords,
  getUserOverview,
  withdrawUserQuota,
}

export default affiliatesAPI
