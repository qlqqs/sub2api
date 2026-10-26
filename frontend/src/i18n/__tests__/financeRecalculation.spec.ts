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

  it('states the baseline full-day limits, other-account scope and manual backup responsibility in both locales', () => {
    const english = en.admin.financeRecalculation.baseline
    const chinese = zh.admin.financeRecalculation.baseline
    expect(english.hint).toContain('31 affected days')
    expect(english.hint).toContain('2,000 retained source rows across those full days, including other accounts')
    expect(english.hint).toContain('Selecting fewer records does not bypass this limit')
    expect(chinese.hint).toContain('31 天')
    expect(chinese.hint).toContain('2000 条现存明细（包括其他账号）')
    expect(chinese.hint).toContain('仅减少选中记录不能绕过此限制')
    expect(english.backup).toContain('does not stop requests or create or verify a backup')
    expect(english.backup).toContain('stop incoming requests and wait for all in-flight requests to finish → back up the database → preview and execute → verify the statistics → resume requests')
    expect(chinese.backup).toContain('不会创建或验证备份')
    expect(chinese.backup).toContain('停止新请求并等待所有在途请求结束 → 备份数据库 → 预览并执行 → 核对统计 → 恢复请求')
  })

  it('keeps every registered English and Chinese key aligned', () => {
    expect(paths(en.admin.financeRecalculation)).toEqual(paths(zh.admin.financeRecalculation))
  })
})
