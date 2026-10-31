import { describe, expect, it } from 'vitest'
import type { UserPricingInterval } from '@/api/channels'
import { videoPriceTiers } from '../videoPricingTiers'

function iv(tier_label: string, per_request_price: number | null): UserPricingInterval {
  return {
    min_tokens: 0, max_tokens: null, tier_label, per_request_price,
    input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
  }
}

describe('videoPriceTiers', () => {
  it('returns no tiers for a single-price model', () => {
    expect(videoPriceTiers({ per_request_price: 0.75, intervals: [] })).toEqual([])
    expect(videoPriceTiers(null)).toEqual([])
  })

  it('uses the base price as 480p and merges -ref tiers by resolution in order', () => {
    const tiers = videoPriceTiers({
      per_request_price: 0.5318,
      intervals: [iv('4k-ref', 4.665), iv('720p', 0.75), iv('480p-ref', 0.5956), iv('4k', 3.75), iv('1080p', 1.875), iv('720p-ref', null)],
    })
    expect(tiers).toEqual([
      { resolution: '480p', price: 0.5318, refPrice: 0.5956 },
      { resolution: '720p', price: 0.75, refPrice: null },
      { resolution: '1080p', price: 1.875, refPrice: null },
      { resolution: '4k', price: 3.75, refPrice: 4.665 },
    ])
  })

  it('prefers an explicit 480p tier and keeps unknown labels last', () => {
    const tiers = videoPriceTiers({ per_request_price: 0.4, intervals: [iv('2k', 2), iv('480p', 0.5)] })
    expect(tiers).toEqual([
      { resolution: '480p', price: 0.5, refPrice: null },
      { resolution: '2k', price: 2, refPrice: null },
    ])
  })
})
