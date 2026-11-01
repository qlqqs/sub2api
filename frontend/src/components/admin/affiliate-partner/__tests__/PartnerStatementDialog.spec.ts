import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PartnerStatementDialog from '../PartnerStatementDialog.vue'
import {
  normalizePartnerMonthTotals,
  normalizePartnerStatementDetail,
  type PartnerMonthStatus
} from '@/api/affiliatePartner'

const api = vi.hoisted(() => ({
  getStatement: vi.fn(),
  confirmStatement: vi.fn(),
  payStatement: vi.fn(),
  createAdjustment: vi.fn(),
  deleteAdjustment: vi.fn(),
  exportStatement: vi.fn()
}))
const toast = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin/affiliatePartners', () => ({ default: api, affiliatePartnersAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const stubs = {
  BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
  ConfirmDialog: {
    props: ['show', 'message'],
    emits: ['confirm', 'cancel'],
    template: '<div v-if="show" data-test="confirm"><button type="button" data-test="confirm-ok" @click="$emit(\'confirm\')">ok</button></div>'
  },
  Icon: true,
  RouterLink: { template: '<a><slot /></a>' }
}

function statements(...items: Array<[string, PartnerMonthStatus]>) {
  return items.map(([month, status]) => normalizePartnerMonthTotals({ month, status }))
}

function detail(month: string, status: PartnerMonthStatus, adjustments: unknown[] = []) {
  return normalizePartnerStatementDetail({ totals: { month, status, share_rate: '0.4000' }, adjustments })
}

async function mountDialog(month: string, list: ReturnType<typeof statements>) {
  const status = list.find((item) => item.month === month)!.status
  api.getStatement.mockResolvedValue(detail(month, status, [{ id: 3, amount: '-5.00', note: '补差', created_at: '' }]))
  const wrapper = mount(PartnerStatementDialog, {
    props: { show: true, partnerId: 75, month, statements: list },
    global: { stubs }
  })
  await flushPromises()
  return wrapper
}

const LIST = statements(['2026-12', 'current'], ['2026-11', 'draft'], ['2026-10', 'draft'], ['2026-09', 'confirmed'], ['2026-08', 'paid'])

describe('PartnerStatementDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('confirms the earliest draft only after the second confirmation', async () => {
    api.confirmStatement.mockResolvedValue(detail('2026-10', 'confirmed'))
    const wrapper = await mountDialog('2026-10', LIST)
    expect(api.getStatement).toHaveBeenCalledWith(75, '2026-10', expect.anything())

    const confirm = wrapper.get('[data-test="partner-statement-confirm"]')
    expect(confirm.attributes('disabled')).toBeUndefined()
    expect(wrapper.find('[data-test="partner-statement-pay"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="partner-adjustment-form"]').exists()).toBe(true)

    await confirm.trigger('click')
    expect(api.confirmStatement).not.toHaveBeenCalled()
    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(api.confirmStatement).toHaveBeenCalledWith(75, '2026-10')
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('blocks a later draft and the current month from confirming but allows adjustments', async () => {
    const later = await mountDialog('2026-11', LIST)
    expect(later.get('[data-test="partner-statement-confirm"]').attributes('disabled')).toBeDefined()
    expect(later.get('[data-test="partner-confirm-hint"]').text()).toBe('admin.partners.statement.previousHint')
    expect(later.findAll('[data-test="partner-adjustment-delete"]')).toHaveLength(1)

    const current = await mountDialog('2026-12', LIST)
    expect(current.get('[data-test="partner-statement-confirm"]').attributes('disabled')).toBeDefined()
    expect(current.get('[data-test="partner-confirm-hint"]').text()).toBe('admin.partners.statement.currentMonthHint')
    expect(current.find('[data-test="partner-adjustment-form"]').exists()).toBe(true)
  })

  it('marks a confirmed month as paid with a required note; confirmed months take no adjustments', async () => {
    api.payStatement.mockResolvedValue(detail('2026-09', 'paid'))
    const wrapper = await mountDialog('2026-09', LIST)
    expect(wrapper.find('[data-test="partner-statement-confirm"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="partner-adjustment-form"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="partner-adjustment-delete"]').exists()).toBe(false)

    await wrapper.get('[data-test="partner-statement-pay"]').trigger('click')
    expect(wrapper.get('[data-test="partner-pay-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="partner-pay-date"]').setValue('2026-10-03')
    await wrapper.get('[data-test="partner-pay-note"]').setValue('招行转账 尾号1234')
    await wrapper.get('[data-test="partner-pay-form"]').trigger('submit')
    await flushPromises()
    expect(api.payStatement).toHaveBeenCalledWith(75, '2026-09', { paid_on: '2026-10-03', note: '招行转账 尾号1234' })
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('shows no actions on a paid month', async () => {
    const wrapper = await mountDialog('2026-08', LIST)
    expect(wrapper.find('[data-test="partner-statement-confirm"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="partner-statement-pay"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="partner-statement-export"]').exists()).toBe(true)
  })

  it('adds and deletes adjustments on a draft and toasts backend errors', async () => {
    api.createAdjustment.mockResolvedValue({ id: 4 })
    api.deleteAdjustment.mockRejectedValue({ status: 409, reason: 'PARTNER_MONTH_CONFIRMED', message: '该月已确认' })
    const wrapper = await mountDialog('2026-10', LIST)

    const add = () => wrapper.get('[data-test="partner-adjustment-add"]')
    await wrapper.get('[data-test="partner-adjustment-amount"]').setValue('1.234')
    await wrapper.get('[data-test="partner-adjustment-note"]').setValue('补差')
    expect(add().attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="partner-adjustment-amount"]').setValue('-5')
    expect(add().attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-test="partner-adjustment-form"]').trigger('submit')
    await flushPromises()
    expect(api.createAdjustment).toHaveBeenCalledWith(75, { month: '2026-10', amount: '-5', note: '补差' })
    expect(api.getStatement).toHaveBeenCalledTimes(2)

    await wrapper.get('[data-test="partner-adjustment-delete"]').trigger('click')
    await flushPromises()
    expect(api.deleteAdjustment).toHaveBeenCalledWith(75, 3)
    expect(toast.showError).toHaveBeenCalledWith('该月已确认')
  })
})
