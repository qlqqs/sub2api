/** Shared cost editor adapters. Keep API rules in their original order and only convert edited entries. */
import type { ChannelModelPricing } from '@/api/admin/channels'
import { apiIntervalsToForm, apiTimePricingToForm, formIntervalsToAPI, formReasoningEffortMultipliersToAPI, formTimePricingToAPI, mTokToPerToken, perTokenToMTok, toNullableNumber, type PricingFormEntry } from './types'

export function statsPricingToForm(p: ChannelModelPricing): PricingFormEntry {
  return {
    ...p, models: [...p.models],
    input_price: perTokenToMTok(p.input_price), output_price: perTokenToMTok(p.output_price),
    cache_write_price: perTokenToMTok(p.cache_write_price), cache_write_1h_price: perTokenToMTok(p.cache_write_1h_price),
    cache_read_price: perTokenToMTok(p.cache_read_price), image_input_price: perTokenToMTok(p.image_input_price),
    image_output_price: perTokenToMTok(p.image_output_price),
    reasoning_effort_multipliers: p.reasoning_effort_multipliers ? { ...p.reasoning_effort_multipliers } : null,
    intervals: apiIntervalsToForm(p.intervals), time_pricing: apiTimePricingToForm(p.time_pricing)
  }
}
export function statsPricingToAPI(p: PricingFormEntry, original: ChannelModelPricing): ChannelModelPricing {
  return {
    ...original, models: [...p.models], billing_mode: p.billing_mode,
    input_price: mTokToPerToken(p.input_price), output_price: mTokToPerToken(p.output_price),
    cache_write_price: mTokToPerToken(p.cache_write_price), cache_write_1h_price: mTokToPerToken(p.cache_write_1h_price),
    cache_read_price: mTokToPerToken(p.cache_read_price), image_input_price: mTokToPerToken(p.image_input_price),
    image_output_price: mTokToPerToken(p.image_output_price), per_request_price: toNullableNumber(p.per_request_price),
    fast_multiplier: toNullableNumber(p.fast_multiplier), flex_multiplier: toNullableNumber(p.flex_multiplier),
    reasoning_effort_multipliers: formReasoningEffortMultipliersToAPI(p.reasoning_effort_multipliers),
    intervals: formIntervalsToAPI(p.intervals), time_pricing: formTimePricingToAPI(p.time_pricing)
  }
}
export function emptyStatsPricing(platform: string): ChannelModelPricing {
  return { platform, models: [], billing_mode: 'token', input_price: null, output_price: null,
    cache_write_price: null, cache_write_1h_price: null, cache_read_price: null,
    image_input_price: null, image_output_price: null, per_request_price: null, intervals: [], time_pricing: null }
}
