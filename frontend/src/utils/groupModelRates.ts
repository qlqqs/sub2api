/**
 * CUSTOM: 分组模型单独倍率的客户端校验，与后端 `GroupModelRateService.ReplaceForGroup` 保持一致：
 * - 模型名去空格后非空、中间不含空白，≤ 255 字符；`*` 只允许在末尾且前面至少一个字符；
 * - 同组内按归一化后的模型名（小写，`claude-` 前缀的 `.` 换成 `-`）不重复；
 * - 0 < 倍率 ≤ 10000，且按 4 位小数落库后仍 > 0；单组最多 200 条。
 * 完全空白的行（模型名与倍率都没填）忽略。
 */

import type { GroupModelRateRule } from '@/api/admin/groupModelRates'

export const GROUP_MODEL_RATE_MAX_RULES = 200
export const GROUP_MODEL_RATE_MAX = 10000
export const GROUP_MODEL_PATTERN_MAX_LENGTH = 255

export type GroupModelRateRowError =
  | 'modelRequired'
  | 'modelTooLong'
  | 'modelWhitespace'
  | 'wildcardPosition'
  | 'duplicate'
  | 'rateRequired'
  | 'rateInvalid'
  | 'rateTooLarge'
  | 'rateTooSmall'

export interface GroupModelRateDraft {
  model: string
  rate: string | number | null | undefined
}

export interface GroupModelRateValidation {
  /** 与输入行一一对应；空白行与合法行为 null。 */
  errors: Array<GroupModelRateRowError | null>
  /** 有效规则超过上限。 */
  tooMany: boolean
  valid: boolean
  /** 合法时提交给后端的规则（模型名已去空格）。 */
  rules: GroupModelRateRule[]
}

/** 与后端 normalizeChannelPricingModelName 相同的归一化。 */
export function normalizeGroupModelPattern(model: string): string {
  const lower = model.trim().toLowerCase()
  return lower.startsWith('claude-') ? lower.replace(/\./g, '-') : lower
}

function isBlank(row: GroupModelRateDraft): boolean {
  return row.model.trim() === '' && (row.rate == null || String(row.rate).trim() === '')
}

function parseRate(rate: GroupModelRateDraft['rate']): number | null {
  if (rate == null) return null
  const text = String(rate).trim()
  if (text === '') return null
  const n = Number(text)
  return Number.isFinite(n) ? n : Number.NaN
}

function modelError(model: string): GroupModelRateRowError | null {
  if (model === '') return 'modelRequired'
  if (/\s/.test(model)) return 'modelWhitespace'
  if (model.length > GROUP_MODEL_PATTERN_MAX_LENGTH) return 'modelTooLong'
  const star = model.indexOf('*')
  if (star !== -1 && (star !== model.length - 1 || model.length < 2)) return 'wildcardPosition'
  return null
}

function rateError(rate: number | null): GroupModelRateRowError | null {
  if (rate == null) return 'rateRequired'
  if (!Number.isFinite(rate) || rate <= 0) return 'rateInvalid'
  if (rate > GROUP_MODEL_RATE_MAX) return 'rateTooLarge'
  // 后端按 DECIMAL(10,4) 四舍五入落库，舍入为 0 的倍率会被拒绝
  if (Math.round(rate * 10000) / 10000 <= 0) return 'rateTooSmall'
  return null
}

export function validateGroupModelRates(rows: GroupModelRateDraft[]): GroupModelRateValidation {
  const seen = new Set<string>()
  const rules: GroupModelRateRule[] = []
  const errors = rows.map<GroupModelRateRowError | null>((row) => {
    if (isBlank(row)) return null
    const model = row.model.trim()
    const rate = parseRate(row.rate)
    const error = modelError(model) ?? rateError(rate)
    if (error) return error
    const key = normalizeGroupModelPattern(model)
    if (seen.has(key)) return 'duplicate'
    seen.add(key)
    rules.push({ model, rate_multiplier: rate as number })
    return null
  })
  const tooMany = rules.length > GROUP_MODEL_RATE_MAX_RULES
  return { errors, tooMany, valid: !tooMany && errors.every((e) => e === null), rules }
}

/** 规则集合的比较键（与顺序、大小写无关），用于判断是否有改动。 */
export function groupModelRatesSignature(rules: GroupModelRateRule[]): string {
  return rules
    .map((r) => `${normalizeGroupModelPattern(r.model)}=${Number(r.rate_multiplier)}`)
    .sort()
    .join('\n')
}
