import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'

const { copyToClipboard, getAffiliateDetail, setInviteeAgentLevel, showError, showSuccess } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getAffiliateDetail: vi.fn(),
  setInviteeAgentLevel: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota: vi.fn(),
    setInviteeAgentLevel,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      // Echo params so tests can assert the generated copy without locale files.
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}${JSON.stringify(params)}` : key,
    }),
  }
})

const ConfirmDialogStub = {
  props: ['show', 'title', 'message'],
  emits: ['confirm', 'cancel'],
  template: `<div v-if="show" data-test="confirm-dialog"><p data-test="confirm-message">{{ message }}</p><button type="button" data-test="confirm-ok" @click="$emit('confirm')">ok</button></div>`,
}

function mountView() {
  return mount(AffiliateView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true,
        ConfirmDialog: ConfirmDialogStub,
      },
    },
  })
}

function baseDetail(overrides: Record<string, unknown> = {}) {
  return {
    user_id: 1,
    aff_code: 'CODE',
    inviter_id: null,
    aff_count: 0,
    aff_quota: 0,
    aff_frozen_quota: 0,
    aff_history_quota: 0,
    rebate_base: 'consumption',
    agent_level: 3,
    agent_level_locked: false,
    agent_upgrade_mode: 'manual',
    current_invite_count: 0,
    owned_customer_count: 0,
    reward_totals: { agent: 0, invite: 0, invitee_bonus: 0 },
    invitees: [],
    ...overrides,
  }
}

describe('AffiliateView', () => {
  const affiliateCode = 'affiliate-code-that-is-long-enough-to-overflow-a-mobile-viewport'

  beforeEach(() => {
    vi.clearAllMocks()
    copyToClipboard.mockResolvedValue(true)
    getAffiliateDetail.mockResolvedValue(baseDetail({ aff_code: affiliateCode, agent_level: 2, agent_rate_percent: 10 }))
  })

  it('stacks long values and copy controls on mobile while retaining desktop rows', async () => {
    const wrapper = mountView()

    await flushPromises()

    const values = wrapper.findAll('code')
    expect(values).toHaveLength(2)
    for (const value of values) {
      expect(value.classes()).toEqual(expect.arrayContaining([
        'min-w-0',
        'break-all',
        'sm:flex-1',
        'sm:truncate',
      ]))
      expect(Array.from(value.element.parentElement?.classList ?? [])).toEqual(expect.arrayContaining([
        'flex-col',
        'items-stretch',
        'sm:flex-row',
        'sm:items-center',
      ]))
    }

    const copyButtons = wrapper.findAll('button').filter((button) =>
      ['affiliate.copyCode', 'affiliate.copyLink'].includes(button.text()),
    )
    expect(copyButtons).toHaveLength(2)
    for (const button of copyButtons) {
      expect(button.classes()).toEqual(expect.arrayContaining([
        'w-full',
        'sm:w-auto',
        'sm:shrink-0',
      ]))
    }

    await copyButtons[0].trigger('click')
    await copyButtons[1].trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenNthCalledWith(1, affiliateCode, 'affiliate.codeCopied')
    expect(copyToClipboard).toHaveBeenNthCalledWith(
      2,
      `${window.location.origin}/register?aff=${encodeURIComponent(affiliateCode)}`,
      'affiliate.linkCopied',
    )
  })

  it('shows a normal user the generated invite rewards and auto-upgrade progress, never agent rates', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({
      agent_upgrade_mode: 'auto',
      next_agent_level: 2,
      next_level_threshold: 5,
      current_invite_count: 2,
      invite_reward: { mode: 'first_recharge', rate_percent: 0, duration_days: 0, per_invitee_cap: 0, fixed_amount: 2, min_recharge: 10 },
      invitee_bonus: { mode: 'percent', value: 10, min_recharge: 0, max: 0 },
      invitees: [{ user_id: 9, email: 'b***@x.com', username: 'b', created_at: '2026-10-01T00:00:00Z', total_rebate: 2, owned_by_me: false }],
    }))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="affiliate-agent-card"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="affiliate-agent-rate"]').exists()).toBe(false)
    const inviteText = wrapper.get('[data-test="affiliate-invite-text"]').text()
    expect(inviteText).toContain('affiliate.invite.rewardFirstRecharge{"min":"¥10","amount":"¥2"}')
    expect(inviteText).toContain('affiliate.invite.separator')
    expect(inviteText).toContain('affiliate.invite.inviteeBonus{"bonus":"10%"}')

    const progress = wrapper.get('[data-test="affiliate-become-agent"] [data-test="affiliate-upgrade-progress"]').text()
    expect(progress).toContain('affiliate.becomeAgent.progress')
    expect(progress).toContain('"remaining":3')
    expect(progress).toContain('"level":"affiliate.levels.2"')

    // Normal users get neither the ownership column nor the appoint column.
    expect(wrapper.find('[data-test="affiliate-col-ownership"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="affiliate-col-agent-role"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('affiliate.agent.rate')
  })

  it('tells a normal user agent status is admin-granted in manual mode or when locked', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({
      agent_upgrade_mode: 'auto',
      agent_level_locked: true,
      next_agent_level: 2,
      next_level_threshold: 5,
    }))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="affiliate-become-agent"]').text()).toContain('affiliate.becomeAgent.adminOnly')
    expect(wrapper.get('[data-test="affiliate-invite-text"]').text()).toBe('affiliate.invite.noReward')
  })

  it('shows an agent its rate, customers, split totals and ownership, but not the inviter reward', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({
      agent_level: 2,
      agent_rate_percent: 20,
      owned_customer_count: 15,
      agent_upgrade_mode: 'auto',
      next_agent_level: 1,
      next_level_threshold: 20,
      current_invite_count: 8,
      aff_history_quota: 96,
      reward_totals: { agent: 90, invite: 6, invitee_bonus: 0 },
      // Even if the backend sent it, an agent never sees the inviter reward.
      invite_reward: { mode: 'first_recharge', rate_percent: 0, duration_days: 0, per_invitee_cap: 0, fixed_amount: 2, min_recharge: 10 },
      invitee_bonus: { mode: 'percent', value: 10, min_recharge: 10, max: 20 },
      invitees: [
        { user_id: 9, email: 'b***@x.com', username: 'b', total_rebate: 12, owned_by_me: true },
        { user_id: 10, email: 'c***@x.com', username: 'c', total_rebate: 0, owned_by_me: false },
      ],
    }))

    const wrapper = mountView()
    await flushPromises()

    const card = wrapper.get('[data-test="affiliate-agent-card"]')
    expect(card.get('[data-test="affiliate-agent-level"]').text()).toBe('affiliate.levels.2')
    expect(card.get('[data-test="affiliate-agent-rate"]').text()).toBe('affiliate.agent.rateConsumption{"rate":"20"}')
    expect(card.get('[data-test="affiliate-owned-customers"]').text()).toBe('affiliate.agent.customers{"n":15}')
    expect(card.get('[data-test="affiliate-upgrade-progress"]').text()).toContain('affiliate.agent.progress')
    expect(wrapper.find('[data-test="affiliate-become-agent"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="affiliate-total-breakdown"]').text()).toContain('affiliate.stats.totalBreakdown')

    const inviteText = wrapper.get('[data-test="affiliate-invite-text"]').text()
    expect(inviteText).not.toContain('affiliate.invite.rewardFirstRecharge')
    expect(inviteText).toContain('affiliate.invite.inviteeBonusWithMin')

    const ownership = wrapper.findAll('[data-test="affiliate-invitee-ownership"]').map((cell) => cell.text())
    expect(ownership).toEqual(['affiliate.invitees.ownedByMe', 'affiliate.invitees.ownedByOther'])
    // Level-2 agents have no appoint column.
    expect(wrapper.find('[data-test="affiliate-col-agent-role"]').exists()).toBe(false)
  })

  it('lets a level-1 agent appoint and revoke level-2 agents with confirmations', async () => {
    const level1 = baseDetail({
      agent_level: 1,
      agent_rate_percent: 30,
      appoint: { max: 0, used: 1 },
      invitees: [
        { user_id: 9, email: 'b***@x.com', username: 'b', total_rebate: 12, owned_by_me: true, agent_level: 2, appointed_by_me: true, appointable: false },
        { user_id: 10, email: 'c***@x.com', username: 'c', total_rebate: 3, owned_by_me: true, agent_level: 3, appointed_by_me: false, appointable: true },
        { user_id: 11, email: 'd***@x.com', username: 'd', total_rebate: 0, owned_by_me: false, agent_level: 3, appointed_by_me: false, appointable: false },
      ],
    })
    getAffiliateDetail.mockResolvedValue(level1)
    setInviteeAgentLevel.mockResolvedValue({ user_id: 10, agent_level: 2 })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="affiliate-appoint-summary"]').text()).toBe(
      'affiliate.invitees.appointSummary{"used":1,"max":"affiliate.invitees.unlimited"}',
    )
    expect(wrapper.find('[data-test="affiliate-col-agent-role"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="affiliate-invitee-9"]').find('[data-test="affiliate-revoke"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="affiliate-invitee-11"]').find('[data-test="affiliate-not-appointable"]').exists()).toBe(true)

    const appointButton = wrapper.get('[data-test="affiliate-invitee-10"] [data-test="affiliate-appoint"]')
    expect(appointButton.attributes('disabled')).toBeUndefined()
    await appointButton.trigger('click')
    expect(wrapper.get('[data-test="confirm-message"]').text()).toBe('affiliate.invitees.appointConfirm')
    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(setInviteeAgentLevel).toHaveBeenCalledWith(10, 2)
    expect(showSuccess).toHaveBeenCalledWith('affiliate.invitees.appointSuccess')

    await wrapper.get('[data-test="affiliate-invitee-9"] [data-test="affiliate-revoke"]').trigger('click')
    expect(wrapper.get('[data-test="confirm-message"]').text()).toBe('affiliate.invitees.revokeConfirm')
    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(setInviteeAgentLevel).toHaveBeenLastCalledWith(9, 3)
  })

  it('greys out appointing at the limit', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({
      agent_level: 1,
      agent_rate_percent: 30,
      appoint: { max: 2, used: 2 },
      invitees: [
        { user_id: 10, email: 'c***@x.com', username: 'c', total_rebate: 3, owned_by_me: true, agent_level: 3, appointed_by_me: false, appointable: true },
      ],
    }))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="affiliate-appoint-summary"]').text()).toContain('"max":2')
    expect(wrapper.find('[data-test="affiliate-appoint-limit"]').exists()).toBe(true)
    const button = wrapper.get('[data-test="affiliate-appoint"]')
    expect(button.attributes('disabled')).toBeDefined()
    expect(button.attributes('title')).toBe('affiliate.invitees.limitReached')
  })
})
