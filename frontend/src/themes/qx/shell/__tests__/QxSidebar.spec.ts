import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { createStores } from './mocks'

const stores = vi.hoisted(() => ({
  current: null as unknown as ReturnType<typeof import('./mocks').createStores>,
  route: null as unknown as { path: string },
  push: null as unknown as (path: string) => void
}))

vi.mock('@/stores', () => ({
  useAppStore: () => stores.current.appStore,
  useAuthStore: () => stores.current.authStore,
  useAdminSettingsStore: () => stores.current.adminSettingsStore,
  useOnboardingStore: () => stores.current.onboardingStore
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => stores.current.appStore }))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: stores.current.canUseBatchImage, refreshBatchImageAccess: vi.fn() })
}))
vi.mock('vue-router', () => ({
  useRoute: () => stores.route,
  useRouter: () => ({ push: (p: string) => stores.push(p) })
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import QxSidebar from '../QxSidebar.vue'

function mountSidebar(path: string, admin = false) {
  stores.route = reactive({ path })
  stores.current.authStore.isAdmin = admin
  return mount(QxSidebar, {
    global: { stubs: { RouterLink: RouterLinkStub, VersionBadge: { template: '<span class="vb" />' } } }
  })
}

describe('QxSidebar', () => {
  beforeEach(() => {
    stores.current = createStores()
    stores.current.appStore.cachedPublicSettings = { affiliate_enabled: true, available_channels_enabled: true }
    stores.push = vi.fn()
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })
  afterEach(() => {
    vi.useRealTimers()
    localStorage.removeItem('theme')
    document.documentElement.classList.remove('dark')
  })

  it('renders section kickers, brand and highlights the active item', () => {
    const wrapper = mountSidebar('/usage')
    expect(wrapper.findAll('.qx-side-kicker').map((k) => k.text())).toEqual([
      'qxTheme.nav.sections.console',
      'qxTheme.nav.sections.billing',
      'qxTheme.nav.sections.account'
    ])
    expect(wrapper.find('.qx-side-name').text()).toBe('QxAPI')
    const active = wrapper.findAll('.qx-side-link.is-active')
    expect(active).toHaveLength(1)
    expect(active[0].text()).toBe('nav.usage')
  })

  it('keeps the user onboarding selector and advances the tour on /keys click', async () => {
    const { onboardingStore } = stores.current
    const nextStep = vi.spyOn(onboardingStore, 'nextStep')
    const wrapper = mountSidebar('/dashboard')
    const keys = wrapper.find('[data-tour="sidebar-my-keys"]')
    expect(keys.exists()).toBe(true)

    await keys.trigger('click')
    expect(nextStep).not.toHaveBeenCalled()

    onboardingStore.current = '[data-tour="sidebar-my-keys"]'
    await keys.trigger('click')
    expect(nextStep).toHaveBeenCalledWith(500)
  })

  it('keeps the admin onboarding selectors', async () => {
    const { onboardingStore } = stores.current
    const nextStep = vi.spyOn(onboardingStore, 'nextStep')
    const wrapper = mountSidebar('/admin/dashboard', true)
    expect(wrapper.find('#sidebar-channel-manage').exists()).toBe(true)
    expect(wrapper.find('#sidebar-group-manage').exists()).toBe(true)
    expect(wrapper.find('#sidebar-wallet').exists()).toBe(true)
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(true)
    const to = (id: string) => wrapper.findAllComponents(RouterLinkStub).find((c) => c.attributes('id') === id)?.props('to')
    expect(to('sidebar-channel-manage')).toBe('/admin/accounts')
    expect(to('sidebar-group-manage')).toBe('/admin/groups')
    expect(to('sidebar-wallet')).toBe('/admin/redeem')

    onboardingStore.current = '#sidebar-group-manage'
    await wrapper.find('#sidebar-group-manage').trigger('click')
    expect(nextStep).toHaveBeenCalledWith(500)
  })

  it('expands groups on child routes and toggles expandOnly groups without navigating', async () => {
    const wrapper = mountSidebar('/admin/affiliates/team/42', true)
    const groupButton = () =>
      wrapper.findAll('button.qx-side-link').find((b) => b.text().includes('nav.affiliateManagement'))!
    // 下钻页：分组自动展开，最具体的子项高亮
    expect(groupButton().attributes('aria-expanded')).toBe('true')
    const activeChild = wrapper.findAll('.qx-side-children .is-active')
    expect(activeChild.map((a) => a.text())).toEqual(['nav.affiliateTeam'])

    await groupButton().trigger('click')
    expect(groupButton().attributes('aria-expanded')).toBe('false')
    expect(groupButton().classes()).toContain('is-active')
    expect(stores.push).not.toHaveBeenCalled()

    // 订单组同时包含 /admin/orders 与 /admin/orders/dashboard，只高亮最具体的一项
    const orders = mountSidebar('/admin/orders/dashboard', true)
    expect(orders.findAll('.qx-side-children .is-active').map((a) => a.text())).toEqual(['nav.paymentDashboard'])
  })

  it('collapses to icons with title tooltips', async () => {
    stores.current.appStore.sidebarCollapsed = true
    const wrapper = mountSidebar('/keys')
    expect(wrapper.find('aside').classes()).toContain('is-collapsed')
    const keys = wrapper.find('[data-tour="sidebar-my-keys"]')
    expect(keys.attributes('title')).toBe('nav.apiKeys')
    await wrapper.find('.qx-side-collapse').trigger('click')
    expect(stores.current.appStore.sidebarCollapsed).toBe(false)
  })

  it('closes the mobile drawer 150ms after a navigation click', async () => {
    vi.useFakeTimers()
    stores.current.appStore.mobileOpen = true
    const wrapper = mountSidebar('/dashboard')
    expect(wrapper.find('aside').classes()).toContain('is-open')
    expect(wrapper.find('.qx-side-overlay').exists()).toBe(true)
    await wrapper.find('[data-tour="sidebar-my-keys"]').trigger('click')
    expect(stores.current.appStore.mobileOpen).toBe(true)
    vi.advanceTimersByTime(150)
    expect(stores.current.appStore.mobileOpen).toBe(false)
  })

  it('toggles dark mode on <html> and persists it', async () => {
    const wrapper = mountSidebar('/dashboard')
    const toggle = wrapper.findAll('.qx-side-foot button')[0]
    expect(toggle.text()).toBe('nav.darkMode')
    await toggle.trigger('click')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(toggle.text()).toBe('nav.lightMode')
  })

  it('fetches admin settings for admins', async () => {
    const fetch = vi.spyOn(stores.current.adminSettingsStore, 'fetch')
    mountSidebar('/admin/dashboard', true)
    await flushPromises()
    expect(fetch).toHaveBeenCalled()
  })
})
