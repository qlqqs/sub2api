/**
 * 高峰时段倍率的共享展示逻辑。
 *
 * 高峰窗口由后端按服务器全局时区判定（Group.PeakMultiplierAt），
 * 前端展示必须带上服务器时区标注（来自公共设置 server_utc_offset），
 * 避免用户按浏览器本地时间误读计费窗口。
 */

import { formatRateRatio } from './balanceCurrency'

export interface PeakRateFields {
  peak_rate_enabled?: boolean
  peak_start?: string
  peak_end?: string
  peak_rate_multiplier?: number
}

export function hasPeakRate(fields?: PeakRateFields | null): boolean {
  return Boolean(fields?.peak_rate_enabled && fields.peak_start && fields.peak_end)
}

/** "+08:00" → "UTC+08:00"；旧缓存无该字段时返回空串，调用方降级为不带时区标注 */
export function serverTimezoneLabel(utcOffset?: string | null): string {
  return utcOffset ? `UTC${utcOffset}` : ''
}

/**
 * 高峰时段的生效倍率 = 基础生效倍率（专属 ?? 分组）× 高峰倍率，去掉浮点噪声。
 * baseRate 缺省按 1 处理。
 */
export function peakEffectiveRate(
  fields: PeakRateFields | null | undefined,
  baseRate?: number | null
): number {
  const base = baseRate ?? 1
  return Math.round(base * (fields?.peak_rate_multiplier ?? 1) * 10000) / 10000
}

/**
 * CUSTOM: "14:00-18:00 ¥1 = $1 (UTC+08:00)"，tzLabel 为空时省略括号部分。
 * 比例按该时段的生效倍率（baseRate × 高峰倍率）展示，见 utils/balanceCurrency.ts。
 */
export function formatPeakRateWindow(
  fields: PeakRateFields | null | undefined,
  tzLabel?: string,
  baseRate?: number | null
): string {
  if (!hasPeakRate(fields) || !fields) return ''
  const base = `${fields.peak_start}-${fields.peak_end} ${formatRateRatio(peakEffectiveRate(fields, baseRate))}`
  return tzLabel ? `${base} (${tzLabel})` : base
}

/** CUSTOM: 仅时间窗口 "14:00-18:00 (UTC+08:00)"（不含比例），用于另行说明高峰倍率的文案。 */
export function formatPeakTimeWindow(
  fields: PeakRateFields | null | undefined,
  tzLabel?: string
): string {
  if (!hasPeakRate(fields) || !fields) return ''
  const base = `${fields.peak_start}-${fields.peak_end}`
  return tzLabel ? `${base} (${tzLabel})` : base
}
