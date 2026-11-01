import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post, put, delete: del } }))

import {
  createExpense,
  exportCsv,
  getBreakdown,
  getReconcile,
  getSummary,
  isReconcileMismatch,
  listExpenses,
  normalizeRecharge,
  normalizeSummary,
  updateSettings
} from '@/api/admin/finance'
import { getForAccount, listAll, replaceForAccount } from '@/api/admin/accountModelCost'

describe('admin finance api (CUSTOM)', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    del.mockReset()
  })

  it('requests the summary with range params and keeps amount strings untouched', async () => {
    get.mockResolvedValue({
      data: {
        range: { start: '2026-10-01', end: '2026-10-25', granularity: 'day' },
        totals: { revenue: '12.34000000', profit: '-1.5', profit_margin: null },
        series: [{ period: '2026-10-25', revenue: '1.00000001', profit_margin: '0.3200' }],
        snapshot: { balance_total: '100.00' },
        warnings: ['范围内有 2 笔订阅订单，未计入利润', ''],
        computed_at: '2026-10-25T21:00:00+08:00'
      }
    })
    const controller = new AbortController()
    const summary = await getSummary(
      { start: '2026-10-01', end: '2026-10-25', granularity: 'day' },
      { signal: controller.signal }
    )
    expect(get).toHaveBeenCalledWith('/admin/finance/summary', {
      params: { start: '2026-10-01', end: '2026-10-25', granularity: 'day' },
      signal: controller.signal
    })
    expect(summary.totals.revenue).toBe('12.34000000')
    expect(summary.totals.profit).toBe('-1.5')
    expect(summary.totals.account_cost).toBe('0')
    expect(summary.totals.profit_margin).toBeNull()
    expect(summary.series[0]).toMatchObject({ period: '2026-10-25', revenue: '1.00000001', commission: '0', profit_margin: '0.3200' })
    expect(summary.snapshot).toMatchObject({ balance_total: '100.00', balance_free: '0' })
    expect(summary.warnings).toEqual(['范围内有 2 笔订阅订单，未计入利润'])
  })

  it('fills a fully missing summary with zeros and empty arrays', () => {
    const summary = normalizeSummary(null)
    expect(summary.series).toEqual([])
    expect(summary.warnings).toEqual([])
    expect(summary.range.granularity).toBe('day')
    expect(summary.totals.revenue).toBe('0')
    expect(summary.snapshot.affiliate_frozen).toBe('0')
  })

  it('normalizes recharge totals, gifts and orders', () => {
    const recharge = normalizeRecharge({
      totals: { order_count: 4, cash_received: '26.00', gift: { invitee_bonus: '2.00', total: '112.00' } },
      orders: [{ id: 59, user_id: 60, user_email: 'a@b.c', pay_amount: '10.00', channel_fee_rate: '0.0320', status: 'COMPLETED' }],
      orders_truncated: true
    })
    expect(recharge.totals.order_count).toBe(4)
    expect(recharge.totals.cash_received).toBe('26.00')
    expect(recharge.totals.net_cash).toBe('0')
    expect(recharge.totals.gift).toMatchObject({ invitee_bonus: '2.00', invitee_signup_bonus: '0', total: '112.00', promo: '0' })
    expect(recharge.orders[0]).toMatchObject({ id: 59, pay_amount: '10.00', channel_fee_rate: '0.0320', refund_cash: '0' })
    expect(recharge.orders_truncated).toBe(true)
  })

  it('注册赠送财务分类单列且保留金额精度', () => {
    const gift = normalizeRecharge({ totals: { gift: { invitee_bonus: '2.00', invitee_signup_bonus: '5.12345678', total: '7.12345678' } } }).totals.gift
    expect(gift.invitee_signup_bonus).toBe('5.12345678')
    expect(gift.invitee_bonus).toBe('2.00')
    expect(gift.total).toBe('7.12345678')
  })

  it('passes the breakdown dimension and drops unknown cost modes', async () => {
    get.mockResolvedValue({
      data: {
        dim: 'account',
        rows: [
          { key: '4', label: 'img', cost_mode: 'per_image', requests: 1, images: 1, account_cost: '0.05' },
          { key: '9', cost_mode: 'weird' }
        ]
      }
    })
    const result = await getBreakdown({ start: '2026-10-01', end: '2026-10-25', dim: 'account', limit: 200 })
    expect(get.mock.calls[0][1].params).toEqual({ start: '2026-10-01', end: '2026-10-25', dim: 'account', limit: 200 })
    expect(result.rows[0]).toMatchObject({ key: '4', cost_mode: 'per_image', consumption: '0', account_cost: '0.05' })
    expect(result.rows[1]).toMatchObject({ label: '9', cost_mode: null })
    expect(result.truncated).toBe(false)
  })

  it('omits a blank reconcile search and maps the paginated payload', async () => {
    get.mockResolvedValue({
      data: { items: [{ user_id: 60, difference: '-0.02', remaining: { paid: '9.95' } }], total: 9, page: 1, page_size: 50, pages: 1 }
    })
    const page = await getReconcile({ page: 1, page_size: 50, only_mismatch: true, search: '  ' })
    expect(get.mock.calls[0][1].params).toEqual({ page: 1, page_size: 50, only_mismatch: true })
    expect(page.total).toBe(9)
    expect(page.items[0].remaining).toEqual({ paid: '9.95', commission: '0', free: '0' })
    expect(page.items[0].actual_balance).toBe('0')
  })

  it('treats |difference| >= 0.01 as a mismatch', () => {
    expect(isReconcileMismatch('0')).toBe(false)
    expect(isReconcileMismatch('0.00999999')).toBe(false)
    expect(isReconcileMismatch('-0.009')).toBe(false)
    expect(isReconcileMismatch('0.01')).toBe(true)
    expect(isReconcileMismatch('-0.01000000')).toBe(true)
    expect(isReconcileMismatch('12')).toBe(true)
  })

  it('downloads CSV as a blob', async () => {
    const blob = new Blob(['a,b'])
    get.mockResolvedValue({ data: blob })
    await expect(exportCsv({ start: '2026-10-01', end: '2026-10-25', granularity: 'month' })).resolves.toBe(blob)
    expect(get).toHaveBeenCalledWith('/admin/finance/export', {
      params: { start: '2026-10-01', end: '2026-10-25', granularity: 'month' },
      responseType: 'blob'
    })
  })

  it('lists, creates and normalizes expenses', async () => {
    get.mockResolvedValue({
      data: {
        items: [{ id: 3, occurred_on: '2026-10-25', category: 'upstream_subscription', amount: '1400.00', amortize_days: 30, account_id: 1, daily_amount: '46.66666667' }],
        total: 1,
        page: 1,
        page_size: 20,
        pages: 1
      }
    })
    const page = await listExpenses({ start: '2026-10-01', end: '2026-10-25', category: '', page: 1, page_size: 20 })
    expect(get.mock.calls[0][1].params).toEqual({ page: 1, page_size: 20, start: '2026-10-01', end: '2026-10-25' })
    expect(page.items[0]).toMatchObject({ id: 3, amount: '1400.00', amortize_days: 30, account_id: 1, account_name: null, note: '' })

    const input = {
      occurred_on: '2026-10-25',
      category: 'server' as const,
      amount: '20.00',
      amortize_days: 1,
      account_id: null,
      partner_user_id: null,
      note: ''
    }
    post.mockResolvedValue({ data: { id: 4, ...input, account_id: null } })
    const created = await createExpense(input)
    expect(post).toHaveBeenCalledWith('/admin/finance/expenses', input)
    expect(created).toMatchObject({ id: 4, category: 'server', account_id: null })
  })

  // CUSTOM: 合伙人分成与区域专项支出
  it('normalizes partner share fields on monthly rows and totals', () => {
    const summary = normalizeSummary({
      range: { start: '2026-10-01', end: '2026-11-30', granularity: 'month' },
      totals: { profit: '100', partner_share: '22.32', net_profit: '77.68', partner_share_estimated: true, partner_share_scope: 'whole_months' },
      series: [
        { period: '2026-10', profit: '40', partner_share: '10.00', net_profit: '30', partner_share_estimated: false },
        { period: '2026-11', profit: '60' }
      ]
    })
    expect(summary.totals).toMatchObject({
      partner_share: '22.32',
      net_profit: '77.68',
      partner_share_estimated: true,
      partner_share_scope: 'whole_months'
    })
    expect(summary.series[0]).toMatchObject({ partner_share: '10.00', net_profit: '30', partner_share_estimated: false })
    expect(summary.series[1]).toMatchObject({ partner_share: '0', net_profit: '0', partner_share_estimated: false })
  })

  it('keeps the expense region and its name', async () => {
    get.mockResolvedValue({
      data: { items: [{ id: 5, amount: '30.00', partner_user_id: 75, partner_region_name: '三科大' }, { id: 6, amount: '1' }], total: 2 }
    })
    const page = await listExpenses({})
    expect(page.items[0]).toMatchObject({ partner_user_id: 75, partner_region_name: '三科大' })
    expect(page.items[1]).toMatchObject({ partner_user_id: null, partner_region_name: null })
  })

  it('normalizes settings even when PUT omits the account list', async () => {
    put.mockResolvedValue({ data: { fixed_cost_account_ids: [1, '2'] } })
    await expect(updateSettings({ fixed_cost_account_ids: [1, 2] })).resolves.toEqual({
      fixed_cost_account_ids: [1, 2],
      accounts: []
    })
    expect(put).toHaveBeenCalledWith('/admin/finance/settings', { fixed_cost_account_ids: [1, 2] })
  })

  it('account model cost multipliers mirror the group model rate API', async () => {
    get.mockResolvedValueOnce({ data: { rules: [{ model: 'gpt-5.6-luna', rate_multiplier: 0.3 }] } })
    await expect(getForAccount(2)).resolves.toEqual([{ model: 'gpt-5.6-luna', rate_multiplier: 0.3 }])
    expect(get).toHaveBeenLastCalledWith('/admin/accounts/2/model-cost-multipliers')

    get.mockResolvedValueOnce({ data: null })
    await expect(listAll()).resolves.toEqual({})
    expect(get).toHaveBeenLastCalledWith('/admin/accounts/model-cost-multipliers')

    put.mockResolvedValue({ data: { rules: null } })
    await expect(replaceForAccount(2, [])).resolves.toEqual([])
    expect(put).toHaveBeenCalledWith('/admin/accounts/2/model-cost-multipliers', { rules: [] })
  })
})
