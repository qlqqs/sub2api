import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'

const { appStore, authStore, getModelPlaza } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Fallback',
    siteLogo: '',
    docUrl: '',
    apiBaseUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn()
  },
  authStore: { isAuthenticated: false, isAdmin: false, checkAuth: vi.fn() },
  getModelPlaza: vi.fn()
}))

vi.mock('@/stores', () => ({ useAppStore: () => appStore, useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/api/modelPlaza', () => ({ getModelPlaza }))
vi.mock('@/utils/featureFlags', () => ({ FeatureFlags: { modelPlaza: 'modelPlaza' }, isFeatureFlagEnabled: () => true }))
vi.mock('@/views/HomeView.vue', () => ({ __esModule: true, default: { template: '<div data-testid="upstream-home" />' } }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})

import QxHomeView from '../QxHomeView.vue'
import { qxThemeConfig } from '../config'

const plazaGroup = {
  id: 2,
  name: 'Codex 稳定分组',
  description: '稳定不降智',
  platform: 'openai',
  rate_multiplier: 0.35,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  models: [{
    name: 'gpt-6.1-sol',
    platform: 'openai',
    official_pricing: null,
    pricing: { billing_mode: 'token', input_price: 2e-6, output_price: 1e-5, cache_read_price: 1e-7 }
  }]
}

function mountHome(settings: Record<string, unknown> = {}) {
  appStore.cachedPublicSettings = { site_name: 'QxAPI', site_subtitle: '副标题', registration_enabled: true, ...settings }
  return mount(QxHomeView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: { template: '<div />' },
        Icon: { template: '<span />' }
      }
    }
  })
}

describe('QxHomeView', () => {
  beforeEach(() => {
    qxThemeConfig.enabled = true
    authStore.isAuthenticated = false
    appStore.apiBaseUrl = ''
    getModelPlaza.mockReset().mockResolvedValue({ description: '', groups: [plazaGroup] })
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })

  it('renders the theme home with model plaza prices', async () => {
    appStore.apiBaseUrl = 'https://api.example.com'
    const wrapper = mountHome()
    await flushPromises()
    expect(wrapper.find('[data-testid="upstream-home"]').exists()).toBe(false)
    expect(wrapper.find('h1').text()).toBe('QxAPI')
    expect(wrapper.text()).toContain('¥0.70')
    expect(wrapper.text()).toContain('¥3.50')
    expect(wrapper.text()).toContain('https://api.example.com/v1')
    expect(wrapper.text()).toContain('gpt-6.1-sol')
    expect(wrapper.text()).not.toContain('0.35')
  })

  it.each([
    ['custom home content', { home_content: '<p>hi</p>' }],
    ['compact home', { compact_home_enabled: true }]
  ])('falls back to the upstream home when %s is configured', async (_, settings) => {
    const wrapper = mountHome(settings)
    await flushPromises()
    expect(wrapper.find('[data-testid="upstream-home"]').exists()).toBe(true)
    expect(getModelPlaza).not.toHaveBeenCalled()
  })

  it('falls back to the upstream home when the theme is disabled', async () => {
    qxThemeConfig.enabled = false
    const wrapper = mountHome()
    await flushPromises()
    expect(wrapper.find('[data-testid="upstream-home"]').exists()).toBe(true)
  })

  it('hides the pricing section when the model plaza request fails', async () => {
    getModelPlaza.mockRejectedValue(new Error('404'))
    const wrapper = mountHome()
    await flushPromises()
    expect(wrapper.text()).not.toContain('qxTheme.pricing.title')
    expect(wrapper.text()).toContain('qxTheme.steps.title')
  })

  it('branches step 3 into CC Switch import and manual setup', async () => {
    appStore.apiBaseUrl = 'https://api.example.com'
    const wrapper = mountHome()
    await flushPromises()
    expect(wrapper.findAll('.qx-flow-branch')).toHaveLength(2)
    const ends = wrapper.findAll('.qx-flow-end')
    expect(ends).toHaveLength(2)
    expect(ends[0].text()).toContain('qxTheme.steps.ccs.title')
    expect(ends[0].findComponent(RouterLinkStub).props('to')).toBe('/keys')
    expect(ends[1].text()).toContain('https://api.example.com/v1')
    expect(wrapper.find('.qx-flow-foot').text()).toContain('/v1/messages')
  })

  it('places setup before pricing and links to the CC Switch download page', async () => {
    const wrapper = mountHome()
    await flushPromises()
    expect(wrapper.findAll('main > section h2').map((heading) => heading.text())).toEqual([
      'qxTheme.steps.title',
      'qxTheme.pricing.title'
    ])
    expect(wrapper.find('.qx-flow-links a[target="_blank"]').attributes('href'))
      .toBe('https://ccswitch.io/en/download')
  })

  it('shows curl examples for both protocols without duplicating v1', async () => {
    appStore.apiBaseUrl = 'https://api.example.com/v1/'
    const wrapper = mountHome()
    await flushPromises()
    const console = wrapper.find('.qx-console')
    expect(console.find('pre').text()).toContain('curl https://api.example.com/v1/responses')
    await console.findAll('[role="tab"]')[1].trigger('click')
    const example = console.find('pre').text()
    expect(example).toContain('curl https://api.example.com/v1/messages')
    expect(example).toContain('"x-api-key: $API_KEY"')
    expect(example).toContain('"anthropic-version: 2023-06-01"')
    expect(example).not.toContain('export ANTHROPIC_')
    const payload = JSON.parse(example.match(/-d '([\s\S]+)'/)![1])
    expect(payload).toEqual({
      model: 'gpt-6.1-sol',
      max_tokens: 1024,
      messages: [{ role: 'user', content: 'qxTheme.console.prompt' }]
    })
    await console.findAll('[role="tab"]')[0].trigger('click')
    expect(console.find('pre').text()).toContain('curl https://api.example.com/v1/responses')
  })

  it('keeps a model placeholder in the Anthropic example if prices are unavailable', async () => {
    getModelPlaza.mockRejectedValue(new Error('404'))
    const wrapper = mountHome()
    await flushPromises()
    await wrapper.findAll('.qx-console [role="tab"]')[1].trigger('click')
    expect(wrapper.find('.qx-console pre').text()).toContain('"model": "MODEL"')
  })

  it('keeps only manual setup when the CCS import button is hidden', async () => {
    const wrapper = mountHome({ hide_ccs_import_button: true })
    await flushPromises()
    expect(wrapper.find('.qx-flow').classes()).toContain('qx-flow-single')
    expect(wrapper.findAll('.qx-flow-end')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('qxTheme.steps.ccs.title')
    expect(wrapper.text()).toContain('qxTheme.steps.manual.title')
  })

  it('shows group tabs only when there is more than one group', async () => {
    getModelPlaza.mockResolvedValue({ description: '', groups: [plazaGroup, { ...plazaGroup, id: 3, name: '第二分组' }] })
    const wrapper = mountHome()
    await flushPromises()
    const tabs = wrapper.findAll('.qx-tab')
    expect(tabs.map((t) => t.find('.qx-tab-name').text())).toEqual(['Codex 稳定分组', '第二分组'])
    expect(tabs[0].find('b').text()).toBe('1')
    await tabs[1].trigger('click')
    expect(tabs[1].attributes('aria-selected')).toBe('true')
  })
})
