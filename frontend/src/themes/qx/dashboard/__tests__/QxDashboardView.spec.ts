import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { makeStats } from './fixtures'

const { appStore, authStore, usageAPI, batchImage } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'QxAPI',
    siteLogo: '',
    docUrl: '',
    apiBaseUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn()
  },
  authStore: {
    isAuthenticated: true,
    isAdmin: false,
    isSimpleMode: false,
    user: { balance: 88.5 } as { balance: number } | null,
    refreshUser: vi.fn()
  },
  usageAPI: {
    getDashboardStats: vi.fn(),
    getDashboardTrend: vi.fn(),
    getDashboardModels: vi.fn()
  },
  batchImage: { allowed: false, refresh: vi.fn() }
}))

vi.mock('@/stores', () => ({ useAppStore: () => appStore, useAuthStore: () => authStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/api/usage', () => ({ usageAPI }))
vi.mock('@/composables/useBatchImageAccess', async () => {
  const { computed } = await import('vue')
  return {
    useBatchImageAccess: () => ({ canUseBatchImage: computed(() => batchImage.allowed), refreshBatchImageAccess: batchImage.refresh })
  }
})
vi.mock('@/utils/featureFlags', () => ({ FeatureFlags: { modelPlaza: 'modelPlaza' }, isFeatureFlagEnabled: () => true }))
vi.mock('@/views/user/DashboardView.vue', () => ({ __esModule: true, default: { template: '<div data-testid="upstream-dashboard" />' } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div data-testid="app-layout"><slot /></div>' } }))
vi.mock('@/components/user/dashboard/AnnouncementBanner.vue', () => ({
  default: { props: ['variant'], template: '<div data-testid="banner" :data-variant="variant" />' }
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})

import QxDashboardView from '../QxDashboardView.vue'
import { qxThemeConfig } from '../../config'

const modelStat = (model: string, actual: number, tokens: number, requests: number) => ({
  model, requests, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: tokens, cost: actual * 3, actual_cost: actual
})

const trendPoint = (date: string, actual: number, requests: number) => ({
  date, requests, input_tokens: 1, output_tokens: 1, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 2, cost: actual * 3, actual_cost: actual
})

function mountView() {
  return mount(QxDashboardView, { global: { stubs: { RouterLink: RouterLinkStub, LoadingSpinner: true } } })
}

describe('QxDashboardView', () => {
  beforeEach(() => {
    qxThemeConfig.enabled = true
    authStore.isSimpleMode = false
    authStore.user = { balance: 88.5 }
    appStore.cachedPublicSettings = {}
    batchImage.allowed = false
    batchImage.refresh.mockReset()
    authStore.refreshUser.mockReset().mockResolvedValue({})
    usageAPI.getDashboardStats.mockReset().mockResolvedValue(
      makeStats({
        total_api_keys: 3,
        active_api_keys: 2,
        today_requests: 1284,
        total_requests: 38912,
        today_actual_cost: 3.42,
        today_cost: 9.76,
        total_actual_cost: 1.4,
        rpm: 4,
        tpm: 18200
      })
    )
    usageAPI.getDashboardTrend.mockReset().mockResolvedValue({ trend: [trendPoint('2026-10-22', 1, 10), trendPoint('2026-10-23', 2, 20)] })
    usageAPI.getDashboardModels
      .mockReset()
      .mockResolvedValue({ models: [modelStat('gpt-small', 1, 1000, 12), modelStat('gpt-big', 3, 4000, 30)] })
  })

  it('renders the upstream dashboard when the theme is disabled', async () => {
    qxThemeConfig.enabled = false
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="upstream-dashboard"]').exists()).toBe(true)
    expect(wrapper.find('.qx-dash').exists()).toBe(false)
    expect(usageAPI.getDashboardStats).not.toHaveBeenCalled()
  })

  it('renders the banner, today overview, model mix and quick actions', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="upstream-dashboard"]').exists()).toBe(false)
    const dash = wrapper.find('[data-testid="app-layout"] .qx-dash')
    expect(dash.exists()).toBe(true)
    expect(dash.findAll(':scope > *').map((el) => el.attributes('data-testid') ?? el.classes()[0])).toEqual(['banner', 'qd-overview', 'qd-row'])
    expect(dash.findAll('.qd-row > section').map((el) => el.attributes('data-testid'))).toEqual(['qd-mix', 'qd-quick'])
    expect(wrapper.find('[data-testid="banner"]').attributes('data-variant')).toBe('qx')

    const overview = wrapper.find('[data-testid="qd-overview"]')
    expect(wrapper.find('[data-testid="qd-balance"]').text()).toContain('¥88.50')
    expect(wrapper.find('[data-testid="qd-balance"]').findComponent(RouterLinkStub).props('to')).toBe('/purchase')
    expect(overview.find('[data-testid="qd-today-cost"]').text()).toBe('¥3.42')
    expect(overview.text()).toContain('$9.76')
    expect(overview.text()).toContain('1,284')
    expect(overview.text()).toContain('38,912')
    expect(overview.text()).toContain('RPM 4')
    expect(overview.text()).toContain('TPM 18.2K')
    expect(overview.text()).toContain('2 / 3')
    expect(overview.findAll('.qd-spark')).toHaveLength(2)
  })

  it('loads the last 7 days by day for the sparklines and the model mix', async () => {
    mountView()
    await flushPromises()
    expect(usageAPI.getDashboardTrend).toHaveBeenCalledTimes(1)
    const params = usageAPI.getDashboardTrend.mock.calls[0][0]
    expect(params.granularity).toBe('day')
    expect((new Date(params.end_date).getTime() - new Date(params.start_date).getTime()) / 86400000).toBe(6)
    expect(usageAPI.getDashboardModels).toHaveBeenCalledWith({ start_date: params.start_date, end_date: params.end_date })
  })

  it('lists models by actual cost with their share', async () => {
    const wrapper = mountView()
    await flushPromises()
    const rows = wrapper.findAll('[data-testid="qd-mix-row"]')
    expect(rows.map((r) => r.find('.qd-mix-name').text())).toEqual(['gpt-big', 'gpt-small'])
    expect(rows.map((r) => r.find('.qd-mix-share').text())).toEqual(['75%', '25%'])
    expect(rows[0].find('.qd-mix-cost').text()).toBe('¥3.00')
    expect(wrapper.find('[data-testid="qd-mix"]').text()).toContain('qxTheme.dashboard.mix.range')
  })

  it('shows an empty model mix without usage', async () => {
    usageAPI.getDashboardModels.mockResolvedValue({ models: [] })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="qd-mix"]').text()).toContain('dashboard.noDataAvailable')
  })

  it('hides the balance in simple mode', async () => {
    authStore.isSimpleMode = true
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="qd-balance"]').exists()).toBe(false)
    expect(wrapper.find('.qd-kpis').classes()).toContain('qd-kpis-3')
  })

  it('draws no sparklines without trend data', async () => {
    usageAPI.getDashboardTrend.mockResolvedValue({ trend: [] })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="qd-overview"]').exists()).toBe(true)
    expect(wrapper.find('.qd-spark').exists()).toBe(false)
  })

  describe('quick actions', () => {
    const actions = (w: ReturnType<typeof mountView>) => w.findAll('[data-testid="qd-action"]').map((a) => a.attributes('data-action'))

    it('shows create key, CC Switch import and redeem by default', async () => {
      const wrapper = mountView()
      await flushPromises()
      expect(actions(wrapper)).toEqual(['create-key', 'ccs-import', 'redeem'])
      const links = wrapper.findAll('[data-testid="qd-action"]').map((a) => a.findComponent(RouterLinkStub).props('to'))
      expect(links).toEqual(['/keys', '/keys', '/redeem'])
      expect(wrapper.find('[data-action="ccs-import"]').text()).toContain('qxTheme.dashboard.quick.recommended')
      expect(batchImage.refresh).toHaveBeenCalled()
    })

    it('hides the CC Switch import when the admin hides the CCS button', async () => {
      appStore.cachedPublicSettings = { hide_ccs_import_button: true }
      const wrapper = mountView()
      await flushPromises()
      expect(actions(wrapper)).toEqual(['create-key', 'redeem'])
    })

    it('shows batch image only when a key allows it', async () => {
      batchImage.allowed = true
      const wrapper = mountView()
      await flushPromises()
      expect(actions(wrapper)).toEqual(['create-key', 'ccs-import', 'batch-image', 'redeem'])
      expect(wrapper.find('[data-action="batch-image"]').findComponent(RouterLinkStub).props('to')).toBe('/batch-image')
    })
  })
})
