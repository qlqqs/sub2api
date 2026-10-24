import type { PlazaTimePricing, PlazaTimePricingPeriod } from '@/api/modelPlaza'

/** 同一行两套价格的时段倍率。官方时区无法对齐时不计算该行折扣。 */
export interface PlazaPriceTimeRow {
  period: PlazaTimePricingPeriod | null
  schedule?: PlazaTimePricing
  weekendsOnly?: boolean
  paidMultiplier: number
  officialMultiplier: number | null
}

function second(clock: string, end = false): number {
  const [hour, minute, seconds = 0] = clock.split(':').map(Number)
  const value = hour * 3600 + minute * 60 + seconds
  return end && value === 0 ? 86400 : value
}

function clock(value: number): string {
  return [Math.floor(value / 3600) % 24, Math.floor(value / 60) % 60, value % 60]
    .map(n => String(n).padStart(2, '0')).join(':')
}

/** 切分双方窗口的交集，不能将渠道的倍率直接用于官方参考价。 */
export function plazaPriceTimeRows(paid?: PlazaTimePricing, official?: PlazaTimePricing): PlazaPriceTimeRow[] {
  const paidSchedule = paid?.periods.length ? paid : undefined
  const officialSchedule = official?.periods.length ? official : undefined
  const base: PlazaPriceTimeRow = { period: null, paidMultiplier: 1, officialMultiplier: 1 }
  if (!officialSchedule || (paidSchedule && paidSchedule.timezone !== officialSchedule.timezone)) {
    const officialMultiplier = officialSchedule ? null : 1
    return [
      { ...base, officialMultiplier },
      ...(paidSchedule?.periods ?? []).map(period => ({
        period, schedule: paidSchedule, paidMultiplier: period.multiplier, officialMultiplier
      }))
    ]
  }

  const schedules = [paidSchedule, officialSchedule].filter((s): s is PlazaTimePricing => !!s)
  const allWeekdays = schedules.every(s => s.weekdays_only)
  const mixedDays = !allWeekdays && schedules.some(s => s.weekdays_only)
  const days: Array<'all' | 'weekdays' | 'weekends'> = mixedDays ? ['weekdays', 'weekends'] : [allWeekdays ? 'weekdays' : 'all']
  const result = [base]
  for (const day of days) {
    const periods = (s?: PlazaTimePricing) => day === 'weekends' && s?.weekdays_only ? [] : s?.periods ?? []
    const paidPeriods = periods(paidSchedule)
    const officialPeriods = periods(officialSchedule)
    const boundaries = [...new Set([0, 86400, ...[...paidPeriods, ...officialPeriods]
      .flatMap(p => [second(p.start_time), second(p.end_time, true)])])].sort((a, b) => a - b)
    const multiplierAt = (ps: PlazaTimePricingPeriod[], at: number) =>
      ps.find(p => at >= second(p.start_time) && at < second(p.end_time, true))?.multiplier ?? 1
    let previous: PlazaPriceTimeRow | undefined
    for (let i = 0; i < boundaries.length - 1; i++) {
      const start = boundaries[i]
      const end = boundaries[i + 1]
      const paidMultiplier = multiplierAt(paidPeriods, start)
      const officialMultiplier = multiplierAt(officialPeriods, start)
      if (paidMultiplier === 1 && officialMultiplier === 1) {
        previous = undefined
        continue
      }
      if (previous?.period && previous.paidMultiplier === paidMultiplier && previous.officialMultiplier === officialMultiplier) {
        previous.period.end_time = clock(end)
        continue
      }
      previous = {
        period: { start_time: clock(start), end_time: clock(end), multiplier: paidMultiplier },
        schedule: { timezone: officialSchedule.timezone, weekdays_only: day === 'weekdays', periods: [] },
        weekendsOnly: day === 'weekends', paidMultiplier, officialMultiplier
      }
      result.push(previous)
    }
  }
  return result
}
