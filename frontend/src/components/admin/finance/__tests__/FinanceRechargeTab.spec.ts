import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import FinanceRechargeTab from '../FinanceRechargeTab.vue'
import { normalizeRecharge } from '@/api/admin/finance'

const { getRecharge } = vi.hoisted(() => ({ getRecharge: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { finance: { getRecharge } } }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))
enableAutoUnmount(afterEach)
beforeEach(() => vi.clearAllMocks())

describe('财务充值与赠送分类', () => {
  it.each([undefined, '5.12345678'])('注册赠送独立于首充显示，兼容旧接口：%s', async invitee_signup_bonus => {
    getRecharge.mockResolvedValue(normalizeRecharge({ totals: { gift: { invitee_bonus: '2.00', invitee_signup_bonus } } }))
    const wrapper = mount(FinanceRechargeTab, {
      props: { start: '2026-10-01', end: '2026-10-27', refreshToken: 0 },
      global: { stubs: { RouterLink: true, FinanceMoney: { props: ['value'], template: '<span>{{ value }}</span>' } } }
    })
    await flushPromises()
    const gifts = wrapper.findAll('dl > div')
    const signup = gifts.find(row => row.find('dt').text() === 'admin.finance.recharge.gift.invitee_signup_bonus')
    const firstRecharge = gifts.find(row => row.find('dt').text() === 'admin.finance.recharge.gift.invitee_bonus')
    expect(signup?.get('dd').text()).toBe(invitee_signup_bonus ?? '0')
    expect(firstRecharge?.get('dd').text()).toBe('2.00')
  })
})
