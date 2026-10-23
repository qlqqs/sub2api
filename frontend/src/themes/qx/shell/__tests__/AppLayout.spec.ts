// 上游 AppLayout.vue 的主题钩子：主题开启渲染 QxShell，关闭时渲染上游原版（AppSidebar + AppHeader）。
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const replayTour = vi.fn()
const setReplayCallback = vi.fn()

vi.mock('@/stores', () => ({ useAppStore: () => ({ sidebarCollapsed: false }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { role: 'user' } }) }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ setReplayCallback }) }))
vi.mock('@/composables/useOnboardingTour', () => ({ useOnboardingTour: () => ({ replayTour }) }))
vi.mock('@/components/layout/AppSidebar.vue', () => ({ default: { template: '<aside data-testid="upstream-sidebar" />' } }))
vi.mock('@/components/layout/AppHeader.vue', () => ({ default: { template: '<header data-testid="upstream-header" />' } }))
vi.mock('@/themes/qx/shell/QxShell.vue', () => ({
  default: { template: '<div data-testid="qx-shell"><slot /></div>' }
}))

import AppLayout from '@/components/layout/AppLayout.vue'
import { qxThemeConfig } from '@/themes/qx/config'

const mountLayout = () => mount(AppLayout, { slots: { default: '<p data-testid="page">page</p>' } })

describe('AppLayout theme hook', () => {
  afterEach(() => {
    qxThemeConfig.enabled = true
    setReplayCallback.mockClear()
  })

  it('renders QxShell with the page when the theme is enabled', () => {
    const wrapper = mountLayout()
    expect(wrapper.find('[data-testid="qx-shell"] [data-testid="page"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="upstream-sidebar"]').exists()).toBe(false)
    expect(setReplayCallback).toHaveBeenCalledWith(replayTour)
  })

  it('renders the upstream markup when the theme is disabled', () => {
    qxThemeConfig.enabled = false
    const wrapper = mountLayout()
    expect(wrapper.find('[data-testid="qx-shell"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="upstream-sidebar"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="upstream-header"]').exists()).toBe(true)
    expect(wrapper.find('main [data-testid="page"]').exists()).toBe(true)
    expect(setReplayCallback).toHaveBeenCalledWith(replayTour)
  })
})
