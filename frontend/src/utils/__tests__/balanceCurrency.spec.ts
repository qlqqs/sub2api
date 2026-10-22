import { describe, expect, it } from 'vitest'
import {
  BALANCE_SYMBOL,
  OFFICIAL_SYMBOL,
  effectiveRate,
  formatBalance,
  formatRateRatio,
  formatRateValue
} from '../balanceCurrency'

describe('balanceCurrency symbols', () => {
  it('uses ¥ for the balance unit and $ for official prices', () => {
    expect(BALANCE_SYMBOL).toBe('¥')
    expect(OFFICIAL_SYMBOL).toBe('$')
  })
})

describe('formatBalance', () => {
  it('prefixes the balance symbol with two decimals by default', () => {
    expect(formatBalance(1.5)).toBe('¥1.50')
    expect(formatBalance(0)).toBe('¥0.00')
    expect(formatBalance('12.345')).toBe('¥12.35')
  })

  it('puts the minus sign before the symbol', () => {
    expect(formatBalance(-1.234)).toBe('-¥1.23')
  })

  it('does not show a sign when the value rounds to zero', () => {
    expect(formatBalance(-0.001)).toBe('¥0.00')
    expect(formatBalance(0.001, { signed: true })).toBe('¥0.00')
  })

  it('supports signed, decimals, min/max decimals and grouping options', () => {
    expect(formatBalance(2, { signed: true })).toBe('+¥2.00')
    expect(formatBalance(-2, { signed: true })).toBe('-¥2.00')
    expect(formatBalance(1.23456, { decimals: 4 })).toBe('¥1.2346')
    expect(formatBalance(1.5, { minDecimals: 0, maxDecimals: 4 })).toBe('¥1.5')
    expect(formatBalance(1234567.891, { grouping: true })).toBe('¥1,234,567.89')
    expect(formatBalance(1234567.891)).toBe('¥1234567.89')
  })

  it('returns the fallback for null / undefined / non-finite values', () => {
    expect(formatBalance(null)).toBe('¥0.00')
    expect(formatBalance(undefined)).toBe('¥0.00')
    expect(formatBalance(Number.NaN)).toBe('¥0.00')
    expect(formatBalance(null, { fallback: '-' })).toBe('-')
  })
})

describe('formatRateRatio', () => {
  it('formats the effective rate as "¥x = $1" without trailing zeros', () => {
    expect(formatRateRatio(0.5)).toBe('¥0.5 = $1')
    expect(formatRateRatio(1)).toBe('¥1 = $1')
    expect(formatRateRatio(0.125)).toBe('¥0.125 = $1')
    expect(formatRateRatio(0.30000000000000004)).toBe('¥0.3 = $1')
    expect(formatRateRatio(2.5)).toBe('¥2.5 = $1')
  })

  it('keeps at most 4 decimals', () => {
    expect(formatRateRatio(0.123456)).toBe('¥0.1235 = $1')
    expect(formatRateValue(0.12344)).toBe('0.1234')
  })

  it('returns "-" for null, zero, negative or non-finite rates', () => {
    expect(formatRateRatio(null)).toBe('-')
    expect(formatRateRatio(undefined)).toBe('-')
    expect(formatRateRatio(0)).toBe('¥0 = $1')
    expect(formatRateRatio(-0.5)).toBe('-')
    expect(formatRateRatio(Number.POSITIVE_INFINITY)).toBe('-')
  })
})

describe('effectiveRate', () => {
  it('prefers the user exclusive rate over the group rate', () => {
    expect(effectiveRate(0.5, 0.3)).toBe(0.3)
    expect(formatRateRatio(effectiveRate(0.5, 0.3))).toBe('¥0.3 = $1')
  })

  it('falls back to the group rate when there is no exclusive rate', () => {
    expect(effectiveRate(0.5, null)).toBe(0.5)
    expect(effectiveRate(0.5, undefined)).toBe(0.5)
  })

  it('keeps an explicit exclusive rate of 0', () => {
    expect(effectiveRate(0.5, 0)).toBe(0)
  })
})
