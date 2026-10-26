import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ChannelsView from '../ChannelsView.vue'
import { emptyStatsPricing } from '@/components/admin/channel/accountStatsPricing'
const api = vi.hoisted(() => ({ channels: { list: vi.fn(), update: vi.fn() }, groups: { getAll: vi.fn() }, settings: { getWebSearchEmulationConfig: vi.fn() } }))
vi.mock('@/api/admin', () => ({ adminAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
const rules = [
  { id: 20, name: 'Cross-platform first', group_ids: [11, 12], account_ids: [90], pricing: [
    { ...emptyStatsPricing('gemini'), models: ['gemini-*'], billing_mode: 'image', per_request_price: 0, fast_multiplier: 2, flex_multiplier: 0.5, reasoning_effort_multipliers: { high: 3 } },
    { ...emptyStatsPricing('openai'), models: ['gpt-*'], input_price: 0.000001 }
  ] },
  { id: 21, name: 'Second, account-only', group_ids: [], account_ids: [91], pricing: [{ ...emptyStatsPricing('openai'), models: ['gpt-5'] }] }
]
beforeEach(() => {
  vi.clearAllMocks()
  api.channels.list.mockResolvedValue({ items: [{ id: 1, name: 'Original channel', description: '', status: 'active', group_ids: [11, 12], model_pricing: [], model_mapping: {}, account_stats_pricing_rules: rules, apply_pricing_to_account_stats: true }], total: 1 })
  api.groups.getAll.mockResolvedValue([{ id: 11, name: 'OpenAI', platform: 'openai' }, { id: 12, name: 'Gemini', platform: 'gemini' }])
  api.settings.getWebSearchEmulationConfig.mockResolvedValue({ enabled: false })
  api.channels.update.mockResolvedValue({})
})
it('round-trips the same ordered cross-platform cost rules through the original channel entrance', async () => {
  const wrapper = mount(ChannelsView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' }, TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/><slot name="pagination"/></div>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' },
    DataTable: { props: ['data'], template: '<div><slot v-for="row in data" name="cell-actions" :row="row" /></div>' },
    PricingEntryCard: true, Select: true, Pagination: true, ConfirmDialog: true, PlatformIcon: true, Toggle: true, Icon: true
  } } })
  await flushPromises()
  await wrapper.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click'); await flushPromises()
  expect(wrapper.findAll('[data-testid="stats-rule"]')).toHaveLength(2)
  await wrapper.get('#channel-form').trigger('submit'); await flushPromises()
  expect(api.channels.update).toHaveBeenCalledOnce()
  expect(api.channels.update.mock.calls[0][1].account_stats_pricing_rules).toEqual(rules)
  expect(api.channels.update.mock.calls[0][1].apply_pricing_to_account_stats).toBe(true)
})
