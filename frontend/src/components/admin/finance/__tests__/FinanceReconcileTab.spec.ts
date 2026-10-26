import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import FinanceReconcileTab from '../FinanceReconcileTab.vue'
import { normalizeReconcileRow } from '@/api/admin/finance'

const api = vi.hoisted(() => ({ getReconcile: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { finance: api } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function mountTab() {
  return mount(FinanceReconcileTab, {
    props: { refreshToken: 0 },
    global: { stubs: { Pagination: true, SearchInput: true, Icon: true, LoadingSpinner: true } }
  })
}

describe('FinanceReconcileTab (CUSTOM)', () => {
  beforeEach(() => {
    api.getReconcile.mockReset().mockResolvedValue({
      items: [
        normalizeReconcileRow({ user_id: 1, user_email: 'ok@x', difference: '0', actual_balance: '10.95', expected_balance: '10.95' }),
        normalizeReconcileRow({ user_id: 2, user_email: 'bad@x', difference: '-0.50000000', actual_balance: '1', expected_balance: '1.5' }),
        normalizeReconcileRow({ user_id: 3, user_email: 'tiny@x', difference: '0.004' })
      ],
      total: 3,
      page: 1,
      page_size: 50,
      pages: 1
    })
  })

  it('marks only rows with |difference| >= 0.01 in red', async () => {
    const wrapper = mountTab()
    await flushPromises()

    const rows = wrapper.findAll('[data-testid="finance-reconcile-row"]')
    expect(rows).toHaveLength(3)
    expect(rows.map((r) => r.attributes('data-mismatch'))).toEqual([undefined, 'true', undefined])

    const flagged = rows[1].get('[data-testid="finance-reconcile-difference"]')
    expect(flagged.classes()).toContain('text-red-600')
    expect(flagged.text()).toContain('-0.50')
    expect(flagged.text()).toContain('admin.finance.reconcile.mismatch')
    expect(rows[0].find('[data-testid="finance-reconcile-difference"]').exists()).toBe(false)
    expect(rows[2].find('[data-testid="finance-reconcile-difference"]').exists()).toBe(false)
  })

  it('reloads from page 1 with only_mismatch when the toggle is switched on', async () => {
    const wrapper = mountTab()
    await flushPromises()
    await wrapper.get('[data-testid="finance-reconcile-only-mismatch"]').trigger('click')
    await flushPromises()
    expect(api.getReconcile).toHaveBeenLastCalledWith(
      { page: 1, page_size: 50, only_mismatch: true, search: '' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })
})
