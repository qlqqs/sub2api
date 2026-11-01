/**
 * CUSTOM: 合伙人页面的展示与操作规则（契约 `.trellis/tasks/10-31-affiliate-partner/contract.md`）。
 * 金额、比例都是字符串，只做格式化，不做运算。
 */

import type { RouteLocationRaw } from 'vue-router'
import type { PartnerLine, PartnerMonthStatus, PartnerMonthTotals, PartnerUserRef } from '@/api/affiliatePartner'
import { decimalRateToPercent, percentToDecimalRate, type PercentRateError } from '@/utils/financeFormat'

/** 比例「0.4000」→「40%」，「0.3550」→「35.5%」。 */
export function formatPartnerRate(rate: string | null | undefined): string {
  return `${decimalRateToPercent(rate)}%`
}

/** 比例输入框（百分比，最多 2 位小数，0–100）→ 提交的小数字符串。空输入视为无效。 */
export function partnerRateFromPercent(input: string | number | null | undefined): { value: string } | { error: PercentRateError } {
  if (input == null || String(input).trim() === '') return { error: 'invalid' }
  return percentToDecimalRate(input, 100)
}

/** 名称：昵称，其次邮箱；已删除用 deletedLabel。 */
export function partnerUserName(user: PartnerUserRef | null | undefined, deletedLabel: string): string {
  if (!user) return '-'
  if (user.deleted) return deletedLabel
  return user.username || '-'
}

/** 结算单状态的徽标样式（style.css 的 badge-*）。 */
export const PARTNER_STATUS_BADGE: Record<PartnerMonthStatus, string> = {
  current: 'badge-gray',
  draft: 'badge-warning',
  confirmed: 'badge-primary',
  paid: 'badge-success'
}

export interface PartnerStatementActions {
  /** 草稿且之前的月份都已确认（设立当月没有之前的月份）。 */
  confirm: boolean
  /** 已确认、未付。 */
  pay: boolean
  /** 未确认（草稿或当月）才能加、删调整项。 */
  adjust: boolean
}

/** 结算单按钮的可用性：按月份先后确认，当月不能确认。statements 为整张列表（任意顺序）。 */
export function partnerStatementActions(statements: PartnerMonthTotals[], month: string): PartnerStatementActions {
  const target = statements.find((item) => item.month === month)
  if (!target) return { confirm: false, pay: false, adjust: false }
  const earlierUnconfirmed = statements.some(
    (item) => item.month < month && (item.status === 'draft' || item.status === 'current')
  )
  return {
    confirm: target.status === 'draft' && !earlierUnconfirmed,
    pay: target.status === 'confirmed',
    adjust: target.status === 'draft' || target.status === 'current'
  }
}

/** 调整项金额：可正可负、不为 0、最多 2 位小数；返回规范字符串或 null。 */
export function normalizeAdjustmentAmount(input: string): string | null {
  const text = input.trim()
  if (!/^[+-]?\d+(?:\.\d{1,2})?$/.test(text) || !/[1-9]/.test(text)) return null
  return text.replace(/^\+/, '')
}

/** 用户端结算单的线点进团队消费：直属团队 → 自己的团队；渠道经理 → 区域视角里他的页面。 */
export function partnerLineRoute(line: PartnerLine): RouteLocationRaw | undefined {
  if (!line.line_agent) return { name: 'AffiliateRegionDirect' }
  return line.line_agent.deleted ? undefined : { name: 'AffiliateRegionAgent', params: { agentId: String(line.line_agent.id) } }
}
