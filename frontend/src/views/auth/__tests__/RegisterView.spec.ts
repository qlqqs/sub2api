import { reactive } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import RegisterView from '@/views/auth/RegisterView.vue'
import { clearAffiliateReferralCode } from '@/utils/oauthAffiliate'

const {
  getPublicSettingsMock,
  getAffiliateSignupBonusMock,
  registerMock,
  showErrorMock,
  pushMock,
  verifyActionMock,
  appStoreMock
} = vi.hoisted(() => ({
  getPublicSettingsMock: vi.fn(),
  getAffiliateSignupBonusMock: vi.fn(),
  registerMock: vi.fn(),
  showErrorMock: vi.fn(),
  pushMock: vi.fn(),
  verifyActionMock: vi.fn(),
  appStoreMock: {
    cachedPublicSettings: null as { promo_code_enabled?: boolean } | null,
    showError: (...args: unknown[]) => showErrorMock(...args),
    showSuccess: vi.fn(),
    showWarning: vi.fn()
  }
}))

const publicSettings = {
  registration_enabled: true,
  email_verify_enabled: false,
  promo_code_enabled: false,
  invitation_code_enabled: false,
  affiliate_enabled: true,
  turnstile_enabled: true,
  turnstile_site_key: 'site-key',
  site_name: 'Sub2API',
  registration_email_suffix_whitelist: [],
  linuxdo_oauth_enabled: false,
  wechat_oauth_enabled: false,
  oidc_oauth_enabled: false,
  github_oauth_enabled: false,
  google_oauth_enabled: false
}

const routeMock = reactive({ query: {} as Record<string, unknown> })

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
  useRoute: () => routeMock
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key
    }
  }),
  useI18n: () => ({
    t: (key: string) =>
      key === 'auth.emailDomainRegistrationLimit'
        ? '该邮箱域名无法注册新账户。请使用主流邮箱注册；如需使用企业邮箱，请联系客服添加域名白名单。'
        : key,
    locale: { value: 'en' }
  })
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({ register: (...args: unknown[]) => registerMock(...args) }),
  useAppStore: () => appStoreMock
}))

vi.mock('@/api/auth', async () => {
  const actual = await vi.importActual<typeof import('@/api/auth')>('@/api/auth')
  return {
    ...actual,
    getPublicSettings: (...args: unknown[]) => getPublicSettingsMock(...args),
    getAffiliateSignupBonus: (...args: unknown[]) => getAffiliateSignupBonusMock(...args)
  }
})

function mountRegister() {
  return mount(RegisterView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        Icon: true,
        TurnstileWidget: {
          template: '<div data-testid="turnstile-widget" />',
          methods: { verifyAction: verifyActionMock, reset: vi.fn() }
        },
        LoginAgreementPrompt: true,
        EmailOAuthButtons: true,
        LinuxDoOAuthSection: true,
        WechatOAuthSection: true,
        OidcOAuthSection: true,
        RouterLink: true,
        transition: false
      }
    }
  })
}

enableAutoUnmount(afterEach)

describe('RegisterView', () => {
  beforeEach(() => {
    getPublicSettingsMock.mockReset()
    getAffiliateSignupBonusMock.mockReset().mockResolvedValue(0)
    registerMock.mockReset()
    showErrorMock.mockReset()
    pushMock.mockReset()
    verifyActionMock.mockReset()
    appStoreMock.cachedPublicSettings = null
    routeMock.query = {}
    clearAffiliateReferralCode()
    sessionStorage.removeItem('register_data')
    verifyActionMock.mockResolvedValue({ token: 'ticket', randstr: 'randstr' })
    getPublicSettingsMock.mockResolvedValue(publicSettings)
    registerMock.mockResolvedValue({})
  })

  it('does not flash the promo-code field before disabled settings finish loading', async () => {
    let resolveSettings!: (settings: typeof publicSettings) => void
    getPublicSettingsMock.mockReturnValueOnce(
      new Promise<typeof publicSettings>((resolve) => {
        resolveSettings = resolve
      })
    )

    const wrapper = mountRegister()

    expect(wrapper.find('#promo_code').exists()).toBe(false)

    resolveSettings(publicSettings)
    await flushPromises()

    expect(wrapper.find('#promo_code').exists()).toBe(false)
  })

  it('uses injected public settings to show an enabled promo-code field on first render', () => {
    appStoreMock.cachedPublicSettings = { promo_code_enabled: true }
    getPublicSettingsMock.mockReturnValueOnce(new Promise(() => {}))

    const wrapper = mountRegister()

    expect(wrapper.find('#promo_code').exists()).toBe(true)
  })

  it.each([
    ['', 'auth.confirmPasswordRequired'],
    ['different-password', 'auth.passwordsDoNotMatch']
  ])('blocks invalid confirmation %j before captcha and allows correction', async (confirmation, error) => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      turnstile_enabled: false,
      tencent_captcha_enabled: true,
      tencent_captcha_app_id: 'app-id'
    })
    const wrapper = mountRegister()
    await flushPromises()
    await wrapper.get('#email').setValue('user@example.com')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue(confirmation)
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(showErrorMock).toHaveBeenCalledWith(error)
    expect(wrapper.get('#confirmPassword').classes()).toContain('input-error')
    expect(registerMock).not.toHaveBeenCalled()
    expect(verifyActionMock).not.toHaveBeenCalled()
    expect(pushMock).not.toHaveBeenCalled()
    expect(sessionStorage.getItem('register_data')).toBeNull()

    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.get('#confirmPassword').classes()).not.toContain('input-error')
    expect(verifyActionMock).toHaveBeenCalledOnce()
    expect(registerMock).toHaveBeenCalledWith({
      email: 'user@example.com',
      password: 'secret-123',
      turnstile_token: undefined,
      tencent_captcha_ticket: 'ticket',
      tencent_captcha_randstr: 'randstr',
      promo_code: undefined,
      invitation_code: undefined
    })
    expect(pushMock).toHaveBeenCalledWith('/dashboard')
  })

  it('requires matching confirmation before storing only the registration fields (with the trimmed username) for email verification', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      turnstile_enabled: false,
      email_verify_enabled: true
    })
    const wrapper = mountRegister()
    await flushPromises()
    await wrapper.get('#email').setValue('user@example.com')
    await wrapper.get('#username').setValue('  小明 ')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('different-password')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(sessionStorage.getItem('register_data')).toBeNull()
    expect(pushMock).not.toHaveBeenCalled()

    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(JSON.parse(sessionStorage.getItem('register_data')!)).toEqual({
      email: 'user@example.com',
      password: 'secret-123',
      username: '小明'
    })
    expect(pushMock).toHaveBeenCalledWith('/email-verify')
    expect(registerMock).not.toHaveBeenCalled()
  })

  it('keeps the optional affiliate invitation field before Turnstile', async () => {
    const wrapper = mountRegister()
    await flushPromises()

    const invitationField = wrapper.get('[data-testid="affiliate-invitation-field"]')
    const turnstile = wrapper.get('[data-testid="registration-turnstile"]')

    expect(invitationField.get('input').attributes('id')).toBe('affiliate_code')
    expect(invitationField.text()).toContain('common.optional')
    expect(invitationField.get('[data-testid="affiliate-invitation-empty-hint"]').text()).toBe('auth.invitationCodeEmptyHint')
    expect(
      invitationField.element.compareDocumentPosition(turnstile.element) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  it('describes the invitee first-recharge bonus when opened from an invite link', async () => {
    routeMock.query = { aff: 'INVITE1' }
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      affiliate_invitee_bonus_mode: 'fixed',
      affiliate_invitee_bonus_value: 2,
      affiliate_invitee_bonus_min_recharge: 10,
      affiliate_invitee_bonus_max: 0
    })

    const wrapper = mountRegister()
    await flushPromises()

    expect(wrapper.get('[data-testid="register-invitee-bonus-hint"]').text()).toBe('auth.inviteeBonusHintWithMin')
  })

  it('hides the invitee bonus hint without an invite link or when the bonus is off', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      affiliate_invitee_bonus_mode: 'percent',
      affiliate_invitee_bonus_value: 10
    })
    const withoutLink = mountRegister()
    await flushPromises()
    expect(withoutLink.find('[data-testid="register-invitee-bonus-hint"]').exists()).toBe(false)

    routeMock.query = { aff: 'INVITE1' }
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      affiliate_invitee_bonus_mode: 'off'
    })
    const bonusOff = mountRegister()
    await flushPromises()
    expect(bonusOff.find('[data-testid="register-invitee-bonus-hint"]').exists()).toBe(false)
  })

  it('注册赠送与首充奖励提示可以同时出现', async () => {
    getAffiliateSignupBonusMock.mockResolvedValue(5)
    routeMock.query = { aff: 'INVITE1' }
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      affiliate_invitee_bonus_mode: 'fixed',
      affiliate_invitee_bonus_value: 2,
      affiliate_invitee_signup_bonus: 5,
    })
    const wrapper = mountRegister()
    await flushPromises()
    expect(wrapper.find('[data-testid="register-invitee-bonus-hint"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="register-invitee-signup-bonus-hint"]').exists()).toBe(true)
  })

  it.each([
    { query: { aff: 'INVITE1' }, enabled: true, amount: 5, visible: true },
    { query: { aff_code: 'INVITE1' }, enabled: true, amount: 5, visible: true },
    { query: {}, enabled: true, amount: 5, visible: false },
    { query: { aff: 'INVITE1' }, enabled: false, amount: 5, visible: false },
    { query: { aff: 'INVITE1' }, enabled: true, amount: 0, visible: false },
    { query: { aff: 'INVITE1' }, enabled: true, amount: -5, visible: false },
    { query: { aff: 'INVITE1' }, enabled: true, amount: undefined, visible: false },
  ])('注册赠送提示独立于首充开关：$query / $enabled / $amount', async ({ query, enabled, amount, visible }) => {
    routeMock.query = query
    getAffiliateSignupBonusMock.mockResolvedValue(amount)
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      affiliate_enabled: enabled,
      affiliate_invitee_bonus_mode: 'off',
      affiliate_invitee_signup_bonus: amount,
    })
    const wrapper = mountRegister()
    await flushPromises()
    expect(wrapper.find('[data-testid="register-invitee-signup-bonus-hint"]').exists()).toBe(visible)
    expect(wrapper.find('[data-testid="register-invitee-bonus-hint"]').exists()).toBe(false)
  })

  it('uses the invitation preview instead of the global gift amount', async () => {
    routeMock.query = { aff: 'INVITE1' }
    getPublicSettingsMock.mockResolvedValue({ ...publicSettings, affiliate_invitee_signup_bonus: 10 })
    getAffiliateSignupBonusMock.mockResolvedValue(0)
    const wrapper = mountRegister()
    await flushPromises()
    expect(getAffiliateSignupBonusMock).toHaveBeenCalledWith('INVITE1', expect.any(AbortSignal))
    expect(wrapper.find('[data-testid="register-invitee-signup-bonus-hint"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('clears old amounts and ignores late preview responses after the link changes', async () => {
    routeMock.query = { aff: 'FIRST1' }
    let resolveFirst!: (amount: number) => void
    getAffiliateSignupBonusMock.mockReturnValueOnce(new Promise<number>(resolve => { resolveFirst = resolve }))
    getAffiliateSignupBonusMock.mockResolvedValueOnce(0)
    const wrapper = mountRegister()
    await flushPromises()
    routeMock.query = { aff: 'SECOND2' }
    await flushPromises()
    resolveFirst(5)
    await flushPromises()
    expect(wrapper.find('[data-testid="register-invitee-signup-bonus-hint"]').exists()).toBe(false)
    expect(getAffiliateSignupBonusMock.mock.calls[0][1].aborted).toBe(true)
    wrapper.unmount()
  })

  it('clears and refreshes the gift when the referral input is edited', async () => {
    routeMock.query = { aff: 'AGENT1' }
    getAffiliateSignupBonusMock.mockResolvedValueOnce(5).mockResolvedValueOnce(0)
    const wrapper = mountRegister()
    await flushPromises()
    expect(wrapper.find('[data-testid="register-invitee-signup-bonus-hint"]').exists()).toBe(true)
    await wrapper.get('#affiliate_code').setValue('USER22')
    expect(wrapper.find('[data-testid="register-invitee-signup-bonus-hint"]').exists()).toBe(false)
    await vi.waitFor(() => expect(getAffiliateSignupBonusMock).toHaveBeenCalledWith('USER22', expect.any(AbortSignal)))
    await flushPromises()
    expect(wrapper.find('[data-testid="register-invitee-signup-bonus-hint"]').exists()).toBe(false)
    // Signup already falls back to the remembered link when the input is empty.
    getAffiliateSignupBonusMock.mockResolvedValueOnce(5)
    await wrapper.get('#affiliate_code').setValue('')
    await flushPromises()
    expect(getAffiliateSignupBonusMock).toHaveBeenLastCalledWith('AGENT1', expect.any(AbortSignal))
    expect(wrapper.find('[data-testid="register-invitee-signup-bonus-hint"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('hides the hint on preview failure and cancels pending requests on unmount', async () => {
    routeMock.query = { aff: 'INVITE1' }
    getAffiliateSignupBonusMock.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountRegister()
    await flushPromises()
    expect(wrapper.find('[data-testid="register-invitee-signup-bonus-hint"]').exists()).toBe(false)
    expect(showErrorMock).not.toHaveBeenCalled()
    getAffiliateSignupBonusMock.mockReturnValueOnce(new Promise(() => {}))
    routeMock.query = { aff: 'INVITE2' }
    await flushPromises()
    const signal = getAffiliateSignupBonusMock.mock.calls[1][1] as AbortSignal
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
  })

  it('uses the mandatory invitation field without duplicating the affiliate field', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      invitation_code_enabled: true
    })

    const wrapper = mountRegister()
    await flushPromises()

    expect(wrapper.find('[data-testid="affiliate-invitation-field"]').exists()).toBe(false)
    expect(wrapper.get('#invitation_code').exists()).toBe(true)
  })

  it('submits a non-whitelist email domain so the backend can enforce its registration quota', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      turnstile_enabled: false,
      registration_email_suffix_whitelist: ['allowed.com'],
      registration_email_domain_quota_enabled: true
    })

    const wrapper = mountRegister()
    await flushPromises()
    await wrapper.get('#email').setValue('first@custom.example')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(registerMock).toHaveBeenCalledWith(
      expect.objectContaining({ email: 'first@custom.example' })
    )
    expect(showErrorMock).not.toHaveBeenCalled()
  })

  it('shows the localized registration domain quota message returned by the backend', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      turnstile_enabled: false,
      registration_email_suffix_whitelist: ['allowed.com'],
      registration_email_domain_quota_enabled: true
    })
    registerMock.mockRejectedValueOnce({
      reason: 'EMAIL_DOMAIN_REGISTRATION_LIMIT',
      message: 'raw backend message'
    })

    const wrapper = mountRegister()
    await flushPromises()
    await wrapper.get('#email').setValue('second@custom.example')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(showErrorMock).toHaveBeenCalledWith(
      '该邮箱域名无法注册新账户。请使用主流邮箱注册；如需使用企业邮箱，请联系客服添加域名白名单。'
    )
  })

  // 域名限量注册开关默认关闭：恢复 PR5423 之前的客户端白名单预检，非白名单域名不发起注册请求。
  it('rejects a non-whitelist email domain locally when the domain quota switch is disabled', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      turnstile_enabled: false,
      registration_email_suffix_whitelist: ['allowed.com']
    })

    const wrapper = mountRegister()
    await flushPromises()
    await wrapper.get('#email').setValue('first@custom.example')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(registerMock).not.toHaveBeenCalled()
    // 校验失败通过 validationToastMessage watcher 弹 toast
    expect(showErrorMock).toHaveBeenCalledWith('auth.emailSuffixNotAllowedWithAllowed')
    expect(wrapper.get('#email').classes()).toContain('input-error')
  })

  it('still submits whitelisted email domains when the domain quota switch is disabled', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      turnstile_enabled: false,
      registration_email_suffix_whitelist: ['allowed.com']
    })

    const wrapper = mountRegister()
    await flushPromises()
    await wrapper.get('#email').setValue('user@allowed.com')
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(registerMock).toHaveBeenCalledWith(
      expect.objectContaining({ email: 'user@allowed.com' })
    )
    expect(showErrorMock).not.toHaveBeenCalled()
  })
})
