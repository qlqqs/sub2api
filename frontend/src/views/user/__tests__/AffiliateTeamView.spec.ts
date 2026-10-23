import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { createMemoryHistory, createRouter, RouterView, type Router } from 'vue-router'

import type { AffiliateTeamOverview, AffiliateTeamUser } from '@/types'
import AffiliateTeamView from '../AffiliateTeamView.vue'

const { getOverview, listCustomers, listAgents, listRequests, showError, showInfo } = vi.hoisted(() => ({
  getOverview: vi.fn(),
  listCustomers: vi.fn(),
  listAgents: vi.fn(),
  listRequests: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn()
}))

vi.mock('@/api/affiliateTeam', () => ({
  default: { getOverview, listCustomers, listAgents, listRequests }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showInfo })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      // Echo params so assertions cover the generated copy without compiling locale files.
      t: (key: string, params?: unknown) => (params === undefined ? key : `${key}${JSON.stringify(params)}`),
      locale: { value: 'zh' }
    })
  }
})

vi.mock('vue-chartjs', () => ({
  Bar: { props: ['data', 'options'], template: '<div data-test="bar-chart" />' }
}))

function teamUser(id: number, nickname: string, email = `${nickname[0] ?? 'x'}***@x.com`): AffiliateTeamUser {
  return { user_id: id, nickname, email, deleted: false }
}

function overview(overrides: Partial<AffiliateTeamOverview> = {}): AffiliateTeamOverview {
  return {
    viewer: { agent_level: 1, downgraded: false, settle_interval_minutes: 30 },
    counts: { direct_customers: 3, sub_agents: 2 },
    agent: null,
    customer: null,
    summary: {
      cost: 1280.5,
      direct_cost: 900.2,
      sub_cost: 380.3,
      earnings: 256.1,
      active_customers: 312,
      customers: 1234,
      requests: 3201,
      tokens: 4_200_000
    },
    trend: [
      { date: '2026-10-21', cost: 10, earnings: 2 },
      { date: '2026-10-22', cost: 20, earnings: 4 }
    ],
    models: [
      { model: 'claude-sonnet-4-5', cost: 62, share: 0.62 },
      { model: 'gpt-5', cost: 21, share: 0.21 },
      { model: '', cost: 17, share: 0.17 }
    ],
    filters: null,
    ...overrides
  }
}

function page<T>(items: T[], total = items.length) {
  return { items, total, page: 1, page_size: 20, pages: Math.max(1, Math.ceil(total / 20)) }
}

const Stub = defineComponent({ render: () => h('div') })

function createTestRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/affiliate', name: 'Affiliate', component: Stub },
      { path: '/affiliate/team', name: 'AffiliateTeam', component: AffiliateTeamView },
      { path: '/affiliate/team/agents/:id', name: 'AffiliateTeamAgent', component: AffiliateTeamView },
      { path: '/affiliate/team/customers/:id', name: 'AffiliateTeamCustomer', component: AffiliateTeamView }
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
  // 通过 RouterView 渲染，三个路由名共用同一个视图实例的行为与线上一致。
  const wrapper = mount(defineComponent({ render: () => h(RouterView) }), {
    global: {
      plugins: [router],
      stubs: { AppLayout: { template: '<main><slot /></main>' }, 'i18n-t': I18nTStub }
    },
    attachTo: document.body
  })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, router }
}

describe('AffiliateTeamView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listCustomers.mockResolvedValue(page([]))
    listAgents.mockResolvedValue(page([]))
    listRequests.mockResolvedValue(page([]))
  })

  afterEach(() => {
    mounted.splice(0).forEach((wrapper) => wrapper.unmount())
  })

  it('shows both tabs and the direct / sub split for a level-1 agent', async () => {
    getOverview.mockResolvedValue(overview())
    listCustomers.mockResolvedValue(page([
      { user: teamUser(11, '张三', 'z***@qq.com'), agent_level: 3, cost: 120.5, requests: 3201, earnings: 24.1, last_used_at: null, cost_share: 0.4 },
      { user: { user_id: 12, nickname: '', email: '', deleted: true }, agent_level: 3, cost: 0, requests: 0, earnings: 0, last_used_at: null, cost_share: 0 }
    ]))
    const { wrapper } = await mountAt('/affiliate/team')

    expect(wrapper.find('[data-test="team-tab-customers"]').text()).toContain('affiliate.team.tabs.customers')
    expect(wrapper.find('[data-test="team-tab-agents"]').text()).toContain('affiliate.team.tabs.agents')
    expect(wrapper.find('[data-test="team-summary-split"]').text()).toBe('affiliate.team.summary.split{"direct":"¥900.20","sub":"¥380.30"}')
    expect(wrapper.find('[data-test="team-summary-active"]').text()).toContain('312 / 1,234')
    expect(wrapper.find('[data-test="team-downgraded"]').exists()).toBe(false)
    // 根页左侧是「邀请 | 团队消费」分区 Tab（团队消费为当前页），不再显示面包屑根；日期与刷新在同一行。
    const sectionTabs = wrapper.get('[data-test="affiliate-section-tabs"]')
    expect(wrapper.find('[data-test="team-breadcrumb"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="affiliate-section-tab-team"]').attributes('aria-current')).toBe('page')
    expect(wrapper.get('[data-test="affiliate-section-tab-invite"]').attributes('href')).toBe('/affiliate')
    expect(sectionTabs.element.parentElement?.contains(wrapper.get('[data-test="team-refresh"]').element)).toBe(true)

    expect(listCustomers).toHaveBeenCalledWith(
      expect.objectContaining({ sort_by: 'cost', sort_order: 'desc', page: 1, page_size: 20 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    const table = wrapper.find('[data-test="team-customer-table"]')
    expect(table.text()).toContain('z***@qq.com')
    expect(table.text()).toContain('affiliate.team.deleted')
  })

  it('switches to the sub-agent list and keeps only the date range when drilling into an agent', async () => {
    getOverview.mockResolvedValue(overview())
    listAgents.mockResolvedValue(page([
      { user: teamUser(21, '小王'), historical: false, customers: 230, active_customers: 41, team_cost: 380, earnings: 38, cost_share: 0.5 },
      { user: teamUser(22, '老赵'), historical: true, customers: 12, active_customers: 0, team_cost: 0, earnings: 0, cost_share: 0 }
    ]))
    const { wrapper, router } = await mountAt('/affiliate/team?start=2026-10-01&end=2026-10-07&q=abc')

    await wrapper.find('[data-test="team-tab-agents"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ start: '2026-10-01', end: '2026-10-07', tab: 'agents' })
    expect(listAgents).toHaveBeenCalledWith(
      expect.objectContaining({ start_date: '2026-10-01', end_date: '2026-10-07', sort_by: 'team_cost' }),
      expect.anything()
    )
    const table = wrapper.find('[data-test="team-agent-table"]')
    expect(table.text()).toContain('41 / 230')
    expect(table.text()).toContain('affiliate.team.historical')

    await table.findAll('tbody tr')[0].trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('AffiliateTeamAgent')
    expect(router.currentRoute.value.params.id).toBe('21')
    expect(router.currentRoute.value.query).toEqual({ start: '2026-10-01', end: '2026-10-07' })
    // 同一视图实例换到二级团队范围：按新范围重新加载。
    expect(getOverview).toHaveBeenLastCalledWith(
      expect.objectContaining({ agent_id: 21, start_date: '2026-10-01', end_date: '2026-10-07' }),
      expect.anything()
    )
    expect(listCustomers).toHaveBeenLastCalledWith(expect.objectContaining({ agent_id: 21, sort_by: 'cost' }), expect.anything())
    // 下钻页回到面包屑。
    expect(wrapper.find('[data-test="team-breadcrumb"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="affiliate-section-tabs"]').exists()).toBe(false)
  })

  it('renders a plain heading without tabs for a level-2 agent', async () => {
    getOverview.mockResolvedValue(overview({
      viewer: { agent_level: 2, downgraded: false, settle_interval_minutes: 60 },
      counts: { direct_customers: 7, sub_agents: 0 }
    }))
    const { wrapper } = await mountAt('/affiliate/team')

    expect(wrapper.find('[role="tablist"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="team-list-title"]').text().replace(/\s+/g, ' ')).toBe('affiliate.team.tabs.customers 7')
    expect(wrapper.find('[data-test="team-summary-split"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="team-empty"]').text()).toContain('affiliate.team.empty.noCustomers')
    expect(wrapper.find('[data-test="team-empty"] a').attributes('href')).toBe('/affiliate')
  })

  it('shows the downgrade notice for a former agent', async () => {
    getOverview.mockResolvedValue(overview({
      viewer: { agent_level: 3, downgraded: true, settle_interval_minutes: 60 },
      counts: { direct_customers: 2, sub_agents: 0 }
    }))
    const { wrapper } = await mountAt('/affiliate/team')

    expect(wrapper.find('[data-test="team-downgraded"]').text()).toBe('affiliate.team.downgradedNotice')
  })

  it('builds the customer breadcrumb from the owning agent, not from navigation history', async () => {
    getOverview.mockResolvedValue(overview({
      counts: null,
      customer: {
        user: teamUser(31, '张三', 'z***@qq.com'),
        agent_level: 3,
        relation: 'upline',
        owner_agent: teamUser(21, '小王'),
        is_sub_agent: false,
        bound_at: '2026-10-01T08:00:00+08:00',
        lifetime_earnings: 88
      },
      filters: { models: ['claude-sonnet-4-5', 'gpt-5'], groups: [{ id: 0, name: '' }, { id: 3, name: '默认' }] }
    }))
    const { wrapper } = await mountAt('/affiliate/team/customers/31?start=2026-10-01&end=2026-10-07')

    expect(getOverview).toHaveBeenCalledWith(
      expect.objectContaining({ user_id: 31, start_date: '2026-10-01', end_date: '2026-10-07' }),
      expect.anything()
    )
    expect(wrapper.find('[data-test="affiliate-section-tabs"]').exists()).toBe(false)
    const crumbs = wrapper.find('[data-test="team-breadcrumb"]')
    expect(crumbs.text().replace(/\s+/g, '')).toBe('affiliate.team.title/小王/张三')
    const links = crumbs.findAll('a')
    expect(links.map((link) => link.attributes('href'))).toEqual([
      '/affiliate/team?start=2026-10-01&end=2026-10-07',
      '/affiliate/team/agents/21?start=2026-10-01&end=2026-10-07'
    ])

    const subject = wrapper.find('[data-test="team-subject"]').text()
    expect(subject).toContain('affiliate.team.customerOf小王')
    expect(subject).toContain('affiliate.team.lifetimeEarnings{"amount":"¥88.00"}')
    expect(wrapper.find('[data-test="team-summary-tokens"]').text()).toContain('4.2M')
  })

  it('shows the not-found state with a link back to the team page on 404', async () => {
    getOverview.mockRejectedValue({ status: 404, reason: 'AFFILIATE_TEAM_NOT_FOUND', message: 'customer or agent not found' })
    listRequests.mockRejectedValue({ status: 404, reason: 'AFFILIATE_TEAM_NOT_FOUND', message: 'customer or agent not found' })
    const { wrapper } = await mountAt('/affiliate/team/customers/999')

    const state = wrapper.find('[data-test="team-not-found"]')
    expect(state.text()).toContain('affiliate.team.notFound')
    expect(state.text()).toContain('affiliate.team.backToTeam')
    expect(state.find('a').attributes('href')).toMatch(/^\/affiliate\/team\?start=/)
    expect(wrapper.find('[data-test="team-summary"]').exists()).toBe(false)
    expect(showError).not.toHaveBeenCalled()
  })

  it('tells users without team access that the page is for agents only', async () => {
    getOverview.mockRejectedValue({ status: 403, reason: 'AFFILIATE_TEAM_FORBIDDEN', message: 'team consumption is only available to agents' })
    listCustomers.mockRejectedValue({ status: 403, reason: 'AFFILIATE_TEAM_FORBIDDEN', message: 'team consumption is only available to agents' })
    const { wrapper } = await mountAt('/affiliate/team')

    const state = wrapper.find('[data-test="team-not-found"]')
    expect(state.text()).toContain('affiliate.team.forbidden')
    expect(state.text()).toContain('affiliate.team.backToAffiliate')
    expect(state.find('a').attributes('href')).toBe('/affiliate')
    expect(wrapper.find('[data-test="affiliate-section-tabs"]').exists()).toBe(false)
  })

  it('filters customer requests by clicking a model in the model share list and clears on a second click', async () => {
    getOverview.mockResolvedValue(overview({
      counts: null,
      customer: {
        user: teamUser(31, '张三'),
        agent_level: 3,
        relation: 'owner',
        owner_agent: null,
        is_sub_agent: false,
        bound_at: null,
        lifetime_earnings: 0
      },
      filters: { models: ['claude-sonnet-4-5', 'gpt-5'], groups: [] }
    }))
    const { wrapper, router } = await mountAt('/affiliate/team/customers/31?page=2')
    expect(listRequests).toHaveBeenLastCalledWith(31, expect.objectContaining({ page: 2, model: undefined }), expect.anything())

    const rows = () => wrapper.findAll('[data-test="team-model-row"]')
    expect(rows()[2].element.tagName).toBe('DIV') // 「其他」不可点
    await rows()[1].trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ model: 'gpt-5' })
    expect(listRequests).toHaveBeenLastCalledWith(31, expect.objectContaining({ model: 'gpt-5', page: 1 }), expect.anything())
    expect(rows()[1].attributes('aria-pressed')).toBe('true')

    await rows()[1].trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({})
    expect(listRequests).toHaveBeenLastCalledWith(31, expect.objectContaining({ model: undefined }), expect.anything())
  })

  it('keeps the model share read-only outside the customer view', async () => {
    getOverview.mockResolvedValue(overview())
    const { wrapper } = await mountAt('/affiliate/team')

    expect(wrapper.findAll('[data-test="team-model-row"]').every((row) => row.element.tagName === 'DIV')).toBe(true)
  })
})
