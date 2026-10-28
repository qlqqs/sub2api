import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'

const { copyToClipboard, getAffiliateDetail, listAffiliateInvitees, setInviteeAgentLevel, showError, showSuccess } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getAffiliateDetail: vi.fn(),
  listAffiliateInvitees: vi.fn(),
  setInviteeAgentLevel: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    listAffiliateInvitees,
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
        RouterLink: RouterLinkStub,
        // 分区 Tab 依赖路由，单独测试（AffiliateSectionTabs.spec.ts）。
        AffiliateSectionTabs: { template: '<nav data-test="affiliate-section-tabs" />' },
      },
    },
  })
}

type InviteeFixture = Record<string, unknown> & { user_id: number }

/** listAffiliateInvitees 的分页响应。 */
function inviteePage(items: InviteeFixture[], total = items.length, page = 1) {
  return { items, total, page, page_size: 20, pages: Math.max(1, Math.ceil(total / 20)) }
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
    team_access: false,
    ...overrides,
  }
}

describe('AffiliateView', () => {
  const affiliateCode = 'affiliate-code-that-is-long-enough-to-overflow-a-mobile-viewport'

  beforeEach(() => {
    vi.clearAllMocks()
    copyToClipboard.mockResolvedValue(true)
    getAffiliateDetail.mockResolvedValue(baseDetail({ aff_code: affiliateCode, agent_level: 2, agent_rate_percent: 10 }))
    listAffiliateInvitees.mockResolvedValue(inviteePage([]))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it.each([1, 2, 3])('所有身份都提示好友注册赠送，且不依赖首充：%s', async (agent_level) => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({ agent_level, invitee_signup_bonus: 5 }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-test="affiliate-invite-text"]').text()).toContain('affiliate.invite.friendSignupBonus{"amount":"¥5"}')
  })

  it.each([0, undefined, -1])('关闭或缺省时不展示好友注册赠送：%s', async (invitee_signup_bonus) => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({ invitee_signup_bonus }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-test="affiliate-invite-text"]').text()).not.toContain('affiliate.invite.friendSignupBonus')
    expect(wrapper.find('[data-test="affiliate-signup-bonus-received"]').exists()).toBe(false)
  })

  it('已领注册赠送独立展示为余额，不混入返利余额', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({
      invitee_signup_bonus: 0,
      aff_quota: 2,
      reward_totals: { agent: 0, invite: 0, invitee_bonus: 3, invitee_signup_bonus: 5 },
    }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-test="affiliate-signup-bonus-received"]').text()).toContain('¥5.00')
    expect(wrapper.get('[data-test="affiliate-signup-bonus-received"]').text()).toContain('affiliate.stats.signupBonusReceivedHint')
    expect(wrapper.get('[data-test="affiliate-available-quota"]').text()).toBe('¥2.00')
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

  it('shows a normal user the generated invite rewards, but no become-agent card or agent rates', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({
      agent_upgrade_mode: 'auto',
      next_agent_level: 2,
      next_level_threshold: 5,
      current_invite_count: 2,
      invite_reward: { mode: 'first_recharge', rate_percent: 0, duration_days: 0, per_invitee_cap: 0, fixed_amount: 2, min_recharge: 10 },
      invitee_bonus: { mode: 'percent', value: 10, min_recharge: 0, max: 0 },
    }))
    listAffiliateInvitees.mockResolvedValueOnce(inviteePage([
      { user_id: 9, email: 'b***@x.com', username: 'b', created_at: '2026-10-01T00:00:00Z', total_rebate: 2, owned_by_me: false },
    ]))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="affiliate-agent-card"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="affiliate-agent-rate"]').exists()).toBe(false)
    const inviteText = wrapper.get('[data-test="affiliate-invite-text"]').text()
    // Different thresholds: one first-recharge clause, each side with its own threshold.
    expect(inviteText).toBe([
      'affiliate.invite.firstRecharge',
      'affiliate.invite.youGetMin{"min":"¥10","amount":"¥2"}',
      'affiliate.invite.friendGets{"bonus":"10%"}',
    ].join('affiliate.invite.clauseSeparator'))

    expect(wrapper.find('[data-test="affiliate-become-agent"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="affiliate-upgrade-progress"]').exists()).toBe(false)

    // Normal users get neither the ownership column nor the appoint column.
    expect(wrapper.find('[data-test="affiliate-col-ownership"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="affiliate-col-agent-role"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('affiliate.agent.rate')
  })

  it('puts a shared first-recharge threshold up front and keeps the percent reward as its own sentence', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({
      invite_reward: { mode: 'first_recharge', rate_percent: 0, duration_days: 0, per_invitee_cap: 0, fixed_amount: 0.5, min_recharge: 5 },
      invitee_bonus: { mode: 'fixed', value: 1, min_recharge: 5, max: 0 },
    }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-test="affiliate-invite-text"]').text()).toBe([
      'affiliate.invite.firstRechargeMin{"min":"¥5"}',
      'affiliate.invite.youGet{"amount":"¥0.50"}',
      'affiliate.invite.friendGets{"bonus":"¥1"}',
    ].join('affiliate.invite.clauseSeparator'))

    getAffiliateDetail.mockResolvedValueOnce(baseDetail({
      rebate_base: 'recharge',
      invite_reward: { mode: 'percent', rate_percent: 5, duration_days: 0, per_invitee_cap: 0, fixed_amount: 0, min_recharge: 0 },
      invitee_bonus: { mode: 'percent', value: 10, min_recharge: 0, max: 20 },
    }))
    const second = mountView()
    await flushPromises()
    expect(second.get('[data-test="affiliate-invite-text"]').text()).toBe(
      'affiliate.invite.rewardPercent{"base":"affiliate.invite.baseRecharge","rate":"5"}'
      + 'affiliate.invite.separator'
      + ['affiliate.invite.firstRecharge', 'affiliate.invite.friendGets{"bonus":"10%"}', 'affiliate.invite.friendCap{"max":"¥20"}']
        .join('affiliate.invite.clauseSeparator'),
    )
  })

  it('tells a normal user with no rewards configured to just share the invite', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail())

    const wrapper = mountView()
    await flushPromises()

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
    }))
    listAffiliateInvitees.mockResolvedValueOnce(inviteePage([
      { user_id: 9, email: 'b***@x.com', username: 'b', total_rebate: 12, owned_by_me: true },
      { user_id: 10, email: 'c***@x.com', username: 'c', total_rebate: 0, owned_by_me: false },
    ]))

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
    expect(inviteText).toBe([
      'affiliate.invite.firstRechargeMin{"min":"¥10"}',
      'affiliate.invite.friendGets{"bonus":"10%"}',
      'affiliate.invite.friendCap{"max":"¥20"}',
    ].join('affiliate.invite.clauseSeparator'))

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
    })
    getAffiliateDetail.mockResolvedValue(level1)
    listAffiliateInvitees.mockResolvedValue(inviteePage([
      { user_id: 9, email: 'b***@x.com', username: 'b', total_rebate: 12, owned_by_me: true, agent_level: 2, appointed_by_me: true, appointable: false },
      { user_id: 10, email: 'c***@x.com', username: 'c', total_rebate: 3, owned_by_me: true, agent_level: 3, appointed_by_me: false, appointable: true },
      { user_id: 11, email: 'd***@x.com', username: 'd', total_rebate: 0, owned_by_me: false, agent_level: 3, appointed_by_me: false, appointable: false },
    ]))
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
    // 任命后同时刷新详情（任命计数）与当前页列表（身份与按钮）。
    expect(getAffiliateDetail).toHaveBeenCalledTimes(2)
    expect(listAffiliateInvitees).toHaveBeenCalledTimes(2)
    expect(listAffiliateInvitees).toHaveBeenLastCalledWith(
      { search: undefined, page: 1, page_size: 20 },
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )

    await wrapper.get('[data-test="affiliate-invitee-9"] [data-test="affiliate-revoke"]').trigger('click')
    expect(wrapper.get('[data-test="confirm-message"]').text()).toBe('affiliate.invitees.revokeConfirm')
    await wrapper.get('[data-test="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(setInviteeAgentLevel).toHaveBeenLastCalledWith(9, 3)
  })

  it('splits a level-1 agent rate into direct customers and sub-agent teams', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({ agent_level: 1, agent_rate_percent: 11, team_rate_percent: 4 }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-test="affiliate-agent-rate"]').text()).toBe(
      'affiliate.agent.rateTopConsumption{"rate":"11","team":"4"}',
    )
  })

  it('greys out appointing at the limit', async () => {
    getAffiliateDetail.mockResolvedValueOnce(baseDetail({
      agent_level: 1,
      agent_rate_percent: 30,
      appoint: { max: 2, used: 2 },
    }))
    listAffiliateInvitees.mockResolvedValueOnce(inviteePage([
      { user_id: 10, email: 'c***@x.com', username: 'c', total_rebate: 3, owned_by_me: true, agent_level: 3, appointed_by_me: false, appointable: true },
    ]))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="affiliate-appoint-summary"]').text()).toContain('"max":2')
    expect(wrapper.find('[data-test="affiliate-appoint-limit"]').exists()).toBe(true)
    const button = wrapper.get('[data-test="affiliate-appoint"]')
    expect(button.attributes('disabled')).toBeDefined()
    expect(button.attributes('title')).toBe('affiliate.invitees.limitReached')
  })

  describe('section tabs', () => {
    it('shows the 邀请 | 团队消费 tabs as the first row only with team access, without the old entry links', async () => {
      getAffiliateDetail.mockResolvedValueOnce(baseDetail({ agent_level: 1, agent_rate_percent: 30, team_access: true }))
      const wrapper = mountView()
      await flushPromises()

      const firstRow = wrapper.find('main > div > *')
      expect(firstRow.attributes('data-test')).toBe('affiliate-section-tabs')
      expect(wrapper.find('[data-test="affiliate-team-entry"]').exists()).toBe(false)
      expect(wrapper.get('[data-test="affiliate-agent-card"]').findComponent(RouterLinkStub).exists()).toBe(false)
    })

    it('shows downgraded users the tabs but no extra team links', async () => {
      getAffiliateDetail.mockResolvedValueOnce(baseDetail({ agent_level: 3, team_access: true }))
      const wrapper = mountView()
      await flushPromises()

      expect(wrapper.find('[data-test="affiliate-section-tabs"]').exists()).toBe(true)
      expect(wrapper.find('[data-test="affiliate-become-agent"]').exists()).toBe(false)
      expect(wrapper.find('[data-test="affiliate-team-entry-downgraded"]').exists()).toBe(false)
    })

    it('hides the tabs when team access is false', async () => {
      getAffiliateDetail.mockResolvedValueOnce(baseDetail({ agent_level: 3, team_access: false }))
      const wrapper = mountView()
      await flushPromises()

      expect(wrapper.find('[data-test="affiliate-section-tabs"]').exists()).toBe(false)
    })
  })

  describe('invite list', () => {
    it('loads invitees from the paginated API and shows a separate nickname column', async () => {
      listAffiliateInvitees.mockResolvedValueOnce(inviteePage([
        { user_id: 9, email: 'b***@x.com', username: '小白', total_rebate: 2, owned_by_me: true },
        { user_id: 10, email: 'c***@x.com', username: '', total_rebate: 0, owned_by_me: true },
      ]))
      const wrapper = mountView()
      await flushPromises()

      expect(listAffiliateInvitees).toHaveBeenCalledWith(
        { search: undefined, page: 1, page_size: 20 },
        expect.objectContaining({ signal: expect.any(AbortSignal) }),
      )
      const headers = wrapper.findAll('[data-test="affiliate-invitees"] thead th').map((th) => th.text())
      expect(headers.slice(0, 3)).toEqual([
        'affiliate.invitees.columns.nickname',
        'affiliate.invitees.columns.email',
        'affiliate.invitees.columns.joinedAt',
      ])
      expect(wrapper.get('[data-test="affiliate-invitee-9"] [data-test="affiliate-invitee-nickname"]').text()).toBe('小白')
      expect(wrapper.get('[data-test="affiliate-invitee-10"] [data-test="affiliate-invitee-nickname"]').text()).toBe('-')
      expect(wrapper.get('[data-test="affiliate-invitee-9"]').text()).toContain('b***@x.com')
      // 只有一页：不显示分页。
      expect(wrapper.find('[data-test="affiliate-invitees"] nav[aria-label="Pagination"]').exists()).toBe(false)
    })

    it('searches after typing stops and resets to page 1', async () => {
      listAffiliateInvitees.mockResolvedValue(inviteePage(
        [{ user_id: 9, email: 'b***@x.com', username: 'b', total_rebate: 0, owned_by_me: true }],
        45,
      ))
      const wrapper = mountView()
      await flushPromises()

      await wrapper.findAll('[data-test="affiliate-invitees"] nav[aria-label="Pagination"] button')
        .find((button) => button.text() === '2')!
        .trigger('click')
      await flushPromises()
      expect(listAffiliateInvitees).toHaveBeenLastCalledWith({ search: undefined, page: 2, page_size: 20 }, expect.anything())

      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
      await wrapper.get('[data-test="affiliate-invitee-search"] input').setValue(' 客户12 ')
      expect(listAffiliateInvitees).toHaveBeenCalledTimes(2)
      vi.advanceTimersByTime(300)
      vi.useRealTimers()
      await flushPromises()

      expect(listAffiliateInvitees).toHaveBeenCalledTimes(3)
      expect(listAffiliateInvitees).toHaveBeenLastCalledWith({ search: '客户12', page: 1, page_size: 20 }, expect.anything())
    })

    it('pages through invitees with the shared pagination footer', async () => {
      listAffiliateInvitees
        .mockResolvedValueOnce(inviteePage([{ user_id: 9, email: 'b***@x.com', username: 'b', total_rebate: 0 }], 45))
        .mockResolvedValueOnce(inviteePage([{ user_id: 29, email: 'z***@x.com', username: 'z', total_rebate: 0 }], 45, 3))
      const wrapper = mountView()
      await flushPromises()

      const pager = wrapper.get('[data-test="affiliate-invitees"] nav[aria-label="Pagination"]')
      expect(wrapper.find('[data-test="affiliate-invitees"]').text()).not.toContain('pagination.perPage')
      await pager.findAll('button').find((button) => button.text() === '3')!.trigger('click')
      await flushPromises()

      expect(listAffiliateInvitees).toHaveBeenLastCalledWith({ search: undefined, page: 3, page_size: 20 }, expect.anything())
      expect(wrapper.find('[data-test="affiliate-invitee-29"]').exists()).toBe(true)
    })

    it('distinguishes no invitees from a search without matches', async () => {
      const wrapper = mountView()
      await flushPromises()
      expect(wrapper.get('[data-test="affiliate-invitees-empty"]').text()).toBe('affiliate.invitees.empty')
      // 没有任何好友时不显示搜索框。
      expect(wrapper.find('[data-test="affiliate-invitee-search"]').exists()).toBe(false)

      listAffiliateInvitees.mockResolvedValueOnce(inviteePage([{ user_id: 9, email: 'b***@x.com', username: 'b', total_rebate: 0 }]))
      const withFriends = mountView()
      await flushPromises()
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
      await withFriends.get('[data-test="affiliate-invitee-search"] input').setValue('nobody')
      vi.advanceTimersByTime(300)
      vi.useRealTimers()
      await flushPromises()

      expect(withFriends.get('[data-test="affiliate-invitees-empty"]').text()).toBe('affiliate.invitees.noMatches')
      expect(withFriends.find('[data-test="affiliate-invitee-search"]').exists()).toBe(true)
    })
  })
})
