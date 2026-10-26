import { describe, expect, it } from 'vitest'
import { emptyStatsPricing, statsPricingToAPI, statsPricingToForm } from '../accountStatsPricing'

describe('shared account statistics pricing adapters', () => {
  it('preserves platform, zero prices, advanced multipliers, time scope and interval order', () => {
    const pricing = {
      ...emptyStatsPricing('openai'), id: 9, models: ['gpt-*'], input_price: 0.000003, cache_write_1h_price: 0,
      fast_multiplier: 2, flex_multiplier: 0.5, reasoning_effort_multipliers: { high: 3 },
      time_pricing: { timezone: 'Asia/Shanghai', weekdays_only: true, periods: [{ start_time: '12:00:00', end_time: '14:00:00', multiplier: 2 }] },
      intervals: [
        { min_tokens: 100, max_tokens: null, tier_label: 'second', input_price: 0, output_price: null, cache_write_price: null, cache_write_1h_price: 0.000001, cache_read_price: null, input_multiplier: 2, output_multiplier: null, cache_write_multiplier: null, cache_read_multiplier: null, per_request_price: null, sort_order: 5 },
        { min_tokens: 0, max_tokens: 100, tier_label: 'first', input_price: 0.000002, output_price: null, cache_write_price: null, cache_write_1h_price: null, cache_read_price: null, input_multiplier: null, output_multiplier: null, cache_write_multiplier: null, cache_read_multiplier: null, per_request_price: null, sort_order: 2 }
      ]
    }
    const form = statsPricingToForm(pricing)
    expect(form.input_price).toBe(3)
    expect(form.cache_write_1h_price).toBe(0)
    const roundTrip = statsPricingToAPI(form, pricing)
    expect(roundTrip).toEqual(pricing)
    expect(roundTrip.intervals.map(i => i.sort_order)).toEqual([5, 2])
    expect(pricing.input_price).toBe(0.000003)
  })
  it('does not convert per-request USD prices to token prices', () => {
    const pricing = { ...emptyStatsPricing('gemini'), billing_mode: 'image' as const, models: ['image'], per_request_price: 0.25 }
    expect(statsPricingToAPI(statsPricingToForm(pricing), pricing).per_request_price).toBe(0.25)
  })
})
