/**
 * CUSTOM: 财务明细页的日期：统计口径按北京时间，默认范围为北京时间本月 1 日到今天。
 */

const BEIJING_TZ = 'Asia/Shanghai'

/** 某一时刻在北京时间的日期 YYYY-MM-DD。 */
export function beijingDate(now: Date = new Date()): string {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: BEIJING_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  }).formatToParts(now)
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? ''
  return `${get('year')}-${get('month')}-${get('day')}`
}

export function defaultFinanceRange(now: Date = new Date()): { start: string; end: string } {
  const today = beijingDate(now)
  return { start: `${today.slice(0, 8)}01`, end: today }
}

/** 闭区间天数；格式不对返回 NaN。 */
export function inclusiveDays(start: string, end: string): number {
  const re = /^\d{4}-\d{2}-\d{2}$/
  if (!re.test(start) || !re.test(end)) return Number.NaN
  const ms = Date.parse(`${end}T00:00:00Z`) - Date.parse(`${start}T00:00:00Z`)
  return Math.round(ms / 86_400_000) + 1
}
