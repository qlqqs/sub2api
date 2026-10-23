import { describe, expect, it } from 'vitest'
import {
  GROUP_MODEL_RATE_MAX_RULES,
  groupModelRatesSignature,
  normalizeGroupModelPattern,
  validateGroupModelRates
} from '../groupModelRates'

describe('groupModelRates (CUSTOM)', () => {
  it('normalizes like the backend (lowercase; claude- dots become dashes)', () => {
    expect(normalizeGroupModelPattern('  Claude-Opus-4.5* ')).toBe('claude-opus-4-5*')
    expect(normalizeGroupModelPattern('GPT-5.5')).toBe('gpt-5.5')
  })

  it('accepts a trailing wildcard and trims model names', () => {
    const result = validateGroupModelRates([
      { model: ' claude-opus-* ', rate: '1.0' },
      { model: 'gpt-5.5', rate: 0.8 },
      { model: '', rate: '' }
    ])
    expect(result.valid).toBe(true)
    expect(result.errors).toEqual([null, null, null])
    expect(result.rules).toEqual([
      { model: 'claude-opus-*', rate_multiplier: 1 },
      { model: 'gpt-5.5', rate_multiplier: 0.8 }
    ])
  })

  it.each([
    ['a*b', 1, 'wildcardPosition'],
    ['**', 1, 'wildcardPosition'],
    ['*', 1, 'wildcardPosition'],
    ['', 1, 'modelRequired'],
    ['x'.repeat(256), 1, 'modelTooLong'],
    ['m', '', 'rateRequired'],
    ['m', 'abc', 'rateInvalid'],
    ['m', 0, 'rateInvalid'],
    ['m', -0.5, 'rateInvalid'],
    ['m', 10000.01, 'rateTooLarge'],
    // 与后端一致:中间空白、4 位小数舍入为 0
    ['gpt 5', 1, 'modelWhitespace'],
    ['m', 0.00004, 'rateTooSmall']
  ])('%s / %s → %s', (model, rate, error) => {
    const result = validateGroupModelRates([{ model, rate }])
    expect(result.valid).toBe(false)
    expect(result.errors).toEqual([error])
  })

  it('accepts the upper bound 10000 and the smallest 4-decimal rate', () => {
    expect(validateGroupModelRates([{ model: 'm', rate: 10000 }]).valid).toBe(true)
    expect(validateGroupModelRates([{ model: 'm', rate: 0.00005 }]).valid).toBe(true)
  })

  it('flags later duplicates (case-insensitive, normalized)', () => {
    const result = validateGroupModelRates([
      { model: 'Claude-Opus-4.5', rate: 1 },
      { model: 'claude-opus-4-5', rate: 2 }
    ])
    expect(result.errors).toEqual([null, 'duplicate'])
  })

  it(`limits a group to ${GROUP_MODEL_RATE_MAX_RULES} rules`, () => {
    const rows = Array.from({ length: GROUP_MODEL_RATE_MAX_RULES + 1 }, (_, i) => ({ model: `m-${i}`, rate: 1 }))
    expect(validateGroupModelRates(rows).tooMany).toBe(true)
    expect(validateGroupModelRates(rows.slice(1)).valid).toBe(true)
  })

  it('signature ignores order and case', () => {
    expect(
      groupModelRatesSignature([
        { model: 'B', rate_multiplier: 1 },
        { model: 'a', rate_multiplier: 2 }
      ])
    ).toBe(
      groupModelRatesSignature([
        { model: 'A', rate_multiplier: 2 },
        { model: 'b', rate_multiplier: 1 }
      ])
    )
  })
})
