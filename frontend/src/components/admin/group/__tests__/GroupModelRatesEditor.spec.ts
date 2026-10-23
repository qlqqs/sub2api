import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import GroupModelRatesEditor from '../GroupModelRatesEditor.vue'

const api = vi.hoisted(() => ({
  getForGroup: vi.fn(),
  replaceForGroup: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { groupModelRates: api }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

type EditorExpose = {
  save: (groupId: number) => Promise<void>
  validate: () => string | null
}

function mountEditor(props: Record<string, unknown> = {}) {
  return mount(GroupModelRatesEditor, { props, global: { stubs: { Icon: true } } })
}

function exposed(wrapper: ReturnType<typeof mountEditor>): EditorExpose {
  return wrapper.vm as unknown as EditorExpose
}

async function fillRow(wrapper: ReturnType<typeof mountEditor>, index: number, model: string, rate: string) {
  await wrapper.findAll('[data-testid="group-model-rates-model"]')[index].setValue(model)
  await wrapper.findAll('[data-testid="group-model-rates-rate"]')[index].setValue(rate)
}

async function addRow(wrapper: ReturnType<typeof mountEditor>, model: string, rate: string) {
  await wrapper.get('[data-testid="group-model-rates-add"]').trigger('click')
  const index = wrapper.findAll('[data-testid="group-model-rates-row"]').length - 1
  await fillRow(wrapper, index, model, rate)
}

describe('GroupModelRatesEditor (CUSTOM)', () => {
  beforeEach(() => {
    api.getForGroup.mockReset()
    api.replaceForGroup.mockReset().mockImplementation(async (_id: number, rules: unknown[]) => rules)
  })

  it('create mode: does not load, and saves new rules for the created group', async () => {
    const wrapper = mountEditor()
    await flushPromises()
    expect(api.getForGroup).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.groups.modelRates.empty')

    await addRow(wrapper, ' claude-opus-* ', '2')
    await addRow(wrapper, 'gpt-5.5', '0.8')
    expect(exposed(wrapper).validate()).toBeNull()
    await exposed(wrapper).save(42)

    expect(api.replaceForGroup).toHaveBeenCalledWith(42, [
      { model: 'claude-opus-*', rate_multiplier: 2 },
      { model: 'gpt-5.5', rate_multiplier: 0.8 }
    ])
  })

  it('create mode: skips the request when nothing was entered (blank rows ignored)', async () => {
    const wrapper = mountEditor()
    await wrapper.get('[data-testid="group-model-rates-add"]').trigger('click')
    expect(exposed(wrapper).validate()).toBeNull()
    await exposed(wrapper).save(42)
    expect(api.replaceForGroup).not.toHaveBeenCalled()
  })

  it('edit mode: loads existing rules and only saves when they change', async () => {
    api.getForGroup.mockResolvedValue([
      { model: 'claude-opus-*', rate_multiplier: 1 },
      { model: 'gpt-5.5', rate_multiplier: 0.8 }
    ])
    const wrapper = mountEditor({ groupId: 9 })
    await flushPromises()
    expect(api.getForGroup).toHaveBeenCalledWith(9)
    const models = wrapper.findAll<HTMLInputElement>('[data-testid="group-model-rates-model"]')
    expect(models.map((input) => input.element.value)).toEqual(['claude-opus-*', 'gpt-5.5'])

    await exposed(wrapper).save(9)
    expect(api.replaceForGroup).not.toHaveBeenCalled()

    await wrapper.findAll('[data-testid="group-model-rates-remove"]')[1].trigger('click')
    await exposed(wrapper).save(9)
    expect(api.replaceForGroup).toHaveBeenCalledWith(9, [{ model: 'claude-opus-*', rate_multiplier: 1 }])
  })

  it('edit mode: removing every rule clears the group (empty array)', async () => {
    api.getForGroup.mockResolvedValue([{ model: 'gpt-5.5', rate_multiplier: 0.8 }])
    const wrapper = mountEditor({ groupId: 9 })
    await flushPromises()
    await wrapper.get('[data-testid="group-model-rates-remove"]').trigger('click')
    await exposed(wrapper).save(9)
    expect(api.replaceForGroup).toHaveBeenCalledWith(9, [])
  })

  it('shows normalized rules returned by the server after saving', async () => {
    api.replaceForGroup.mockResolvedValue([{ model: 'claude-opus-4-5', rate_multiplier: 1.5 }])
    const wrapper = mountEditor()
    await addRow(wrapper, 'Claude-Opus-4.5', '1.5')
    await exposed(wrapper).save(1)
    const models = wrapper.findAll<HTMLInputElement>('[data-testid="group-model-rates-model"]')
    expect(models.map((input) => input.element.value)).toEqual(['claude-opus-4-5'])
    // 保存后与服务端一致,不再重复提交
    await exposed(wrapper).save(1)
    expect(api.replaceForGroup).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['wildcard in the middle', 'claude-*-opus', '1', 'admin.groups.modelRates.errors.wildcardPosition'],
    ['bare wildcard', '*', '1', 'admin.groups.modelRates.errors.wildcardPosition'],
    ['missing model', '', '1', 'admin.groups.modelRates.errors.modelRequired'],
    ['zero rate', 'gpt-5', '0', 'admin.groups.modelRates.errors.rateInvalid'],
    ['negative rate', 'gpt-5', '-1', 'admin.groups.modelRates.errors.rateInvalid'],
    ['rate too large', 'gpt-5', '10001', 'admin.groups.modelRates.errors.rateTooLarge'],
    ['missing rate', 'gpt-5', '', 'admin.groups.modelRates.errors.rateRequired']
  ])('rejects %s before the group is saved', async (_label, model, rate, message) => {
    const wrapper = mountEditor()
    await addRow(wrapper, model, rate)
    expect(exposed(wrapper).validate()).toBe(message)
    await flushPromises()
    expect(wrapper.get('[data-testid="group-model-rates-error"]').text()).toBe(message)
    await expect(exposed(wrapper).save(1)).rejects.toThrow(message)
    expect(api.replaceForGroup).not.toHaveBeenCalled()
  })

  it('rejects duplicates case-insensitively, including claude dot/dash variants', async () => {
    const wrapper = mountEditor()
    await addRow(wrapper, 'GPT-5', '1')
    await addRow(wrapper, 'gpt-5', '2')
    expect(exposed(wrapper).validate()).toBe('admin.groups.modelRates.errors.duplicate')

    const claude = mountEditor()
    await addRow(claude, 'claude-opus-4.5', '1')
    await addRow(claude, 'claude-opus-4-5', '2')
    expect(exposed(claude).validate()).toBe('admin.groups.modelRates.errors.duplicate')
  })

  it('does not wipe existing rules when loading failed', async () => {
    api.getForGroup.mockRejectedValue({ message: 'not found' })
    const wrapper = mountEditor({ groupId: 9 })
    await flushPromises()
    expect(wrapper.get('[data-testid="group-model-rates-load-error"]').text()).toContain(
      'admin.groups.modelRates.loadFailed'
    )
    expect(wrapper.get('[data-testid="group-model-rates-add"]').attributes('disabled')).toBeDefined()
    expect(exposed(wrapper).validate()).toBeNull()
    await exposed(wrapper).save(9)
    expect(api.replaceForGroup).not.toHaveBeenCalled()
  })

  it('offers model suggestions through a datalist', () => {
    const wrapper = mountEditor({ suggestions: ['claude-sonnet-4-5', 'gpt-5.5'] })
    const options = wrapper.findAll('datalist option').map((o) => o.attributes('value'))
    expect(options).toEqual(['claude-sonnet-4-5', 'gpt-5.5'])
  })
})

describe('GroupModelRatesEditor native validation (CUSTOM)', () => {
  it('does not let browser constraints block the surrounding group form (4-decimal rates)', async () => {
    const wrapper = mountEditor()
    await addRow(wrapper, 'gpt-5', '0.1234')
    const rate = wrapper.get<HTMLInputElement>('[data-testid="group-model-rates-rate"]')
    expect(rate.attributes('step')).toBe('any')
    expect(rate.attributes('min')).toBeUndefined()
    expect(rate.attributes('max')).toBeUndefined()
    expect(rate.element.checkValidity()).toBe(true)
    expect(exposed(wrapper).validate()).toBeNull()
    await exposed(wrapper).save(3)
    expect(api.replaceForGroup).toHaveBeenCalledWith(3, [{ model: 'gpt-5', rate_multiplier: 0.1234 }])
  })
})
