import { computed, ref } from 'vue'
import { useAppStore, useAuthStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

/** CUSTOM: QxAPI 主题页面共用的站点信息（取值方式与上游 HomeView / AuthLayout 相同）。 */
export function useQxSite() {
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const settings = computed(() => appStore.cachedPublicSettings)

  const siteName = computed(() => settings.value?.site_name || appStore.siteName || 'Sub2API')
  const siteLogo = computed(() =>
    sanitizeUrl(settings.value?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
  )
  const siteSubtitle = computed(() => settings.value?.site_subtitle || '')
  const docUrl = computed(() => sanitizeUrl(settings.value?.doc_url || appStore.docUrl || ''))
  /** 客户端填写的接口根地址：优先后台配置的 API 地址，否则用当前站点。 */
  const apiOrigin = computed(() => {
    const configured = sanitizeUrl(appStore.apiBaseUrl || settings.value?.api_base_url || '')
    return (configured || window.location?.origin || '').replace(/\/+$/, '')
  })

  const isAuthenticated = computed(() => authStore.isAuthenticated)
  const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
  const backendMode = computed(() => settings.value?.backend_mode_enabled === true)
  const registrationEnabled = computed(() => settings.value?.registration_enabled === true && !backendMode.value)
  const modelPlazaVisible = computed(
    () =>
      isFeatureFlagEnabled(FeatureFlags.modelPlaza) &&
      (isAuthenticated.value || settings.value?.model_plaza_require_auth !== true)
  )
  /** API 密钥页是否显示「导入到 CCS」按钮（后台可隐藏）。 */
  const ccsImportVisible = computed(() => settings.value?.hide_ccs_import_button !== true)

  return {
    siteName,
    siteLogo,
    siteSubtitle,
    docUrl,
    apiOrigin,
    isAuthenticated,
    dashboardPath,
    registrationEnabled,
    modelPlazaVisible,
    ccsImportVisible
  }
}

/** 深浅色切换，与上游首页使用同一个 localStorage 键。 */
export function useQxColorScheme() {
  const isDark = ref(document.documentElement.classList.contains('dark'))

  function init() {
    const saved = localStorage.getItem('theme')
    if (saved === 'dark' || (!saved && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
      isDark.value = true
      document.documentElement.classList.add('dark')
    }
  }

  function toggle() {
    isDark.value = !isDark.value
    document.documentElement.classList.toggle('dark', isDark.value)
    localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
  }

  return { isDark, init, toggle }
}
