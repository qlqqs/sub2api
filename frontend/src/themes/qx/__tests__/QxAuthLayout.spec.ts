import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

const { appStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: { site_name: 'QxAPI' } as Record<string, unknown>,
    siteName: 'QxAPI',
    siteLogo: '',
    docUrl: '',
    apiBaseUrl: 'https://api.example.com',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn()
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => ({ isAuthenticated: false, isAdmin: false })
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/components/layout/AuthLayout.vue', () => ({
  default: { template: '<div data-testid="upstream-auth"><slot /><slot name="footer" /></div>' }
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import QxAuthLayout from '../QxAuthLayout.vue'
import { qxThemeConfig } from '../config'

function mountLayout() {
  return mount(QxAuthLayout, {
    slots: { default: '<form data-testid="form" />', footer: '<p data-testid="footer">sign up</p>' },
    global: {
      stubs: { RouterLink: RouterLinkStub, LocaleSwitcher: { template: '<div />' }, Icon: { template: '<span />' } }
    }
  })
}

describe('QxAuthLayout', () => {
  beforeEach(() => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })
  afterEach(() => {
    qxThemeConfig.enabled = true
  })

  it('renders the default and footer slots inside the theme shell', () => {
    const wrapper = mountLayout()
    expect(wrapper.find('.qx-card [data-testid="form"]').exists()).toBe(true)
    expect(wrapper.find('.qx-auth-footer [data-testid="footer"]').exists()).toBe(true)
    const endpoints = wrapper.findAll('.qx-auth-stats .qx-stats-item').map((e) => e.text())
    expect(endpoints).toEqual(['OpenAIhttps://api.example.com/v1', 'Anthropichttps://api.example.com'])
    expect(wrapper.find('[data-testid="upstream-auth"]').exists()).toBe(false)
  })

  it('renders the upstream AuthLayout with the same slots when the theme is disabled', () => {
    qxThemeConfig.enabled = false
    const wrapper = mountLayout()
    expect(wrapper.find('[data-testid="upstream-auth"] [data-testid="form"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="upstream-auth"] [data-testid="footer"]').exists()).toBe(true)
  })
})
