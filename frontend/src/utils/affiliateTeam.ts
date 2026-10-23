/**
 * 代理团队消费页的展示格式（金额、占比、用户名）。金额一律走 formatBalance，不手写货币符号。
 */
import type { AffiliateTeamUser } from '@/types'
import { formatBalance } from '@/utils/balanceCurrency'

/** 汇总 / 列表金额：两位小数 + 千分位；不足 0.01 的正数与占比的「<1%」一样显示「<¥0.01」，不显示成 0.00。 */
export function formatTeamMoney(amount: number | null | undefined): string {
  const n = amount ?? 0
  if (n > 0 && n < 0.01) return `<${formatBalance(0.01, { decimals: 2 })}`
  return formatBalance(n, { decimals: 2, grouping: true })
}

/** 单条请求扣费：四位小数。 */
export function formatTeamCharge(amount: number | null | undefined): string {
  return formatBalance(amount, { decimals: 4, grouping: true })
}

export function formatTeamCount(value: number | null | undefined): string {
  return (value ?? 0).toLocaleString('en-US')
}

/** 占比 0-1 → 整数百分比；大于 0 但不足 1% 显示 <1%。 */
export function formatTeamShare(share: number): string {
  if (!Number.isFinite(share) || share <= 0) return '0%'
  if (share < 0.01) return '<1%'
  return `${Math.round(share * 100)}%`
}

/** 比率 0-1 → 一位小数的百分比（分润率这类通常不到 50% 的比例）；大于 0 但不足 0.1% 显示 <0.1%。 */
export function formatTeamRate(rate: number): string {
  if (!Number.isFinite(rate) || rate <= 0) return '0%'
  if (rate < 0.001) return '<0.1%'
  return `${(rate * 100).toFixed(1)}%`
}

/** 底条宽度（CSS 百分比），限定在 0–100%。 */
export function teamShareWidth(share: number): string {
  const pct = Number.isFinite(share) ? Math.min(Math.max(share, 0), 1) * 100 : 0
  return `${pct.toFixed(1)}%`
}

/** 叙述性位置（面包屑、「X 的客户」）的名称：昵称，其次脱敏邮箱；已删除返回 deletedLabel。 */
export function teamUserName(user: AffiliateTeamUser, deletedLabel: string): string {
  if (user.deleted) return deletedLabel
  return user.nickname || user.email || '-'
}
