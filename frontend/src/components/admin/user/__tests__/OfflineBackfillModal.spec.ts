// CUSTOM(fork): Backfill has its own endpoint and does not call balance mutation.
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import OfflineBackfillModal from '../OfflineBackfillModal.vue'
import type { BalanceHistoryItem } from '@/api/admin'
const mocks = vi.hoisted(() => ({ backfillOfflineRecharge: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (k: string) => k }) }))
enableAutoUnmount(afterEach)
beforeEach(() => vi.clearAllMocks())
describe('backfill receipt', () => {
  function render() { return mount(OfflineBackfillModal, { props: { userId: 1, item: { id: 2, value: 110 } as BalanceHistoryItem }, global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } } }) }
  it('requires receipt details, warns about no balance/reward changes, and reloads after success', async () => {
    const w = render()
    expect(w.text()).toContain('admin.users.offlineRecharge.backfillHint')
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await w.get('#backfill-paid').setValue(100)
    await w.get('#backfill-channel').setValue('alipay')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.backfillOfflineRecharge).toHaveBeenCalledWith(1, 2, { paid_amount: 100, offline_channel: 'alipay', trade_no: '', notes: '' })
    expect(w.emitted('success')).toHaveLength(1)
  })
  it('keeps the dialog open after a duplicate receipt error', async () => {
    mocks.backfillOfflineRecharge.mockRejectedValueOnce({ reason: 'OFFLINE_ALREADY_LINKED', message: 'Already linked' })
    const w = render()
    await w.get('#backfill-paid').setValue(100)
    await w.get('#backfill-channel').setValue('other')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.showError).toHaveBeenCalled()
    expect(w.emitted('success')).toBeUndefined()
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })
})
