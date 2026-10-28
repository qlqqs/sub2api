import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { AdminUser } from '@/types'
import UserBalanceHistoryModal from '../UserBalanceHistoryModal.vue'

const mocks = vi.hoisted(() => ({ getUserBalanceHistory: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: () => 'date' }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: { amount: string }) => params ? `${key} ${params.amount}` : key }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => vi.restoreAllMocks())
function result(id: number) {
  return { items: [{ id, type: 'admin_balance', value: id, notes: `History ${id}` }], total: 1, total_recharged: id, total_gifted: 25 }
}
function deferred() {
  let resolve!: (value: ReturnType<typeof result>) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<ReturnType<typeof result>>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
async function openDialog() {
  const wrapper = mount(UserBalanceHistoryModal, {
    props: { show: false, user: { id: 1, email: 'one@example.com', balance: 1 } as AdminUser },
    global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, Icon: true, Select: true } }
  })
  await wrapper.setProps({ show: true })
  return wrapper
}

describe('UserBalanceHistoryModal request ordering', () => {
  it('keeps the new user history when an old response finishes later', async () => {
    const old = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(old.promise).mockResolvedValueOnce(result(20))
    const wrapper = await openDialog()
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, user: { id: 2, email: 'two@example.com', balance: 2 } as AdminUser })
    await flushPromises()
    old.resolve(result(10))
    await flushPromises()
    expect(wrapper.text()).toContain('History 20')
    expect(wrapper.text()).not.toContain('History 10')
    expect(wrapper.text()).toContain('¥20.00')
  })

  it('does not end the current filter loading state when an old request finishes', async () => {
    const old = deferred()
    const current = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const wrapper = await openDialog()
    wrapper.findComponent({ name: 'Select' }).vm.$emit('change', 'balance')
    await flushPromises()
    old.resolve(result(10))
    await flushPromises()
    expect(wrapper.find('svg.animate-spin').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('History 10')
    current.resolve(result(20))
    await flushPromises()
    expect(wrapper.find('svg.animate-spin').exists()).toBe(false)
    expect(wrapper.text()).toContain('History 20')
  })

  it.each(['close', 'unmount'])('ignores failures after %s', async (action) => {
    const pending = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(pending.promise)
    const wrapper = await openDialog()
    if (action === 'close') await wrapper.setProps({ show: false })
    else wrapper.unmount()
    pending.reject(new Error('Stale error'))
    await flushPromises()
    expect(console.error).not.toHaveBeenCalled()
  })

  it('still reports current failures and ends loading', async () => {
    const error = new Error('Current error')
    mocks.getUserBalanceHistory.mockRejectedValueOnce(error)
    const wrapper = await openDialog()
    await flushPromises()
    expect(console.error).toHaveBeenCalledWith('Failed to load balance history:', error)
    expect(wrapper.find('svg.animate-spin').exists()).toBe(false)
  })
})

// CUSTOM(fork): Only positive unlinked adjustments offer receipt backfill.
describe('offline history metadata', () => {
  it('renders receipt labels and allows only eligible historical gifts', async () => {
    mocks.getUserBalanceHistory.mockResolvedValueOnce({ items: [
      { id: 1, type: 'admin_balance', value: 110, used_at: '2026-10-27', offline_order: { id: 9, out_trade_no: 'sub2_offline', paid_amount: 100 } },
      { id: 2, type: 'admin_balance', value: 10, used_at: '2026-10-27' },
      { id: 3, type: 'admin_balance', value: -10, used_at: '2026-10-27' },
      { id: 4, type: 'admin_concurrency', value: 10, used_at: '2026-10-27' },
      { id: 5, type: 'admin_balance', value: 10, used_at: '2026-10-27', has_payment_order: true },
    ], total: 5 })
    const w = await openDialog(); await flushPromises()
    expect(w.text()).toContain('sub2_offline')
    expect(w.text()).toContain('¥100.00')
    const buttons = w.findAll('button').filter(b => b.text() === 'admin.users.offlineRecharge.backfill')
    expect(buttons).toHaveLength(1)
    await w.setProps({ hideActions: true })
    expect(w.findAll('button').filter(b => b.text() === 'admin.users.offlineRecharge.backfill')).toHaveLength(0)
  })
})

// CUSTOM(fork): Display API totals, never sum the current filtered page.
describe('balance history cash and gifts', () => {
  it('注册赠送按余额展示金额和标签，并支持独立筛选', async () => {
    mocks.getUserBalanceHistory.mockResolvedValue({ items: [{ id: 1, code: 'SIGNUP-BONUS', type: 'invitee_signup_bonus', value: 5 }], total: 1, total_gifted: 5, total_recharged: 0 })
    const w = await openDialog()
    await flushPromises()
    const row = w.get('[data-testid="balance-history-item-1"]')
    expect(row.text()).toContain('redeem.balanceAddedInviteeSignupBonus')
    expect(row.text()).toContain('+¥5.00')
    expect(row.find('p.text-emerald-600').exists()).toBe(true)
    const filter = w.findComponent({ name: 'Select' })
    expect(filter.props('options')).toContainEqual({ value: 'invitee_signup_bonus', label: 'admin.users.typeInviteeSignupBonus' })
    filter.vm.$emit('update:modelValue', 'invitee_signup_bonus')
    filter.vm.$emit('change', 'invitee_signup_bonus')
    await flushPromises()
    expect(mocks.getUserBalanceHistory).toHaveBeenLastCalledWith(1, 1, 15, 'invitee_signup_bonus')
    expect(w.get('[data-testid="balance-history-totals"]').text()).toContain('admin.users.totalGifted ¥5.00')
  })

  it('preserves both global totals across filtering and pagination', async () => {
    mocks.getUserBalanceHistory.mockResolvedValue({ ...result(999), total: 31, total_recharged: 115.5, total_gifted: 30 })
    const w = await openDialog()
    await flushPromises()
    const totals = () => w.get('[data-testid="balance-history-totals"]').text()
    expect(totals()).toContain('admin.users.totalRecharged ¥115.50')
    expect(totals()).toContain('admin.users.totalGifted ¥30.00')
    expect(w.text()).toContain('admin.users.totalGiftedHint')
    expect(totals()).not.toContain('999')
    const filter = w.findComponent({ name: 'Select' })
    filter.vm.$emit('update:modelValue', 'subscription')
    filter.vm.$emit('change', 'subscription')
    await flushPromises()
    expect(mocks.getUserBalanceHistory).toHaveBeenLastCalledWith(1, 1, 15, 'subscription')
    await w.findAll('button').find(b => b.text() === 'pagination.next')!.trigger('click')
    await flushPromises()
    expect(mocks.getUserBalanceHistory).toHaveBeenLastCalledWith(1, 2, 15, 'subscription')
    expect(totals()).toContain('¥115.50')
    expect(totals()).toContain('¥30.00')
  })
})

// CUSTOM(fork): Only this admin modal reclassifies gifts and annotates completed refunds.
describe('administrator history titles and refunds', () => {
  it.each([
    { name: 'gift', type: 'admin_balance', value: 10, linked: false, offline: false, title: 'admin.users.offlineRecharge.gift', label: 'admin.users.offlineRecharge.giftLabel' },
    { name: 'offline', type: 'admin_balance', value: 110, linked: true, offline: true, title: 'admin.users.offlineRecharge.offline', label: 'admin.users.offlineRecharge.offline' },
    { name: 'deduction', type: 'admin_balance', value: -10, linked: false, offline: false, title: 'redeem.balanceDeductedAdmin', label: 'redeem.adminAdjustment' },
    { name: 'concurrency', type: 'admin_concurrency', value: 10, linked: false, offline: false, title: 'redeem.concurrencyAddedAdmin', label: 'redeem.adminAdjustment' },
    { name: 'other linked order', type: 'admin_balance', value: 10, linked: true, offline: false, title: 'redeem.balanceAddedAdmin', label: 'redeem.adminAdjustment' },
    { name: 'zero adjustment', type: 'admin_balance', value: 0, linked: false, offline: false, title: 'redeem.balanceAddedAdmin', label: 'redeem.adminAdjustment' },
    { name: 'redeem code', type: 'balance', value: 10, linked: false, offline: false, title: 'redeem.balanceAddedRedeem', label: 'ABCDEFGH...' },
  ])('renders $name without changing unrelated history', async ({ type, value, linked, offline, title, label }) => {
    mocks.getUserBalanceHistory.mockResolvedValueOnce({ items: [{
      id: 1, code: 'ABCDEFGHIJK', type, value, has_payment_order: linked,
      offline_order: offline ? { id: 9, out_trade_no: 'sub2_offline', paid_amount: 100, amount: 110, status: 'COMPLETED', refund_amount: 0 } : undefined,
    }], total: 1 })
    const w = await openDialog()
    await flushPromises()
    const row = w.get('[data-testid="balance-history-item-1"]')
    expect(row.get('p.text-sm.font-medium').text()).toBe(title)
    expect(row.get('div.text-right p.text-xs').text()).toBe(label)
    if (value < 0) expect(row.text()).toContain('-¥10.00')
  })

  it.each([
    { status: 'REFUNDED', refund: 110, key: 'refunded' },
    { status: 'PARTIALLY_REFUNDED', refund: 55.25, key: 'partiallyRefunded' },
    { status: 'COMPLETED', refund: 0, key: null },
    { status: 'REFUND_REQUESTED', refund: 110, key: null },
    { status: 'REFUNDING', refund: 110, key: null },
    { status: 'REFUND_PENDING', refund: 110, key: null },
    { status: 'REFUND_FAILED', refund: 110, key: null },
  ])('shows only settled refund badges for $status', async ({ status, refund, key }) => {
    mocks.getUserBalanceHistory.mockResolvedValueOnce({ items: [{
      id: 1, type: 'admin_balance', value: 110, has_payment_order: true,
      offline_order: { id: 9, out_trade_no: 'sub2_refund', paid_amount: 100, amount: 110, status, refund_amount: refund },
    }], total: 1 })
    const w = await openDialog()
    await flushPromises()
    const row = w.get('[data-testid="balance-history-item-1"]')
    expect(row.get('p.text-sm.font-medium').text()).toBe('admin.users.offlineRecharge.offline')
    const badge = row.find('span.bg-red-100')
    expect(badge.exists()).toBe(Boolean(key))
    if (key) expect(badge.text()).toBe(`admin.users.offlineRecharge.${key} ¥${refund.toFixed(2)}`)
  })
})
