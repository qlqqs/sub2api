import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import FinanceExpenseDialog from '../FinanceExpenseDialog.vue'
import type { FinanceExpense } from '@/api/admin/finance'

const api = vi.hoisted(() => ({ createExpense: vi.fn(), updateExpense: vi.fn() }))
const toast = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { finance: api } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

/** 用原生 select 代替通用 Select，便于在测试里切换选项。 */
const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template:
    '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="o in options" :key="String(o.value)" :value="o.value">{{ o.label }}</option></select>'
}

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(FinanceExpenseDialog, {
    props: { show: true, ...props },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: SelectStub
      }
    }
  })
}

const daysInput = (wrapper: ReturnType<typeof mountDialog>) =>
  wrapper.get('[data-testid="finance-expense-days"]').element as HTMLInputElement

const checkedMode = (wrapper: ReturnType<typeof mountDialog>) =>
  wrapper.get('[data-testid="finance-expense-mode"] [aria-checked="true"]').attributes('data-testid')

async function chooseCategory(wrapper: ReturnType<typeof mountDialog>, category: string) {
  await wrapper.get('[data-testid="finance-expense-category"]').setValue(category)
  await nextTick()
}

describe('FinanceExpenseDialog (CUSTOM)', () => {
  beforeEach(() => {
    api.createExpense.mockReset().mockImplementation(async (payload: Record<string, unknown>) => ({ id: 1, ...payload }))
    api.updateExpense.mockReset()
    toast.showSuccess.mockReset()
    toast.showError.mockReset()
  })

  it('defaults to the subscription category spread over 30 days', () => {
    const wrapper = mountDialog()
    expect((wrapper.get('[data-testid="finance-expense-category"]').element as HTMLSelectElement).value).toBe(
      'upstream_subscription'
    )
    expect(checkedMode(wrapper)).toBe('finance-expense-mode-daily')
    expect(daysInput(wrapper).value).toBe('30')
  })

  it('switches the default mode with the category unless picked by hand', async () => {
    const wrapper = mountDialog()
    await chooseCategory(wrapper, 'server')
    expect(checkedMode(wrapper)).toBe('finance-expense-mode-once')
    expect(wrapper.find('[data-testid="finance-expense-days"]').exists()).toBe(false)
    await chooseCategory(wrapper, 'upstream_subscription')
    expect(checkedMode(wrapper)).toBe('finance-expense-mode-daily')

    await chooseCategory(wrapper, 'server')
    await wrapper.get('[data-testid="finance-expense-mode-daily"]').trigger('click')
    await chooseCategory(wrapper, 'domain')
    expect(checkedMode(wrapper)).toBe('finance-expense-mode-daily')
  })

  it('posts a one-off subscription deduction as one day', async () => {
    const wrapper = mountDialog()
    await wrapper.get('[data-testid="finance-expense-mode-once"]').trigger('click')
    await wrapper.get('[data-testid="finance-expense-amount"]').setValue('200')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.createExpense).toHaveBeenCalledWith(
      expect.objectContaining({ category: 'upstream_subscription', amount: '200', amortize_days: 1 })
    )
  })

  it('keeps the stored days when editing an existing expense', () => {
    const expense: FinanceExpense = {
      id: 7,
      occurred_on: '2026-10-01',
      category: 'upstream_subscription',
      amount: '1400.00',
      amortize_days: 31,
      account_id: 1,
      account_name: 'vm2api - pro',
      note: 'ChatGPT Pro #3',
      daily_amount: '45.16129032',
      created_by_email: 'admin@x',
      created_at: '',
      updated_at: ''
    }
    const wrapper = mountDialog({ expense })
    expect(checkedMode(wrapper)).toBe('finance-expense-mode-daily')
    expect(daysInput(wrapper).value).toBe('31')
    expect((wrapper.get('[data-testid="finance-expense-amount"]').element as HTMLInputElement).value).toBe('1400.00')
  })

  it('validates before submitting and posts the amount as a string', async () => {
    const wrapper = mountDialog()
    await wrapper.get('form').trigger('submit')
    expect(api.createExpense).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.finance.expenses.errors.amountInvalid')

    await wrapper.get('[data-testid="finance-expense-amount"]').setValue('1400')
    await wrapper.get('[data-testid="finance-expense-note"]').setValue('Pro #3')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(api.createExpense).toHaveBeenCalledWith(
      expect.objectContaining({ category: 'upstream_subscription', amount: '1400', amortize_days: 30, account_id: null, note: 'Pro #3' })
    )
    expect(wrapper.emitted('saved')).toHaveLength(1)
  })
})
