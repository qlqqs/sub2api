import type { UserPricingInterval } from '@/api/channels'
import { VIDEO_BASE_RESOLUTION, VIDEO_REF_TIER_SUFFIX, VIDEO_RESOLUTIONS } from '@/constants/channel'

export interface VideoPriceTier {
  resolution: string
  /** 普通单价；未配置为 null。 */
  price: number | null
  /** 带参考视频的单价；未配置为 null。 */
  refPrice: number | null
}

interface VideoPricingLike {
  per_request_price: number | null
  intervals?: UserPricingInterval[] | null
}

/**
 * 视频分档按分辨率合并：基础价是 480p 普通价，`<分辨率>-ref` 档并入同分辨率的 refPrice。
 * 没有分档时返回 []（按单一价格展示）。顺序为 480p → 4k，未知标签按出现顺序排在后面。
 */
export function videoPriceTiers(pricing: VideoPricingLike | null | undefined): VideoPriceTier[] {
  const intervals = (pricing?.intervals ?? []).filter((iv) => iv.tier_label && iv.per_request_price != null)
  if (intervals.length === 0) return []

  const tiers = new Map<string, VideoPriceTier>()
  const tierOf = (resolution: string) => {
    let tier = tiers.get(resolution)
    if (!tier) {
      tier = { resolution, price: null, refPrice: null }
      tiers.set(resolution, tier)
    }
    return tier
  }
  if (pricing?.per_request_price != null) tierOf(VIDEO_BASE_RESOLUTION).price = pricing.per_request_price
  for (const iv of intervals) {
    const label = iv.tier_label!
    if (label.endsWith(VIDEO_REF_TIER_SUFFIX)) {
      tierOf(label.slice(0, -VIDEO_REF_TIER_SUFFIX.length)).refPrice = iv.per_request_price
    } else {
      tierOf(label).price = iv.per_request_price
    }
  }

  const rank = (resolution: string) => {
    const i = (VIDEO_RESOLUTIONS as readonly string[]).indexOf(resolution)
    return i < 0 ? VIDEO_RESOLUTIONS.length : i
  }
  return [...tiers.values()].sort((a, b) => rank(a.resolution) - rank(b.resolution))
}
