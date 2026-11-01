import affiliateTeamAPI from '@/api/affiliateTeam'
import { listTeamLines, partnerAgentTeamAPI, partnerRegionTeamAPI } from '@/api/affiliatePartner'
import type { AffiliateTeamContext } from './teamContext'

/**
 * CUSTOM: 合伙人的三种团队消费视角（/affiliate/region/*），都回到区域汇总页：
 * - 区域：整个区域，下级列表是渠道经理线；
 * - 直属团队：合伙人自己的团队，接口即代理端 /user/aff/team/*（合伙人内部是渠道经理）；
 * - 渠道经理：以挂靠的渠道经理为根，与他自己看到的页面相同。
 */

function regionSummary(range: { start: string; end: string }) {
  return { name: 'AffiliateRegion', query: { start: range.start, end: range.end } }
}

const REGION_LABELS: Readonly<Record<string, string>> = {
  'affiliate.team.tabs.customers': 'partner.region.tabs.customers',
  'affiliate.team.tabs.agents': 'partner.region.tabs.lines',
  'affiliate.team.myCustomer': 'partner.region.noOwner',
  'affiliate.team.forbidden': 'partner.region.forbidden',
  'affiliate.team.notFound': 'partner.region.notFound'
}

/** 下钻到直属团队 / 渠道经理后共用的文案：面包屑首项与返回链接指向区域。 */
const DRILL_LABELS: Readonly<Record<string, string>> = {
  'admin.affiliates.team.title': 'partner.region.title',
  'affiliate.team.backToTeam': 'partner.region.backToRegion',
  'affiliate.team.notFound': 'partner.region.notFound'
}

/** 渠道经理视角：第三人称（收益是该渠道经理的代理分润）。 */
export const PARTNER_AGENT_LABELS: Readonly<Record<string, string>> = {
  ...DRILL_LABELS,
  'affiliate.team.tabs.customers': 'partner.agentView.directCustomers',
  'affiliate.team.myCustomer': 'partner.agentView.directCustomers',
  'affiliate.team.summary.earnings': 'partner.agentView.earnings',
  'affiliate.team.columns.earnings': 'partner.agentView.earnings',
  'affiliate.team.chart.earnings': 'partner.agentView.earnings',
  'affiliate.team.lifetimeEarnings': 'partner.agentView.lifetimeEarnings',
  'affiliate.team.historicalHint': 'partner.agentView.historicalHint'
}

export function createPartnerRegionContext(rootLabel: () => string): AffiliateTeamContext {
  return {
    audience: 'partner',
    api: partnerRegionTeamAPI,
    routes: {
      team: (query) => ({ name: 'AffiliateRegion', query }),
      // 区域根页的「下级」是渠道经理线，点进去是该渠道经理的视角。
      agent: (id, query) => ({ name: 'AffiliateRegionAgent', params: { agentId: String(id) }, query }),
      customer: (id, query) => ({ name: 'AffiliateRegionCustomer', params: { id: String(id) }, query }),
      direct: (query) => ({ name: 'AffiliateRegionDirect', query })
    },
    labels: REGION_LABELS,
    rootLabel,
    region: { listLines: listTeamLines }
  }
}

export function createPartnerDirectContext(rootLabel: () => string): AffiliateTeamContext {
  return {
    audience: 'partner',
    api: affiliateTeamAPI,
    routes: {
      team: (query) => ({ name: 'AffiliateRegionDirect', query }),
      agent: (id, query) => ({ name: 'AffiliateRegionDirectAgent', params: { id: String(id) }, query }),
      customer: (id, query) => ({ name: 'AffiliateRegionDirectCustomer', params: { id: String(id) }, query }),
      summary: regionSummary
    },
    labels: DRILL_LABELS,
    rootLabel
  }
}

/** agentId 在调用时读取；换到另一位渠道经理时页面整页重建。 */
export function createPartnerAgentContext(agentId: () => number, rootLabel: () => string): AffiliateTeamContext {
  const agent = () => String(agentId())
  return {
    audience: 'partner',
    api: partnerAgentTeamAPI(agentId),
    routes: {
      team: (query) => ({ name: 'AffiliateRegionAgent', params: { agentId: agent() }, query }),
      agent: (id, query) => ({ name: 'AffiliateRegionAgentAgent', params: { agentId: agent(), id: String(id) }, query }),
      customer: (id, query) => ({ name: 'AffiliateRegionAgentCustomer', params: { agentId: agent(), id: String(id) }, query }),
      summary: regionSummary
    },
    labels: PARTNER_AGENT_LABELS,
    rootLabel
  }
}
