/**
 * CUSTOM: 站内余额单位与分组价格比例的唯一定义处。
 *
 * - 余额单位（余额、到账、扣费 actual_cost、各类限额、兑换码、返利等）显示为 ¥，
 *   数字不变、不做汇率换算（充值 ¥1 到账 1 个余额单位）。
 * - 官方单价与「按官方价折合」的标准费用（total_cost）保持 $。
 * - 分组倍率展示为「¥x = $1」，x 为生效倍率（用户专属倍率优先于分组倍率）。
 *
 * 各页面不得再写死 `$` / `¥` 表示余额单位，一律调用本文件导出的工具。
 */

/** 站内余额单位符号。 */
export const BALANCE_SYMBOL = '¥'

/** 官方用量 / 官方单价符号。 */
export const OFFICIAL_SYMBOL = '$'

export interface FormatBalanceOptions {
  /** 固定小数位，默认 2；被 minDecimals / maxDecimals 覆盖。 */
  decimals?: number
  minDecimals?: number
  maxDecimals?: number
  /** 千分位分隔，默认 false。 */
  grouping?: boolean
  /** amount 为 null / undefined / 非有限数时的返回值，默认 `¥0.00`。 */
  fallback?: string
  /** 正数前加 `+`（流水类展示）。 */
  signed?: boolean
}

function toFiniteNumber(value: unknown): number | null {
  if (value === null || value === undefined || value === '') return null
  const n = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(n) ? n : null
}

/**
 * `¥` + 数字；负数为 `-¥1.23`。
 *
 *   formatBalance(1.5)                      → "¥1.50"
 *   formatBalance(-1.234)                   → "-¥1.23"
 *   formatBalance(1234.5, { grouping: true }) → "¥1,234.50"
 *   formatBalance(null)                     → "¥0.00"
 */
export function formatBalance(
  amount: number | string | null | undefined,
  opts: FormatBalanceOptions = {}
): string {
  const n = toFiniteNumber(amount)
  if (n === null) return opts.fallback ?? `${BALANCE_SYMBOL}0.00`
  const decimals = opts.decimals ?? 2
  const minimumFractionDigits = opts.minDecimals ?? decimals
  const maximumFractionDigits = Math.max(opts.maxDecimals ?? decimals, minimumFractionDigits)
  const body = new Intl.NumberFormat('en-US', {
    minimumFractionDigits,
    maximumFractionDigits,
    useGrouping: opts.grouping ?? false
  }).format(Math.abs(n))
  // 舍入后为 0 时不显示负号（-0.001 → ¥0.00）
  const isZero = Number(body.replace(/,/g, '')) === 0
  const sign = n < 0 && !isZero ? '-' : opts.signed && n > 0 && !isZero ? '+' : ''
  return `${sign}${BALANCE_SYMBOL}${body}`
}

/** 倍率数字去掉多余尾零，最多 4 位小数：0.5 → "0.5"，1 → "1"，0.125 → "0.125"。 */
export function formatRateValue(rate: number): string {
  return Number(rate.toFixed(4)).toString()
}

/**
 * 分组价格比例：`¥{x} = $1`（x = 生效倍率，充值倍率固定为 1 不参与计算）。
 * 免费分组（rate = 0）显示 `¥0 = $1`；rate 为 null / 负数 / 非有限数时返回 `-`。
 */
export function formatRateRatio(rate: number | null | undefined): string {
  if (rate === null || rate === undefined || !Number.isFinite(rate) || rate < 0) return '-'
  return `${BALANCE_SYMBOL}${formatRateValue(rate)} = ${OFFICIAL_SYMBOL}1`
}

/** 生效倍率：用户专属倍率优先，没有则用分组倍率。 */
export function effectiveRate(
  groupRate: number | null | undefined,
  userRate: number | null | undefined
): number | null | undefined {
  return userRate ?? groupRate
}
