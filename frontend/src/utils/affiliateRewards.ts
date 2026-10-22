/**
 * Helpers shared by the affiliate views, the recharge page and the register page
 * to describe and quote the invitee first-recharge bonus. Amounts are in the
 * site balance unit (shown as ¥, see utils/balanceCurrency.ts).
 */

import { BALANCE_SYMBOL } from './balanceCurrency'

export interface InviteeBonusLike {
  mode: string
  /** percent mode: 0-100; fixed mode: USD amount. */
  value: number
  min_recharge: number
  /** percent mode cap; 0 = no cap. */
  max: number
}

type Translate = (key: string, params?: Record<string, unknown>) => string

function finite(value: unknown): number {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

/** Formats a rate without trailing zeros: 20 → "20", 12.5 → "12.5". */
export function formatRatePercent(value: number | null | undefined): string {
  const rounded = Math.round(finite(value) * 100) / 100
  return String(rounded)
}

/**
 * CUSTOM: formats a balance amount compactly with the balance symbol:
 * 10 → "¥10", 2.5 → "¥2.50", 0.125 → "¥0.13". (Name kept for existing callers.)
 */
export function formatUsdShort(value: number | null | undefined): string {
  const n = Math.round(finite(value) * 100) / 100
  const sign = n < 0 ? '-' : ''
  const abs = Math.abs(n)
  return `${sign}${BALANCE_SYMBOL}${Number.isInteger(abs) ? abs.toString() : abs.toFixed(2)}`
}

export function isInviteeBonusActive(bonus: InviteeBonusLike | null | undefined): bonus is InviteeBonusLike {
  return !!bonus && (bonus.mode === 'percent' || bonus.mode === 'fixed') && finite(bonus.value) > 0
}

/**
 * Bonus granted for a recharge of `creditedUsd` (real balance credited, without
 * any recharge-tier bonus). Returns 0 below the threshold.
 */
export function quoteInviteeBonus(bonus: InviteeBonusLike | null | undefined, creditedUsd: number): number {
  if (!isInviteeBonusActive(bonus)) return 0
  const base = finite(creditedUsd)
  if (base <= 0 || base < finite(bonus.min_recharge)) return 0
  if (bonus.mode === 'fixed') return Math.round(finite(bonus.value) * 100) / 100
  let amount = (base * finite(bonus.value)) / 100
  const max = finite(bonus.max)
  if (max > 0) amount = Math.min(amount, max)
  return Math.round(amount * 100) / 100
}

/** "10%", "10% (up to ¥20)" or "¥2" — the bonus part of a sentence. */
export function describeInviteeBonusValue(bonus: InviteeBonusLike, t: Translate): string {
  if (bonus.mode === 'fixed') return formatUsdShort(bonus.value)
  const value = formatRatePercent(bonus.value)
  if (finite(bonus.max) > 0) {
    return t('affiliate.bonusValuePercentMax', { value, max: formatUsdShort(bonus.max) })
  }
  return `${value}%`
}
