import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'

import AffiliatePartnerStatementView from '../AffiliatePartnerStatementView.vue'
import AffiliatePartnerView from '../AffiliatePartnerView.vue'
import { normalizePartnerOverview, normalizePartnerStatementDetail, normalizePartnerStatements } from '@/api/affiliatePartner'

const api = vi.hoisted(() => ({ getOverview: vi.fn(), listStatements: vi.fn(), getStatement: vi.fn() }))

vi.mock('@/api/affiliatePartner', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/affiliatePartner')>()
  return { ...actual, default: api }
})
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}${JSON.stringify(params)}` : key) })
  }
})

const Stub = defineComponent({ render: () => h('div') })

async function mountAt(path: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/affiliate', name: 'Affiliate', component: Stub },
      { path: '/affiliate/region', name: 'AffiliateRegion', component: Stub },
      { path: '/affiliate/region/direct', name: 'AffiliateRegionDirect', component: Stub },
      { path: '/affiliate/region/agents/:agentId', name: 'AffiliateRegionAgent', component: Stub },
      { path: '/affiliate/partner', name: 'AffiliatePartner', component: AffiliatePartnerView },
      { path: '/affiliate/partner/:month', name: 'AffiliatePartnerStatement', component: AffiliatePartnerStatementView }
    ]
  })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(defineComponent({ render: () => h(RouterView) }), {
    global: { plugins: [router], stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } }
  })
  await flushPromises()
  return wrapper
}

const LINES = [
  { line_agent: null, members: 3, revenue: '10', profit: '5' },
  { line_agent: { id: 59, email: 'a***@x.com', username: 'agentx', deleted: false }, members: 6, revenue: '96.8', profit: '55.8' }
]

describe('AffiliatePartnerView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the region, the current estimate, the lines and the statements', async () => {
    api.getOverview.mockResolvedValue(
      normalizePartnerOverview({
        region_name: '三科大',
        share_rate: '0.4000',
        next_share_rate: '0.3500',
        next_rate_month: '2026-12',
        loss_carried: '3.2',
        agent_count: 1,
        member_count: 9,
        current_month: { month: '2026-11', status: 'current', share_rate: '0.4000', revenue: '106.8', profit: '60.8', share_amount: '22.32' },
        lines: LINES
      })
    )
    api.listStatements.mockResolvedValue(
      normalizePartnerStatements({ items: [{ month: '2026-11', status: 'current' }, { month: '2026-10', status: 'paid', paid_on: '2026-11-03', share_amount: '8.00' }] })
    )
    const wrapper = await mountAt('/affiliate/partner')

    expect(wrapper.get('[data-test="affiliate-section-tab-partner"]').attributes('aria-current')).toBe('page')
    expect(wrapper.get('[data-test="partner-rate"]').text()).toContain('partner.rate{"rate":"40%"}')
    expect(wrapper.get('[data-test="partner-rate"]').text()).toContain('partner.nextRate{"month":"2026-12","rate":"35%"}')
    expect(wrapper.get('[data-test="partner-loss-carried"]').text()).toBe('¥3.20')
    expect(wrapper.get('[data-test="partner-share-amount"]').text()).toBe('¥22.32')
    expect(wrapper.get('[data-test="partner-summary-revenue"]').text()).toContain('¥106.80')

    const lines = wrapper.findAll('[data-test="partner-line-row"]')
    expect(lines[0].text()).toContain('partner.region.directTeam')
    expect(lines[0].get('a').attributes('href')).toBe('/affiliate/region/direct')
    expect(lines[1].get('a').attributes('href')).toBe('/affiliate/region/agents/59')
    // 只有昵称和脱敏邮箱，没有账号 ID。
    expect(lines[1].text()).not.toContain('59')

    const paid = wrapper.get('[data-test="partner-statement-2026-10"]')
    expect(paid.get('a').attributes('href')).toBe('/affiliate/partner/2026-10')
    expect(paid.text()).toContain('partner.status.paid')
    expect(paid.text()).toContain('2026-11-03')
  })

  it('shows a statement with costs, adjustments and warnings', async () => {
    api.getStatement.mockResolvedValue(
      normalizePartnerStatementDetail({
        totals: { month: '2026-10', status: 'confirmed', share_rate: '0.4000', loss_before: '5', loss_after: '0', share_amount: '20.32' },
        lines: LINES,
        cost_by_group: [{ group_id: 0, group_name: '', consumption: '1', account_cost: '0.3' }],
        cost_by_model: [{ model: 'gpt-6-sol', consumption: '1', account_cost: '0.3' }],
        adjustments: [{ id: 1, amount: '-5.00', note: '服务器分摊', created_at: '2026-10-20T00:00:00+08:00' }],
        warnings: ['2 条请求走固定成本账号，成本按 0 计']
      })
    )
    const wrapper = await mountAt('/affiliate/partner/2026-10')
    expect(api.getStatement).toHaveBeenCalledWith('2026-10', expect.anything())
    expect(wrapper.get('[data-test="partner-breadcrumb"]').text()).toContain('2026-10')
    expect(wrapper.get('[data-test="partner-warnings"]').text()).toContain('固定成本账号')
    expect(wrapper.get('[data-test="partner-cost-group"]').text()).toContain('affiliate.team.filters.noGroup')
    expect(wrapper.get('[data-test="partner-cost-model"]').text()).toContain('gpt-6-sol')
    const adjustment = wrapper.get('[data-test="partner-adjustment-row"]').text()
    expect(adjustment).toContain('服务器分摊')
    expect(adjustment).toContain('-¥5.00')
    expect(wrapper.find('[data-test="partner-adjustment-delete"]').exists()).toBe(false)
  })
})
