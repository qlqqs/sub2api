import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import FinanceAccountCostSettings from '../FinanceAccountCostSettings.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { emptyStatsPricing } from '@/components/admin/channel/accountStatsPricing'
const api = vi.hoisted(() => ({
  accounts: { list: vi.fn(), getById: vi.fn(), update: vi.fn() },
  finance: { getSettings: vi.fn(), updateSettings: vi.fn() },
  channels: { list: vi.fn(), update: vi.fn() },
  groups: { getAllIncludingInactive: vi.fn() },
  accountModelCost: { listAll: vi.fn(), getForAccount: vi.fn(), replaceForAccount: vi.fn() }
}))
const costs = vi.hoisted(() => ({ getChannelCostConfig: vi.fn(), updateChannelCostConfig: vi.fn() }))
const toast = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: api }))
vi.mock('@/api/admin/financeCostConfig', () => costs)
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string, p?: Record<string, unknown>) => p ? `${key}:${JSON.stringify(p)}` : key }) }))
enableAutoUnmount(afterEach)
const account = { id: 1, name: 'Cost account', platform: 'openai', group_ids: [5], rate_multiplier: 0.5 }
const rules = [{ id: 8, name: 'Shared rule', group_ids: [5], account_ids: [2], pricing: [{ ...emptyStatsPricing('openai'), models: ['gpt-*'], cache_write_1h_price: 0.000002, reasoning_effort_multipliers: { high: 2 } }] }]
const channel = { id: 3, name: 'Shared channel', group_ids: [5], apply_pricing_to_account_stats: false, account_stats_pricing_rules: rules }
function config() { return JSON.parse(JSON.stringify({ channel_id: 3, version: 'v1', apply_pricing_to_account_stats: false, account_stats_pricing_rules: rules })) }
function mountSettings() { return mount(FinanceAccountCostSettings, { props: { refreshToken: 0 }, global: { stubs: { teleport: true } } }) }
async function openAccount() { const wrapper = mountSettings(); await flushPromises(); await wrapper.get('[data-testid="finance-account-edit-1"]').trigger('click'); await flushPromises(); return wrapper }
async function openChannel() { const wrapper = mountSettings(); await flushPromises(); await wrapper.get('[data-testid="finance-channel-edit-3"]').trigger('click'); await flushPromises(); return wrapper }
beforeEach(() => {
  vi.clearAllMocks()
  api.accounts.list.mockResolvedValue({ items: [account, { ...account, id: 2, name: 'Second account' }], total: 2 })
  api.accounts.getById.mockResolvedValue({ ...account })
  api.accounts.update.mockImplementation(async (_id, body) => ({ ...account, ...body }))
  api.finance.getSettings.mockResolvedValue({ fixed_cost_account_ids: [], accounts: [account] })
  api.finance.updateSettings.mockImplementation(async body => ({ ...body, accounts: [account] }))
  api.channels.list.mockResolvedValue({ items: [channel], total: 1 })
  api.groups.getAllIncludingInactive.mockResolvedValue([{ id: 5, name: 'Shared group', platform: 'openai' }])
  api.accountModelCost.listAll.mockResolvedValue({})
  api.accountModelCost.getForAccount.mockResolvedValue([])
  api.accountModelCost.replaceForAccount.mockImplementation(async (_id, body) => body)
  costs.getChannelCostConfig.mockResolvedValue(config())
  costs.updateChannelCostConfig.mockImplementation(async (_id, body) => ({ ...body, channel_id: 3, version: 'v2' }))
})
describe('FinanceAccountCostSettings', () => {
  it('shows all shared impacts and conditionally writes ONLY cost config, keeping supported advanced data', async () => {
    const wrapper = await openChannel()
    expect(wrapper.get('[data-testid="finance-channel-impact"]').text()).toContain('Second account')
    expect(wrapper.get('[data-testid="finance-channel-impact"]').text()).toContain('Shared group')
    await wrapper.get('[data-testid="stats-rule"] input').setValue('Renamed')
    await wrapper.get('[data-testid="finance-channel-save"]').trigger('click'); await flushPromises()
    expect(costs.updateChannelCostConfig).toHaveBeenCalledWith(3, {
      expected_version: 'v1', apply_pricing_to_account_stats: false,
      account_stats_pricing_rules: [{ ...rules[0], name: 'Renamed' }]
    })
    expect(api.channels.update).not.toHaveBeenCalled()
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })
  it.each([
    { fast_multiplier: 2 },
    { flex_multiplier: 0.5 },
    { image_input_price: 0 }
  ])('retains unsupported raw pricing %j when the server rejects it, without a success or fallback write', async unsupported => {
    const loaded = config()
    Object.assign(loaded.account_stats_pricing_rules[0].pricing[0], unsupported)
    costs.getChannelCostConfig.mockResolvedValueOnce(loaded)
    costs.updateChannelCostConfig.mockRejectedValueOnce({
      status: 400, reason: 'FINANCE_COST_FIELD_UNSUPPORTED', message: 'Unsupported cost pricing field'
    })
    const wrapper = await openChannel()
    expect(wrapper.get('[data-testid="stats-pricing-support-warning"]').text()).toBe('admin.finance.costSettings.advancedSupport')
    await wrapper.get('[data-testid="stats-rule"] input').setValue('Keep my draft')
    await wrapper.get('[data-testid="finance-channel-save"]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('Unsupported cost pricing field')
    expect(costs.updateChannelCostConfig.mock.calls[0][1].account_stats_pricing_rules[0].pricing[0]).toMatchObject(unsupported)
    expect((wrapper.get('[data-testid="stats-rule"] input').element as HTMLInputElement).value).toBe('Keep my draft')
    expect(wrapper.emitted('changed')).toBeUndefined()
    expect(toast.showSuccess).not.toHaveBeenCalled()
    expect(api.channels.update).not.toHaveBeenCalled()
  })
  it('rejects stale shared saves and keeps draft with reload guidance', async () => {
    costs.updateChannelCostConfig.mockRejectedValueOnce({ status: 409, reason: 'FINANCE_COST_CONFIG_CHANGED' })
    const wrapper = await openChannel()
    await wrapper.get('[data-testid="stats-rule"] input').setValue('My draft')
    await wrapper.get('[data-testid="finance-channel-save"]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('admin.finance.costSettings.conflict')
    expect(wrapper.get('[data-testid="finance-channel-save"]').attributes('disabled')).toBeDefined()
    expect((wrapper.get('[data-testid="stats-rule"] input').element as HTMLInputElement).value).toBe('My draft')
    expect(wrapper.emitted('changed')).toBeUndefined()
    expect(api.channels.update).not.toHaveBeenCalled()
  })
  it('saves default zero independently and does not send credentials or other account properties', async () => {
    const wrapper = await openAccount()
    await wrapper.get('[data-testid="finance-default-rate"]').setValue('0')
    await wrapper.get('[data-testid="finance-default-save"]').trigger('click'); await flushPromises()
    expect(api.accounts.update).toHaveBeenCalledWith(1, { rate_multiplier: 0 })
    expect(api.finance.updateSettings).not.toHaveBeenCalled()
    expect(api.accountModelCost.replaceForAccount).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.finance.costSettings.schedulingHint')
  })
  it('reports a model save failure without rolling back or claiming other sections saved', async () => {
    const wrapper = await openAccount()
    await wrapper.get('[data-testid="finance-default-rate"]').setValue('0.8')
    await wrapper.get('[data-testid="finance-default-save"]').trigger('click'); await flushPromises()
    await wrapper.get('[data-testid="account-model-cost-add"]').trigger('click')
    await wrapper.get('[data-testid="account-model-cost-model"]').setValue('gpt-*')
    await wrapper.get('[data-testid="account-model-cost-rate"]').setValue('0.2')
    api.accountModelCost.replaceForAccount.mockRejectedValueOnce(new Error('model save down'))
    await wrapper.get('[data-testid="finance-model-save"]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('model save down')
    expect(wrapper.text()).toContain('admin.finance.costSettings.partSaved')
    expect(api.accounts.update).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('changed')).toHaveLength(1)
    expect((wrapper.get('[data-testid="account-model-cost-rate"]').element as HTMLInputElement).value).toBe('0.2')
    expect(wrapper.get('[data-testid="finance-model-save"]').attributes('disabled')).toBeUndefined()
  })
  it('merges the fixed flag into a fresh full list without clearing other IDs', async () => {
    const wrapper = await openAccount()
    api.finance.getSettings.mockResolvedValue({ fixed_cost_account_ids: [90], accounts: [] })
    await wrapper.get('[data-testid="finance-fixed-cost"]').setValue(true)
    await wrapper.get('[data-testid="finance-fixed-save"]').trigger('click'); await flushPromises()
    expect(api.finance.updateSettings).toHaveBeenCalledWith({ fixed_cost_account_ids: [1, 90] })
    expect(api.accounts.update).not.toHaveBeenCalled()
  })
  it('does not write drafts on cancel and asks before discarding', async () => {
    const wrapper = await openAccount()
    await wrapper.get('[data-testid="finance-default-rate"]').setValue('9')
    await wrapper.findAll('button').find(b => b.text() === 'common.close')!.trigger('click')
    expect(wrapper.text()).toContain('admin.finance.costSettings.discardHint')
    await wrapper.findAll('button').find(b => b.text() === 'admin.finance.costSettings.discard')!.trigger('click')
    await flushPromises()
    expect(api.accounts.update).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="finance-default-rate"]').exists()).toBe(false)
  })
  it('keeps unsaved model drafts while another section saves or fails', async () => {
    const wrapper = await openAccount()
    await wrapper.get('[data-testid="account-model-cost-add"]').trigger('click')
    await wrapper.get('[data-testid="account-model-cost-model"]').setValue('gpt-*')
    await wrapper.get('[data-testid="account-model-cost-rate"]').setValue('0.3')
    await wrapper.get('[data-testid="finance-default-rate"]').setValue('0.7')
    api.accounts.update.mockRejectedValueOnce(new Error('default unavailable'))
    await wrapper.get('[data-testid="finance-default-save"]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('default unavailable')
    expect((wrapper.get('[data-testid="account-model-cost-rate"]').element as HTMLInputElement).value).toBe('0.3')
    expect(api.accountModelCost.replaceForAccount).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="finance-default-save"]').trigger('click'); await flushPromises()
    expect((wrapper.get('[data-testid="account-model-cost-rate"]').element as HTMLInputElement).value).toBe('0.3')
    expect(api.accountModelCost.getForAccount).toHaveBeenCalledTimes(1)
  })
  it('fails closed when versioned channel configuration is unavailable', async () => {
    costs.getChannelCostConfig.mockRejectedValueOnce(new Error('cost endpoint unavailable'))
    const wrapper = await openChannel()
    expect(toast.showError).toHaveBeenCalledWith('cost endpoint unavailable')
    expect(wrapper.find('[data-testid="finance-channel-save"]').exists()).toBe(false)
    expect(api.channels.update).not.toHaveBeenCalled()
  })
  it('shows initial loading failures and can retry without any writes', async () => {
    api.accounts.list.mockRejectedValueOnce(new Error('list unavailable'))
    const wrapper = mountSettings(); await flushPromises()
    expect(wrapper.get('[data-testid="finance-load-error"]').text()).toContain('list unavailable')
    await wrapper.get('[data-testid="finance-load-error"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[data-testid="finance-account-edit-1"]').exists()).toBe(true)
    expect(api.accounts.update).not.toHaveBeenCalled()
  })

  it('keeps the default multiplier readonly when the original account editor delegates it to upstream sync', async () => {
    api.accounts.getById.mockResolvedValue({ ...account, type: 'apikey', extra: { upstream_billing_probe_enabled: true, upstream_billing_rate_sync_enabled: true } })
    const wrapper = await openAccount()
    expect(wrapper.get('[data-testid="finance-default-rate"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="finance-default-save"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('admin.accounts.upstreamBilling.syncRateManagedHint')
    expect(api.accounts.update).not.toHaveBeenCalled()
  })

  it('mounts discard confirmation only when needed and Escape cancels confirmation without discarding the draft', async () => {
    const wrapper = await openAccount()
    expect(wrapper.findComponent(ConfirmDialog).exists()).toBe(false)
    await wrapper.get('[data-testid="finance-default-rate"]').setValue('9')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(wrapper.findComponent(ConfirmDialog).exists()).toBe(true)
    expect(wrapper.text()).toContain('admin.finance.costSettings.discardHint')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(wrapper.findComponent(ConfirmDialog).exists()).toBe(false)
    expect((wrapper.get('[data-testid="finance-default-rate"]').element as HTMLInputElement).value).toBe('9')
    expect(api.accounts.update).not.toHaveBeenCalled()
  })

})
