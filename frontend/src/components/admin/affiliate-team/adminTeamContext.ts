import adminAffiliateTeamAPI from '@/api/admin/affiliateTeam'
import type { AffiliateTeamContext } from '@/components/user/affiliate-team/teamContext'
import { adminAffiliateTeamSummaryRoute } from '@/composables/useAdminAffiliateTeamQuery'

/** 管理端替换的文案（第三人称、「收益」改称「分润」等）；其余与代理端相同。 */
export const ADMIN_AFFILIATE_TEAM_LABELS: Readonly<Record<string, string>> = {
  'affiliate.team.tabs.customers': 'admin.affiliates.team.labels.directCustomers',
  'affiliate.team.myCustomer': 'admin.affiliates.team.labels.directCustomers',
  'affiliate.team.summary.earnings': 'admin.affiliates.team.labels.earnings',
  'affiliate.team.columns.earnings': 'admin.affiliates.team.labels.earnings',
  'affiliate.team.chart.earnings': 'admin.affiliates.team.labels.earnings',
  'affiliate.team.lifetimeEarnings': 'admin.affiliates.team.labels.lifetimeEarnings',
  'affiliate.team.historicalHint': 'admin.affiliates.team.labels.historicalHint',
  'affiliate.team.searchPlaceholder': 'admin.affiliates.team.labels.searchPlaceholder',
  'affiliate.team.notFound': 'admin.affiliates.team.labels.notFound',
  'affiliate.team.backToTeam': 'admin.affiliates.team.labels.backToSummary',
  'affiliate.team.deleted': 'admin.affiliates.team.labels.deleted'
}

/**
 * 管理员查看某代理视角的上下文：接口与路由都以被查看的代理为根。viewerId 在调用时读取，
 * 同一页面实例在代理之间跳转（上级链接）时不需要重建上下文。
 */
export function createAdminAffiliateTeamContext(viewerId: () => number): AffiliateTeamContext {
  const viewer = () => String(viewerId())
  return {
    audience: 'admin',
    api: adminAffiliateTeamAPI.viewer(viewerId),
    routes: {
      team: (query) => ({ name: 'AdminAffiliateTeamViewer', params: { viewerId: viewer() }, query }),
      agent: (id, query) => ({ name: 'AdminAffiliateTeamViewerAgent', params: { viewerId: viewer(), id: String(id) }, query }),
      customer: (id, query) => ({ name: 'AdminAffiliateTeamViewerCustomer', params: { viewerId: viewer(), id: String(id) }, query }),
      viewer: (id, query) => ({ name: 'AdminAffiliateTeamViewer', params: { viewerId: String(id) }, query }),
      usage: (userId, range) => ({
        name: 'AdminUsage',
        query: { user_id: String(userId), start_date: range.start, end_date: range.end }
      }),
      summary: adminAffiliateTeamSummaryRoute
    },
    labels: ADMIN_AFFILIATE_TEAM_LABELS
  }
}
