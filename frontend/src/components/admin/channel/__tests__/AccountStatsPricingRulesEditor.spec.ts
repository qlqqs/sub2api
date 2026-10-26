import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import AccountStatsPricingRulesEditor from '../AccountStatsPricingRulesEditor.vue'
import PricingEntryCard from '../PricingEntryCard.vue'
import { emptyStatsPricing } from '../accountStatsPricing'
import { statsPricingToForm } from '../accountStatsPricing'
import en from '@/i18n/locales/en/admin/finance'
import zh from '@/i18n/locales/zh/admin/finance'
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { list: vi.fn().mockResolvedValue({ items: [], total: 0 }) } } }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
const pricing = { ...emptyStatsPricing('openai'), models: ['gpt-*'], input_price: 0.000001 }
const rules = [{ id: 1, name: 'first', group_ids: [99], account_ids: [42], pricing: [pricing] }, { id: 2, name: 'second', group_ids: [], account_ids: [], pricing: [] }]
function mountEditor() {
  return mount(defineComponent({ components: { AccountStatsPricingRulesEditor }, setup() { return { rules: ref(JSON.parse(JSON.stringify(rules))) } }, template: '<AccountStatsPricingRulesEditor v-model="rules" :groups="[]"/>' }), { global: { stubs: { PricingEntryCard: { props: ['entry'], template: '<div/>' } } } })
}
describe('shared cost-rule editor', () => {
  it('warns explicitly that unsupported finance cost fields are rejected, not silently saved', () => {
    const wrapper = mountEditor()
    expect(wrapper.get('[data-testid="stats-pricing-support-warning"]').text()).toBe('admin.finance.costSettings.advancedSupport')
    expect(en.finance.costSettings.advancedSupport).toMatch(/endpoint rejects/)
    for (const field of ['Fast/Flex', 'image input', 'interval input/output/cache', 'non-empty time pricing']) {
      expect(en.finance.costSettings.advancedSupport).toContain(field)
    }
    expect(zh.finance.costSettings.advancedSupport).toContain('会被服务器拒绝')
    for (const field of ['Fast/Flex', '图片输入价', '区间输入/输出/缓存倍率', '非空分时规则']) {
      expect(zh.finance.costSettings.advancedSupport).toContain(field)
    }
  })
  it('preserves absent group scopes and reorders whole rules without losing prices', async () => {
    const wrapper = mountEditor()
    expect(wrapper.text()).toContain('#99')
    await wrapper.get('[aria-label="admin.finance.costSettings.moveDown"]').trigger('click')
    const editor = wrapper.getComponent(AccountStatsPricingRulesEditor)
    expect(editor.props('modelValue').map(r => r.id)).toEqual([2, 1])
    expect(editor.props('modelValue')[1]).toEqual(rules[0])
  })
  it('keeps decimal entry drafts until commit and validates malformed numeric input before write', async () => {
    const wrapper = mountEditor()
    const card = wrapper.getComponent(PricingEntryCard)
    card.vm.$emit('update', { ...statsPricingToForm(pricing), input_price: '0.' })
    await flushPromises()
    expect(wrapper.getComponent(PricingEntryCard).props('entry').input_price).toBe('0.')
    wrapper.getComponent(PricingEntryCard).vm.$emit('update', { ...statsPricingToForm(pricing), input_price: 'oops' })
    await flushPromises()
    expect(wrapper.getComponent(AccountStatsPricingRulesEditor).vm.validate()).toBe('admin.finance.costSettings.invalidPrice')
  })
})
