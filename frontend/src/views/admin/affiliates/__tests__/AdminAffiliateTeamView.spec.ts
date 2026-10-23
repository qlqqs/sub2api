import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { createMemoryHistory, createRouter, RouterView, type Router } from 'vue-router'

import SearchInput from '@/components/common/SearchInput.vue'
import TeamToolbar from '@/components/user/affiliate-team/TeamToolbar.vue'
import type { AffiliateTeamOverview, AffiliateTeamOwner, AffiliateTeamPlatformOverview, AffiliateTeamUser } from '@/types'
import AdminAffiliateTeamView from '../AdminAffiliateTeamView.vue'
import AdminAffiliateTeamViewerView from '../AdminAffiliateTeamViewerView.vue'

const { getOverview, listOwners, viewerOverview, viewerCustomers, viewerAgents, viewerRequests, showError, showInfo } = vi.hoisted(() => ({
  getOverview: vi.fn(),
  listOwners: vi.fn(),
  viewerOverview: vi.fn(),
  viewerCustomers: vi.fn(),
  viewerAgents: vi.fn(),
  viewerRequests: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn()
}))

vi.mock('@/api/admin/affiliateTeam', () => {
  const api = {
    getOverview,
    listOwners,
    // 记下每次调用时的代理 ID，验证上下文按当前路由取值。
    viewer: (viewerId: () => number) => ({
      getOverview: (...args: unknown[]) => viewerOverview(viewerId(), ...args),
      listCustomers: (...args: unknown[]) => viewerCustomers(viewerId(), ...args),
      listAgents: (...args: unknown[]) => viewerAgents(viewerId(), ...args),
      listRequests: (...args: unknown[]) => viewerRequests(viewerId(), ...args)
    })
  }
  return { default: api, adminAffiliateTeamAPI: api }
})

// 管理端页面不得调用代理端接口。
vi.mock('@/api/affiliateTeam', () => {
  const fail = () => Promise.reject(new Error('agent API must not be used by admin pages'))
  const api = { getOverview: fail, listCustomers: fail, listAgents: fail, listRequests: fail }
  return { default: api, affiliateTeamAPI: api }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showInfo })
}))

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
  Bar: { props: ['data', 'options'], template: '<div data-test="bar-chart" />' }
}))

function user(id: number, nickname: string, email = `${id}@example.com`): AffiliateTeamUser {
  return { user_id: id, nickname, email, deleted: false }
}

function platform(overrides: Partial<AffiliateTeamPlatformOverview> = {}): AffiliateTeamPlatformOverview {
  return {
    settle_interval_minutes: 30,
    counts: { all: 25, level1: 5, level2: 18, downgraded: 2 },
    summary: { cost: 12800.5, platform_cost: 33685.5, earnings: 2561, active_customers: 312, customers: 4567 },
    trend: [{ date: '2026-10-22', cost: 20, earnings: 4 }],
    models: [{ model: 'claude-sonnet-4-5', cost: 62, share: 1 }],
    ...overrides
  }
}

function owner(overrides: Partial<AffiliateTeamOwner>): AffiliateTeamOwner {
  return {
    user: user(1024, '张三', 'zhang@qq.com'),
    agent_level: 1,
    upline: null,
    customers: 1234,
    active_customers: 120,
    team_cost: 5200,
    earnings: 1040,
    cost_share: 0.41,
    ...overrides
  }
}

function viewerData(overrides: Partial<AffiliateTeamOverview> = {}): AffiliateTeamOverview {
  return {
    viewer: { agent_level: 1, downgraded: false, settle_interval_minutes: 30 },
    counts: { direct_customers: 3, sub_agents: 0 },
    agent: null,
    customer: null,
    summary: { cost: 10, direct_cost: 10, sub_cost: 0, earnings: 2, active_customers: 1, customers: 3, requests: 5, tokens: 1000 },
    trend: [],
    models: [],
    filters: null,
    subject: { user: user(1024, '张三', 'zhang@qq.com'), upline: null, lifetime_earnings: 8800 },
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
      { path: '/admin/usage', name: 'AdminUsage', component: Stub },
      { path: '/admin/affiliates/team', name: 'AdminAffiliateTeam', component: AdminAffiliateTeamView },
      { path: '/admin/affiliates/team/:viewerId', name: 'AdminAffiliateTeamViewer', component: AdminAffiliateTeamViewerView },
      { path: '/admin/affiliates/team/:viewerId/agents/:id', name: 'AdminAffiliateTeamViewerAgent', component: AdminAffiliateTeamViewerView },
      { path: '/admin/affiliates/team/:viewerId/customers/:id', name: 'AdminAffiliateTeamViewerCustomer', component: AdminAffiliateTeamViewerView }
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

beforeEach(() => {
  vi.clearAllMocks()
  getOverview.mockResolvedValue(platform())
  listOwners.mockResolvedValue(page([]))
  viewerOverview.mockResolvedValue(viewerData())
  viewerCustomers.mockResolvedValue(page([]))
  viewerAgents.mockResolvedValue(page([]))
  viewerRequests.mockResolvedValue(page([]))
})

afterEach(() => {
  mounted.splice(0).forEach((wrapper) => wrapper.unmount())
})

describe('AdminAffiliateTeamView (summary)', () => {
  it('renders the platform summary, level tabs and one row per agent', async () => {
    listOwners.mockResolvedValue(page([
      owner({}),
      owner({ user: user(2048, '小王', 'wang@163.com'), agent_level: 2, upline: user(1024, '张三') }),
      owner({ user: user(3001, '老赵'), agent_level: 3 }),
      owner({ user: { user_id: 3056, nickname: '', email: '', deleted: true }, agent_level: 2, upline: user(1024, '张三') })
    ], 25))
    const { wrapper } = await mountAt('/admin/affiliates/team')

    expect(wrapper.find('[data-test="team-breadcrumb"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="team-refresh"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="team-summary-cost"]').text()).toContain('¥12,800.50')
    expect(wrapper.get('[data-test="team-summary-cost"]').text()).toContain('admin.affiliates.team.summary.costShare{"percent":"38%"}')
    expect(wrapper.get('[data-test="team-summary-earnings"]').text()).toContain('admin.affiliates.team.summary.earningsShare{"percent":"20.0%"}')
    expect(wrapper.get('[data-test="team-summary-active"]').text()).toContain('312 / 4,567')
    // 趋势图的折线用管理端文案。
    expect(wrapper.get('[data-test="team-trend"]').text()).toContain('admin.affiliates.team.labels.earnings')

    const tabs = wrapper.findAll('[role="tab"]').map((tab) => tab.text().replace(/\s+/g, ' '))
    expect(tabs).toEqual([
      'admin.affiliates.team.levels.all 25',
      'admin.affiliates.team.levels.level1 5',
      'admin.affiliates.team.levels.level2 18',
      'admin.affiliates.team.levels.downgraded 2'
    ])
    expect(listOwners).toHaveBeenCalledWith(
      expect.objectContaining({ sort_by: 'team_cost', sort_order: 'desc', page: 1, page_size: 20, level: undefined, search: undefined }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )

    const table = wrapper.get('[data-test="team-owner-table"]')
    expect(table.text()).toContain('zhang@qq.com')
    const levels = table.findAll('[data-test="team-owner-level"]').map((cell) => cell.text())
    expect(levels).toEqual([
      'admin.affiliates.team.levels.level1',
      'admin.affiliates.team.levels.level2With{"name":"张三"}',
      'admin.affiliates.team.levels.downgraded',
      'admin.affiliates.team.levels.level2With{"name":"张三"}'
    ])
    // 已删除的代理显示 ID，仍可点进。
    const deleted = table.get('[data-test="team-owner-deleted"]')
    expect(deleted.text()).toBe('admin.affiliates.team.labels.deleted{"id":3056}')
    expect(deleted.attributes('href')).toMatch(/^\/admin\/affiliates\/team\/3056\?start=/)
  })

  it('hides the downgraded tab when nobody is downgraded and drops a stale level filter', async () => {
    getOverview.mockResolvedValue(platform({ counts: { all: 6, level1: 1, level2: 5, downgraded: 0 } }))
    const { wrapper, router } = await mountAt('/admin/affiliates/team?level=downgraded&q=wang')

    expect(wrapper.find('[data-test="team-level-downgraded"]').exists()).toBe(false)
    expect(router.currentRoute.value.query).toEqual({ q: 'wang' })
  })

  it('resets the page on level, search and sort changes while keeping the other filters', async () => {
    const { wrapper, router } = await mountAt('/admin/affiliates/team?q=wang&sort=earnings&page=3')
    expect(listOwners).toHaveBeenLastCalledWith(
      expect.objectContaining({ search: 'wang', sort_by: 'earnings', page: 3 }),
      expect.anything()
    )

    await wrapper.get('[data-test="team-level-1"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ q: 'wang', sort: 'earnings', level: '1' })
    expect(listOwners).toHaveBeenLastCalledWith(expect.objectContaining({ level: '1', search: 'wang', page: 1 }), expect.anything())

    wrapper.getComponent(SearchInput).vm.$emit('search', ' 3056 ')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ q: '3056', sort: 'earnings', level: '1' })
  })

  it('refreshes the summary, charts and table together on a range change and clamps ranges over 90 days', async () => {
    const { wrapper, router } = await mountAt('/admin/affiliates/team?level=2&q=wang&page=3')
    getOverview.mockClear()
    listOwners.mockClear()

    wrapper.getComponent(TeamToolbar).vm.$emit('rangeChange', { start: '2026-10-01', end: '2026-10-07' })
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ start: '2026-10-01', end: '2026-10-07', level: '2', q: 'wang' })
    expect(getOverview).toHaveBeenCalledWith({ start_date: '2026-10-01', end_date: '2026-10-07' }, expect.anything())
    expect(listOwners).toHaveBeenLastCalledWith(
      expect.objectContaining({ start_date: '2026-10-01', end_date: '2026-10-07', level: '2', search: 'wang', page: 1 }),
      expect.anything()
    )

    wrapper.getComponent(TeamToolbar).vm.$emit('rangeChange', { start: '2026-01-01', end: '2026-10-07' })
    await flushPromises()
    expect(showInfo).toHaveBeenCalledWith('affiliate.team.rangeClamped{"days":90}')
    expect(router.currentRoute.value.query.start).toBe('2026-07-10')
  })

  it('drills into an agent with only the date range and the first breadcrumb restores the summary filters', async () => {
    listOwners.mockResolvedValue(page([owner({})]))
    const { wrapper, router } = await mountAt('/admin/affiliates/team?start=2026-10-01&end=2026-10-07&level=1&q=zh&sort=earnings&page=2')

    await wrapper.get('[data-test="team-owner-table"] tbody tr').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('AdminAffiliateTeamViewer')
    expect(router.currentRoute.value.params.viewerId).toBe('1024')
    expect(router.currentRoute.value.query).toEqual({ start: '2026-10-01', end: '2026-10-07' })
    expect(viewerOverview).toHaveBeenCalledWith(
      1024,
      expect.objectContaining({ start_date: '2026-10-01', end_date: '2026-10-07', agent_id: undefined, user_id: undefined }),
      expect.anything()
    )

    const crumbs = wrapper.get('[data-test="team-breadcrumb"]')
    expect(crumbs.text().replace(/\s+/g, '')).toBe('admin.affiliates.team.title/张三')
    expect(crumbs.get('a').attributes('href')).toBe(
      '/admin/affiliates/team?start=2026-10-01&end=2026-10-07&level=1&q=zh&sort=earnings&page=2'
    )
  })
})

describe('AdminAffiliateTeamViewerView (an agent\'s view)', () => {
  it('shows the viewed agent with full email, ID, level, upline link and lifetime profit sharing', async () => {
    viewerOverview.mockResolvedValue(viewerData({
      viewer: { agent_level: 2, downgraded: false, settle_interval_minutes: 30 },
      subject: { user: user(2048, '小王', 'wang@163.com'), upline: user(1024, '张三'), lifetime_earnings: 88 }
    }))
    const { wrapper, router } = await mountAt('/admin/affiliates/team/2048?start=2026-10-01&end=2026-10-07')

    expect(viewerOverview).toHaveBeenCalledWith(2048, expect.anything(), expect.anything())
    expect(viewerCustomers).toHaveBeenCalledWith(2048, expect.objectContaining({ sort_by: 'cost' }), expect.anything())
    const meta = wrapper.get('[data-test="team-viewer"]')
    expect(meta.text()).toContain('wang@163.com')
    expect(meta.text()).toContain('#2048')
    expect(meta.get('[data-test="team-viewer-level"]').text()).toBe('admin.affiliates.team.meta.level2')
    // 管理端替换文案统一带上被查看代理的名字（{name}），多余参数 vue-i18n 会忽略。
    expect(meta.text()).toContain('admin.affiliates.team.labels.lifetimeEarnings{"name":"小王","amount":"¥88.00"}')
    const upline = meta.get('[data-test="team-viewer-upline"] a')
    expect(upline.attributes('href')).toBe('/admin/affiliates/team/1024?start=2026-10-01&end=2026-10-07')

    expect(wrapper.find('[data-test="affiliate-section-tabs"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="team-downgraded"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="team-list-title"]').text()).toContain('admin.affiliates.team.labels.directCustomers')
    expect(wrapper.get('[data-test="team-summary-earnings"]').text()).toContain('admin.affiliates.team.labels.earnings')
    // 管理端没有「去邀请」。
    expect(wrapper.get('[data-test="team-empty"]').find('a').exists()).toBe(false)

    // 上级链接换到另一位代理的视角：按新的根重新加载。
    viewerOverview.mockResolvedValue(viewerData())
    await upline.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.params.viewerId).toBe('1024')
    expect(viewerOverview).toHaveBeenLastCalledWith(1024, expect.anything(), expect.anything())
    expect(wrapper.get('[data-test="team-viewer-level"]').text()).toBe('admin.affiliates.team.meta.level1')
  })

  it('names a deleted agent by ID only once, in the breadcrumb', async () => {
    viewerOverview.mockResolvedValue(viewerData({
      viewer: { agent_level: 2, downgraded: false, settle_interval_minutes: 30 },
      subject: { user: { user_id: 3056, nickname: '', email: '', deleted: true }, upline: user(1024, '张三'), lifetime_earnings: 0 }
    }))
    const { wrapper } = await mountAt('/admin/affiliates/team/3056')

    expect(wrapper.get('[data-test="team-breadcrumb"]').text()).toContain('admin.affiliates.team.labels.deleted')
    expect(wrapper.find('[data-test="team-viewer-id"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="team-viewer-level"]').text()).toBe('admin.affiliates.team.meta.level2')
  })

  it('puts the downgrade note into the meta line', async () => {
    viewerOverview.mockResolvedValue(viewerData({ viewer: { agent_level: 3, downgraded: true, settle_interval_minutes: 30 } }))
    const { wrapper } = await mountAt('/admin/affiliates/team/1024')

    expect(wrapper.get('[data-test="team-viewer-level"]').text()).toBe('admin.affiliates.team.meta.downgraded')
    expect(wrapper.find('[data-test="team-downgraded"]').exists()).toBe(false)
  })

  it('links sub-agents inside the viewed team and names the viewer in the former-agent hint', async () => {
    viewerOverview.mockResolvedValue(viewerData({ counts: { direct_customers: 3, sub_agents: 1 } }))
    viewerAgents.mockResolvedValue(page([
      { user: user(21, '老赵'), historical: true, customers: 12, active_customers: 0, team_cost: 0, earnings: 0, cost_share: 0 }
    ]))
    viewerCustomers.mockResolvedValue(page([
      { user: { user_id: 12, nickname: '', email: '', deleted: true }, agent_level: 3, cost: 0, requests: 0, earnings: 0, last_used_at: null, cost_share: 0 }
    ]))
    const { wrapper, router } = await mountAt('/admin/affiliates/team/1024')

    expect(wrapper.get('[data-test="team-tab-customers"]').text()).toContain('admin.affiliates.team.labels.directCustomers')
    expect(wrapper.get('[data-test="team-customer-table"]').text()).toContain('admin.affiliates.team.labels.deleted{"name":"张三","id":12}')

    await wrapper.get('[data-test="team-tab-agents"]').trigger('click')
    await flushPromises()
    const table = wrapper.get('[data-test="team-agent-table"]')
    expect(table.text()).toContain('affiliate.team.historical')
    const hint = table.findComponent({ name: 'HelpTooltip' })
    expect(hint.props('content')).toBe('admin.affiliates.team.labels.historicalHint{"name":"张三"}')

    await table.get('tbody tr').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('AdminAffiliateTeamViewerAgent')
    expect(router.currentRoute.value.params).toEqual({ viewerId: '1024', id: '21' })
    expect(viewerOverview).toHaveBeenLastCalledWith(1024, expect.objectContaining({ agent_id: 21 }), expect.anything())
  })

  it('builds the customer breadcrumb under the viewer and links to the usage records with the current range', async () => {
    viewerOverview.mockResolvedValue(viewerData({
      counts: null,
      customer: {
        user: user(31, '李四', 'li@163.com'),
        agent_level: 3,
        relation: 'upline',
        owner_agent: user(21, '小王'),
        is_sub_agent: false,
        bound_at: '2026-10-01T08:00:00+08:00',
        lifetime_earnings: 88
      },
      filters: { models: [], groups: [] }
    }))
    const { wrapper } = await mountAt('/admin/affiliates/team/1024/customers/31?start=2026-10-01&end=2026-10-07')

    expect(viewerOverview).toHaveBeenCalledWith(1024, expect.objectContaining({ user_id: 31 }), expect.anything())
    expect(viewerRequests).toHaveBeenCalledWith(1024, 31, expect.anything(), expect.anything())
    const crumbs = wrapper.get('[data-test="team-breadcrumb"]')
    expect(crumbs.text().replace(/\s+/g, '')).toBe('admin.affiliates.team.title/张三/小王/李四')
    const links = crumbs.findAll('a').map((link) => link.attributes('href'))
    // 汇总页链接带回最近一次离开汇总页时的筛选（模块内记忆），日期取当前页的。
    expect(links[0]).toMatch(/^\/admin\/affiliates\/team\?start=2026-10-01&end=2026-10-07(&|$)/)
    expect(links.slice(1)).toEqual([
      '/admin/affiliates/team/1024?start=2026-10-01&end=2026-10-07',
      '/admin/affiliates/team/1024/agents/21?start=2026-10-01&end=2026-10-07'
    ])

    const meta = wrapper.get('[data-test="team-subject"]')
    expect(meta.text()).toContain('li@163.com')
    expect(meta.get('[data-test="team-subject-id"]').text()).toBe('#31')
    expect(meta.text()).toContain('"amount":"¥88.00"')
    expect(meta.get('[data-test="team-subject-usage"]').attributes('href')).toBe(
      '/admin/usage?user_id=31&start_date=2026-10-01&end_date=2026-10-07'
    )
  })

  it('shows not found with a link back to the summary for an invalid or unknown agent', async () => {
    const { wrapper } = await mountAt('/admin/affiliates/team/abc')
    expect(viewerOverview).not.toHaveBeenCalled()
    let state = wrapper.get('[data-test="team-not-found"]')
    expect(state.text()).toContain('admin.affiliates.team.labels.notFound')
    expect(state.text()).toContain('admin.affiliates.team.labels.backToSummary')
    expect(state.get('a').attributes('href')).toMatch(/^\/admin\/affiliates\/team(\?|$)/)
    expect(wrapper.get('[data-test="team-breadcrumb"]').text()).toBe('admin.affiliates.team.title')

    const notFound = { status: 404, reason: 'AFFILIATE_TEAM_NOT_FOUND', message: 'customer or agent not found' }
    viewerOverview.mockRejectedValue(notFound)
    viewerCustomers.mockRejectedValue(notFound)
    const second = await mountAt('/admin/affiliates/team/99')
    state = second.wrapper.get('[data-test="team-not-found"]')
    expect(state.text()).toContain('admin.affiliates.team.labels.notFound')
    expect(second.wrapper.find('[data-test="team-summary"]').exists()).toBe(false)
    expect(showError).not.toHaveBeenCalled()
  })
})
