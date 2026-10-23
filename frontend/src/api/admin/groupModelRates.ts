/**
 * CUSTOM: 分组内单个模型倍率（管理员）。
 *
 * 模型名支持末尾 `*` 通配，大小写不敏感；匹配时精确优先，其次前缀最长的通配规则。
 * 命中的模型按该倍率计费，用户专属倍率不再作用于该模型。接口约定见任务 design.md §15.3。
 */

import { apiClient } from '../client'

export interface GroupModelRateRule {
  model: string
  rate_multiplier: number
}

/** 全部分组的规则：键为分组 id，只含有规则的未删除分组（列表页倍率列提示用）。 */
export async function listAll(): Promise<Record<string, GroupModelRateRule[]>> {
  const { data } = await apiClient.get<Record<string, GroupModelRateRule[]> | null>('/admin/groups/model-rates')
  return data && typeof data === 'object' ? data : {}
}

/** 单个分组的规则，按模型名排序。 */
export async function getForGroup(groupId: number): Promise<GroupModelRateRule[]> {
  const { data } = await apiClient.get<{ rules?: GroupModelRateRule[] | null }>(
    `/admin/groups/${groupId}/model-rates`
  )
  return Array.isArray(data?.rules) ? data.rules : []
}

/** 整组替换（空数组 = 清空），返回归一化后的规则。校验失败 400，message 为可读原因。 */
export async function replaceForGroup(
  groupId: number,
  rules: GroupModelRateRule[]
): Promise<GroupModelRateRule[]> {
  const { data } = await apiClient.put<{ rules?: GroupModelRateRule[] | null }>(
    `/admin/groups/${groupId}/model-rates`,
    { rules }
  )
  return Array.isArray(data?.rules) ? data.rules : []
}

export const groupModelRatesAPI = { listAll, getForGroup, replaceForGroup }

export default groupModelRatesAPI
