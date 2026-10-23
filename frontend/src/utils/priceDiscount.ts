/**
 * CUSTOM: 「相比官方价」折扣的唯一计算与格式化处（模型广场、用户端使用记录共用）。
 *
 * 本站价是余额单位 ¥，官方价是 $，两者不同币种，必须经美元汇率换算才能比较：
 *   ratio    = 本站 ¥ ÷ 官方 $（即「¥ratio = $1」）
 *   discount = 1 − ratio ÷ fx（fx = 1 美元兑人民币）
 *
 * 显示规则：|discount| < 0.5% →「≈ 官方价」（调用方传入文案）；
 * discount > 0 → `-95%`；discount < 0 → `+5%`；比例或汇率无效（≤ 0、NaN）→「—」。
 * 汇率只用于显示，不参与计费。
 */

import { BALANCE_SYMBOL, OFFICIAL_SYMBOL, formatRateValue } from './balanceCurrency'

/** 后端未提供（旧版本）或值无效时使用的默认汇率，与后端默认值一致。 */
export const DEFAULT_USD_CNY_RATE = 6.8

/** 无法计算折扣时的占位。 */
export const DISCOUNT_UNAVAILABLE = '—'

/** 低于该绝对值的折扣视为与官方价持平（四舍五入后为 0%）。 */
const SAME_PRICE_THRESHOLD = 0.005

function isPositiveFinite(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
}

/** 公开设置里的 `usd_cny_rate`：缺省、≤ 0 或非数字时按默认 6.8。 */
export function resolveUsdCnyRate(raw: unknown): number {
  const n = typeof raw === 'string' && raw.trim() !== '' ? Number(raw) : raw
  return isPositiveFinite(n) ? n : DEFAULT_USD_CNY_RATE
}

/** 折扣 d = 1 − ratio ÷ fx；比例或汇率无效时返回 null。 */
export function priceDiscount(ratio: number | null | undefined, fx: number | null | undefined): number | null {
  if (!isPositiveFinite(ratio) || !isPositiveFinite(fx)) return null
  return 1 - ratio / fx
}

/**
 * 折扣文案：`-95%` / `+5%` / sameLabel / `—`。
 * sameLabel 由调用方给出（模型广场为「≈ 官方价」，使用记录为「≈ 原价」）。
 */
export function formatDiscount(discount: number | null | undefined, sameLabel: string): string {
  if (discount == null || !Number.isFinite(discount)) return DISCOUNT_UNAVAILABLE
  if (Math.abs(discount) < SAME_PRICE_THRESHOLD) return sameLabel
  const percent = Math.round(Math.abs(discount) * 100)
  return discount > 0 ? `-${percent}%` : `+${percent}%`
}

export type DiscountTone = 'cheaper' | 'pricier' | 'same' | 'none'

/** 折扣的视觉语义：便宜 / 更贵 / 持平 / 无法计算。 */
export function discountTone(discount: number | null | undefined): DiscountTone {
  if (discount == null || !Number.isFinite(discount)) return 'none'
  if (Math.abs(discount) < SAME_PRICE_THRESHOLD) return 'same'
  return discount > 0 ? 'cheaper' : 'pricier'
}

/** 换算依据里的汇率写法：`$1 = ¥6.8`。 */
export function formatFxRate(fx: number): string {
  return `${OFFICIAL_SYMBOL}1 = ${BALANCE_SYMBOL}${formatRateValue(fx)}`
}

/**
 * 两个比例（如输入价、输出价各自折算）是否一致：相对差 ≤ tolerance（默认 1%）。
 * 两者都为 0 视为一致。
 */
export function ratiosAgree(a: number, b: number, tolerance = 0.01): boolean {
  const max = Math.max(Math.abs(a), Math.abs(b))
  if (max === 0) return true
  return Math.abs(a - b) / max <= tolerance
}

export interface PeakAdjustment {
  /** up = 上浮，down = 下调，none = 不变或无效。 */
  direction: 'up' | 'down' | 'none'
  /** 调整幅度百分数（最多 1 位小数，去掉尾零），如 "50"、"33.3"。 */
  percent: string
}

/**
 * 高峰倍率 → 用户可读的「价格上浮 / 下调 x%」，不暴露倍率数字。
 * 1.5 → up 50；0.8 → down 20；1、缺省或无效 → none。
 */
export function peakAdjustment(multiplier: number | null | undefined): PeakAdjustment {
  if (multiplier == null || !Number.isFinite(multiplier) || multiplier < 0) {
    return { direction: 'none', percent: '0' }
  }
  const percent = Number((Math.abs(multiplier - 1) * 100).toFixed(1))
  if (percent === 0) return { direction: 'none', percent: '0' }
  return { direction: multiplier > 1 ? 'up' : 'down', percent: String(percent) }
}
