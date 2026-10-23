import { describe, expect, it } from 'vitest'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { buildQxPriceRows, qxModelCount, qxSampleModel, qxTierLabel } from '../pricing'
import { qxProtocolEndpoint } from '../config'

function model(name: string, pricing: Record<string, unknown>, officialOutput: number | null = null): PlazaModel {
  return {
    name,
    platform: 'openai',
    pricing: pricing as unknown as PlazaModel['pricing'],
    official_pricing: officialOutput == null
      ? null
      : { input_price: null, output_price: officialOutput, cache_write_price: null, cache_read_price: null }
  }
}

function group(models: PlazaModel[], overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return {
    id: 1,
    name: 'Codex 稳定分组',
    description: '',
    platform: 'openai',
    subscription_type: 'standard',
    rate_multiplier: 0.35,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    video_rate_independent: false,
    video_rate_multiplier: 1,
    long_context_pricing_enabled: true,
    models,
    ...overrides
  }
}

// 一号机 gpt-6.1-sol 的真实定价（≤272K / >272K 两档）
const tiered = model('gpt-6.1-sol', {
  billing_mode: 'token',
  input_price: 2e-6,
  output_price: 1e-5,
  cache_read_price: 1e-7,
  intervals: [
    { min_tokens: 272000, max_tokens: null, tier_label: '>272K', input_price: 4e-6, output_price: 1.5e-5, cache_read_price: 2e-7 },
    { min_tokens: 0, max_tokens: 272000, tier_label: '≤272K', input_price: 2e-6, output_price: 1e-5, cache_read_price: 1e-7 }
  ]
}, 1e-5)

describe('buildQxPriceRows', () => {
  it('prices each context tier like the model plaza (unit price × group rate, ¥ per 1M tokens)', () => {
    const [row] = buildQxPriceRows(group([tiered]))
    expect(row.tiers).toEqual([
      { label: '≤272K', input: '¥0.70', output: '¥3.50', cacheRead: '¥0.035' },
      { label: '>272K', input: '¥1.40', output: '¥5.25', cacheRead: '¥0.07' }
    ])
    expect(row.unitPrice).toBeNull()
  })

  it('uses the user-specific rate when present', () => {
    const [row] = buildQxPriceRows(group([tiered], { user_rate_multiplier: 0.5 }))
    expect(row.tiers[0].input).toBe('¥1.00')
  })

  it('uses each model row rate before the group rate', () => {
    const cheap = { ...tiered, name: 'cheap', rate_multiplier: 0.2 }
    const exclusive = { ...tiered, name: 'exclusive', rate_multiplier: 0.35, user_rate_multiplier: 0.5 }
    const rows = buildQxPriceRows(group([cheap, exclusive], { user_rate_multiplier: 0.9 }))
    expect(rows.find((r) => r.name === 'cheap')?.tiers[0].input).toBe('¥0.40')
    expect(rows.find((r) => r.name === 'exclusive')?.tiers[0].input).toBe('¥1.00')
  })

  it('prices per-request models with the model rate unless an independent rate applies', () => {
    const [row] = buildQxPriceRows(group([{ ...model('search', { billing_mode: 'per_request', per_request_price: 0.1 }), rate_multiplier: 3 }]))
    expect(row.unitPrice).toBe('¥0.30')
  })

  it('falls back to base prices for tiers that only carry multipliers', () => {
    const [row] = buildQxPriceRows(group([model('m', {
      billing_mode: 'token',
      input_price: 1e-6,
      output_price: 2e-6,
      cache_read_price: null,
      intervals: [{ min_tokens: 0, max_tokens: 128000, input_multiplier: 2, output_multiplier: null }]
    })], { rate_multiplier: 1 }))
    expect(row.tiers).toEqual([{ label: '≤128K', input: '¥2.00', output: '¥2.00', cacheRead: '-' }])
  })

  it('shows a single unlabeled tier for flat token pricing', () => {
    const [row] = buildQxPriceRows(group([model('flat', { billing_mode: 'token', input_price: 1e-6, output_price: 3e-6, cache_read_price: null })], { rate_multiplier: 1 }))
    expect(row.tiers).toEqual([{ label: '', input: '¥1.00', output: '¥3.00', cacheRead: '-' }])
  })

  it('prices image models per unit with the independent image rate', () => {
    const [row] = buildQxPriceRows(group([model('gpt-image', { billing_mode: 'image', per_request_price: 0.2 })], {
      image_rate_independent: true,
      image_rate_multiplier: 2
    }))
    expect(row.tiers).toEqual([])
    expect(row.unitPrice).toBe('¥0.40')
  })

  it('orders token models first, then by official output price, then by name', () => {
    const rows = buildQxPriceRows(group([
      model('img', { billing_mode: 'image', per_request_price: 0.1 }, 99),
      model('cheap', { billing_mode: 'token', input_price: 1e-7, output_price: 1e-7 }, 1e-6),
      model('no-official', { billing_mode: 'token', input_price: 1e-7, output_price: 1e-7 }),
      model('pricey', { billing_mode: 'token', input_price: 1e-7, output_price: 1e-7 }, 5e-5)
    ]))
    expect(rows.map((r) => r.name)).toEqual(['pricey', 'cheap', 'no-official', 'img'])
  })

  it('flags models with time-of-day pricing', () => {
    const timed = { ...tiered, time_pricing: { timezone: 'Asia/Shanghai', periods: [{ start_time: '00:00', end_time: '08:00', multiplier: 0.5 }] } }
    expect(buildQxPriceRows(group([timed]))[0].hasTimePricing).toBe(true)
  })
})

describe('helpers', () => {
  it('builds tier labels like the model plaza', () => {
    expect(qxTierLabel({ min_tokens: 0, max_tokens: 200000 } as never)).toBe('≤200K')
    expect(qxTierLabel({ min_tokens: 1000000, max_tokens: null } as never)).toBe('>1M')
  })

  it('picks the newest token model as the sample and counts distinct models', () => {
    const groups = [
      group([model('gpt-6-sol', { billing_mode: 'token' }), model('gpt-6.1-sol', { billing_mode: 'token' }), model('gpt-5.6-luna', { billing_mode: 'token' })]),
      group([model('gpt-6-sol', { billing_mode: 'token' })], { id: 2 })
    ]
    expect(qxSampleModel(groups)).toBe('gpt-6.1-sol')
    expect(qxModelCount(groups)).toBe(3)
    expect(qxSampleModel([])).toBeNull()
  })

  it('derives per-protocol base URLs from the API origin', () => {
    expect(qxProtocolEndpoint('openai', 'https://api.example.com/')).toBe('https://api.example.com/v1')
    expect(qxProtocolEndpoint('openai', 'https://api.example.com/v1')).toBe('https://api.example.com/v1')
    expect(qxProtocolEndpoint('anthropic', 'https://api.example.com/v1')).toBe('https://api.example.com')
  })
})
