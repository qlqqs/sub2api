import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GroupOptionItem from '../GroupOptionItem.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null }),
}))

describe('GroupOptionItem description layout', () => {
  it('applies multiline and overflow-safe text styles', () => {
    const description = 'First section\nvery-long-unbroken-description-value-that-must-not-overflow'
    const wrapper = mount(GroupOptionItem, {
      props: {
        name: 'Example group',
        platform: 'openai',
        description,
      },
      global: {
        stubs: {
          GroupBadge: true,
        },
      },
    })

    const descriptionElement = wrapper
      .findAll('span')
      .find((element) => element.text() === description)

    expect(descriptionElement).toBeDefined()
    expect(descriptionElement?.classes()).toContain('whitespace-pre-line')
    expect(descriptionElement?.classes()).toContain('[overflow-wrap:anywhere]')
    expect(descriptionElement?.classes()).toContain('line-clamp-3')
    expect(wrapper.find('[title]').attributes('title')).toBe(description)
  })
})

describe('GroupOptionItem price ratio', () => {
  it('shows "¥x = $1" ratios and strikes through the group ratio when an exclusive rate exists', () => {
    const wrapper = mount(GroupOptionItem, {
      props: {
        name: 'Example group',
        platform: 'openai',
        rateMultiplier: 0.5,
        userRateMultiplier: 0.25,
      },
      global: { stubs: { GroupBadge: true } },
    })

    expect(wrapper.find('.line-through').text()).toBe('¥0.5 = $1')
    expect(wrapper.text()).toContain('¥0.25 = $1')
    expect(wrapper.text()).not.toMatch(/\d+x/)
  })

  it('shows the group ratio when there is no exclusive rate', () => {
    const wrapper = mount(GroupOptionItem, {
      props: { name: 'Example group', platform: 'openai', rateMultiplier: 1 },
      global: { stubs: { GroupBadge: true } },
    })

    expect(wrapper.text()).toContain('¥1 = $1')
  })
})
