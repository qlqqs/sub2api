import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import FinancePaymentSettings from '../FinancePaymentSettings.vue'
import Pagination from '@/components/common/Pagination.vue'
import type { ProviderInstance } from '@/types/payment'
import { baseCompile } from '@intlify/message-compiler'
import en from '@/i18n/locales/en/admin/finance'
import zh from '@/i18n/locales/zh/admin/finance'

const client = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
const toast = vi.hoisted(() => ({ showSuccess: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)

function provider(overrides: Partial<ProviderInstance> = {}): ProviderInstance {
  return {
    id: 7, name: 'Primary payment', provider_key: 'easypay', enabled: true,
    channel_fee_rate: '0.0320', config: { private_key: 'never-send-this-secret' },
    supported_types: ['alipay'], payment_mode: 'redirect', refund_enabled: true,
    allow_user_refund: false, limits: '{}', sort_order: 1, ...overrides
  }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(r => { resolve = r })
  return { promise, resolve }
}
function mountSettings() {
  return mount(FinancePaymentSettings, { global: { stubs: { teleport: true } } })
}
async function openEditor() {
  const wrapper = mountSettings()
  await flushPromises()
  await wrapper.get('[data-testid="finance-payment-edit-7"]').trigger('click')
  return wrapper
}
function input(wrapper: ReturnType<typeof mountSettings>) {
  return wrapper.get('[data-testid="finance-payment-fee"]')
}
function inputValue(wrapper: ReturnType<typeof mountSettings>) {
  return (input(wrapper).element as HTMLInputElement).value
}
async function submit(wrapper: ReturnType<typeof mountSettings>) {
  await wrapper.get('#finance-payment-fee-form').trigger('submit')
  await flushPromises()
}

beforeEach(() => {
  vi.clearAllMocks()
  client.get.mockReset().mockResolvedValue({ data: [provider()] })
  client.put.mockReset().mockResolvedValue({ data: provider() })
})

describe('FinancePaymentSettings', () => {
  it('reads the existing provider list, shows exact percentages, and excludes credentials', async () => {
    const wrapper = await openEditor()
    expect(client.get).toHaveBeenCalledWith('/admin/payment/providers')
    expect(wrapper.text()).toContain('3.20%')
    expect(inputValue(wrapper)).toBe('3.2')
    expect(wrapper.text()).toContain('admin.finance.costSettings.paymentSource')
    expect(wrapper.text()).toContain('admin.finance.costSettings.paymentHint')
    expect(wrapper.html()).not.toContain('never-send-this-secret')
    expect(client.put).not.toHaveBeenCalled()
  })

  it.each([['2.85', '0.0285'], ['0', '0.0000'], ['20', '0.2000']])(
    'saves %s%% through the existing partial PUT with only channel_fee_rate', async (percent, rate) => {
      const wrapper = await openEditor()
      client.get.mockResolvedValue({ data: [provider({ channel_fee_rate: rate })] })
      await input(wrapper).setValue(percent)
      await submit(wrapper)
      expect(client.put).toHaveBeenCalledTimes(1)
      expect(client.put).toHaveBeenCalledWith('/admin/payment/providers/7', { channel_fee_rate: rate })
      expect(wrapper.emitted('changed')).toHaveLength(1)
      expect(toast.showSuccess).toHaveBeenCalledOnce()
      expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
      expect(client.get).toHaveBeenCalledTimes(2)
    }
  )

  it.each(['', ' ', '-1', '20.01', '100', '1.234', 'NaN', 'Infinity', '1e1'])('rejects invalid fee %j without writing', async value => {
    const wrapper = await openEditor()
    await input(wrapper).setValue(value)
    await submit(wrapper)
    expect(client.put).not.toHaveBeenCalled()
    expect(input(wrapper).attributes('aria-invalid')).toBe('true')
    expect(wrapper.text()).toContain('admin.finance.channelFee.invalid')
  })

  it.each(['cancel', 'escape'])('discards the draft on %s and reopens the saved value', async action => {
    const wrapper = await openEditor()
    await input(wrapper).setValue('12')
    if (action === 'cancel') await wrapper.get('[data-testid="finance-payment-cancel"]').trigger('click')
    else document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    await wrapper.get('[data-testid="finance-payment-edit-7"]').trigger('click')
    expect(inputValue(wrapper)).toBe('3.2')
    expect(client.put).not.toHaveBeenCalled()
    expect(wrapper.emitted('changed')).toBeUndefined()
  })

  it('preserves the draft and original row on failure and allows retry', async () => {
    const wrapper = await openEditor()
    client.put.mockRejectedValueOnce(new Error('Save unavailable'))
    await input(wrapper).setValue('2.85')
    await submit(wrapper)
    expect(wrapper.get('[data-testid="finance-payment-save-error"]').text()).toContain('Save unavailable')
    expect(inputValue(wrapper)).toBe('2.85')
    expect(wrapper.text()).toContain('3.20%')
    expect(wrapper.emitted('changed')).toBeUndefined()
    expect(toast.showSuccess).not.toHaveBeenCalled()
    await submit(wrapper)
    expect(client.put).toHaveBeenCalledTimes(2)
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('prevents duplicate saves and dismissal while the write is pending', async () => {
    const wrapper = await openEditor()
    const pending = deferred<{ data: ProviderInstance }>()
    client.put.mockReturnValue(pending.promise)
    await submit(wrapper)
    await submit(wrapper)
    expect(client.put).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="finance-payment-save"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="finance-payment-cancel"]').attributes('disabled')).toBeDefined()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    pending.resolve({ data: provider() })
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('reloads the current provider into the draft without saving stale values', async () => {
    const wrapper = await openEditor()
    await input(wrapper).setValue('12')
    client.get.mockResolvedValue({ data: [provider({ channel_fee_rate: '0.0700' })] })
    await wrapper.get('[data-testid="finance-payment-editor-reload"]').trigger('click')
    await flushPromises()
    expect(inputValue(wrapper)).toBe('7')
    expect(client.put).not.toHaveBeenCalled()
  })

  it('keeps the draft on reload failure and closes if the provider has been deleted', async () => {
    const wrapper = await openEditor()
    await input(wrapper).setValue('12')
    client.get.mockRejectedValueOnce(new Error('Reload unavailable'))
    await wrapper.get('[data-testid="finance-payment-editor-reload"]').trigger('click')
    await flushPromises()
    expect(inputValue(wrapper)).toBe('12')
    expect(wrapper.get('[data-testid="finance-payment-save-error"]').text()).toContain('Reload unavailable')
    client.get.mockResolvedValue({ data: [] })
    await wrapper.get('[data-testid="finance-payment-editor-reload"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(client.put).not.toHaveBeenCalled()
  })

  it('shows loading, a retryable error, and the empty state', async () => {
    const pending = deferred<{ data: ProviderInstance[] }>()
    client.get.mockReturnValueOnce(pending.promise)
    const wrapper = mountSettings()
    expect(wrapper.get('section').attributes('aria-busy')).toBe('true')
    pending.resolve({ data: [] })
    await flushPromises()
    expect(wrapper.text()).toContain('admin.finance.costSettings.paymentEmpty')
    client.get.mockRejectedValueOnce(new Error('Unavailable'))
    await wrapper.get('[data-testid="finance-payment-reload"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="finance-load-error"]').exists()).toBe(true)
    await wrapper.get('[data-testid="finance-load-error"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="finance-payment-edit-7"]').exists()).toBe(true)
  })

  it('paginates and searches by name, ID and provider type, resetting the page', async () => {
    client.get.mockResolvedValue({ data: Array.from({ length: 22 }, (_, i) => provider({
      id: i + 1, name: `Provider ${i + 1}`, provider_key: i === 21 ? 'stripe' : 'easypay'
    })) })
    const wrapper = mountSettings()
    await flushPromises()
    expect(wrapper.find('[data-testid="finance-payment-edit-21"]').exists()).toBe(false)
    wrapper.getComponent(Pagination).vm.$emit('update:page', 2)
    await flushPromises()
    expect(wrapper.find('[data-testid="finance-payment-edit-21"]').exists()).toBe(true)
    await wrapper.get('[data-testid="finance-payment-search"]').setValue(' STRIPE ')
    expect(wrapper.getComponent(Pagination).props('page')).toBe(1)
    expect(wrapper.getComponent(Pagination).props('total')).toBe(1)
    expect(wrapper.find('[data-testid="finance-payment-edit-22"]').exists()).toBe(true)
    await wrapper.get('[data-testid="finance-payment-search"]').setValue('Provider 3')
    expect(wrapper.find('[data-testid="finance-payment-edit-3"]').exists()).toBe(true)
    await wrapper.get('[data-testid="finance-payment-search"]').setValue('17')
    expect(wrapper.find('[data-testid="finance-payment-edit-17"]').exists()).toBe(true)
    await wrapper.get('[data-testid="finance-payment-search"]').setValue('no-match')
    expect(wrapper.text()).toContain('admin.finance.costSettings.paymentEmpty')
    expect(wrapper.findComponent(Pagination).exists()).toBe(false)
  })

  it('ignores late list responses after a refreshToken reload', async () => {
    const pending = deferred<{ data: ProviderInstance[] }>()
    client.get.mockReturnValueOnce(pending.promise)
    const wrapper = mountSettings()
    await wrapper.setProps({ refreshToken: 1 })
    await flushPromises()
    pending.resolve({ data: [provider({ name: 'Stale response' })] })
    await flushPromises()
    expect(wrapper.text()).toContain('Primary payment')
    expect(wrapper.text()).not.toContain('Stale response')
  })

  it('does not emit, toast or reload when an in-flight save completes after unmount', async () => {
    const wrapper = await openEditor()
    const pending = deferred<{ data: ProviderInstance }>()
    client.put.mockReturnValueOnce(pending.promise)
    await submit(wrapper)
    wrapper.unmount()
    pending.resolve({ data: provider() })
    await flushPromises()
    expect(wrapper.emitted('changed')).toBeUndefined()
    expect(toast.showSuccess).not.toHaveBeenCalled()
    expect(client.get).toHaveBeenCalledTimes(1)
  })
})

// Verify the registered cost-settings messages, not a second locale namespace.
describe('FinancePaymentSettings registered locales', () => {
  it('keeps the English and Chinese namespace and message keys aligned', () => {
    expect(Object.keys(en.finance.costSettings).sort()).toEqual(Object.keys(zh.finance.costSettings).sort())
  })

  it.each([['en', en], ['zh', zh]] as const)('%s messages compile independently', (_locale, messages) => {
    const errors: string[] = []
    for (const message of Object.values(messages.finance.costSettings)) {
      expect(message.trim()).not.toBe('')
      baseCompile(message, { onError: error => { errors.push(error.message) } })
    }
    expect(errors).toEqual([])
  })
})
