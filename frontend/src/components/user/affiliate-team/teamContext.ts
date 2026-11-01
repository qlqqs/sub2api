import { inject, provide, type InjectionKey } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'
import affiliateTeamAPI, { type AffiliateTeamDataSource, type AffiliateTeamRangeParams } from '@/api/affiliateTeam'
import type { PartnerTeamLine } from '@/api/affiliatePartner'

/**
 * 团队消费页的受众上下文：代理本人（/affiliate/team）、管理员查看某代理的视角
 * （/admin/affiliates/team/:viewerId）与合伙人（/affiliate/region/*）共用同一套页面和组件，
 * 只在接口、路由与少量文案上不同。未提供时为代理端。
 */

export type AffiliateTeamAudience = 'agent' | 'admin' | 'partner'

/** 三层页面的路由；query 为下钻携带的筛选（时间范围）。 */
export interface AffiliateTeamRoutes {
  team: (query: Record<string, string>) => RouteLocationRaw
  agent: (id: number, query: Record<string, string>) => RouteLocationRaw
  customer: (id: number, query: Record<string, string>) => RouteLocationRaw
  /** 管理端：另一位代理的视角（元信息行的上级链接）。 */
  viewer?: (id: number, query: Record<string, string>) => RouteLocationRaw
  /** 管理端：该用户的使用记录。 */
  usage?: (userId: number, range: { start: string; end: string }) => RouteLocationRaw
  /** 合伙人：自己的直属团队（渠道经理线表里的「直属团队」一行）。 */
  direct?: (query: Record<string, string>) => RouteLocationRaw
  /** 管理端 / 合伙人：汇总页（面包屑首项、未找到时的返回链接）。 */
  summary?: (range: { start: string; end: string }) => RouteLocationRaw
}

/** CUSTOM: 合伙人的区域视角：根页的下级列表换成渠道经理线，客户表加归属列，不显示收益。 */
export interface AffiliateTeamRegion {
  listLines: (params: AffiliateTeamRangeParams, options: { signal?: AbortSignal }) => Promise<PartnerTeamLine[]>
}

export interface AffiliateTeamContext {
  audience: AffiliateTeamAudience
  api: AffiliateTeamDataSource
  routes: AffiliateTeamRoutes
  /** 受众专用文案：代理端 key → 替换 key。 */
  labels?: Readonly<Record<string, string>>
  /** 替换文案共用的参数，如被查看代理的名字 {name}。 */
  labelParams?: () => Record<string, unknown>
  /** 团队根的面包屑名称；未提供时代理端为「团队消费」，管理端为被查看的代理。空串显示占位条。 */
  rootLabel?: () => string
  region?: AffiliateTeamRegion
}

const agentContext: AffiliateTeamContext = {
  audience: 'agent',
  api: affiliateTeamAPI,
  routes: {
    team: (query) => ({ name: 'AffiliateTeam', query }),
    agent: (id, query) => ({ name: 'AffiliateTeamAgent', params: { id: String(id) }, query }),
    customer: (id, query) => ({ name: 'AffiliateTeamCustomer', params: { id: String(id) }, query })
  }
}

const contextKey: InjectionKey<AffiliateTeamContext> = Symbol('affiliate-team-context')

export function provideAffiliateTeamContext(context: AffiliateTeamContext): void {
  provide(contextKey, context)
}

export function useAffiliateTeamContext(): AffiliateTeamContext {
  return inject(contextKey, agentContext)
}

/** 按受众取文案：有替换 key 时用替换文案（带上下文参数），否则与代理端完全相同。 */
export function useTeamLabel(context: AffiliateTeamContext = useAffiliateTeamContext()) {
  const { t } = useI18n()
  return (key: string, params?: Record<string, unknown>): string => {
    const replacement = context.labels?.[key]
    if (replacement) return t(replacement, { ...context.labelParams?.(), ...params })
    return params === undefined ? t(key) : t(key, params)
  }
}
