import { describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ checkAuth: vi.fn(), isAuthenticated: false, isAdmin: false, isSimpleMode: false })
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ siteName: 'Sub2API', backendModeEnabled: false, cachedPublicSettings: null })
}))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: [] }) }))
vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({ startNavigation: vi.fn(), endNavigation: vi.fn(), isLoading: { value: false } })
}))
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({ triggerPrefetch: vi.fn(), cancelPendingPrefetch: vi.fn(), resetPrefetchState: vi.fn() })
}))

// CUSTOM: 合伙人路由
describe('partner routes', () => {
  it.each([
    ['AffiliateRegion', '/affiliate/region'],
    ['AffiliateRegionCustomer', '/affiliate/region/customers/:id'],
    ['AffiliateRegionDirect', '/affiliate/region/direct'],
    ['AffiliateRegionDirectAgent', '/affiliate/region/direct/agents/:id'],
    ['AffiliateRegionDirectCustomer', '/affiliate/region/direct/customers/:id'],
    ['AffiliateRegionAgent', '/affiliate/region/agents/:agentId'],
    ['AffiliateRegionAgentAgent', '/affiliate/region/agents/:agentId/agents/:id'],
    ['AffiliateRegionAgentCustomer', '/affiliate/region/agents/:agentId/customers/:id'],
    ['AffiliatePartner', '/affiliate/partner'],
    ['AffiliatePartnerStatement', '/affiliate/partner/:month']
  ])('registers %s as a user route under the affiliate title', async (name, path) => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((record) => record.name === name)
    expect(route?.path).toBe(path)
    expect(route?.meta).toMatchObject({ requiresAuth: true, requiresAdmin: false, titleKey: 'affiliate.title', descriptionKey: 'affiliate.description' })
  })

  it.each([
    ['AdminAffiliatePartners', '/admin/affiliates/partners'],
    ['AdminAffiliatePartnerDetail', '/admin/affiliates/partners/:userId']
  ])('registers %s as an admin route', async (name, path) => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((record) => record.name === name)
    expect(route?.path).toBe(path)
    expect(route?.meta).toMatchObject({ requiresAuth: true, requiresAdmin: true, titleKey: 'nav.affiliatePartners' })
  })
})
