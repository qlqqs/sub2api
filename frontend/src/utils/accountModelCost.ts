/**
 * CUSTOM: 账号模型成本倍率的客户端校验（契约 §9），与分组模型倍率的区别：
 * - 倍率允许 0（该模型不计成本），范围 0–10000，最多 4 位小数（不做舍入，超出直接提示）；
 * - 去重只按小写比较。
 * 模型名规则相同：去空格后非空、不含空白、≤ 255 字符，`*` 只能在末尾且前面至少一个字符；最多 200 条。
 * 完全空白的行（模型名与倍率都没填）忽略。
 */

import type { AccountModelCostRule } from '@/api/admin/accountModelCost'

export const ACCOUNT_MODEL_COST_MAX_RULES = 200
export const ACCOUNT_MODEL_COST_MAX = 10000
export const ACCOUNT_MODEL_PATTERN_MAX_LENGTH = 255

export type AccountModelCostRowError =
  | 'modelRequired'
  | 'modelTooLong'
  | 'modelWhitespace'
  | 'wildcardPosition'
  | 'duplicate'
  | 'rateRequired'
  | 'rateInvalid'
  | 'rateTooLarge'
  | 'rateDecimals'

export interface AccountModelCostDraft {
  model: string
  rate: string | number | null | undefined
}

export interface AccountModelCostValidation {
  errors: Array<AccountModelCostRowError | null>
  tooMany: boolean
  valid: boolean
  rules: AccountModelCostRule[]
}

function isBlank(row: AccountModelCostDraft): boolean {
  return row.model.trim() === '' && (row.rate == null || String(row.rate).trim() === '')
}

function modelError(model: string): AccountModelCostRowError | null {
  if (model === '') return 'modelRequired'
  if (/\s/.test(model)) return 'modelWhitespace'
  if (model.length > ACCOUNT_MODEL_PATTERN_MAX_LENGTH) return 'modelTooLong'
  const star = model.indexOf('*')
  if (star !== -1 && (star !== model.length - 1 || model.length < 2)) return 'wildcardPosition'
  return null
}

function rateError(rate: AccountModelCostDraft['rate']): AccountModelCostRowError | null {
  const text = rate == null ? '' : String(rate).trim()
  if (text === '') return 'rateRequired'
  if (!/^\d*(?:\.\d*)?$/.test(text) || !/\d/.test(text)) return 'rateInvalid'
  const n = Number(text)
  if (!Number.isFinite(n) || n < 0) return 'rateInvalid'
  if (n > ACCOUNT_MODEL_COST_MAX) return 'rateTooLarge'
  const frac = text.split('.')[1] ?? ''
  if (frac.replace(/0+$/, '').length > 4) return 'rateDecimals'
  return null
}

export function validateAccountModelCosts(rows: AccountModelCostDraft[]): AccountModelCostValidation {
  const seen = new Set<string>()
  const rules: AccountModelCostRule[] = []
  const errors = rows.map<AccountModelCostRowError | null>((row) => {
    if (isBlank(row)) return null
    const model = row.model.trim()
    const error = modelError(model) ?? rateError(row.rate)
    if (error) return error
    const key = model.toLowerCase()
    if (seen.has(key)) return 'duplicate'
    seen.add(key)
    rules.push({ model, rate_multiplier: Number(String(row.rate).trim()) })
    return null
  })
  const tooMany = rules.length > ACCOUNT_MODEL_COST_MAX_RULES
  return { errors, tooMany, valid: !tooMany && errors.every((e) => e === null), rules }
}

/** 规则集合的比较键（与顺序、大小写无关），用于判断是否有改动。 */
export function accountModelCostSignature(rules: AccountModelCostRule[]): string {
  return rules
    .map((r) => `${r.model.trim().toLowerCase()}=${Number(r.rate_multiplier)}`)
    .sort()
    .join('\n')
}
