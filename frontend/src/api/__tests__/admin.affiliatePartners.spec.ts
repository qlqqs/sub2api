import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post, put, delete: del } }))

import affiliatePartnersAPI, {
  normalizeAdminPartnerDetail,
  normalizePartnerAttachPreview,
  normalizePartnerCreatePreview
} from '@/api/admin/affiliatePartners'

const USER = { id: 75, email: '1185393366@qq.com', username: 'malaixin', deleted: false }

describe('admin affiliate partners api (CUSTOM)', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    del.mockReset()
  })

  it('normalizes the partner list item and detail with agents', () => {
    const detail = normalizeAdminPartnerDetail({
      user: USER,
      region_name: '三科大',
      share_rate: '0.4000',
      next_share_rate: '0.3500',
      next_rate_month: '2026-12',
      since: '2026-11-02T10:00:00+08:00',
      agent_count: 1,
      member_count: '15',
      unconfirmed_months: 1,
      current_month: { month: '2026-11', status: 'current', share_amount: '22.32' },
      agents: [
        {
          user: { id: 59, email: 'a@x.com', username: 'a', deleted: false },
          attached_at: '2026-11-03T00:00:00+08:00',
          source: 'promote',
          attached_by_email: null,
          agent_level: 1,
          team: { owned: 6, via_upline: 3 },
          current_month: { revenue: '9.68' }
        }
      ]
    })
    expect(detail).toMatchObject({
      user: USER,
      region_name: '三科大',
      share_rate: '0.4000',
      next_share_rate: '0.3500',
      member_count: 15,
      loss_carried: '0',
      unconfirmed_months: 1
    })
    expect(detail.current_month).toMatchObject({ status: 'current', share_amount: '22.32', revenue: '0' })
    expect(detail.agents[0]).toMatchObject({
      source: 'promote',
      attached_by_email: null,
      team: { owned: 6, via_upline: 3 },
      current_month: { revenue: '9.68', profit: '0' }
    })
  })

  it('normalizes both previews including the current region hint', () => {
    expect(
      normalizePartnerCreatePreview({
        user: USER,
        agent_level: 3,
        team: { owned: 0, via_upline: 0 },
        current_region: { partner: { id: 62, email: 'p@x.com', username: '', deleted: false }, region_name: '重大' },
        can_create: true,
        reason: null
      })
    ).toMatchObject({ agent_level: 3, is_partner: false, can_create: true, reason: null, current_region: { region_name: '重大', partner: { id: 62 } } })
    expect(normalizePartnerAttachPreview({ agent: USER, can_attach: false, reason: '该用户不是渠道经理' })).toMatchObject({
      can_attach: false,
      reason: '该用户不是渠道经理',
      current_region: null,
      team: { owned: 0, via_upline: 0 }
    })
  })

  it('calls every endpoint with the contract paths and payloads', async () => {
    get.mockResolvedValue({ data: { items: [] } })
    post.mockResolvedValue({ data: {} })
    put.mockResolvedValue({ data: {} })
    del.mockResolvedValue({ data: null })

    await affiliatePartnersAPI.listPartners()
    expect(get).toHaveBeenLastCalledWith('/admin/affiliates/partners', { signal: undefined })
    await affiliatePartnersAPI.previewPartner(62)
    expect(get).toHaveBeenLastCalledWith('/admin/affiliates/partners/preview', { params: { user_id: 62 }, signal: undefined })
    await affiliatePartnersAPI.createPartner({ user_id: 62, region_name: '重大', share_rate: '0.4000' })
    expect(post).toHaveBeenLastCalledWith('/admin/affiliates/partners', { user_id: 62, region_name: '重大', share_rate: '0.4000' })
    await affiliatePartnersAPI.getPartner(62)
    expect(get).toHaveBeenLastCalledWith('/admin/affiliates/partners/62', { signal: undefined })
    await affiliatePartnersAPI.updatePartner(62, { share_rate: '0.3500' })
    expect(put).toHaveBeenLastCalledWith('/admin/affiliates/partners/62', { share_rate: '0.3500' })
    await affiliatePartnersAPI.previewAttach(62, 59)
    expect(get).toHaveBeenLastCalledWith('/admin/affiliates/partners/62/agents/preview', { params: { agent_user_id: 59 }, signal: undefined })
    await affiliatePartnersAPI.attachAgent(62, 59)
    expect(post).toHaveBeenLastCalledWith('/admin/affiliates/partners/62/agents', { agent_user_id: 59 })
    await affiliatePartnersAPI.listStatements(62)
    expect(get).toHaveBeenLastCalledWith('/admin/affiliates/partners/62/statements', { signal: undefined })
    await affiliatePartnersAPI.getStatement(62, '2026-11')
    expect(get).toHaveBeenLastCalledWith('/admin/affiliates/partners/62/statements/2026-11', { signal: undefined })
    await affiliatePartnersAPI.confirmStatement(62, '2026-10')
    expect(post).toHaveBeenLastCalledWith('/admin/affiliates/partners/62/statements/2026-10/confirm')
    await affiliatePartnersAPI.payStatement(62, '2026-10', { paid_on: '2026-11-03', note: '招行转账' })
    expect(post).toHaveBeenLastCalledWith('/admin/affiliates/partners/62/statements/2026-10/pay', { paid_on: '2026-11-03', note: '招行转账' })
    await affiliatePartnersAPI.createAdjustment(62, { month: '2026-11', amount: '-5.00', note: '补差' })
    expect(post).toHaveBeenLastCalledWith('/admin/affiliates/partners/62/adjustments', { month: '2026-11', amount: '-5.00', note: '补差' })
    await affiliatePartnersAPI.deleteAdjustment(62, 3)
    expect(del).toHaveBeenLastCalledWith('/admin/affiliates/partners/62/adjustments/3')

    const blob = new Blob(['x'])
    get.mockResolvedValueOnce({ data: blob })
    expect(await affiliatePartnersAPI.exportStatement(62, '2026-10')).toBe(blob)
    expect(get).toHaveBeenLastCalledWith('/admin/affiliates/partners/62/statements/2026-10/export', { responseType: 'blob' })
  })
})
