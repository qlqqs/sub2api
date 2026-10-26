import { describe, expect, it } from 'vitest'
import {
  decimalRateToPercent,
  formatAmount,
  formatCny,
  formatRatio,
  isNegativeAmount,
  isZeroAmount,
  percentToDecimalRate,
  roundDecimalString
} from '../financeFormat'

describe('financeFormat (CUSTOM)', () => {
  it('rounds decimal strings half-up without float error', () => {
    expect(roundDecimalString('1.005', 2)).toBe('1.01')
    expect(roundDecimalString('12.34000000', 2)).toBe('12.34')
    expect(roundDecimalString('0.12345', 4)).toBe('0.1235')
    expect(roundDecimalString('9.995', 2)).toBe('10.00')
    expect(roundDecimalString('-0.004', 2)).toBe('0.00')
    expect(roundDecimalString('-2.5', 0)).toBe('-3')
    expect(roundDecimalString('abc', 2)).toBeNull()
    expect(roundDecimalString('', 2)).toBeNull()
  })

  it('formats amounts with grouping, 2 decimals by default and 4 for unit prices', () => {
    expect(formatAmount('1234567.891')).toBe('1,234,567.89')
    expect(formatAmount('0')).toBe('0.00')
    expect(formatAmount('0.0312', 4)).toBe('0.0312')
    expect(formatAmount(null)).toBe('—')
    expect(formatCny('26.00')).toBe('¥26.00')
    expect(formatCny('-3.2')).toBe('-¥3.20')
    expect(formatCny(undefined)).toBe('—')
  })

  it('detects negative and zero at display precision', () => {
    expect(isNegativeAmount('-0.01')).toBe(true)
    expect(isNegativeAmount('-0.001')).toBe(false)
    expect(isNegativeAmount('5')).toBe(false)
    expect(isZeroAmount('0.004')).toBe(true)
    expect(isZeroAmount('0.01')).toBe(false)
  })

  it('formats ratios as percentages and null as a dash', () => {
    expect(formatRatio('0.3200')).toBe('32.00%')
    expect(formatRatio('-0.1234')).toBe('-12.34%')
    expect(formatRatio('1')).toBe('100.00%')
    expect(formatRatio(null)).toBe('—')
  })

  it('converts decimal rates to percent text for editing', () => {
    expect(decimalRateToPercent('0.0320')).toBe('3.2')
    expect(decimalRateToPercent('0')).toBe('0')
    expect(decimalRateToPercent('0.2000')).toBe('20')
    expect(decimalRateToPercent('0.0005')).toBe('0.05')
    expect(decimalRateToPercent(undefined)).toBe('0')
  })

  it('converts percent input to a 4-decimal rate string and validates range', () => {
    expect(percentToDecimalRate('3.2')).toEqual({ value: '0.0320' })
    expect(percentToDecimalRate(' 0.05 ')).toEqual({ value: '0.0005' })
    expect(percentToDecimalRate('20')).toEqual({ value: '0.2000' })
    expect(percentToDecimalRate('')).toEqual({ value: '0.0000' })
    expect(percentToDecimalRate('0')).toEqual({ value: '0.0000' })
    expect(percentToDecimalRate('20.01')).toEqual({ error: 'outOfRange' })
    expect(percentToDecimalRate('-1')).toEqual({ error: 'outOfRange' })
    expect(percentToDecimalRate('3.215')).toEqual({ error: 'tooManyDecimals' })
    expect(percentToDecimalRate('3%')).toEqual({ error: 'invalid' })
  })
})
