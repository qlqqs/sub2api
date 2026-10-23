// 测试共用的 store / i18n 替身：reactive 对象，修改字段后 computed 菜单会重新计算。
import { reactive, ref } from 'vue'

export function createStores() {
  const appStore = reactive({
    cachedPublicSettings: {} as Record<string, unknown>,
    backendModeEnabled: false,
    sidebarCollapsed: false,
    mobileOpen: false,
    sidebarScrollTop: 0,
    siteName: 'QxAPI',
    siteLogo: '',
    siteVersion: '0.2.17',
    publicSettingsLoaded: true,
    toggleSidebar() {
      appStore.sidebarCollapsed = !appStore.sidebarCollapsed
    },
    toggleMobileSidebar() {
      appStore.mobileOpen = !appStore.mobileOpen
    },
    setMobileOpen(open: boolean) {
      appStore.mobileOpen = open
    }
  })
  const authStore = reactive({ isAdmin: false, isSimpleMode: false, user: { role: 'user' } as { role: string } | null })
  const adminSettingsStore = reactive({
    opsMonitoringEnabled: true,
    paymentEnabled: true,
    customMenuItems: [] as Array<Record<string, unknown>>,
    fetch: () => Promise.resolve()
  })
  const onboardingStore = reactive({
    current: '' as string,
    isCurrentStep: (selector: string) => onboardingStore.current === selector,
    nextStep: (_delay?: number) => Promise.resolve(),
    setReplayCallback: (_cb: unknown) => {}
  })
  const canUseBatchImage = ref(true)
  return { appStore, authStore, adminSettingsStore, onboardingStore, canUseBatchImage }
}
