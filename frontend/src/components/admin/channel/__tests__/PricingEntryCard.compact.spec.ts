import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, shallowMount } from '@vue/test-utils'
import PricingEntryCard from '../PricingEntryCard.vue'
import { emptyStatsPricing, statsPricingToForm } from '../accountStatsPricing'
vi.mock('@/api/admin/channels', () => ({ default: { getModelDefaultPricing: vi.fn() } }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))
enableAutoUnmount(afterEach)
describe('PricingEntryCard compact header', () => {
  it('keeps the original header layout by default', () => {
    const wrapper = shallowMount(PricingEntryCard, { props: { entry: statsPricingToForm(emptyStatsPricing('openai')) } })
    const header = wrapper.get('[data-testid="pricing-entry-header"]')
    expect(header.classes()).not.toContain('flex-wrap')
    expect(header.get('.flex-1').classes()).not.toContain('basis-48')
  })
  it('opts into wrapping with a shrinkable model field and readable billing selector', () => {
    const wrapper = shallowMount(PricingEntryCard, { props: { entry: statsPricingToForm(emptyStatsPricing('openai')), compactHeader: true } })
    const header = wrapper.get('[data-testid="pricing-entry-header"]')
    expect(header.classes()).toContain('flex-wrap')
    expect(header.get('.flex-1').classes()).toEqual(expect.arrayContaining(['min-w-0', 'basis-48']))
    expect(header.get('.w-40').classes()).toContain('shrink-0')
  })
})
