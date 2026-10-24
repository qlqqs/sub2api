import { describe, expect, it } from 'vitest'
import { plazaPriceTimeRows } from '../modelPlazaTimePricing'
import type { PlazaTimePricing } from '@/api/modelPlaza'

const official: PlazaTimePricing = {
  timezone: 'Asia/Shanghai', weekdays_only: true,
  periods: [
    { start_time: '09:00', end_time: '12:00', multiplier: 2 },
    { start_time: '14:00', end_time: '18:00', multiplier: 2 }
  ]
}

describe('plazaPriceTimeRows', () => {
  it('官方分时可独立于固定本站价格展开，不改变本站倍率', () => {
    const rows = plazaPriceTimeRows(undefined, official)
    expect(rows).toHaveLength(3)
    expect(rows.map(r => [r.paidMultiplier, r.officialMultiplier])).toEqual([[1, 1], [1, 2], [1, 2]])
    expect(rows[1].schedule?.weekdays_only).toBe(true)
  })

  it('合并相同窗口，兼容 HH:mm 与 HH:mm:ss，且不重复乘倍率', () => {
    const paid = { ...official, periods: official.periods.map(p => ({ ...p, start_time: p.start_time + ':00' })) }
    const rows = plazaPriceTimeRows(paid, official)
    expect(rows).toHaveLength(3)
    expect(rows[1]).toMatchObject({ paidMultiplier: 2, officialMultiplier: 2 })
    expect(rows[1].period).toMatchObject({ start_time: '09:00:00', end_time: '12:00:00' })
  })

  it('按交集切分不同窗口，不能误把渠道时段当官方时段', () => {
    const paid = { ...official, periods: [{ start_time: '10:00', end_time: '15:00', multiplier: 0.5 }] }
    expect(plazaPriceTimeRows(paid, official).slice(1).map(r => [r.period?.start_time, r.period?.end_time, r.paidMultiplier, r.officialMultiplier]))
      .toEqual([
        ['09:00:00', '10:00:00', 1, 2], ['10:00:00', '12:00:00', 0.5, 2],
        ['12:00:00', '14:00:00', 0.5, 1], ['14:00:00', '15:00:00', 0.5, 2],
        ['15:00:00', '18:00:00', 1, 2]
      ])
  })

  it('每日渠道窗口和仅工作日官方窗口分别给出工作日和周末的价格', () => {
    const paid = { ...official, weekdays_only: false }
    const rows = plazaPriceTimeRows(paid, official)
    expect(rows).toHaveLength(5)
    expect(rows.slice(1, 3).every(r => r.schedule?.weekdays_only && r.officialMultiplier === 2)).toBe(true)
    expect(rows.slice(3).every(r => r.weekendsOnly && r.paidMultiplier === 2 && r.officialMultiplier === 1)).toBe(true)
  })

  it('跨午夜的结束时间 00:00 表示一天结束', () => {
    const schedule = { timezone: 'Asia/Shanghai', periods: [{ start_time: '22:00', end_time: '00:00', multiplier: 2 }] }
    expect(plazaPriceTimeRows(undefined, schedule)[1].period).toMatchObject({ start_time: '22:00:00', end_time: '00:00:00' })
  })

  it('不同时区不伪造同时段折扣；保留各自的原始配置', () => {
    const paid = { ...official, timezone: 'UTC' }
    const original = JSON.stringify({ paid, official })
    const rows = plazaPriceTimeRows(paid, official)
    expect(rows).toHaveLength(3)
    expect(rows.every(r => r.officialMultiplier === null)).toBe(true)
    expect(JSON.stringify({ paid, official })).toBe(original)
  })

  it('无官方时段的旧接口仍仅使用渠道分时', () => {
    expect(plazaPriceTimeRows(official).map(r => [r.paidMultiplier, r.officialMultiplier]))
      .toEqual([[1, 1], [2, 1], [2, 1]])
    expect(plazaPriceTimeRows()).toEqual([{ period: null, paidMultiplier: 1, officialMultiplier: 1 }])
  })
})
