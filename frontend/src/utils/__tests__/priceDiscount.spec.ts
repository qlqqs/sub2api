import { describe, expect, it } from 'vitest'
import {
  DEFAULT_USD_CNY_RATE,
  discountTone,
  formatDiscount,
  formatFxRate,
  peakAdjustment,
  priceDiscount,
  ratiosAgree,
  resolveUsdCnyRate
} from '../priceDiscount'

describe('priceDiscount', () => {
  it('d = 1 − ratio ÷ fx', () => {
    expect(priceDiscount(0.35, 6.67)).toBeCloseTo(1 - 0.35 / 6.67, 10)
    expect(priceDiscount(6.8, 6.8)).toBe(0)
    expect(priceDiscount(7.14, 6.8)).toBeCloseTo(-0.05, 10)
  })

  it.each([0, -1, Number.NaN, Number.POSITIVE_INFINITY, null, undefined])('invalid ratio %s → null', (ratio) => {
    expect(priceDiscount(ratio as number | null | undefined, 6.8)).toBeNull()
  })

  it.each([0, -6.8, Number.NaN, null, undefined])('invalid fx %s → null', (fx) => {
    expect(priceDiscount(1, fx as number | null | undefined)).toBeNull()
  })
})

describe('formatDiscount', () => {
  it.each([
    [0.9475, '-95%'],
    [0.5, '-50%'],
    [0.005, '-1%'],
    [0.0049, '≈'],
    [-0.0049, '≈'],
    [0, '≈'],
    [-0.005, '+1%'],
    [-0.05, '+5%'],
    [-1.25, '+125%']
  ])('%s → %s', (d, expected) => {
    expect(formatDiscount(d, '≈')).toBe(expected)
  })

  it('uses the caller-provided same-price wording', () => {
    expect(formatDiscount(0.001, '≈ 原价')).toBe('≈ 原价')
    expect(formatDiscount(0.001, '≈ 官方价')).toBe('≈ 官方价')
  })

  it.each([null, undefined, Number.NaN])('invalid discount %s → —', (d) => {
    expect(formatDiscount(d as number | null | undefined, '≈')).toBe('—')
  })

  it('end to end: invalid ratio or fx renders —', () => {
    expect(formatDiscount(priceDiscount(0, 6.8), '≈')).toBe('—')
    expect(formatDiscount(priceDiscount(1, 0), '≈')).toBe('—')
  })
})

describe('discountTone', () => {
  it('classifies discounts', () => {
    expect(discountTone(0.5)).toBe('cheaper')
    expect(discountTone(-0.5)).toBe('pricier')
    expect(discountTone(0.001)).toBe('same')
    expect(discountTone(null)).toBe('none')
  })
})

describe('resolveUsdCnyRate', () => {
  it('uses the public setting when valid', () => {
    expect(resolveUsdCnyRate(7.12)).toBe(7.12)
    expect(resolveUsdCnyRate('7.2')).toBe(7.2)
  })

  it.each([undefined, null, 0, -1, Number.NaN, '', 'abc', {}])('falls back to 6.8 for %s', (raw) => {
    expect(resolveUsdCnyRate(raw)).toBe(DEFAULT_USD_CNY_RATE)
    expect(DEFAULT_USD_CNY_RATE).toBe(6.8)
  })
})

describe('formatFxRate / ratiosAgree / peakAdjustment', () => {
  it('formats the exchange rate basis', () => {
    expect(formatFxRate(6.67)).toBe('$1 = ¥6.67')
    expect(formatFxRate(6.8)).toBe('$1 = ¥6.8')
  })

  it('treats ratios within 1% as consistent', () => {
    expect(ratiosAgree(1, 1.01)).toBe(true)
    expect(ratiosAgree(1, 1.02)).toBe(false)
    expect(ratiosAgree(0, 0)).toBe(true)
  })

  it.each([
    [1.5, 'up', '50'],
    [1.333, 'up', '33.3'],
    [0.8, 'down', '20'],
    [1, 'none', '0'],
    [null, 'none', '0'],
    [Number.NaN, 'none', '0']
  ])('peak multiplier %s → %s %s%%', (m, direction, percent) => {
    expect(peakAdjustment(m as number | null)).toEqual({ direction, percent })
  })
})
