/**
 * CUSTOM: 首页价格表的数据整理。
 *
 * 口径与模型广场 `PlazaModelPricingTable.vue` 保持一致：实付价 = 渠道单价 × 该模型生效倍率，
 * token 计费按 ¥/1M tokens、按档位展示，按图/按次计费乘对应的独立倍率；排序规则相同。
 * 首页不展示倍率与官方价，分时倍率只提示「见模型广场」。
 */

import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import type { UserPricingInterval } from '@/api/channels'
import { formatScaled, resolveIntervalPrices } from '@/utils/pricing'
import { BALANCE_SYMBOL, effectiveRate as resolveEffectiveRate } from '@/utils/balanceCurrency'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_TOKEN,
  BILLING_MODE_VIDEO,
  type BillingMode
} from '@/constants/channel'

const PER_MILLION = 1_000_000
const MIN_DECIMALS = 2

export interface QxPriceTier {
  /** 上下文档位，如「≤272K」；无档位时为空串。 */
  label: string
  input: string
  output: string
  cacheRead: string
}

export interface QxPriceRow {
  name: string
  mode: BillingMode
  /** token 计费的各档价格。 */
  tiers: QxPriceTier[]
  /** 按图/按次计费的单价（不含单位后缀）。 */
  unitPrice: string | null
  /** 配置了分时倍率，首页只展示标准时段价。 */
  hasTimePricing: boolean
}

function billingMode(m: PlazaModel): BillingMode {
  return (m.pricing?.billing_mode || BILLING_MODE_TOKEN) as BillingMode
}

function groupRate(g: ModelPlazaGroup): number {
  return resolveEffectiveRate(g.rate_multiplier, g.user_rate_multiplier) ?? g.rate_multiplier
}

/** 每个模型的生效倍率 = 用户专属 ?? 公开（模型单独倍率 ?? 分组倍率）?? 分组生效倍率（旧后端）。 */
function modelRate(g: ModelPlazaGroup, m: PlazaModel): number {
  return m.user_rate_multiplier ?? m.rate_multiplier ?? groupRate(g)
}

function requestRate(g: ModelPlazaGroup, m: PlazaModel): number {
  if (billingMode(m) === BILLING_MODE_VIDEO && g.video_rate_independent) {
    return Math.max(0, g.video_rate_multiplier ?? 1)
  }
  return billingMode(m) === BILLING_MODE_IMAGE && g.image_rate_independent
    ? (g.image_rate_multiplier ?? 1)
    : modelRate(g, m)
}

function paidPerMillion(value: number | null | undefined, rate: number): string {
  if (value == null) return '-'
  return formatScaled(value * rate, PER_MILLION, MIN_DECIMALS, BALANCE_SYMBOL)
}

function formatTokenCount(n: number): string {
  const trim = (v: number) => String(Math.round(v * 100) / 100)
  if (n >= 1_000_000) return `${trim(n / 1_000_000)}M`
  if (n >= 1_000) return `${trim(n / 1_000)}K`
  return String(n)
}

export function qxTierLabel(iv: UserPricingInterval): string {
  if (iv.tier_label) return iv.tier_label
  return iv.max_tokens == null ? `>${formatTokenCount(iv.min_tokens)}` : `≤${formatTokenCount(iv.max_tokens)}`
}

function tokenTiers(m: PlazaModel, rate: number): QxPriceTier[] {
  const pricing = m.pricing
  if (!pricing) return []
  const intervals = [...(pricing.intervals ?? [])]
    .sort((a, b) => a.min_tokens - b.min_tokens)
    .map((iv) => resolveIntervalPrices(iv, pricing))
  if (intervals.length === 0) {
    return [{
      label: '',
      input: paidPerMillion(pricing.input_price, rate),
      output: paidPerMillion(pricing.output_price, rate),
      cacheRead: paidPerMillion(pricing.cache_read_price, rate)
    }]
  }
  return intervals.map((iv) => ({
    label: qxTierLabel(iv),
    input: paidPerMillion(iv.input_price, rate),
    output: paidPerMillion(iv.output_price, rate),
    cacheRead: paidPerMillion(iv.cache_read_price, rate)
  }))
}

function unitPrice(g: ModelPlazaGroup, m: PlazaModel): string | null {
  const value = m.pricing?.per_request_price ??
    (m.pricing?.intervals ?? []).find((iv) => iv.per_request_price != null)?.per_request_price
  if (value == null) return null
  return formatScaled(value * requestRate(g, m), 1, MIN_DECIMALS, BALANCE_SYMBOL)
}

/** 与模型广场相同：token 计费在前；组内按官方输出价降序，无官方价靠后；同价按名称降序。 */
function compareModels(a: PlazaModel, b: PlazaModel): number {
  const ta = billingMode(a) === BILLING_MODE_TOKEN
  const tb = billingMode(b) === BILLING_MODE_TOKEN
  if (ta !== tb) return ta ? -1 : 1
  const pa = a.official_pricing?.output_price ?? null
  const pb = b.official_pricing?.output_price ?? null
  if (pa != null && pb != null && pa !== pb) return pb - pa
  if (pa != null && pb == null) return -1
  if (pa == null && pb != null) return 1
  return b.name.localeCompare(a.name)
}

export function buildQxPriceRows(group: ModelPlazaGroup): QxPriceRow[] {
  return [...group.models].sort(compareModels).map((m) => {
    const mode = billingMode(m)
    return {
      name: m.name,
      mode,
      tiers: mode === BILLING_MODE_TOKEN ? tokenTiers(m, modelRate(group, m)) : [],
      unitPrice: mode === BILLING_MODE_TOKEN ? null : unitPrice(group, m),
      hasTimePricing: (m.time_pricing?.periods?.length ?? 0) > 0
    }
  })
}

/** 示例代码里用的模型：第一个有 token 计费模型的分组中，名称最大的那个（通常是最新版本）。 */
export function qxSampleModel(groups: ModelPlazaGroup[]): string | null {
  for (const g of groups) {
    const names = g.models.filter((m) => billingMode(m) === BILLING_MODE_TOKEN).map((m) => m.name)
    if (names.length) return names.sort((a, b) => b.localeCompare(a))[0]
  }
  return null
}

export function qxModelCount(groups: ModelPlazaGroup[]): number {
  return new Set(groups.flatMap((g) => g.models.map((m) => m.name))).size
}
