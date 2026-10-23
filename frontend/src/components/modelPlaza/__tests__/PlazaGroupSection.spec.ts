import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlazaGroupSection from '../PlazaGroupSection.vue'
import PlazaModelPricingTable from '../PlazaModelPricingTable.vue'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}|${JSON.stringify(params)}` : key)
    })
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null, usdCnyRate: 7.1 })
}))

function ladderModel(tiers: number): PlazaModel {
  const intervals = Array.from({ length: tiers }, (_, i) => ({
    min_tokens: i * 272000,
    max_tokens: i === tiers - 1 ? null : (i + 1) * 272000,
    tier_label: '',
    input_price: 5e-6,
    output_price: 3e-5,
    cache_write_price: null,
    cache_read_price: null,
    per_request_price: null
  }))
  return {
    name: 'gpt-5.6-sol',
    platform: 'openai',
    pricing: {
      billing_mode: 'token',
      input_price: 5e-6,
      output_price: 3e-5,
      cache_write_price: null,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: []
    },
    official_pricing: {
      input_price: 5e-6,
      output_price: 3e-5,
      cache_write_price: null,
      cache_read_price: null,
      intervals
    }
  }
}

function group(overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id: 1,
    name: 'g',
    description: '',
    platform: 'openai',
    subscription_type: 'standard',
    rate_multiplier: 1,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    video_rate_independent: false,
    video_rate_multiplier: 1,
    long_context_pricing_enabled: true,
    models: [ladderModel(2)],
    ...overrides
  }
}

function mountSection(g: ModelPlazaGroup) {
  return mount(PlazaGroupSection, {
    props: { group: g },
    global: {
      stubs: {
        GroupBadge: true,
        Icon: true,
        PlazaModelPricingTable: true
      }
    }
  })
}

const NOTE = 'modelPlaza.detail.longContextDisabledNote'

describe('PlazaGroupSection 长上下文说明', () => {
  it('passes video-specific rates to the pricing table', () => {
    const wrapper = mountSection(group({ video_rate_independent: true, video_rate_multiplier: 0 }))
    expect(wrapper.findComponent(PlazaModelPricingTable).props()).toMatchObject({
      videoRateIndependent: true,
      videoRateMultiplier: 0
    })
    wrapper.unmount()
  })
  it('分组关闭阶梯且组内有官方阶梯模型时显示说明', () => {
    const wrapper = mountSection(group({ long_context_pricing_enabled: false }))
    expect(wrapper.text()).toContain(NOTE)
  })

  it('分组开启阶梯时不显示', () => {
    const wrapper = mountSection(group({ long_context_pricing_enabled: true }))
    expect(wrapper.text()).not.toContain(NOTE)
  })

  it('分组关闭但没有官方阶梯模型时不显示', () => {
    const wrapper = mountSection(
      group({ long_context_pricing_enabled: false, models: [ladderModel(1)] })
    )
    expect(wrapper.text()).not.toContain(NOTE)
  })

  it('旧后端缺少开关字段时不显示', () => {
    const g = group()
    delete (g as Partial<ModelPlazaGroup>).long_context_pricing_enabled
    const wrapper = mountSection(g)
    expect(wrapper.text()).not.toContain(NOTE)
  })
})

describe('PlazaGroupSection 高峰配置传递', () => {
  it('分组启用高峰时把窗口描述与倍率传给价格表', () => {
    const wrapper = mountSection(
      group({
        subscription_type: 'subscription',
        peak_rate_enabled: true,
        peak_start: '14:00',
        peak_end: '18:00',
        peak_rate_multiplier: 1.5
      })
    )
    const table = wrapper.findComponent(PlazaModelPricingTable)
    // appStore mock 无 server_utc_offset,窗口描述不带时区标注;传给价格表的只是时间窗口(倍率另行说明)
    expect(table.props('peakWindow')).toBe('14:00-18:00')
    expect(table.props('peakRateMultiplier')).toBe(1.5)
  })

  it('CUSTOM: 分组徽章与高峰提示都不出现倍率,高峰提示为「价格上浮 x%」', () => {
    const g = group({
      rate_multiplier: 1,
      user_rate_multiplier: 0.4,
      peak_rate_enabled: true,
      peak_start: '14:00',
      peak_end: '18:00',
      peak_rate_multiplier: 1.5
    })
    // 不 stub GroupBadge,验证徽章不再显示倍率与高峰比例
    const wrapper = mount(PlazaGroupSection, {
      props: { group: g },
      global: { stubs: { Icon: true, PlatformIcon: true, PlazaModelPricingTable: true } }
    })
    const text = wrapper.text()
    expect(text).not.toContain('= $1')
    expect(text).not.toContain('×')
    const note = wrapper.get('[data-testid="plaza-peak-note"]')
    expect(note.text()).toBe('modelPlaza.detail.peakUpNote|{"window":"14:00–18:00","percent":"50"}')
    expect(note.attributes('title')).toBeUndefined()
  })

  it('高峰倍率小于 1 时写「下调」,等于 1 时不提示', () => {
    const down = mountSection(
      group({ peak_rate_enabled: true, peak_start: '14:00', peak_end: '18:00', peak_rate_multiplier: 0.8 })
    )
    expect(down.get('[data-testid="plaza-peak-note"]').text()).toContain('modelPlaza.detail.peakDownNote')
    expect(down.text()).toContain('"percent":"20"')

    const same = mountSection(
      group({ peak_rate_enabled: true, peak_start: '14:00', peak_end: '18:00', peak_rate_multiplier: 1 })
    )
    expect(same.find('[data-testid="plaza-peak-note"]').exists()).toBe(false)
  })

  it('分组未启用高峰时窗口描述为空串', () => {
    const wrapper = mountSection(group())
    expect(wrapper.findComponent(PlazaModelPricingTable).props('peakWindow')).toBe('')
  })
})

describe('PlazaGroupSection 定位与汇率(CUSTOM)', () => {
  it('分组 section 带 plaza-group-<id> 锚点与滚动留白', () => {
    const wrapper = mountSection(group({ id: 42 }))
    const section = wrapper.get('section')
    expect(section.attributes('id')).toBe('plaza-group-42')
    expect(section.classes()).toContain('scroll-mt-24')
  })

  it('把公开设置里的汇率传给价格表', () => {
    const wrapper = mountSection(group())
    expect(wrapper.findComponent(PlazaModelPricingTable).props('fxRate')).toBe(7.1)
  })
})
