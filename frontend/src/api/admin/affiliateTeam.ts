/**
 * 管理端团队消费页：/api/v1/admin/affiliates/team/*（只读）。
 * 汇总页两个接口；代理视角四个接口与代理端同签名，以被查看的代理为根。
 */

import { apiClient } from '../client'
import type {
  AffiliateTeamAgentsParams,
  AffiliateTeamCustomersParams,
  AffiliateTeamDataSource,
  AffiliateTeamOverviewParams,
  AffiliateTeamPageParams,
  AffiliateTeamRangeParams,
  AffiliateTeamRequestsParams
} from '../affiliateTeam'
import type {
  AdminAffiliateTeamLevel,
  AffiliateTeamAgent,
  AffiliateTeamAgentSort,
  AffiliateTeamCustomer,
  AffiliateTeamOverview,
  AffiliateTeamOwner,
  AffiliateTeamPlatformOverview,
  AffiliateTeamRequest,
  AffiliateTeamSortOrder,
  BasePaginationResponse,
  FetchOptions
} from '@/types'

export interface AdminAffiliateTeamOwnersParams extends AffiliateTeamRangeParams, AffiliateTeamPageParams {
  level?: Exclude<AdminAffiliateTeamLevel, ''>
  /** 邮箱、昵称模糊；纯数字时同时按用户 ID 精确匹配。 */
  search?: string
  sort_by?: AffiliateTeamAgentSort
  sort_order?: AffiliateTeamSortOrder
}

export async function getOverview(
  params: AffiliateTeamRangeParams,
  options: FetchOptions = {}
): Promise<AffiliateTeamPlatformOverview> {
  const { data } = await apiClient.get<AffiliateTeamPlatformOverview>('/admin/affiliates/team/overview', {
    params,
    signal: options.signal
  })
  return data
}

export async function listOwners(
  params: AdminAffiliateTeamOwnersParams,
  options: FetchOptions = {}
): Promise<BasePaginationResponse<AffiliateTeamOwner>> {
  const { data } = await apiClient.get<BasePaginationResponse<AffiliateTeamOwner>>('/admin/affiliates/team/agents', {
    params,
    signal: options.signal
  })
  return data
}

/** viewerId 在每次调用时读取：同一页面实例在代理之间跳转时不需要重建。 */
export function viewer(viewerId: () => number): AffiliateTeamDataSource {
  const base = () => `/admin/affiliates/team/${viewerId()}`
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
      return data
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

export const adminAffiliateTeamAPI = {
  getOverview,
  listOwners,
  viewer
}

export default adminAffiliateTeamAPI
