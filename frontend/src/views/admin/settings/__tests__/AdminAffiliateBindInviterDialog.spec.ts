import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'

const { lookupUsers, bindInviter } = vi.hoisted(() => ({ lookupUsers: vi.fn(), bindInviter: vi.fn() }))

vi.mock('@/api/admin/affiliates', () => ({
  affiliatesAPI: { lookupUsers, bindInviter },
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

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

})
