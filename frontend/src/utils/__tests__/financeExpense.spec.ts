import { describe, expect, it } from 'vitest'
import { defaultAmortizeDays } from '@/api/admin/finance'
import {
  amortizeFieldsFromDays,
  amortizeModeAfterCategoryChange,
  newExpenseDraft,
  validateExpenseDraft
} from '../financeExpense'
import { defaultFinanceRange, inclusiveDays } from '../financeDate'

describe('financeExpense (CUSTOM)', () => {
  it('defaults subscriptions to 30 days per day and other categories to a one-off deduction', () => {
    expect(defaultAmortizeDays('upstream_subscription')).toBe(30)
    expect(defaultAmortizeDays('server')).toBe(1)
    expect(defaultAmortizeDays('domain')).toBe(1)
    expect(defaultAmortizeDays('other')).toBe(1)
    expect(newExpenseDraft('2026-10-25')).toMatchObject({ amortize_mode: 'daily', amortize_days: 30 })
    expect(newExpenseDraft('2026-10-25', 'server')).toMatchObject({ amortize_mode: 'once', amortize_days: 30 })
  })

  it('switches the default mode on category change but keeps a hand-picked mode', () => {
    expect(amortizeModeAfterCategoryChange('daily', 'upstream_subscription', 'server')).toBe('once')
    expect(amortizeModeAfterCategoryChange('once', 'other', 'upstream_subscription')).toBe('daily')
    expect(amortizeModeAfterCategoryChange('once', 'upstream_subscription', 'server')).toBe('once')
    expect(amortizeModeAfterCategoryChange('daily', 'server', 'domain')).toBe('daily')
  })

  it('maps stored days back to a mode', () => {
    expect(amortizeFieldsFromDays(1)).toEqual({ amortize_mode: 'once', amortize_days: 30 })
    expect(amortizeFieldsFromDays(31)).toEqual({ amortize_mode: 'daily', amortize_days: 31 })
  })

  it('submits a one-off deduction as one day and ignores the days field', () => {
    const result = validateExpenseDraft({
      occurred_on: '2026-10-25',
      category: 'upstream_subscription',
      amount: '1400',
      amortize_mode: 'once',
      amortize_days: 'abc',
      account_id: 1,
      note: ''
    })
    expect(result).toMatchObject({ ok: true, payload: { amortize_days: 1 } })
  })

  it('validates and builds the payload with the amount kept as a string', () => {
    const result = validateExpenseDraft({
      occurred_on: '2026-10-25',
      category: 'upstream_subscription',
      amount: ' 1400.00 ',
      amortize_mode: 'daily',
      amortize_days: '30',
      account_id: 1,
      note: ' ChatGPT Pro #3 '
    })
    expect(result).toEqual({
      ok: true,
      payload: {
        occurred_on: '2026-10-25',
        category: 'upstream_subscription',
        amount: '1400.00',
        amortize_days: 30,
        account_id: 1,
        note: 'ChatGPT Pro #3'
      }
    })
  })

  it('rejects bad amounts, days, dates and long notes', () => {
    const result = validateExpenseDraft({
      occurred_on: '',
      category: 'other',
      amount: '0',
      amortize_mode: 'daily',
      amortize_days: '1',
      account_id: null,
      note: 'x'.repeat(501)
    })
    expect(result).toEqual({
      ok: false,
      errors: {
        occurred_on: 'occurredOnRequired',
        amount: 'amountInvalid',
        amortize_days: 'amortizeDaysInvalid',
        note: 'noteTooLong'
      }
    })
    const draft = { ...newExpenseDraft('2026-10-25', 'other'), amortize_mode: 'daily' as const }
    expect(validateExpenseDraft({ ...draft, amount: '1.234' }).ok).toBe(false)
    expect(validateExpenseDraft({ ...draft, amount: '1.5', amortize_days: '3661' }).ok).toBe(false)
    expect(validateExpenseDraft({ ...draft, amount: '1.5', amortize_days: '2.5' }).ok).toBe(false)
    expect(validateExpenseDraft({ ...draft, amount: '0.01', amortize_days: '3660' }).ok).toBe(true)
  })

  it('defaults the range to the Beijing-time month and counts inclusive days', () => {
    // 2026-10-31 20:00 UTC 已是北京时间 11 月 1 日
    expect(defaultFinanceRange(new Date('2026-10-31T20:00:00Z'))).toEqual({ start: '2026-11-01', end: '2026-11-01' })
    expect(defaultFinanceRange(new Date('2026-10-25T03:00:00Z'))).toEqual({ start: '2026-10-01', end: '2026-10-25' })
    expect(inclusiveDays('2026-10-01', '2026-10-25')).toBe(25)
    expect(inclusiveDays('2026-10-25', '2026-10-01')).toBe(-23)
    expect(Number.isNaN(inclusiveDays('', '2026-10-01'))).toBe(true)
  })
})
