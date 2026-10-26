// CUSTOM: 支付实例通道费率（百分比输入 ↔ 小数字符串提交）
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import PaymentProviderDialog from '@/components/payment/PaymentProviderDialog.vue'
import type { ProviderInstance } from '@/types/payment'

const showError = vi.hoisted(() => vi.fn())

vi.mock('@/stores', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key)
  })
}))

function provider(overrides: Partial<ProviderInstance> = {}): ProviderInstance {
  return {
    id: 1,
    provider_key: 'easypay',
    name: 'EasyPay',
    config: {
      pid: 'pid-1',
      apiBase: 'https://pay.example.com',
      notifyUrl: 'https://example.com/api/v1/payment/webhook/easypay',
      returnUrl: 'https://example.com/payment/result'
    },
    supported_types: ['alipay', 'wxpay'],
    enabled: true,
    payment_mode: 'qrcode',
    refund_enabled: false,
    allow_user_refund: false,
    limits: '',
    sort_order: 0,
    ...overrides
  }
}

function mountDialog(editing: ProviderInstance | null) {
  return mount(PaymentProviderDialog, {
    props: {
      show: true,
      saving: false,
      editing,
      allKeyOptions: [{ value: 'easypay', label: 'EasyPay' }],
      enabledKeyOptions: [{ value: 'easypay', label: 'EasyPay' }],
      allPaymentTypes: [
        { value: 'alipay', label: 'Alipay' },
        { value: 'wxpay', label: 'WeChat Pay' }
      ],
      redirectLabel: 'Redirect'
    },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: { props: ['modelValue', 'options', 'disabled'], template: '<div />' },
        ToggleSwitch: { template: '<div />' }
      }
    }
  })
}

type DialogExpose = { loadProvider: (p: ProviderInstance) => void; reset: (key: string) => void }

async function open(p: ProviderInstance) {
  const wrapper = mountDialog(p)
  ;(wrapper.vm as unknown as DialogExpose).loadProvider(p)
  await nextTick()
  return wrapper
}

const feeInput = (wrapper: ReturnType<typeof mountDialog>) => wrapper.get('[data-testid="provider-channel-fee-rate"]')

describe('PaymentProviderDialog channel fee rate (CUSTOM)', () => {
  it('shows the stored decimal rate as a percentage', async () => {
    const wrapper = await open(provider({ channel_fee_rate: '0.0320' }))
    expect((feeInput(wrapper).element as HTMLInputElement).value).toBe('3.2')
  })

  it('defaults to 0 for providers without a rate (older backend)', async () => {
    const wrapper = await open(provider())
    expect((feeInput(wrapper).element as HTMLInputElement).value).toBe('0')
  })

  it('submits the percentage as a 4-decimal rate string', async () => {
    const wrapper = await open(provider({ channel_fee_rate: '0.0320' }))
    await feeInput(wrapper).setValue('2.85')
    await wrapper.get('form').trigger('submit')
    const payload = wrapper.emitted('save')?.[0]?.[0] as { channel_fee_rate: string }
    expect(payload.channel_fee_rate).toBe('0.0285')
  })

  it('blocks out-of-range input', async () => {
    showError.mockReset()
    const wrapper = await open(provider())
    await feeInput(wrapper).setValue('25')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('save')).toBeUndefined()
    await vi.waitFor(() => expect(showError).toHaveBeenCalledWith(expect.stringContaining('admin.finance.channelFee.invalid')))
  })

  it('resets to 0 for a new provider', async () => {
    const wrapper = await open(provider({ channel_fee_rate: '0.0320' }))
    ;(wrapper.vm as unknown as DialogExpose).reset('easypay')
    await nextTick()
    expect((feeInput(wrapper).element as HTMLInputElement).value).toBe('0')
  })
})
