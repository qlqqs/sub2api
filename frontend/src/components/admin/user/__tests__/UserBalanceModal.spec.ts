import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import UserBalanceModal from '../UserBalanceModal.vue'
import type { AdminUser } from '@/types'
const mocks = vi.hoisted(() => ({ updateBalance: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => vi.restoreAllMocks())
describe('balance operation error feedback', () => {
  it.each([
    [{ message: 'User balance changed; retry' }, 'User balance changed; retry'],
    [{ response: { data: { detail: 'Legacy detail' } } }, 'Legacy detail'],
    [{}, 'common.error'],
  ])('shows the available API error and keeps the dialog open', async (error, message) => {
    mocks.updateBalance.mockRejectedValueOnce(error)
    const w = mount(UserBalanceModal, { props: { show: true, operation: 'add', user: { id: 1, email: 'a@example.com', balance: 5 } as AdminUser },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } } })
    await w.get('input[value="gift"]').setValue()
    await w.get('input[type="number"]').setValue('1')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.showError).toHaveBeenCalledWith(message)
    expect(w.emitted('success')).toBeUndefined(); expect(w.emitted('close')).toBeUndefined()
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })
})

// CUSTOM(fork): Mandatory classification, exact payload and state reset.
describe('offline receipt classification', () => {
  function mountModal(operation: 'add' | 'subtract' = 'add') {
    return mount(UserBalanceModal, { props: { show: true, operation, user: { id: 1, email: 'a@example.com', balance: 5 } as AdminUser },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } } })
  }
  it('has no default kind, prevents unclassified submission and resets on reopen', async () => {
    const w = mountModal()
    await w.get('#balance-amount').setValue(10)
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await w.get('form').trigger('submit')
    expect(mocks.updateBalance).not.toHaveBeenCalled()
    await w.get('input[value="gift"]').setValue()
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    await w.setProps({ show: false }); await w.setProps({ show: true })
    expect((w.get('input[value="gift"]').element as HTMLInputElement).checked).toBe(false)
  })
  it('requires cash and channel, sends an offline receipt and rejects fractional cents', async () => {
    const w = mountModal()
    await w.get('input[value="offline"]').setValue()
    await w.get('#balance-amount').setValue(110)
    await w.get('#balance-paid').setValue(100)
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await w.get('#balance-channel').setValue('bank')
    await w.get('#balance-trade').setValue(' bank-001 ')
    await w.get('#balance-amount').setValue(110.001)
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await w.get('#balance-amount').setValue(110)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.updateBalance).toHaveBeenCalledWith(1, 110, 'add', '', { kind: 'offline', paid_amount: 100, offline_channel: 'bank', trade_no: 'bank-001' })
    expect(w.emitted('success')).toHaveLength(1)
  })
  it('gift explicitly sends gift without stale offline metadata', async () => {
    const w = mountModal()
    await w.get('input[value="offline"]').setValue()
    await w.get('#balance-paid').setValue(100)
    await w.get('input[value="gift"]').setValue()
    await w.get('#balance-amount').setValue(5)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.updateBalance).toHaveBeenCalledWith(1, 5, 'add', '', { kind: 'gift' })
  })
  it('subtract does not require a kind and keeps fractional balance support', async () => {
    const w = mountModal('subtract')
    expect(w.find('input[type="radio"]').exists()).toBe(false)
    await w.get('#balance-amount').setValue(0.123456)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.updateBalance).toHaveBeenCalledWith(1, 0.123456, 'subtract', '', { kind: 'gift' })
  })
})
