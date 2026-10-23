import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaFilterBar from '../PlazaFilterBar.vue'
import type { ModelPlazaGroup, ModelPlazaResponse } from '@/api/modelPlaza'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: true })
}))

function group(id: number, rate: number, overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id,
    name: `group-${id}`,
    description: '',
    platform: 'anthropic',
    subscription_type: 'standard',
    rate_multiplier: rate,
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
    models: [{ name: `model-${id}`, platform: 'anthropic', pricing: null, official_pricing: null }],
    ...overrides
  }
}

const response: ModelPlazaResponse = {
  description: '',
  groups: [group(1, 1), group(2, 0.5), group(3, 0.8, { platform: 'openai' })]
}

const SectionStub = {
  props: ['group'],
  template: '<section :id="`plaza-group-${group.id}`" data-testid="section">{{ group.name }}</section>'
}

function mountContent(props: Record<string, unknown>) {
  return mount(ModelPlazaContent, {
    attachTo: document.body,
    props: { response, loading: false, ...props },
    global: { stubs: { PlazaGroupSection: SectionStub, Icon: true, PlatformIcon: true } }
  })
}

describe('ModelPlazaContent ?group= 定位(CUSTOM)', () => {
  let scrollIntoView: ReturnType<typeof vi.fn>
  let scrolledIds: string[]

  beforeEach(() => {
    scrolledIds = []
    scrollIntoView = vi.fn(function (this: Element) {
      scrolledIds.push(this.id)
    })
    Object.defineProperty(Element.prototype, 'scrollIntoView', {
      configurable: true,
      writable: true,
      value: scrollIntoView
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('数据就绪后滚动到指定分组', async () => {
    const wrapper = mountContent({ focusGroupId: 3 })
    await flushPromises()
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    expect(scrolledIds).toEqual(['plaza-group-3'])
    wrapper.unmount()
  })

  it('等加载结束再定位,且只定位一次', async () => {
    const wrapper = mountContent({ focusGroupId: 2, loading: true, response: null })
    await flushPromises()
    expect(scrollIntoView).not.toHaveBeenCalled()
    await wrapper.setProps({ loading: false, response })
    await flushPromises()
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    await wrapper.setProps({ response: { ...response } })
    await flushPromises()
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('分组不在列表中(专属分组、未登录)时忽略', async () => {
    const wrapper = mountContent({ focusGroupId: 99 })
    await flushPromises()
    expect(scrollIntoView).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('没有 ?group= 时不滚动', async () => {
    const wrapper = mountContent({})
    await flushPromises()
    expect(scrollIntoView).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})

describe('ModelPlazaContent 筛选(CUSTOM: 去掉倍率筛选)', () => {
  it('筛选栏没有比例维度,分组仍按生效倍率升序', async () => {
    const wrapper = mountContent({})
    await flushPromises()
    const bar = wrapper.getComponent(PlazaFilterBar)
    expect(bar.props()).not.toHaveProperty('rates')
    expect(bar.props('groups')).toEqual([
      { id: 1, name: 'group-1', platform: 'anthropic' },
      { id: 2, name: 'group-2', platform: 'anthropic' },
      { id: 3, name: 'group-3', platform: 'openai' }
    ])
    expect(bar.text()).not.toContain('priceRatioLabel')
    expect(bar.text()).not.toContain('= $1')
    expect(wrapper.findAll('[data-testid="section"]').map((s) => s.text())).toEqual([
      'group-2',
      'group-3',
      'group-1'
    ])
    wrapper.unmount()
  })

  it('平台与分组两个维度仍互相置灰', async () => {
    const wrapper = mountContent({})
    await flushPromises()
    const platformButton = wrapper.findAll('button').find((b) => b.text() === 'openai')!
    await platformButton.trigger('click')
    const groupButton = wrapper.findAll('button').find((b) => b.text() === 'group-1')!
    expect(groupButton.attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('[data-testid="section"]').map((s) => s.text())).toEqual(['group-3'])
    wrapper.unmount()
  })
})
