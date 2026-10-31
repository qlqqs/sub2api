import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PricingEntryCard from '../PricingEntryCard.vue'
import type { IntervalFormEntry, PricingFormEntry } from '../types'

vi.mock('@/api/admin/channels', () => ({ default: { getModelDefaultPricing: vi.fn() } }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

function tier(label: string): IntervalFormEntry {
  return {
    min_tokens: 0, max_tokens: null, tier_label: label, sort_order: 0,
    input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
    input_multiplier: null, output_multiplier: null, cache_write_multiplier: null,
    cache_read_multiplier: null, per_request_price: 1,
  }
}

function videoEntry(labels: string[]): PricingFormEntry {
  return {
    models: [], billing_mode: 'video',
    input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
    fast_multiplier: null, flex_multiplier: null, reasoning_effort_multipliers: null,
    image_input_price: null, image_output_price: null, per_request_price: 0.5318,
    intervals: labels.map(tier),
  }
}

async function addTier(labels: string[]): Promise<string> {
  const wrapper = mount(PricingEntryCard, { props: { entry: videoEntry(labels) }, global: { stubs: { Icon: true } } })
  const add = wrapper.findAll('button').find((b) => b.text().includes('admin.channels.form.addTier'))!
  await add.trigger('click')
  const updated = wrapper.emitted('update')?.at(-1)?.[0] as PricingFormEntry
  return updated.intervals.at(-1)!.tier_label
}

describe('PricingEntryCard video tiers', () => {
  it('adds the next unused tier after the 480p base price', async () => {
    expect(await addTier([])).toBe('720p')
    expect(await addTier(['720p', '1080p'])).toBe('4k')
    expect(await addTier(['720p', '1080p', '4k'])).toBe('480p-ref')
    expect(await addTier(['720p', '1080p', '4k', '480p-ref', '720p-ref', '1080p-ref', '4k-ref'])).toBe('')
  })
})
