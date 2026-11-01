import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

import PartnerAttachDialog from '../PartnerAttachDialog.vue'
import PartnerCreateDialog from '../PartnerCreateDialog.vue'

const api = vi.hoisted(() => ({
  previewPartner: vi.fn(),
  createPartner: vi.fn(),
  previewAttach: vi.fn(),
  attachAgent: vi.fn()
}))
const toast = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin/affiliatePartners', () => ({ default: api, affiliatePartnersAPI: api }))
vi.mock('@/api/admin/affiliates', () => ({ affiliatesAPI: { lookupTopAgents: vi.fn(), lookupUsers: vi.fn() } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}${JSON.stringify(params)}` : key) })
  }
})

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show" data-test="dialog"><slot /><slot name="footer" /></div>'
}
const ConfirmDialogStub = {
  props: ['show', 'title', 'message'],
  emits: ['confirm', 'cancel'],
  template: '<div v-if="show" data-test="confirm"><p data-test="confirm-message">{{ message }}</p><button type="button" data-test="confirm-ok" @click="$emit(\'confirm\')">ok</button></div>'
}
/** 选人组件：点按钮即选中 id 62 的用户。 */
const LookupStub = defineComponent({
  props: ['selected', 'testId'],
  emits: ['select', 'clear'],
  setup(props, { emit }) {
    return () =>
      h('button', { type: 'button', 'data-test': `${props.testId}-pick`, onClick: () => emit('select', { id: 62, email: '578486809@qq.com', username: '' }) })
  }
})

const stubs = { BaseDialog: BaseDialogStub, ConfirmDialog: ConfirmDialogStub, AffiliateUserLookup: LookupStub }

const USER = { id: 62, email: '578486809@qq.com', username: '', deleted: false }

describe('PartnerCreateDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('previews, shows the current region, and creates only after the second confirmation', async () => {
    api.previewPartner.mockResolvedValue({
      user: USER,
      agent_level: 3,
      is_partner: false,
      is_attached: false,
      team: { owned: 0, via_upline: 0 },
      current_region: { partner: { id: 75, email: 'p@x.com', username: '', deleted: false }, region_name: '三科大' },
      can_create: true,
      reason: null
    })
    api.createPartner.mockResolvedValue({ user: USER, region_name: '重大' })
    const wrapper = mount(PartnerCreateDialog, { props: { show: false }, global: { stubs } })
    await wrapper.setProps({ show: true })

    const submit = () => wrapper.get('[data-test="partner-create-submit"]')
    expect(submit().attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="partner-create-user-pick"]').trigger('click')
    await flushPromises()
    expect(api.previewPartner).toHaveBeenCalledWith(62, expect.anything())
    expect(wrapper.get('[data-test="partner-preview-region"]').text()).toContain('三科大')
    // 区域名为空不能提交；比例默认 40。
    expect(submit().attributes('disabled')).toBeDefined()
    expect((wrapper.get('[data-test="partner-create-rate"]').element as HTMLInputElement).value).toBe('40')

    await wrapper.get('[data-test="partner-create-region"]').setValue('重大')
    expect(submit().attributes('disabled')).toBeUndefined()
    await submit().trigger('click')
    expect(api.createPartner).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="confirm-message"]').text()).toContain('admin.partners.create.confirmMessage')

    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(api.createPartner).toHaveBeenCalledWith({ user_id: 62, region_name: '重大', share_rate: '0.4000' })
    expect(wrapper.emitted('created')).toHaveLength(1)
  })

  it('blocks users that cannot become partners and invalid rates', async () => {
    api.previewPartner.mockResolvedValue({
      user: USER,
      agent_level: 1,
      team: { owned: 6, via_upline: 3 },
      current_region: null,
      can_create: false,
      reason: '已挂在其他合伙人名下'
    })
    const wrapper = mount(PartnerCreateDialog, { props: { show: true }, global: { stubs } })
    await wrapper.get('[data-test="partner-create-user-pick"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="partner-create-region"]').setValue('华南')
    expect(wrapper.get('[data-test="partner-preview-reason"]').text()).toBe('已挂在其他合伙人名下')
    expect(wrapper.get('[data-test="partner-create-submit"]').attributes('disabled')).toBeDefined()

    api.previewPartner.mockResolvedValue({ user: USER, agent_level: 3, team: { owned: 0, via_upline: 0 }, current_region: null, can_create: true, reason: null })
    await wrapper.get('[data-test="partner-create-user-pick"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="partner-create-rate"]').setValue('120')
    expect(wrapper.text()).toContain('admin.partners.errors.rate')
    expect(wrapper.get('[data-test="partner-create-submit"]').attributes('disabled')).toBeDefined()
  })

  it('toasts the backend message when creating fails', async () => {
    api.previewPartner.mockResolvedValue({ user: USER, agent_level: 3, team: { owned: 0, via_upline: 0 }, current_region: null, can_create: true, reason: null })
    api.createPartner.mockRejectedValue({ status: 409, reason: 'PARTNER_EXISTS', message: '该用户已经是合伙人' })
    const wrapper = mount(PartnerCreateDialog, { props: { show: true }, global: { stubs } })
    await wrapper.get('[data-test="partner-create-user-pick"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="partner-create-region"]').setValue('重大')
    await wrapper.get('[data-test="partner-create-submit"]').trigger('click')
    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(toast.showError).toHaveBeenCalledWith('该用户已经是合伙人')
    expect(wrapper.emitted('created')).toBeUndefined()
  })
})

describe('PartnerAttachDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('previews the agent and attaches after the second confirmation', async () => {
    api.previewAttach.mockResolvedValue({
      agent: USER,
      agent_level: 1,
      team: { owned: 6, via_upline: 3 },
      current_region: null,
      can_attach: true,
      reason: null
    })
    api.attachAgent.mockResolvedValue({ user: USER })
    const wrapper = mount(PartnerAttachDialog, { props: { show: true, partnerId: 75, regionName: '三科大' }, global: { stubs } })

    expect(wrapper.get('[data-test="partner-attach-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="partner-attach-agent-pick"]').trigger('click')
    await flushPromises()
    expect(api.previewAttach).toHaveBeenCalledWith(75, 62, expect.anything())
    expect(wrapper.get('[data-test="partner-preview"]').text()).toContain('admin.partners.preview.team{"owned":6,"viaUpline":3}')

    await wrapper.get('[data-test="partner-attach-submit"]').trigger('click')
    expect(api.attachAgent).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="confirm-message"]').text()).toContain('"region":"三科大"')
    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(api.attachAgent).toHaveBeenCalledWith(75, 62)
    expect(wrapper.emitted('attached')).toHaveLength(1)
  })

  it('keeps attach disabled with the reason when the preview refuses', async () => {
    api.previewAttach.mockResolvedValue({ agent: USER, agent_level: 3, team: { owned: 0, via_upline: 0 }, current_region: null, can_attach: false, reason: '该用户不是渠道经理' })
    const wrapper = mount(PartnerAttachDialog, { props: { show: true, partnerId: 75, regionName: '三科大' }, global: { stubs } })
    await wrapper.get('[data-test="partner-attach-agent-pick"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="partner-preview-reason"]').text()).toBe('该用户不是渠道经理')
    expect(wrapper.get('[data-test="partner-attach-submit"]').attributes('disabled')).toBeDefined()
  })
})
