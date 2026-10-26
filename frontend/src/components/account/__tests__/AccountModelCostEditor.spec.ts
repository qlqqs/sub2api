import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AccountModelCostEditor from '../AccountModelCostEditor.vue'

const api = vi.hoisted(() => ({ getForAccount: vi.fn(), replaceForAccount: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { accountModelCost: api } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

type EditorExpose = { save: () => Promise<void>; validate: () => string | null }

function mountEditor(accountId: number | null = 2) {
  return mount(AccountModelCostEditor, { props: { accountId }, global: { stubs: { Icon: true } } })
}

describe('AccountModelCostEditor (CUSTOM)', () => {
  beforeEach(() => {
    api.getForAccount.mockReset().mockResolvedValue([{ model: 'gpt-5.6-luna', rate_multiplier: 0.3 }])
    api.replaceForAccount.mockReset().mockImplementation(async (_id: number, rules: unknown[]) => rules)
  })

  it('loads existing rules and does not save when unchanged', async () => {
    const wrapper = mountEditor()
    await flushPromises()
    expect(api.getForAccount).toHaveBeenCalledWith(2)
    const model = wrapper.get('[data-testid="account-model-cost-model"]').element as HTMLInputElement
    expect(model.value).toBe('gpt-5.6-luna')
    await (wrapper.vm as unknown as EditorExpose).save()
    expect(api.replaceForAccount).not.toHaveBeenCalled()
  })

  it('saves added rules (zero allowed) as a full replacement', async () => {
    const wrapper = mountEditor()
    await flushPromises()
    await wrapper.get('[data-testid="account-model-cost-add"]').trigger('click')
    await wrapper.findAll('[data-testid="account-model-cost-model"]')[1].setValue('gpt-5.6-free*')
    await wrapper.findAll('[data-testid="account-model-cost-rate"]')[1].setValue('0')
    await (wrapper.vm as unknown as EditorExpose).save()
    expect(api.replaceForAccount).toHaveBeenCalledWith(2, [
      { model: 'gpt-5.6-luna', rate_multiplier: 0.3 },
      { model: 'gpt-5.6-free*', rate_multiplier: 0 }
    ])
  })

  it('reports invalid rows instead of silently dropping them', async () => {
    const wrapper = mountEditor()
    await flushPromises()
    await wrapper.get('[data-testid="account-model-cost-rate"]').setValue('0.12345')
    expect((wrapper.vm as unknown as EditorExpose).validate()).toBe('admin.finance.accountModelCost.errors.rateDecimals')
    await expect((wrapper.vm as unknown as EditorExpose).save()).rejects.toThrow()
    expect(api.replaceForAccount).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="account-model-cost-error"]').exists()).toBe(true)
  })

  it('keeps existing rules when loading fails', async () => {
    api.getForAccount.mockRejectedValue({ message: 'boom' })
    const wrapper = mountEditor()
    await flushPromises()
    expect(wrapper.find('[data-testid="account-model-cost-load-error"]').exists()).toBe(true)
    await (wrapper.vm as unknown as EditorExpose).save()
    expect(api.replaceForAccount).not.toHaveBeenCalled()
  })
})
