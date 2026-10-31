import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/router', () => ({
  default: {
    currentRoute: {
      value: { name: 'Dashboard', params: {}, meta: { titleKey: 'nav.dashboard' } }
    }
  }
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ siteName: 'QxAPI', cachedPublicSettings: null })
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: false }) }))
vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] })
}))

describe('runtime-only i18n with application locales', () => {
  let runtime: typeof import('../index')

  beforeEach(async () => {
    vi.resetModules()
    localStorage.setItem('sub2api_locale', 'en')
    document.documentElement.removeAttribute('lang')
    document.title = ''
    runtime = await import('../index')
  })

  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.removeItem('sub2api_locale')
    document.documentElement.removeAttribute('lang')
    document.title = ''
  })

  it('lazily loads the saved locale and renders subscription copy', async () => {
    expect(runtime.i18n.global.availableLocales).toEqual([])

    await runtime.initI18n()

    expect(runtime.i18n.global.availableLocales).toEqual(['en'])
    expect(runtime.getLocale()).toBe('en')
    expect(document.documentElement.lang).toBe('en')
    expect(runtime.i18n.global.t('payment.subscribeNow')).toBe('Subscribe Now')
    expect(runtime.i18n.global.t('payment.perMonth')).toBe('month')
  })

  it('switches both languages with interpolated Qx copy, storage and page titles', async () => {
    await runtime.initI18n()
    await runtime.setLocale('zh')

    expect(runtime.getLocale()).toBe('zh')
    expect(runtime.i18n.global.availableLocales).toEqual(['en', 'zh'])
    expect(localStorage.getItem('sub2api_locale')).toBe('zh')
    expect(document.documentElement.lang).toBe('zh')
    expect(document.title).toBe('仪表盘 - QxAPI')
    expect(runtime.i18n.global.t('payment.subscribeNow')).toBe('立即开通')
    expect(runtime.i18n.global.t('qxTheme.hero.description', { tools: 'Tool A' }))
      .toBe('注册后创建一个 API 密钥，在 Tool A 等工具里填入接口地址，就能调用模型。按实际用量从余额扣费。')

    await runtime.setLocale('en')

    expect(runtime.getLocale()).toBe('en')
    expect(localStorage.getItem('sub2api_locale')).toBe('en')
    expect(document.documentElement.lang).toBe('en')
    expect(document.title).toBe('Dashboard - QxAPI')
    expect(runtime.i18n.global.t('qxTheme.hero.description', { tools: 'Tool B' }))
      .toBe('Create an API key after signing up, then enter the base URL in Tool B or similar tools to call the models. Usage is deducted from your balance.')
  })

  it('compiles and interpolates messages without dynamic code generation', async () => {
    await runtime.initI18n()
    await runtime.loadLocaleMessages('zh')
    const blockDynamicCode = () => { throw new Error('Dynamic code generation is disabled') }
    const functionSpy = vi.spyOn(globalThis, 'Function').mockImplementation(blockDynamicCode)
    const evalSpy = vi.spyOn(globalThis, 'eval').mockImplementation(blockDynamicCode)

    expect(runtime.i18n.global.t('dashboard.platformCount', { count: 3 })).toBe('3 platforms')
    runtime.i18n.global.locale.value = 'zh'
    expect(runtime.i18n.global.t('dashboard.platformCount', { count: 3 })).toBe('3 个平台')
    expect(functionSpy).not.toHaveBeenCalled()
    expect(evalSpy).not.toHaveBeenCalled()
  })

  it('ignores unsupported locale selections without changing the active language', async () => {
    await runtime.initI18n()
    await runtime.setLocale('fr')

    expect(runtime.getLocale()).toBe('en')
    expect(runtime.i18n.global.availableLocales).toEqual(['en'])
    expect(localStorage.getItem('sub2api_locale')).toBe('en')
    expect(document.documentElement.lang).toBe('en')
  })
})
