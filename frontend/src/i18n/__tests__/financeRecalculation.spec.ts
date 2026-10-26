import { describe, expect, it } from 'vitest'
import en from '../locales/en'
import zh from '../locales/zh'
import enRecalculation from '../locales/en/admin/financeRecalculation'
import zhRecalculation from '../locales/zh/admin/financeRecalculation'
import enFinance from '../locales/en/admin/finance'
import zhFinance from '../locales/zh/admin/finance'

function paths(value: Record<string, unknown>, prefix = ''): string[] {
  return Object.entries(value).flatMap(([key, child]) => child && typeof child === 'object'
    ? paths(child as Record<string, unknown>, `${prefix}${key}.`)
    : [`${prefix}${key}`]).sort()
}

describe('historical recalculation locale registration', () => {
  it.each([
    ['en', en, enRecalculation, enFinance],
    ['zh', zh, zhRecalculation, zhFinance]
  ] as const)('%s exposes the independent namespace without replacing existing finance labels', (_name, root, module, finance) => {
    expect(root.admin.financeRecalculation).toEqual(module.financeRecalculation)
    expect(root.admin.finance).toEqual(finance.finance)
    expect(Object.keys(module)).toEqual(['financeRecalculation'])
    expect(Object.keys(finance)).not.toContain('financeRecalculation')
  })

  it('states the baseline limits and the manual stop-traffic/backup duty in both locales', () => {
    const english = en.admin.financeRecalculation
    const chinese = zh.admin.financeRecalculation
    expect(english.baseline.hint).toContain('31 days')
    expect(english.baseline.hint).toContain('2,000 records across all accounts')
    expect(chinese.baseline.hint).toContain('31 天')
    expect(chinese.baseline.hint).toContain('所有账号的记录合计不超过 2000 条')
    expect(english.confirmHint).toContain('does not stop the service or make backups')
    expect(chinese.confirmHint).toContain('停止接收请求并备份数据库')
    expect(chinese.confirmHint).toContain('不会帮你停服务或备份')
  })

  it('keeps every registered English and Chinese key aligned', () => {
    expect(paths(en.admin.financeRecalculation)).toEqual(paths(zh.admin.financeRecalculation))
  })
})
