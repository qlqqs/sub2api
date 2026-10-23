import { describe, expect, it, vi } from 'vitest'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isSimpleMode: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  cachedPublicSettings: null as null | Record<string, unknown>,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({
    customMenuItems: [],
  }),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

describe('admin affiliate team routes', () => {
  it.each([
    ['AdminAffiliateTeam', '/admin/affiliates/team'],
    ['AdminAffiliateTeamViewer', '/admin/affiliates/team/:viewerId'],
    ['AdminAffiliateTeamViewerAgent', '/admin/affiliates/team/:viewerId/agents/:id'],
    ['AdminAffiliateTeamViewerCustomer', '/admin/affiliates/team/:viewerId/customers/:id'],
  ])('registers %s as an admin route', async (name, path) => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((record) => record.name === name)

    expect(route?.path).toBe(path)
    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.requiresAdmin).toBe(true)
    // 四个页面的顶部栏一致：「团队消费」+「按代理查看团队的余额消费与分润」。
    expect(route?.meta.titleKey).toBe('nav.affiliateTeam')
    expect(route?.meta.descriptionKey).toBe('admin.affiliates.teamDescription')
  })

  it('serves the three levels of an agent view with the same lazily loaded view', async () => {
    const { default: router } = await import('@/router')
    const loaders = ['AdminAffiliateTeamViewer', 'AdminAffiliateTeamViewerAgent', 'AdminAffiliateTeamViewerCustomer'].map((name) => {
      const component = router.getRoutes().find((record) => record.name === name)?.components?.default
      expect(typeof component).toBe('function')
      return component as () => Promise<{ default: unknown }>
    })

    const views = await Promise.all(loaders.map((load) => load()))
    expect(new Set(views.map((view) => view.default)).size).toBe(1)
  })
})
