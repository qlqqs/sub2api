import { describe, expect, it } from 'vitest'
import { accountModelCostSignature, validateAccountModelCosts } from '../accountModelCost'

describe('accountModelCost (CUSTOM)', () => {
  it('accepts zero, trailing wildcards and up to 4 decimals; ignores blank rows', () => {
    const result = validateAccountModelCosts([
      { model: ' gpt-5.6-* ', rate: '0' },
      { model: 'gpt-5.6-luna', rate: '0.3' },
      { model: 'img', rate: 1.2345 },
      { model: '', rate: '' }
    ])
    expect(result.valid).toBe(true)
    expect(result.rules).toEqual([
      { model: 'gpt-5.6-*', rate_multiplier: 0 },
      { model: 'gpt-5.6-luna', rate_multiplier: 0.3 },
      { model: 'img', rate_multiplier: 1.2345 }
    ])
  })

  it('reports per-row errors', () => {
    const result = validateAccountModelCosts([
      { model: 'a*b', rate: '1' },
      { model: 'GPT-X', rate: '1' },
      { model: 'gpt-x', rate: '2' },
      { model: 'm1', rate: '-1' },
      { model: 'm2', rate: '10001' },
      { model: 'm3', rate: '0.12345' },
      { model: 'm4', rate: '' },
      { model: 'has space', rate: '1' }
    ])
    expect(result.valid).toBe(false)
    expect(result.errors).toEqual([
      'wildcardPosition',
      null,
      'duplicate',
      'rateInvalid',
      'rateTooLarge',
      'rateDecimals',
      'rateRequired',
      'modelWhitespace'
    ])
  })

  it('signature ignores order and case', () => {
    expect(
      accountModelCostSignature([
        { model: 'B', rate_multiplier: 1 },
        { model: 'a', rate_multiplier: 0.5 }
      ])
    ).toBe(accountModelCostSignature([{ model: 'A', rate_multiplier: 0.5 }, { model: 'b', rate_multiplier: 1 }]))
  })
})
