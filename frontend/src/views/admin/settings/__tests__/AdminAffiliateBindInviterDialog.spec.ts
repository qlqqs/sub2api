import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'

const { lookupUsers, bindInviter, previewInviter } = vi.hoisted(() => ({
  lookupUsers: vi.fn(),
  bindInviter: vi.fn(),
  previewInviter: vi.fn(),
}))

vi.mock('@/api/admin/affiliates', () => ({
  affiliatesAPI: { lookupUsers, bindInviter, previewInviter },
}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

import AdminAffiliateBindInviterDialog from '../AdminAffiliateBindInviterDialog.vue'

const BaseDialogStub = defineComponent({
  props: { show: Boolean, title: String },
  emits: ['close'],
  template: '<div v-if="show"><slot/><div><slot name="footer"/></div></div>',
})

async function search(wrapper: ReturnType<typeof mount>, selector: string, query: string) {
  await wrapper.get(selector).setValue(query)
  await wrapper.get(selector).trigger('input')
  vi.advanceTimersByTime(300)
  await flushPromises()
}

describe('AdminAffiliateBindInviterDialog', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    lookupUsers.mockReset()
    bindInviter.mockReset()
    previewInviter.mockReset()
    previewInviter.mockResolvedValue({
      agent_owner_id: null,
      agent_owner_email: '',
      agent_owner_level: null,
      agent_upline_id: null,
      agent_upline_email: '',
    })
  })
  afterEach(() => vi.useRealTimers())

  it('requires a reason, excludes self-invitation, and submits only once while saving', async () => {
    lookupUsers
      .mockResolvedValueOnce([{ id: 1, email: 'invitee@example.com', username: 'invitee' }])
      .mockResolvedValueOnce([
        { id: 1, email: 'invitee@example.com', username: 'invitee' },
        { id: 2, email: 'inviter@example.com', username: 'inviter' },
      ])
    let resolveBind!: () => void
    bindInviter.mockReturnValue(new Promise<void>((resolve) => { resolveBind = resolve }))

    const wrapper = mount(AdminAffiliateBindInviterDialog, {
      props: { show: true },
      global: { stubs: { BaseDialog: BaseDialogStub } },
    })

    await search(wrapper, '#affiliate-bind-invitee', 'invitee')
    await wrapper.get('button').trigger('click')
    await search(wrapper, '#affiliate-bind-inviter', 'inviter')
    expect(wrapper.findAll('button').filter(button => button.text().includes('invitee@example.com'))).toHaveLength(0)
    const inviterButton = wrapper.findAll('button').find(button => button.text().includes('inviter@example.com'))
    expect(inviterButton).toBeDefined()
    await inviterButton!.trigger('click')

    const submit = wrapper.findAll('button').find(button => button.text().includes('affiliate.bind.submit'))!
    expect(submit.attributes('disabled')).toBeDefined()
    await wrapper.get('#affiliate-bind-reason').setValue('support verified')
    await submit.trigger('click')
    await submit.trigger('click')
    expect(bindInviter).toHaveBeenCalledTimes(1)
    expect(bindInviter).toHaveBeenCalledWith(1, { inviter_id: 2, reason: 'support verified' })

    resolveBind()
    await flushPromises()
    expect(wrapper.emitted('success')).toHaveLength(1)
  })
  it('ignores stale user-search responses', async () => {
    let resolveFirst!: (users: Array<{ id: number; email: string; username: string }>) => void
    lookupUsers
      .mockReturnValueOnce(new Promise(resolve => { resolveFirst = resolve }))
      .mockResolvedValueOnce([{ id: 2, email: 'new@example.com', username: 'new' }])

    const wrapper = mount(AdminAffiliateBindInviterDialog, {
      props: { show: true },
      global: { stubs: { BaseDialog: BaseDialogStub } },
    })

    await search(wrapper, '#affiliate-bind-invitee', 'old')
    await search(wrapper, '#affiliate-bind-invitee', 'new')
    expect(wrapper.text()).toContain('new@example.com')

    resolveFirst([{ id: 1, email: 'old@example.com', username: 'old' }])
    await flushPromises()
    expect(wrapper.text()).toContain('new@example.com')
    expect(wrapper.text()).not.toContain('old@example.com')
  })

  it('shows a safe backend conflict message', async () => {
    lookupUsers
      .mockResolvedValueOnce([{ id: 1, email: 'invitee@example.com', username: 'invitee' }])
      .mockResolvedValueOnce([{ id: 2, email: 'inviter@example.com', username: 'inviter' }])
    bindInviter.mockRejectedValue({ status: 409, reason: 'AFFILIATE_ALREADY_BOUND', message: 'affiliate inviter already bound' })

    const wrapper = mount(AdminAffiliateBindInviterDialog, {
      props: { show: true },
      global: { stubs: { BaseDialog: BaseDialogStub } },
    })
    await search(wrapper, '#affiliate-bind-invitee', 'invitee')
    await wrapper.findAll('button').find(button => button.text().includes('invitee@example.com'))!.trigger('click')
    await search(wrapper, '#affiliate-bind-inviter', 'inviter')
    await wrapper.findAll('button').find(button => button.text().includes('inviter@example.com'))!.trigger('click')
    await wrapper.get('#affiliate-bind-reason').setValue('support verified')
    await wrapper.findAll('button').find(button => button.text().includes('affiliate.bind.submit'))!.trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toBe('affiliate inviter already bound')
    expect(wrapper.emitted('success')).toBeUndefined()
  })

  it('previews the ownership snapshot before confirming and blocks binding when the preview fails', async () => {
    lookupUsers
      .mockResolvedValueOnce([{ id: 1, email: 'invitee@example.com', username: 'invitee' }])
      .mockResolvedValueOnce([{ id: 2, email: 'inviter@example.com', username: 'inviter' }])
    previewInviter.mockResolvedValueOnce({
      agent_owner_id: 7,
      agent_owner_email: 'b@x.com',
      agent_owner_level: 2,
      agent_upline_id: 5,
      agent_upline_email: 'a@x.com',
    })

    const wrapper = mount(AdminAffiliateBindInviterDialog, {
      props: { show: true },
      global: { stubs: { BaseDialog: BaseDialogStub } },
    })
    await search(wrapper, '#affiliate-bind-invitee', 'invitee')
    await wrapper.findAll('button').find(button => button.text().includes('invitee@example.com'))!.trigger('click')
    await search(wrapper, '#affiliate-bind-inviter', 'inviter')
    await wrapper.findAll('button').find(button => button.text().includes('inviter@example.com'))!.trigger('click')
    await flushPromises()

    expect(previewInviter).toHaveBeenCalledWith(1, 2, expect.objectContaining({ signal: expect.anything() }))
    const previewBox = wrapper.get('[data-test="affiliate-bind-preview"]')
    expect(previewBox.text()).toContain('admin.settings.features.affiliate.bind.preview:')
    expect(previewBox.text()).toContain('"owner":"b@x.com"')
    expect(previewBox.text()).toContain('"level":"admin.affiliates.levelShort.2"')
    expect(previewBox.text()).toContain('"upline":"a@x.com"')

    // Re-pick the inviter: the new preview fails with a cycle and submitting is blocked.
    previewInviter.mockRejectedValueOnce({ status: 400, reason: 'AFFILIATE_CYCLE', message: 'affiliate cycle' })
    lookupUsers.mockResolvedValueOnce([{ id: 3, email: 'other@example.com', username: 'other' }])
    const clearButtons = wrapper.findAll('button').filter(button => button.text() === '×')
    await clearButtons[clearButtons.length - 1]!.trigger('click')
    await search(wrapper, '#affiliate-bind-inviter', 'other')
    await wrapper.findAll('button').find(button => button.text().includes('other@example.com'))!.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="affiliate-bind-preview"]').text()).toBe('affiliate cycle')
    await wrapper.get('#affiliate-bind-reason').setValue('support verified')
    const submit = wrapper.findAll('button').find(button => button.text().includes('affiliate.bind.submit'))!
    expect(submit.attributes('disabled')).toBeDefined()
    await submit.trigger('click')
    expect(bindInviter).not.toHaveBeenCalled()
  })
})
