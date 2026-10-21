import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'

const { copyToClipboard, getAffiliateDetail } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getAffiliateDetail: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
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
      t: (key: string) => key,
    }),
  }
})

describe('AffiliateView', () => {
  const affiliateCode = 'affiliate-code-that-is-long-enough-to-overflow-a-mobile-viewport'

  beforeEach(() => {
    vi.clearAllMocks()
    copyToClipboard.mockResolvedValue(true)
    getAffiliateDetail.mockResolvedValue({
      user_id: 1,
      aff_code: affiliateCode,
      inviter_id: null,
      aff_count: 0,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 10,
      rebate_levels: 1,
      level_rebate_rate_percents: [10],
      unlocked_layers: 1,
      rebate_base: 'consumption',
      agent_level: 2,
      agent_level_locked: false,
      agent_upgrade_mode: 'manual',
      current_invite_count: 0,
      invitees: [],
    })
  })

  it('stacks long values and copy controls on mobile while retaining desktop rows', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })

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

  it('shows the agent level, locked layers and auto-upgrade progress', async () => {
    getAffiliateDetail.mockResolvedValueOnce({
      user_id: 1,
      aff_code: affiliateCode,
      inviter_id: null,
      aff_count: 3,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 8,
      rebate_levels: 2,
      level_rebate_rate_percents: [8, 3],
      unlocked_layers: 1,
      rebate_base: 'consumption',
      agent_level: 2,
      agent_level_locked: false,
      agent_upgrade_mode: 'auto',
      next_agent_level: 1,
      next_level_threshold: 10,
      current_invite_count: 3,
      invitees: [],
    })

    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    const card = wrapper.find('[data-test="affiliate-agent-level"]')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('affiliate.stats.agentLevelValue')
    expect(card.text()).toContain('affiliate.stats.unlockedLayers')
    expect(wrapper.find('[data-test="affiliate-upgrade-progress"]').text()).toBe('affiliate.stats.upgradeProgress')

    const rates = wrapper.findAll('[data-test="affiliate-level-rates"] li')
    expect(rates).toHaveLength(2)
    expect(rates[0]!.classes()).not.toContain('line-through')
    expect(rates[1]!.classes()).toContain('line-through')
    expect(rates[1]!.text()).toContain('affiliate.stats.levelRateLocked')
  })

  it('tells the user the level is admin-managed when upgrades are manual or locked', async () => {
    getAffiliateDetail.mockResolvedValueOnce({
      user_id: 1,
      aff_code: affiliateCode,
      inviter_id: null,
      aff_count: 0,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 8,
      rebate_levels: 1,
      level_rebate_rate_percents: [8],
      unlocked_layers: 0,
      rebate_base: 'recharge',
      agent_level: 3,
      agent_level_locked: true,
      agent_upgrade_mode: 'auto',
      next_agent_level: 2,
      next_level_threshold: 5,
      current_invite_count: 0,
      invitees: [],
    })

    const wrapper = mount(AffiliateView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } },
    })
    await flushPromises()

    const card = wrapper.find('[data-test="affiliate-agent-level"]')
    expect(card.text()).toContain('affiliate.stats.agentLevelLocked')
    expect(card.text()).toContain('affiliate.stats.noUnlockedLayers')
    expect(wrapper.find('[data-test="affiliate-upgrade-progress"]').text()).toBe('affiliate.stats.upgradeManual')
    expect(wrapper.text()).toContain('affiliate.stats.rebateBaseRecharge')
  })
})
