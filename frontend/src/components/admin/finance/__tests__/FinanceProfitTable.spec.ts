import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import FinanceProfitTable from '../FinanceProfitTable.vue'
import { normalizeSummary } from '@/api/admin/finance'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function summary(granularity: 'day' | 'month') {
  return normalizeSummary({
    range: { start: '2026-10-01', end: '2026-11-30', granularity },
    totals: { profit: '100', partner_share: '32.32', net_profit: '67.68', partner_share_estimated: true, partner_share_scope: 'whole_months' },
    series: [
      { period: '2026-10', profit: '40', partner_share: '10', net_profit: '30', partner_share_estimated: false },
      { period: '2026-11', profit: '60', partner_share: '22.32', net_profit: '37.68', partner_share_estimated: true }
    ]
  })
}

describe('FinanceProfitTable (CUSTOM partner share)', () => {
  it('adds partner share and net profit columns by month and marks estimates', () => {
    const wrapper = mount(FinanceProfitTable, { props: { summary: summary('month') } })
    expect(wrapper.find('[data-testid="finance-col-partner-share"]').exists()).toBe(true)
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('10.00')
    expect(rows[0].text()).toContain('30.00')
    expect(rows[0].find('[data-testid="finance-partner-estimated"]').exists()).toBe(false)
    expect(rows[1].find('[data-testid="finance-partner-estimated"]').text()).toBe('admin.finance.profit.estimated')
    const total = wrapper.get('tfoot').text()
    expect(total).toContain('32.32')
    expect(total).toContain('67.68')
  })

  it('hides both columns by day', () => {
    const wrapper = mount(FinanceProfitTable, { props: { summary: summary('day') } })
    expect(wrapper.find('[data-testid="finance-col-partner-share"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('admin.finance.profit.columns.netProfit')
  })
})
