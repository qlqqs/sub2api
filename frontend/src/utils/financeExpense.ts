/**
 * CUSTOM: 财务「其他支出」表单：摊销方式、默认摊销天数与提交前校验（契约 §8）。
 * 金额保持字符串提交（> 0，最多 2 位小数），不经浮点。
 * 摊销方式只存在于表单：一次性扣除提交 amortize_days = 1，按天摊销提交填写的天数（2–3660）。
 */

import {
  defaultAmortizeDays,
  type FinanceExpenseCategory,
  type FinanceExpenseInput
} from '@/api/admin/finance'

/** 按天摊销的天数下限：1 天等同于一次性扣除。 */
export const EXPENSE_AMORTIZE_MIN = 2
export const EXPENSE_AMORTIZE_MAX = 3660
/** 切到「按天摊销」时预填的天数。 */
export const EXPENSE_AMORTIZE_DAILY_DEFAULT = 30
export const EXPENSE_NOTE_MAX = 500

export type ExpenseAmortizeMode = 'once' | 'daily'

/** 订阅类别默认按天摊销，其他默认一次性扣除。 */
export function defaultAmortizeMode(category: FinanceExpenseCategory): ExpenseAmortizeMode {
  return defaultAmortizeDays(category) > 1 ? 'daily' : 'once'
}

export interface ExpenseDraft {
  occurred_on: string
  category: FinanceExpenseCategory
  amount: string
  amortize_mode: ExpenseAmortizeMode
  /** 仅「按天摊销」时使用。 */
  amortize_days: string | number
  account_id: number | null
  note: string
}

export type ExpenseFieldError = 'occurredOnRequired' | 'amountInvalid' | 'amortizeDaysInvalid' | 'noteTooLong'

export function newExpenseDraft(today: string, category: FinanceExpenseCategory = 'upstream_subscription'): ExpenseDraft {
  return {
    occurred_on: today,
    category,
    amount: '',
    amortize_mode: defaultAmortizeMode(category),
    amortize_days: EXPENSE_AMORTIZE_DAILY_DEFAULT,
    account_id: null,
    note: ''
  }
}

/** 已保存的条目回填表单：1 天即一次性扣除，天数框预填默认值备用。 */
export function amortizeFieldsFromDays(days: number): Pick<ExpenseDraft, 'amortize_mode' | 'amortize_days'> {
  return days > 1
    ? { amortize_mode: 'daily', amortize_days: days }
    : { amortize_mode: 'once', amortize_days: EXPENSE_AMORTIZE_DAILY_DEFAULT }
}

/**
 * 切换类别时的摊销方式：仍是旧类别的默认方式（没手动改过）就换成新类别的默认方式，否则保留手选的方式。
 */
export function amortizeModeAfterCategoryChange(
  current: ExpenseAmortizeMode,
  from: FinanceExpenseCategory,
  to: FinanceExpenseCategory
): ExpenseAmortizeMode {
  return current === defaultAmortizeMode(from) ? defaultAmortizeMode(to) : current
}

export function validateExpenseDraft(
  draft: ExpenseDraft
): { ok: true; payload: FinanceExpenseInput } | { ok: false; errors: Partial<Record<keyof ExpenseDraft, ExpenseFieldError>> } {
  const errors: Partial<Record<keyof ExpenseDraft, ExpenseFieldError>> = {}
  const occurredOn = draft.occurred_on.trim()
  if (!/^\d{4}-\d{2}-\d{2}$/.test(occurredOn)) errors.occurred_on = 'occurredOnRequired'

  const amount = draft.amount.trim()
  if (!/^\d+(?:\.\d{1,2})?$/.test(amount) || !/[1-9]/.test(amount)) errors.amount = 'amountInvalid'

  let days = 1
  if (draft.amortize_mode === 'daily') {
    const daysText = String(draft.amortize_days).trim()
    days = Number(daysText)
    if (!/^\d+$/.test(daysText) || days < EXPENSE_AMORTIZE_MIN || days > EXPENSE_AMORTIZE_MAX) {
      errors.amortize_days = 'amortizeDaysInvalid'
    }
  }

  const note = draft.note.trim()
  if (note.length > EXPENSE_NOTE_MAX) errors.note = 'noteTooLong'

  if (Object.keys(errors).length > 0) return { ok: false, errors }
  return {
    ok: true,
    payload: {
      occurred_on: occurredOn,
      category: draft.category,
      amount,
      amortize_days: days,
      account_id: draft.account_id,
      note
    }
  }
}
