import { beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope } from 'vue'
import { createStores } from './mocks'

const stores = vi.hoisted(() => ({ current: null as unknown as ReturnType<typeof import('./mocks').createStores> }))

vi.mock('@/stores', () => ({
  useAppStore: () => stores.current.appStore,
  useAuthStore: () => stores.current.authStore,
  useAdminSettingsStore: () => stores.current.adminSettingsStore
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => stores.current.appStore }))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: stores.current.canUseBatchImage, refreshBatchImageAccess: vi.fn() })
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import { collectNavPaths, useQxNav, type QxNavSection } from '../useQxNav'

function nav() {
  const scope = effectScope()
  const result = scope.run(() => useQxNav())!
  return result.sections
}
const paths = (s: QxNavSection) => s.items.map((i) => i.path)
const byKey = (sections: QxNavSection[]) => Object.fromEntries(sections.map((s) => [s.key, s]))

describe('useQxNav', () => {
  beforeEach(() => {
    stores.current = createStores()
    // 开启 opt-in 的功能，便于检查完整菜单
    stores.current.appStore.cachedPublicSettings = {
      available_channels_enabled: true,
      affiliate_enabled: true,
      risk_control_enabled: true,
      plugin_management_enabled: true
    }
  })

  it('groups user items into CONSOLE / BILLING / ACCOUNT', () => {
    const sections = nav().value
    expect(sections.map((s) => s.key)).toEqual(['console', 'billing', 'account'])
    expect(sections.map((s) => s.title)).toEqual([
      'qxTheme.nav.sections.console',
      'qxTheme.nav.sections.billing',
      'qxTheme.nav.sections.account'
    ])
    const s = byKey(sections)
    expect(paths(s.console)).toEqual(['/dashboard', '/keys', '/batch-image', '/usage', '/available-channels', '/monitor'])
    expect(paths(s.billing)).toEqual(['/subscriptions', '/purchase', '/orders', '/redeem'])
    expect(paths(s.account)).toEqual(['/affiliate', '/profile'])
  })

  it('appends user-visible custom menus to ACCOUNT, sorted', () => {
    stores.current.appStore.cachedPublicSettings.custom_menu_items = [
      { id: 'b', label: 'B', icon_svg: '<svg/>', url: 'x', visibility: 'user', sort_order: 2 },
      { id: 'a', label: 'A', icon_svg: '', url: 'x', visibility: 'user', sort_order: 1 },
      { id: 'z', label: 'Z', icon_svg: '', url: 'x', visibility: 'admin', sort_order: 0 }
    ]
    expect(paths(byKey(nav().value).account)).toEqual(['/affiliate', '/profile', '/custom/a', '/custom/b'])
  })

  it('hides feature-flagged items and drops sections left empty', () => {
    const { appStore, canUseBatchImage } = stores.current
    appStore.cachedPublicSettings = {
      available_channels_enabled: false,
      channel_monitor_enabled: false,
      payment_enabled: false,
      subscription_enabled: false,
      affiliate_enabled: false
    }
    canUseBatchImage.value = false
    const s = byKey(nav().value)
    expect(paths(s.console)).toEqual(['/dashboard', '/keys', '/usage'])
    expect(paths(s.billing)).toEqual(['/redeem'])
    expect(paths(s.account)).toEqual(['/profile'])
  })

  it('derives the purchase label from the billing mode', () => {
    stores.current.appStore.cachedPublicSettings.subscription_enabled = false
    const purchase = byKey(nav().value).billing?.items.find((i) => i.path === '/purchase')
    expect(purchase?.label).toBe('nav.recharge')
  })

  it('applies simple mode for users', () => {
    stores.current.authStore.isSimpleMode = true
    const sections = nav().value
    expect(sections.map((s) => s.key)).toEqual(['console', 'account'])
    expect(collectNavPaths(sections)).toEqual(['/dashboard', '/keys', '/monitor', '/profile'])
  })

  it('shows no navigation to regular users in backend mode', () => {
    stores.current.appStore.backendModeEnabled = true
    expect(nav().value).toEqual([])
  })

  it('builds admin sections with groups and a MY ACCOUNT section', () => {
    stores.current.authStore.isAdmin = true
    stores.current.adminSettingsStore.customMenuItems = [
      { id: 'adm', label: 'Adm', icon_svg: '', url: 'x', visibility: 'admin', sort_order: 1 }
    ]
    const sections = nav().value
    const s = byKey(sections)
    expect(sections.map((x) => x.key)).toEqual(['overview', 'resources', 'business', 'contentSecurity', 'system', 'myAccount'])
    expect(paths(s.overview)).toEqual(['/admin/dashboard', '/admin/ops', '/admin/usage', '/admin/audit-logs'])
    expect(paths(s.resources)).toEqual(['/admin/groups', '/admin/channels', '/admin/accounts', '/admin/proxies', '/admin/plugins'])
    expect(paths(s.business)).toEqual([
      '/admin/users',
      '/admin/subscriptions',
      '/admin/redeem',
      '/admin/promo-codes',
      '/admin/affiliates',
      '/admin/orders',
      '/admin/finance'
    ])
    expect(paths(s.contentSecurity)).toEqual(['/admin/announcements', '/admin/security-audit'])
    expect(paths(s.system)).toEqual(['/admin/settings', '/custom/adm'])
    expect(paths(s.myAccount)[0]).toBe('/keys')
    expect(paths(s.myAccount)).not.toContain('/dashboard')
    const affiliates = s.business.items.find((i) => i.path === '/admin/affiliates')!
    expect(affiliates.expandOnly).toBe(true)
    expect(affiliates.children?.map((c) => c.path)).toEqual([
      '/admin/affiliates/agents',
      '/admin/affiliates/team',
      '/admin/affiliates/invites',
      '/admin/affiliates/rebates',
      '/admin/affiliates/transfers'
    ])
  })

  it('hides admin-only flags (ops, payment, risk control, channel monitor child)', () => {
    const { authStore, adminSettingsStore, appStore } = stores.current
    authStore.isAdmin = true
    adminSettingsStore.opsMonitoringEnabled = false
    adminSettingsStore.paymentEnabled = false
    appStore.cachedPublicSettings = { channel_monitor_enabled: false }
    const sections = nav().value
    const all = collectNavPaths(sections)
    for (const p of ['/admin/ops', '/admin/orders', '/admin/security-audit', '/admin/channels/monitor', '/admin/plugins', '/admin/affiliates']) {
      expect(all).not.toContain(p)
    }
    expect(all).toContain('/admin/channels/pricing')
  })

  it('admin simple mode keeps settings and puts API keys in its own section', () => {
    stores.current.authStore.isAdmin = true
    stores.current.authStore.isSimpleMode = true
    const sections = nav().value
    const s = byKey(sections)
    expect(sections.map((x) => x.key)).toEqual(['overview', 'resources', 'contentSecurity', 'system', 'myAccount'])
    expect(paths(s.overview)).toEqual(['/admin/dashboard', '/admin/ops', '/admin/usage'])
    expect(paths(s.resources)).toEqual(['/admin/groups', '/admin/accounts', '/admin/proxies', '/admin/plugins'])
    expect(paths(s.myAccount)).toEqual(['/keys'])
    expect(paths(s.system)).toEqual(['/admin/settings'])
  })

  it('admins keep their navigation in backend mode', () => {
    stores.current.authStore.isAdmin = true
    stores.current.appStore.backendModeEnabled = true
    expect(nav().value.length).toBeGreaterThan(0)
  })
})
