import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { createMemoryHistory, createRouter, RouterView, type Router } from 'vue-router'

import type { AffiliateTeamOverview, AffiliateTeamUser } from '@/types'
import AffiliateRegionView from '../AffiliateRegionView.vue'

const { region, agent, agentIds, direct, listTeamLines, showError, showInfo } = vi.hoisted(() => {
  const source = () => ({ getOverview: vi.fn(), listCustomers: vi.fn(), listAgents: vi.fn(), listRequests: vi.fn() })
  return {
    region: source(),
    agent: source(),
    agentIds: [] as number[],
    direct: source(),
    listTeamLines: vi.fn(),
    showError: vi.fn(),
    showInfo: vi.fn()
  }
})

vi.mock('@/api/affiliatePartner', () => ({
  partnerRegionTeamAPI: region,
  // 记录每次调用时读到的渠道经理 ID。
  partnerAgentTeamAPI: (agentId: () => number) => ({
    getOverview: (...args: unknown[]) => {
      agentIds.push(agentId())
      return agent.getOverview(...args)
    },
    listCustomers: agent.listCustomers,
    listAgents: agent.listAgents,
    listRequests: agent.listRequests
  }),
  listTeamLines
}))
vi.mock('@/api/affiliateTeam', () => ({ default: direct }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showInfo }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: unknown) => (params === undefined ? key : `${key}${JSON.stringify(params)}`),
      locale: { value: 'zh' }
    })
  }
})
vi.mock('vue-chartjs', () => ({
  Bar: { props: ['data', 'options'], template: '<div data-test="bar-chart" :data-datasets="data.datasets.length" />' }
}))

function teamUser(id: number, nickname: string): AffiliateTeamUser {
  return { user_id: id, nickname, email: `${nickname}***@x.com`, deleted: false }
}

function overview(overrides: Partial<AffiliateTeamOverview> = {}): AffiliateTeamOverview {
  return {
    viewer: { agent_level: 1, downgraded: false, settle_interval_minutes: 5 },
    counts: { direct_customers: 2, sub_agents: 1 },
    agent: null,
    customer: null,
    summary: { cost: 100, direct_cost: 0, sub_cost: 0, earnings: 0, active_customers: 3, customers: 15, requests: 42, tokens: 1000 },
    trend: [{ date: '2026-11-01', cost: 10, earnings: 0 }],
    models: [],
    filters: null,
    ...overrides
  }
}

function page<T>(items: T[]) {
  return { items, total: items.length, page: 1, page_size: 20, pages: 1 }
}

const Stub = defineComponent({ render: () => h('div') })

function createTestRouter(): Router {
  const names = [
    ['/affiliate/region', 'AffiliateRegion'],
    ['/affiliate/region/customers/:id', 'AffiliateRegionCustomer'],
    ['/affiliate/region/direct', 'AffiliateRegionDirect'],
    ['/affiliate/region/direct/agents/:id', 'AffiliateRegionDirectAgent'],
    ['/affiliate/region/direct/customers/:id', 'AffiliateRegionDirectCustomer'],
    ['/affiliate/region/agents/:agentId', 'AffiliateRegionAgent'],
    ['/affiliate/region/agents/:agentId/agents/:id', 'AffiliateRegionAgentAgent'],
    ['/affiliate/region/agents/:agentId/customers/:id', 'AffiliateRegionAgentCustomer']
  ]
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/affiliate', name: 'Affiliate', component: Stub },
      { path: '/affiliate/team', name: 'AffiliateTeam', component: Stub },
      { path: '/affiliate/partner', name: 'AffiliatePartner', component: Stub },
      ...names.map(([path, name]) => ({ path, name, component: AffiliateRegionView }))
    ]
  })
}

const I18nTStub = defineComponent({
  props: { keypath: { type: String, required: true } },
  setup(props, { slots }) {
    return () => h('span', [props.keypath, ...Object.values(slots).map((slot) => slot?.())])
  }
})

const mounted: Array<{ unmount: () => void }> = []

async function mountAt(path: string) {
  const router = createTestRouter()
  await router.push(path)
  await router.isReady()
  const wrapper = mount(defineComponent({ render: () => h(RouterView) }), {
    global: { plugins: [router], stubs: { AppLayout: { template: '<main><slot /></main>' }, 'i18n-t': I18nTStub } },
    attachTo: document.body
  })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, router }
}

const LINES = [
  { line_agent: null, customers: 2, active_customers: 1, requests: 5, team_cost: 30, cost_share: 0.3 },
  { line_agent: teamUser(59, 'agentx'), customers: 13, active_customers: 2, requests: 37, team_cost: 70, cost_share: 0.7 }
]

describe('AffiliateRegionView (partner)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    agentIds.splice(0)
    for (const source of [region, agent, direct]) {
      source.getOverview.mockResolvedValue(overview())
      source.listCustomers.mockResolvedValue(page([]))
      source.listAgents.mockResolvedValue(page([]))
      source.listRequests.mockResolvedValue(page([]))
    }
    listTeamLines.mockResolvedValue(LINES)
  })

  afterEach(() => {
    mounted.splice(0).forEach((wrapper) => wrapper.unmount())
  })

  it('shows the region without earnings: three section tabs, owner columns and region tabs', async () => {
    region.listCustomers.mockResolvedValue(page([
      {
        user: teamUser(11, 'cust'),
        agent_level: 3,
        cost: 12,
        requests: 3,
        earnings: 0,
        last_used_at: null,
        cost_share: 0.1,
        owner: teamUser(80, 'dist'),
        line_agent: teamUser(59, 'agentx')
      },
      { user: teamUser(12, 'mine'), agent_level: 3, cost: 1, requests: 1, earnings: 0, last_used_at: null, cost_share: 0, owner: teamUser(75, 'partner'), line_agent: null }
    ]))
    const { wrapper } = await mountAt('/affiliate/region')

    expect(wrapper.get('[data-test="affiliate-section-tab-team"]').attributes('aria-current')).toBe('page')
    expect(wrapper.get('[data-test="affiliate-section-tab-team"]').attributes('href')).toBe('/affiliate/region')
    expect(wrapper.get('[data-test="affiliate-section-tab-partner"]').attributes('href')).toBe('/affiliate/partner')

    expect(wrapper.find('[data-test="team-summary-earnings"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="team-summary-requests"]').text()).toContain('42')
    expect(wrapper.get('[data-test="bar-chart"]').attributes('data-datasets')).toBe('1')

    expect(wrapper.get('[data-test="team-tab-customers"]').text()).toContain('partner.region.tabs.customers')
    expect(wrapper.get('[data-test="team-tab-customers"]').text()).toContain('15')
    expect(wrapper.get('[data-test="team-tab-agents"]').text()).toMatch(/^partner.region.tabs.lines\S*$/)

    const table = wrapper.get('[data-test="team-customer-table"]')
    expect(table.text()).toContain('partner.region.columns.lineAgent')
    expect(table.text()).toContain('partner.region.columns.owner')
    expect(table.text()).not.toContain('affiliate.team.columns.earnings')
    expect(table.text()).toContain('agentx')
    expect(table.text()).toContain('dist')
    expect(table.text()).toContain('partner.region.directTeam')
  })

  it('lists channel manager lines and drills into the direct team and an attached manager', async () => {
    const { wrapper, router } = await mountAt('/affiliate/region?start=2026-11-01&end=2026-11-07')
    await wrapper.get('[data-test="team-tab-agents"]').trigger('click')
    await flushPromises()

    expect(listTeamLines).toHaveBeenCalledWith({ start_date: '2026-11-01', end_date: '2026-11-07' }, expect.anything())
    const table = wrapper.get('[data-test="team-line-table"]')
    expect(table.text()).toContain('1 / 2')
    expect(table.text()).toContain('agentx')
    expect(wrapper.get('[data-test="team-line-direct"]').attributes('href')).toBe('/affiliate/region/direct?start=2026-11-01&end=2026-11-07')

    await table.findAll('tbody tr')[1].trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('AffiliateRegionAgent')
    expect(router.currentRoute.value.params.agentId).toBe('59')
    expect(agentIds).toContain(59)

    // 面包屑：区域团队 / 渠道经理名（取自渠道经理线）。
    const crumbs = wrapper.get('[data-test="team-breadcrumb"]')
    expect(crumbs.text()).toContain('partner.region.title')
    expect(crumbs.text()).toContain('agentx')
    expect(crumbs.find('a').attributes('href')).toBe('/affiliate/region?start=2026-11-01&end=2026-11-07')
    expect(wrapper.find('[data-test="affiliate-section-tabs"]').exists()).toBe(false)
  })

  it('uses the agent team API for the partner direct team under the region breadcrumb', async () => {
    const { wrapper } = await mountAt('/affiliate/region/direct')
    expect(direct.getOverview).toHaveBeenCalled()
    expect(region.getOverview).not.toHaveBeenCalled()
    const crumbs = wrapper.get('[data-test="team-breadcrumb"]').text()
    expect(crumbs).toContain('partner.region.title')
    expect(crumbs).toContain('partner.region.directTeam')
  })

  it('opens a region customer with the region requests API', async () => {
    region.getOverview.mockResolvedValue(overview({
      counts: null,
      customer: {
        user: teamUser(11, 'cust'),
        agent_level: 3,
        relation: 'upline',
        owner_agent: teamUser(80, 'dist'),
        is_sub_agent: false,
        bound_at: null,
        lifetime_earnings: 0
      }
    }))
    const { wrapper } = await mountAt('/affiliate/region/customers/11')
    expect(region.getOverview).toHaveBeenCalledWith(expect.objectContaining({ user_id: 11 }), expect.anything())
    expect(region.listRequests).toHaveBeenCalledWith(11, expect.anything(), expect.anything())
    const subject = wrapper.get('[data-test="team-subject"]').text()
    expect(subject).toContain('dist')
    expect(subject).not.toContain('affiliate.team.lifetimeEarnings')
    // 归属代理只显示名字，不链接到可能不在区域视角下的代理页。
    expect(wrapper.get('[data-test="team-subject-owner"]').find('a').exists()).toBe(false)
    const crumbs = wrapper.get('[data-test="team-breadcrumb"]').text()
    expect(crumbs).toContain('partner.region.title')
    expect(crumbs).not.toContain('dist')
  })

  it('tells non-partners the region page is for partners only', async () => {
    region.getOverview.mockRejectedValue({ status: 403, reason: 'PARTNER_FORBIDDEN', message: 'forbidden' })
    region.listCustomers.mockRejectedValue({ status: 403, reason: 'PARTNER_FORBIDDEN', message: 'forbidden' })
    const { wrapper } = await mountAt('/affiliate/region')
    const state = wrapper.get('[data-test="team-not-found"]')
    expect(state.text()).toContain('partner.region.forbidden')
    expect(state.find('a').attributes('href')).toBe('/affiliate')
    expect(showError).not.toHaveBeenCalled()
  })

  it('sends partners back to the region when a manager is not attached to them', async () => {
    agent.getOverview.mockRejectedValue({ status: 403, reason: 'PARTNER_FORBIDDEN', message: 'forbidden' })
    agent.listCustomers.mockRejectedValue({ status: 403, reason: 'PARTNER_FORBIDDEN', message: 'forbidden' })
    const { wrapper } = await mountAt('/affiliate/region/agents/99')
    const state = wrapper.get('[data-test="team-not-found"]')
    expect(state.text()).toContain('partner.region.notFound')
    expect(state.text()).toContain('partner.region.backToRegion')
    expect(state.find('a').attributes('href')).toMatch(/^\/affiliate\/region\?start=/)
  })
})
