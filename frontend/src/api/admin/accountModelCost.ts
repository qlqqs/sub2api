/**
 * CUSTOM: 账号模型成本倍率（管理员），与分组模型倍率同构（契约 §9）。
 *
 * 按发往上游的模型匹配：精确优先，其次前缀最长的末尾 `*` 通配；命中时替换账号倍率（不是相乘），
 * 只影响账号成本（请求记录里的账号倍率快照），不影响用户计费。
 */

import { apiClient } from '../client'

export interface AccountModelCostRule {
  model: string
  rate_multiplier: number
}

function rulesOf(data: unknown): AccountModelCostRule[] {
  const rules = data && typeof data === 'object' ? (data as { rules?: unknown }).rules : null
  return Array.isArray(rules) ? (rules as AccountModelCostRule[]) : []
}

/** 全部账号的规则：键为账号 id，只含有规则的账号（账号列表提示用）。 */
export async function listAll(): Promise<Record<string, AccountModelCostRule[]>> {
  const { data } = await apiClient.get<Record<string, AccountModelCostRule[]> | null>(
    '/admin/accounts/model-cost-multipliers'
  )
  return data && typeof data === 'object' ? data : {}
}

/** 单个账号的规则，按模型名排序。 */
export async function getForAccount(accountId: number): Promise<AccountModelCostRule[]> {
  const { data } = await apiClient.get(`/admin/accounts/${accountId}/model-cost-multipliers`)
  return rulesOf(data)
}

/** 整组替换（空数组 = 清空），返回归一化后的规则。校验失败 400，message 为可读原因。 */
export async function replaceForAccount(
  accountId: number,
  rules: AccountModelCostRule[]
): Promise<AccountModelCostRule[]> {
  const { data } = await apiClient.put(`/admin/accounts/${accountId}/model-cost-multipliers`, { rules })
  return rulesOf(data)
}

export const accountModelCostAPI = { listAll, getForAccount, replaceForAccount }

export default accountModelCostAPI
