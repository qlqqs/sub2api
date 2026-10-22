import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminAffiliateRecordsTable from '../AdminAffiliateRecordsTable.vue'

const { listInviteRecords, listRebateRecords, listTransferRecords, getUserOverview, getSettings } = vi.hoisted(() => ({
  listInviteRecords: vi.fn(),
  listRebateRecords: vi.fn(),
  listTransferRecords: vi.fn(),
  getUserOverview: vi.fn(),
  getSettings: vi.fn(),
}))

vi.mock('@/api/admin/settings', () => ({
  settingsAPI: { getSettings },
  default: { getSettings },
}))

vi.mock('@/api/admin/affiliates', () => {
  const affiliatesAPI = { listInviteRecords, listRebateRecords, listTransferRecords, getUserOverview }
  const AFFILIATE_REWARD_TYPES = ['agent_direct', 'agent_diff', 'invite_percent', 'invite_fixed', 'invitee_bonus']
  return { affiliatesAPI, AFFILIATE_REWARD_TYPES, default: affiliatesAPI }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
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

const TablePageLayoutStub = {
  template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
}

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <table>
      <tr v-for="(row, index) in data" :key="index" :data-test="'row-' + index">
        <td v-for="column in columns" :key="column.key" :data-test="'cell-' + column.key">
          <slot :name="'cell-' + column.key" :row="row" />
        </td>
      </tr>
    </table>
  `,
}

const WithdrawDialogStub = {
  name: 'AffiliateOfflineWithdrawDialog',
  props: ['show'],
  emits: ['close', 'success'],
  template: '<div data-test="withdraw-dialog" :data-show="String(show)" />',
}

function mountTable(type: 'invites' | 'rebates' | 'transfers') {
  return mount(AdminAffiliateRecordsTable, {
    props: { type },
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: true,
        Icon: true,
        OrderStatusBadge: true,
        BaseDialog: true,
        AffiliateOfflineWithdrawDialog: WithdrawDialogStub,
      },
    },
  })
}

function page<T>(items: T[]) {
  return { items, total: items.length, page: 1, page_size: 20, pages: 1 }
}

describe('AdminAffiliateRecordsTable', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    getSettings.mockResolvedValue({ affiliate_rebate_base: 'recharge' })
  })

  it('renders non-order rebate accruals with empty order fields', async () => {
    listRebateRecords.mockResolvedValue(page([
      {
        order_id: null,
        out_trade_no: '',
        inviter_id: 1,
        inviter_email: 'inviter@example.com',
        inviter_username: 'inviter',
        invitee_id: 2,
        invitee_email: 'invitee@example.com',
        invitee_username: 'invitee',
        order_amount: null,
        pay_amount: null,
        rebate_amount: 2,
        reward_type: 'agent_diff',
        payment_type: '',
        order_status: '',
        created_at: '2026-09-17T08:00:00Z',
      },
    ]))

    const wrapper = mountTable('rebates')
    await flushPromises()

    const row = wrapper.get('[data-test="row-0"]')
    expect(row.get('[data-test="cell-order"]').text()).toBe('-')
    expect(row.get('[data-test="cell-order_amount"]').text()).toBe('-')
    expect(row.get('[data-test="cell-pay_amount"]').text()).toBe('-')
    expect(row.get('[data-test="cell-payment_type"]').text()).toBe('-')
    expect(row.get('[data-test="cell-order_status"]').text()).toBe('-')
    expect(row.get('[data-test="cell-rebate_amount"]').text()).toBe('¥2.00')
    expect(row.get('[data-test="cell-reward_type"]').text()).toBe('admin.affiliates.rewardTypes.agent_diff')
    expect(wrapper.find('[data-test="cell-rebate_level"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="affiliate-withdraw-open"]').exists()).toBe(false)
  })

  it('switches rebate columns to source, base and usage cutoff when the rebate base is consumption', async () => {
    getSettings.mockResolvedValue({ affiliate_rebate_base: 'consumption' })
    // A sort saved under the recharge columns must not survive into the consumption columns.
    localStorage.setItem('admin-affiliate-rebates-table-sort', JSON.stringify({ key: 'payment_type', order: 'asc' }))
    const base = {
      out_trade_no: '',
      inviter_id: 1,
      inviter_email: 'agent@example.com',
      inviter_username: 'agent',
      invitee_id: 2,
      invitee_email: 'customer@example.com',
      invitee_username: 'customer',
      payment_type: '',
      order_status: '',
      created_at: '2026-10-22T08:13:18Z',
    }
    listRebateRecords.mockResolvedValue(page([
      {
        ...base,
        order_id: null,
        order_amount: null,
        pay_amount: null,
        rebate_amount: 0.19,
        reward_type: 'agent_direct',
        base_amount: 1.74,
        source_usage_until: '2026-10-22T08:11:18Z',
      },
      {
        ...base,
        order_id: 12,
        out_trade_no: 'sub2_abc',
        order_amount: 100,
        pay_amount: 98,
        rebate_amount: 1,
        reward_type: 'invite_fixed',
        payment_type: 'alipay',
        order_status: 'COMPLETED',
        base_amount: 100,
        source_usage_until: null,
      },
      {
        ...base,
        order_id: null,
        order_amount: null,
        pay_amount: null,
        rebate_amount: 0.5,
        reward_type: 'agent_direct',
        base_amount: null,
        source_usage_until: '2026-10-21T08:00:00Z',
      },
    ]))

    const wrapper = mountTable('rebates')
    await flushPromises()

    expect(listRebateRecords.mock.calls[0][0]).toMatchObject({ sort_by: 'created_at', sort_order: 'desc' })
    const keys = wrapper.get('[data-test="row-0"]').findAll('td').map((cell) => cell.attributes('data-test'))
    expect(keys).toEqual([
      'cell-source', 'cell-inviter', 'cell-invitee', 'cell-base_amount',
      'cell-rebate_amount', 'cell-reward_type', 'cell-usage_until', 'cell-created_at',
    ])

    const usage = wrapper.get('[data-test="row-0"]')
    expect(usage.get('[data-test="cell-source"]').text()).toBe('admin.affiliates.records.sourceConsumption')
    expect(usage.get('[data-test="cell-base_amount"]').text()).toBe('admin.affiliates.records.baseConsumption ¥1.74')
    expect(usage.get('[data-test="cell-usage_until"]').text()).not.toBe('-')

    const order = wrapper.get('[data-test="row-1"]')
    expect(order.get('[data-test="cell-source"]').text()).toContain('#12')
    expect(order.get('[data-test="cell-base_amount"]').text()).toContain('admin.affiliates.records.baseRecharge ¥100.00')
    expect(order.get('[data-test="cell-base_amount"]').text()).toContain('¥98.00')
    expect(order.get('[data-test="cell-usage_until"]').text()).toBe('-')

    // Settled before the base was recorded: no amount to show.
    expect(wrapper.get('[data-test="row-2"]').get('[data-test="cell-base_amount"]').text()).toBe('-')
  })

  it('keeps the order columns when the rebate base setting cannot be loaded', async () => {
    getSettings.mockRejectedValue(new Error('forbidden'))
    listRebateRecords.mockResolvedValue(page([{
      order_id: 3, out_trade_no: 'x', inviter_id: 1, inviter_email: '', inviter_username: '',
      invitee_id: 2, invitee_email: '', invitee_username: '', order_amount: 10, pay_amount: 10,
      rebate_amount: 1, reward_type: 'agent_direct', payment_type: 'alipay', order_status: 'COMPLETED',
      created_at: '2026-10-22T08:00:00Z',
    }]))

    const wrapper = mountTable('rebates')
    await flushPromises()

    expect(listRebateRecords).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-test="cell-order_status"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="cell-base_amount"]').exists()).toBe(false)
  })

  it('filters rebate records by reward type', async () => {
    listRebateRecords.mockResolvedValue(page([]))

    const wrapper = mountTable('rebates')
    await flushPromises()
    expect(listRebateRecords).toHaveBeenCalledTimes(1)
    expect(listRebateRecords.mock.calls[0][0]).toMatchObject({ reward_type: undefined })

    await wrapper.get('[data-test="affiliate-reward-type-filter"]').setValue('invitee_bonus')
    await flushPromises()

    expect(listRebateRecords).toHaveBeenCalledTimes(2)
    expect(listRebateRecords.mock.calls[1][0]).toMatchObject({ page: 1, reward_type: 'invitee_bonus' })
  })

  it('prefills the search box from initialSearch', async () => {
    listRebateRecords.mockResolvedValue(page([]))

    mount(AdminAffiliateRecordsTable, {
      props: { type: 'rebates', initialSearch: 'agent@example.com' },
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
          OrderStatusBadge: true,
          BaseDialog: true,
        },
      },
    })
    await flushPromises()

    expect(listRebateRecords.mock.calls[0][0]).toMatchObject({ search: 'agent@example.com' })
  })

  it('shows the current agent level under inviter and invitee on invite records', async () => {
    listInviteRecords.mockResolvedValue(page([
      {
        inviter_id: 3,
        inviter_email: 'u1@example.com',
        inviter_username: '',
        inviter_level: 2,
        invitee_id: 4,
        invitee_email: 'u2@example.com',
        invitee_username: 'u2',
        invitee_level: 3,
        aff_code: 'U1CODE',
        total_rebate: 5,
        created_at: '2026-10-01T08:00:00Z',
      },
    ]))

    const wrapper = mountTable('invites')
    await flushPromises()

    expect(wrapper.find('[data-test="cell-agent_owner"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="cell-agent_upline"]').exists()).toBe(false)
    const row = wrapper.get('[data-test="row-0"]')
    const inviter = row.get('[data-test="cell-inviter"]').text()
    expect(inviter).toContain('admin.affiliates.levels.2')
    expect(inviter).not.toContain('-')
    const invitee = row.get('[data-test="cell-invitee"]').text()
    expect(invitee).toContain('u2')
    expect(invitee).toContain('admin.affiliates.levels.3')
  })

  it('lists offline withdrawals next to balance transfers with their type', async () => {
    listTransferRecords.mockResolvedValue(page([
      {
        ledger_id: 11,
        action: 'withdraw',
        user_id: 42,
        user_email: 'inviter@example.com',
        username: 'inviter',
        amount: 12.5,
        balance_after: 5.5,
        available_quota_after: 7.5,
        frozen_quota_after: 0,
        history_quota_after: 30,
        snapshot_available: true,
        created_at: '2026-09-17T08:00:00Z',
      },
      {
        ledger_id: 10,
        action: 'transfer',
        user_id: 42,
        user_email: 'inviter@example.com',
        username: 'inviter',
        amount: 3,
        balance_after: 5.5,
        available_quota_after: 20,
        frozen_quota_after: 0,
        history_quota_after: 30,
        snapshot_available: true,
        created_at: '2026-09-16T08:00:00Z',
      },
    ]))

    const wrapper = mountTable('transfers')
    await flushPromises()

    const withdrawRow = wrapper.get('[data-test="row-0"]')
    expect(withdrawRow.get('[data-test="cell-action"]').text()).toBe('admin.affiliates.outflowTypes.withdraw')
    expect(withdrawRow.get('[data-test="cell-amount"]').text()).toBe('¥12.50')

    const transferRow = wrapper.get('[data-test="row-1"]')
    expect(transferRow.get('[data-test="cell-action"]').text()).toBe('admin.affiliates.outflowTypes.transfer')
  })

  it('opens the offline withdrawal dialog and reloads the first page after recording', async () => {
    listTransferRecords.mockResolvedValue(page([]))

    const wrapper = mountTable('transfers')
    await flushPromises()
    expect(listTransferRecords).toHaveBeenCalledTimes(1)

    const button = wrapper.get('[data-test="affiliate-withdraw-open"]')
    expect(button.text()).toBe('admin.affiliates.withdraw.button')
    await button.trigger('click')
    expect(wrapper.get('[data-test="withdraw-dialog"]').attributes('data-show')).toBe('true')

    wrapper.findComponent(WithdrawDialogStub).vm.$emit('success', { ledger_id: 12 })
    await flushPromises()

    expect(wrapper.get('[data-test="withdraw-dialog"]').attributes('data-show')).toBe('false')
    expect(listTransferRecords).toHaveBeenCalledTimes(2)
    expect(listTransferRecords.mock.calls[1][0]).toMatchObject({ page: 1 })
  })
})
