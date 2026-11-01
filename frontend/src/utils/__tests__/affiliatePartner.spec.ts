import { describe, expect, it } from 'vitest'
import type { PartnerMonthStatus, PartnerMonthTotals } from '@/api/affiliatePartner'
import { normalizePartnerMonthTotals } from '@/api/affiliatePartner'
import {
  formatPartnerRate,
  normalizeAdjustmentAmount,
  partnerLineRoute,
  partnerRateFromPercent,
  partnerStatementActions,
  partnerUserName
} from '@/utils/affiliatePartner'

function months(...items: Array<[string, PartnerMonthStatus]>): PartnerMonthTotals[] {
  return items.map(([month, status]) => normalizePartnerMonthTotals({ month, status }))
}

describe('affiliatePartner utils (CUSTOM)', () => {
  it('formats decimal rates as percentages', () => {
    expect(formatPartnerRate('0.4000')).toBe('40%')
    expect(formatPartnerRate('0.3550')).toBe('35.5%')
    expect(formatPartnerRate('0')).toBe('0%')
  })

  it('converts the percent input to a 4-decimal rate', () => {
    expect(partnerRateFromPercent('40')).toEqual({ value: '0.4000' })
    expect(partnerRateFromPercent('35.5')).toEqual({ value: '0.3550' })
    expect(partnerRateFromPercent('100')).toEqual({ value: '1.0000' })
    expect(partnerRateFromPercent('')).toEqual({ error: 'invalid' })
    expect(partnerRateFromPercent('101')).toEqual({ error: 'outOfRange' })
    expect(partnerRateFromPercent('1.234')).toEqual({ error: 'tooManyDecimals' })
  })

  it('confirms months in order and never the current month', () => {
    const list = months(['2026-12', 'current'], ['2026-11', 'draft'], ['2026-10', 'draft'], ['2026-09', 'confirmed'])
    expect(partnerStatementActions(list, '2026-10')).toEqual({ confirm: true, pay: false, adjust: true })
    // 10 月还没确认：11 月不能确认，但可以加调整项。
    expect(partnerStatementActions(list, '2026-11')).toEqual({ confirm: false, pay: false, adjust: true })
    expect(partnerStatementActions(list, '2026-12')).toEqual({ confirm: false, pay: false, adjust: true })
    expect(partnerStatementActions(list, '2026-09')).toEqual({ confirm: false, pay: true, adjust: false })
  })

  it('allows only nothing on paid months and unknown months', () => {
    const list = months(['2026-11', 'current'], ['2026-10', 'paid'])
    expect(partnerStatementActions(list, '2026-10')).toEqual({ confirm: false, pay: false, adjust: false })
    expect(partnerStatementActions(list, '2026-01')).toEqual({ confirm: false, pay: false, adjust: false })
    // 设立当月已结束：没有更早的月份，草稿可以确认。
    expect(partnerStatementActions(months(['2026-11', 'current'], ['2026-10', 'draft']), '2026-10').confirm).toBe(true)
  })

  it('accepts signed adjustment amounts with up to 2 decimals', () => {
    expect(normalizeAdjustmentAmount('-5.00')).toBe('-5.00')
    expect(normalizeAdjustmentAmount(' +12.5 ')).toBe('12.5')
    expect(normalizeAdjustmentAmount('0')).toBeNull()
    expect(normalizeAdjustmentAmount('-0.00')).toBeNull()
    expect(normalizeAdjustmentAmount('1.234')).toBeNull()
    expect(normalizeAdjustmentAmount('abc')).toBeNull()
  })

  it('names users and builds line routes without exposing ids', () => {
    expect(partnerUserName({ id: 1, email: 'a***@x.com', username: '', deleted: false }, 'deleted')).toBe('-')
    expect(partnerUserName({ id: 1, email: 'a', username: 'nick', deleted: true }, 'deleted')).toBe('deleted')
    expect(partnerUserName(null, 'deleted')).toBe('-')

    const base = { members: 1, consumption: '0', revenue: '0', account_cost: '0', commission: '0', profit: '0' }
    expect(partnerLineRoute({ ...base, line_agent: null })).toEqual({ name: 'AffiliateRegionDirect' })
    expect(partnerLineRoute({ ...base, line_agent: { id: 59, email: '', username: 'a', deleted: false } })).toEqual({
      name: 'AffiliateRegionAgent',
      params: { agentId: '59' }
    })
    expect(partnerLineRoute({ ...base, line_agent: { id: 59, email: '', username: 'a', deleted: true } })).toBeUndefined()
  })
})
