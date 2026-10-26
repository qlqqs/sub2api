import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import FinanceMoney from '../FinanceMoney.vue'

describe('FinanceMoney (CUSTOM)', () => {
  it('formats with ¥ and 2 decimals, red when negative', () => {
    const negative = mount(FinanceMoney, { props: { value: '-12.345' } })
    expect(negative.text()).toBe('-¥12.35')
    expect(negative.classes()).toContain('text-red-600')

    const positive = mount(FinanceMoney, { props: { value: '1234.5' } })
    expect(positive.text()).toBe('¥1,234.50')
    expect(positive.classes()).not.toContain('text-red-600')
  })

  it('supports 4-decimal unit prices, no currency and neutral negatives', () => {
    const wrapper = mount(FinanceMoney, { props: { value: '-0.03125', digits: 4, currency: false, colorNegative: false } })
    expect(wrapper.text()).toBe('-0.0313')
    expect(wrapper.classes()).not.toContain('text-red-600')
  })
})
