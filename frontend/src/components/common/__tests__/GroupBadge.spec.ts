import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GroupBadge from '../GroupBadge.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key),
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: { server_utc_offset: '+08:00' } }),
}))

function mountBadge(props: Record<string, unknown>) {
  return mount(GroupBadge, {
    props: { name: 'Claude Pro', platform: 'anthropic', ...props },
    global: { stubs: { PlatformIcon: true } },
  })
}

describe('GroupBadge price ratio', () => {
  it('shows the group rate as a "¥x = $1" ratio with the raw multiplier in the title', () => {
    const wrapper = mountBadge({ rateMultiplier: 0.5 })

    expect(wrapper.text()).toContain('¥0.5 = $1')
    expect(wrapper.text()).not.toContain('0.5x')
    const label = wrapper.findAll('span').find(el => el.text() === '¥0.5 = $1')
    expect(label?.attributes('title')).toBe('common.rateRatioTitle:{"rate":"0.5"}')
  })

  it('uses the exclusive user rate and strikes through the group ratio', () => {
    const wrapper = mountBadge({ rateMultiplier: 0.5, userRateMultiplier: 0.3 })

    const struck = wrapper.find('.line-through')
    expect(struck.text()).toBe('¥0.5 = $1')
    expect(wrapper.find('.font-bold').text()).toBe('¥0.3 = $1')
    expect(wrapper.html()).toContain('common.rateRatioTitleCustom')
  })

  it('shows the peak window as the peak-period ratio (effective rate × peak multiplier)', () => {
    const wrapper = mountBadge({
      rateMultiplier: 0.5,
      userRateMultiplier: 0.4,
      peakRateEnabled: true,
      peakStart: '14:00',
      peakEnd: '18:00',
      peakRateMultiplier: 1.5,
    })

    expect(wrapper.text()).toContain('14:00-18:00 ¥0.6 = $1 (UTC+08:00)')
  })

  it('keeps the days-remaining label for subscription groups', () => {
    const wrapper = mountBadge({ rateMultiplier: 1, subscriptionType: 'subscription', daysRemaining: 10 })

    expect(wrapper.text()).toContain('admin.users.daysRemaining')
    expect(wrapper.text()).not.toContain('= $1')
  })

  it('shows the ratio for subscription groups when alwaysShowRate is set', () => {
    const wrapper = mountBadge({ rateMultiplier: 2, subscriptionType: 'subscription', alwaysShowRate: true })

    expect(wrapper.text()).toContain('¥2 = $1')
  })
})
