/**
 * CUSTOM: 用户端「查看价格」入口的可见条件与跳转目标（与首页 / 顶栏的模型广场入口同一口径）。
 *
 * 一个分组内各模型倍率可能不同，用户端不再显示分组倍率，价格一律以模型广场为准。
 */
import type { RouteLocationRaw } from 'vue-router'
import type { PublicSettings } from '@/types'
import { FeatureFlags, resolveFeatureFlag } from './featureFlags'

/** 模型广场已启用，且（已登录或广场不要求登录）。 */
export function canViewModelPlaza(
  settings: Partial<PublicSettings> | null | undefined,
  isAuthenticated: boolean
): boolean {
  if (!resolveFeatureFlag(settings, FeatureFlags.modelPlaza)) return false
  return isAuthenticated || settings?.model_plaza_require_auth !== true
}

/** 已登录时在后台布局内打开（embedded=1），未登录打开独立页；带 group 时定位到该分组。 */
export function modelPlazaPriceRoute(
  groupId: number | null | undefined,
  isAuthenticated: boolean
): RouteLocationRaw {
  const query: Record<string, string> = {}
  if (isAuthenticated) query.embedded = '1'
  if (groupId != null) query.group = String(groupId)
  return { path: '/model-plaza', query }
}
