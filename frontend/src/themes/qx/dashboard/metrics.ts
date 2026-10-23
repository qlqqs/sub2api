/**
 * CUSTOM: QxAPI 主题仪表盘的纯函数：金额 / 数字格式、迷你折线、近 N 天的时间范围、模型分布。
 */
import type { ModelStat } from '@/types'
import { BALANCE_SYMBOL, OFFICIAL_SYMBOL } from '@/utils/balanceCurrency'
import { formatDateLocalInput } from '@/utils/format'

function decimalsFormatter(min: number, max: number, grouping = true) {
  return new Intl.NumberFormat('en-US', { minimumFractionDigits: min, maximumFractionDigits: max, useGrouping: grouping })
}
const bigMoney = decimalsFormatter(2, 2)
const smallMoney = decimalsFormatter(2, 4)

/** 金额数字：≥1 保留 2 位，<1 最多 4 位（小额扣费不被抹成 0.00）。 */
export function formatMoneyValue(amount: number | null | undefined): string {
  const n = typeof amount === 'number' && Number.isFinite(amount) ? amount : 0
  const body = (Math.abs(n) >= 1 ? bigMoney : smallMoney).format(Math.abs(n))
  return `${n < 0 && Number(body.replace(/,/g, '')) !== 0 ? '-' : ''}${body}`
}

/** 余额单位金额（¥）。 */
export function formatActual(amount: number | null | undefined): string {
  const v = formatMoneyValue(amount)
  return v.startsWith('-') ? `-${BALANCE_SYMBOL}${v.slice(1)}` : `${BALANCE_SYMBOL}${v}`
}

/** 官方价金额（$）。 */
export function formatOfficial(amount: number | null | undefined): string {
  const v = formatMoneyValue(amount)
  return v.startsWith('-') ? `-${OFFICIAL_SYMBOL}${v.slice(1)}` : `${OFFICIAL_SYMBOL}${v}`
}

/** 1.2K / 3.4M / 5.6B */
export function formatCompact(n: number | null | undefined): string {
  const v = typeof n === 'number' && Number.isFinite(n) ? n : 0
  const abs = Math.abs(v)
  const trim = (x: string) => x.replace(/\.?0+$/, '')
  if (abs >= 1e9) return `${trim((v / 1e9).toFixed(2))}B`
  if (abs >= 1e6) return `${trim((v / 1e6).toFixed(2))}M`
  if (abs >= 1e3) return `${trim((v / 1e3).toFixed(1))}K`
  return String(Math.round(v * 10) / 10)
}

export function formatCount(n: number | null | undefined): string {
  return (typeof n === 'number' && Number.isFinite(n) ? n : 0).toLocaleString('en-US')
}

export function formatDuration(ms: number | null | undefined): string {
  const v = typeof ms === 'number' && Number.isFinite(ms) ? ms : 0
  return v >= 1000 ? `${(v / 1000).toFixed(2)}s` : `${v.toFixed(0)}ms`
}

/** 迷你折线的 points 属性；少于 2 个点时返回空串（不画）。 */
export function sparklinePoints(values: number[], width = 84, height = 28, pad = 3): string {
  if (values.length < 2) return ''
  const max = Math.max(...values)
  const min = Math.min(...values)
  const span = max - min
  return values
    .map((v, i) => {
      const x = (i * width) / (values.length - 1)
      const y = span === 0 ? height / 2 : pad + (1 - (v - min) / span) * (height - pad * 2)
      return `${+x.toFixed(1)},${+y.toFixed(1)}`
    })
    .join(' ')
}

/** 迷你折线和模型分布取数的时间范围：含今天在内的最近 N 天，按天汇总。 */
export function recentDaysRange(days: number, now: Date = new Date()): { startDate: string; endDate: string } {
  return {
    startDate: formatDateLocalInput(new Date(now.getTime() - (days - 1) * 86400000)),
    endDate: formatDateLocalInput(now)
  }
}

export interface ModelMixRow {
  model: string
  actual_cost: number
  cost: number
  total_tokens: number
  requests: number
  /** 0–100 的整数，按实际花费占比；花费全为 0 时按 Token 占比。 */
  share: number
}

export function buildModelMix(models: ModelStat[] | null | undefined, limit = 6): { rows: ModelMixRow[]; rest: number } {
  const list = [...(models ?? [])].sort((a, b) => b.actual_cost - a.actual_cost || b.total_tokens - a.total_tokens)
  const totalCost = list.reduce((s, m) => s + (m.actual_cost || 0), 0)
  const totalTokens = list.reduce((s, m) => s + (m.total_tokens || 0), 0)
  const byCost = totalCost > 0
  const rows = list.slice(0, limit).map((m) => {
    const part = byCost ? m.actual_cost : m.total_tokens
    const whole = byCost ? totalCost : totalTokens
    return {
      model: m.model,
      actual_cost: m.actual_cost,
      cost: m.cost,
      total_tokens: m.total_tokens,
      requests: m.requests,
      share: whole > 0 ? Math.round((part / whole) * 100) : 0
    }
  })
  return { rows, rest: Math.max(0, list.length - rows.length) }
}
