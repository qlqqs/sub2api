import { inject, provide, type InjectionKey } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'
import affiliateTeamAPI, { type AffiliateTeamDataSource } from '@/api/affiliateTeam'

/**
 * 团队消费页的受众上下文：代理本人（/affiliate/team）与管理员查看某代理的视角
 * （/admin/affiliates/team/:viewerId）共用同一套页面和组件，只在接口、路由与少量文案上不同。
 * 未提供时为代理端。
 */

export type AffiliateTeamAudience = 'agent' | 'admin'

/** 三层页面的路由；query 为下钻携带的筛选（时间范围）。 */
export interface AffiliateTeamRoutes {
  team: (query: Record<string, string>) => RouteLocationRaw
  agent: (id: number, query: Record<string, string>) => RouteLocationRaw
  customer: (id: number, query: Record<string, string>) => RouteLocationRaw
  /** 管理端：另一位代理的视角（元信息行的上级链接）。 */
  viewer?: (id: number, query: Record<string, string>) => RouteLocationRaw
  /** 管理端：该用户的使用记录。 */
  usage?: (userId: number, range: { start: string; end: string }) => RouteLocationRaw
  /** 管理端：汇总页（面包屑首项、未找到时的返回链接）。 */
  summary?: (range: { start: string; end: string }) => RouteLocationRaw
}

export interface AffiliateTeamContext {
  audience: AffiliateTeamAudience
  api: AffiliateTeamDataSource
  routes: AffiliateTeamRoutes
  /** 受众专用文案：代理端 key → 替换 key。 */
  labels?: Readonly<Record<string, string>>
  /** 替换文案共用的参数，如被查看代理的名字 {name}。 */
  labelParams?: () => Record<string, unknown>
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
