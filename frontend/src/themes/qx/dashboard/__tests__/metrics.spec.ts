import { describe, expect, it } from 'vitest'
import type { ModelStat } from '@/types'
import { buildModelMix, formatActual, formatCompact, formatDuration, formatOfficial, recentDaysRange, sparklinePoints } from '../metrics'

function model(name: string, actual: number, tokens = 100, requests = 1): ModelStat {
  return { model: name, requests, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: tokens, cost: actual * 3, actual_cost: actual }
}

describe('金额与数字格式', () => {
  it('≥1 保留 2 位，<1 最多 4 位', () => {
    expect(formatActual(88.5)).toBe('¥88.50')
    expect(formatActual(1234.567)).toBe('¥1,234.57')
    expect(formatActual(0.052)).toBe('¥0.052')
    expect(formatActual(0.00004)).toBe('¥0.00')
    expect(formatActual(-0.5)).toBe('-¥0.50')
    expect(formatActual(null)).toBe('¥0.00')
    expect(formatOfficial(9.76)).toBe('$9.76')
  })

  it('紧凑数字去掉多余的 0', () => {
    expect(formatCompact(4_810_000)).toBe('4.81M')
    expect(formatCompact(600_000)).toBe('600K')
    expect(formatCompact(162_400_000)).toBe('162.4M')
    expect(formatCompact(18_200)).toBe('18.2K')
    expect(formatCompact(2_000_000_000)).toBe('2B')
    expect(formatCompact(0.333)).toBe('0.3')
    expect(formatCompact(4)).toBe('4')
  })

  it('响应时间', () => {
    expect(formatDuration(900)).toBe('900ms')
    expect(formatDuration(1530)).toBe('1.53s')
  })
})

describe('sparklinePoints', () => {
  it('按最小/最大值缩放到画布', () => {
    expect(sparklinePoints([0, 10], 84, 28, 3)).toBe('0,25 84,3')
  })
  it('全部相等时画在中线，少于 2 个点不画', () => {
    expect(sparklinePoints([5, 5, 5], 84, 28)).toBe('0,14 42,14 84,14')
    expect(sparklinePoints([5])).toBe('')
  })
})

describe('recentDaysRange', () => {
  it('含今天在内的最近 N 天', () => {
    const now = new Date(2026, 9, 23, 15, 0, 0)
    expect(recentDaysRange(7, now)).toEqual({ startDate: '2026-10-17', endDate: '2026-10-23' })
    expect(recentDaysRange(1, now)).toEqual({ startDate: '2026-10-23', endDate: '2026-10-23' })
  })
})

describe('buildModelMix', () => {
  it('按实际花费排序、计算占比，并截取前 N 个', () => {
    const { rows, rest } = buildModelMix([model('b', 1), model('a', 3), model('c', 0.5), model('d', 0.5)], 3)
    expect(rows.map((r) => r.model)).toEqual(['a', 'b', 'c'])
    expect(rows.map((r) => r.share)).toEqual([60, 20, 10])
    expect(rest).toBe(1)
  })
  it('花费全为 0 时按 Token 占比', () => {
    const { rows } = buildModelMix([model('a', 0, 300), model('b', 0, 100)])
    expect(rows.map((r) => [r.model, r.share])).toEqual([['a', 75], ['b', 25]])
  })
  it('空列表', () => {
    expect(buildModelMix([])).toEqual({ rows: [], rest: 0 })
    expect(buildModelMix(null)).toEqual({ rows: [], rest: 0 })
  })
})
