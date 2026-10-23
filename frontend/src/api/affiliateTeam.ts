/**
 * 代理团队消费页：/api/v1/user/aff/team/*（只读，按归属快照限定在当前用户的团队内）。
 */

import { apiClient } from './client'
import type {
  AffiliateTeamAgent,
  AffiliateTeamAgentSort,
  AffiliateTeamCustomer,
  AffiliateTeamCustomerSort,
  AffiliateTeamOverview,
  AffiliateTeamRequest,
  AffiliateTeamSortOrder,
  BasePaginationResponse,
  FetchOptions
} from '@/types'

/** 闭区间日期 YYYY-MM-DD（后端按北京时间自然日解析，跨度 1–90 天）。 */
export interface AffiliateTeamRangeParams {
  start_date: string
  end_date: string
}

/** agent_id 与 user_id 互斥；都不传为团队总览。 */
export interface AffiliateTeamOverviewParams extends AffiliateTeamRangeParams {
  agent_id?: number
  user_id?: number
}

export interface AffiliateTeamPageParams {
  page?: number
  page_size?: number
}

export interface AffiliateTeamCustomersParams extends AffiliateTeamRangeParams, AffiliateTeamPageParams {
  /** 不传为「我的客户」；传入为该二级代理在我下面的团队。 */
  agent_id?: number
  search?: string
  sort_by?: AffiliateTeamCustomerSort
  sort_order?: AffiliateTeamSortOrder
}

export interface AffiliateTeamAgentsParams extends AffiliateTeamRangeParams, AffiliateTeamPageParams {
  sort_by?: AffiliateTeamAgentSort
  sort_order?: AffiliateTeamSortOrder
}

export interface AffiliateTeamRequestsParams extends AffiliateTeamRangeParams, AffiliateTeamPageParams {
  model?: string
  /** 0 = 无分组。 */
  group_id?: number
}

export async function getOverview(
  params: AffiliateTeamOverviewParams,
  options: FetchOptions = {}
): Promise<AffiliateTeamOverview> {
  const { data } = await apiClient.get<AffiliateTeamOverview>('/user/aff/team/overview', {
    params,
    signal: options.signal
  })
  return data
}

export async function listCustomers(
  params: AffiliateTeamCustomersParams,
  options: FetchOptions = {}
): Promise<BasePaginationResponse<AffiliateTeamCustomer>> {
  const { data } = await apiClient.get<BasePaginationResponse<AffiliateTeamCustomer>>(
    '/user/aff/team/customers',
    { params, signal: options.signal }
  )
  return data
}

export async function listAgents(
  params: AffiliateTeamAgentsParams,
  options: FetchOptions = {}
): Promise<BasePaginationResponse<AffiliateTeamAgent>> {
  const { data } = await apiClient.get<BasePaginationResponse<AffiliateTeamAgent>>(
    '/user/aff/team/agents',
    { params, signal: options.signal }
  )
  return data
}

export async function listRequests(
  userId: number,
  params: AffiliateTeamRequestsParams,
  options: FetchOptions = {}
): Promise<BasePaginationResponse<AffiliateTeamRequest>> {
  const { data } = await apiClient.get<BasePaginationResponse<AffiliateTeamRequest>>(
    `/user/aff/team/customers/${userId}/requests`,
    { params, signal: options.signal }
  )
  return data
}

export const affiliateTeamAPI = {
  getOverview,
  listCustomers,
  listAgents,
  listRequests
}

/** 团队页的四个数据接口；管理端的代理视角提供同签名的实现（api/admin/affiliateTeam.ts）。 */
export type AffiliateTeamDataSource = typeof affiliateTeamAPI

export default affiliateTeamAPI
